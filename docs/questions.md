# FulfillOps Rewards & Compliance Console — Key Questions, Assumptions & Solutions

---

## Q1: How should reward tier inventory be reserved and decremented to prevent overselling when multiple Fulfillment Specialists process orders at the same time?

### Assumption

Inventory must be reserved at the point of fulfillment creation or transition to Ready to Ship, and decremented atomically so that two specialists cannot allocate the same stock. Canceled fulfillments must return reserved stock. The system does not support backorders — if inventory reaches zero, new fulfillments for that tier are blocked until stock is replenished by an Administrator.

### Solution

Implement a transactional reservation flow within the atomic transaction boundary already required by the prompt. When creating a fulfillment or transitioning to Ready to Ship, acquire a row-level lock on the reward tier (`SELECT ... FOR UPDATE`), validate that available stock is greater than zero, decrement the inventory count, and create a reservation record linked to the fulfillment — all within a single database transaction. If the fulfillment is later canceled or placed on hold, the cancellation transaction restocks the inventory by incrementing the tier count and voiding the reservation record. The Templ dashboard displays real-time inventory counts per tier, and threshold alerts fire when stock falls below an Administrator-configured minimum (e.g., 10% of original stock), giving the team time to replenish before sellout.

---

## Q2: What is the exact business rule for enforcing the per-customer purchase limit (default 2 per tier per 30 days)?

### Assumption

The limit applies across all fulfillment records for a given customer and tier within a rolling 30-day window based on the fulfillment creation timestamp. Canceled fulfillments do not count toward the limit. The default limit of 2 is configurable per tier by Administrators. The rolling window means the system looks back 30 days from the current moment, not from a fixed calendar month.

### Solution

During fulfillment creation, the backend queries the count of non-canceled fulfillments for the same customer and tier created within the last 30 days: `SELECT COUNT(*) FROM fulfillments WHERE customer_id = $1 AND tier_id = $2 AND status != 'Canceled' AND created_at >= NOW() - INTERVAL '30 days'`. If the count is greater than or equal to the tier's configured limit, the creation is rejected with a validation error displayed as in-app feedback: "Customer has reached the purchase limit for this tier (2 of 2 in the last 30 days)." The limit value is stored on the `reward_tiers` table as `purchase_limit` with a default of 2, editable by Administrators. This check runs inside the same atomic transaction as inventory reservation to prevent race conditions.

---

## Q3: What is the source of truth for fulfillment status transitions and which transitions are allowed?

### Assumption

Only valid predefined transitions are permitted. The fulfillment lifecycle follows a strict state machine, and all transitions — both successful and rejected — must be auditable and immutable. The allowed transitions are:

- **Draft** → Ready to Ship
- **Ready to Ship** → Shipped | Voucher Issued | On Hold | Canceled
- **Shipped** → Delivered | On Hold | Canceled
- **Voucher Issued** → Completed | On Hold | Canceled
- **Delivered** → Completed
- **On Hold** → Ready to Ship (resume)
- **Canceled** → (terminal, no further transitions)
- **Completed** → (terminal, no further transitions)

On Hold and Canceled require a mandatory reason field.

### Solution

Define the state machine as a map of allowed transitions in the backend Go code. Store the full status history in a separate append-only `fulfillment_timeline` table with columns for `fulfillment_id`, `from_status`, `to_status`, `changed_by`, `changed_at`, `reason`, and `metadata`. Each transition endpoint validates the current status against the allowed-next-status map before proceeding. If the transition is invalid, the handler returns a validation error and the attempt is still logged in the audit trail. The Templ UI renders the timeline as a visible status stepper on the fulfillment detail page, showing every state change with timestamps and the responsible user. The reason field is required and enforced at both the UI (form validation) and API (backend validation) layers for On Hold and Canceled transitions.

---

## Q4: How should overdue alert timing be calculated, especially for the 4 business hour voucher SLA?

### Assumption

Business hours follow a configurable schedule defaulting to Monday through Friday, 8:00 AM to 6:00 PM in the server's local time zone. Weekends are excluded by default. Administrators can configure the business-hours window, active weekdays, time zone, and a holiday/blackout calendar that pauses SLA clocks. The 48-hour physical shipment SLA counts calendar hours (not business hours), while the 4-hour voucher SLA counts only business hours.

### Solution

Create a `system_settings` table with entries for `business_hours_start`, `business_hours_end`, `business_days` (e.g., `[1,2,3,4,5]` for Mon–Fri), and `timezone`. Add a `blackout_dates` table for Administrator-managed holidays and non-business dates. Implement a business-time calculator service in Go that, given a start timestamp, computes elapsed business hours by iterating over the time range, subtracting non-business windows, weekends, and blackout dates. The overdue-check scheduled job (running every 15 minutes) uses this calculator to evaluate each open fulfillment: for physical shipments, it compares calendar hours since `ready_at` against 48 hours; for vouchers, it compares business hours since `ready_at` against 4 hours. When a threshold is exceeded, the job creates an overdue exception record and opens a work-order thread. The Templ dashboard displays overdue items with a color-coded urgency indicator (e.g., amber at 75% of SLA, red at 100%).

---

## Q5: How should the system handle offline message delivery outputs (SMS/email as queued/printed) and retries?

### Assumption

There is no live SMS or email gateway integration. The system is fully offline, so messages targeting SMS or email channels are rendered as printable output documents that staff manually process through separate systems or physical handoff. The 3-retry logic over 30 minutes applies only to the in-app notification channel (re-queuing if the write fails). SMS/email entries are marked "queued" on creation and transition to "printed" when a staff member acknowledges the handoff.

### Solution

When a notification is dispatched, the system creates a `send_logs` entry with `channel` (in-app, sms, email), `status` (queued, sent, printed, failed), `attempt_count`, and `next_retry_at`. For the in-app channel, a background job processes queued entries, writes the notification to the user's in-app inbox, and on failure retries up to 3 times at 10-minute intervals. For SMS and email channels, the entry is immediately set to `queued` and appears in a dedicated "Pending Handoff" queue in the message center UI. Staff can click a print button that renders a formatted page (recipient details, message body, timestamp) and marks the entry as `printed` with the printing user and timestamp. The message center supports filtering send logs by date range, recipient, channel, and status. Administrators configure which delivery channels are active per message template, with in-app always enabled.

---

## Q6: What qualifies as an exception and how should exception threads/work-orders be structured?

### Assumption

Exceptions are primarily created automatically by the overdue detection job but can also be opened manually by staff for any fulfillment issue (e.g., damaged goods, incorrect address, customer dispute). Each exception acts as a work-order-style entity with a threaded conversation where staff document resolution steps. Exceptions have their own lifecycle and must be resolved with a documented outcome before closing.

### Solution

Model exceptions as a `fulfillment_exceptions` table linked to a fulfillment record, with columns for `exception_type` (overdue_shipment, overdue_voucher, manual), `status` (Open, Investigating, Resolved, Escalated), `opened_by`, `opened_at`, `resolved_at`, and `resolution_note`. Store threaded resolution steps in an `exception_events` table with `exception_id`, `author_id`, `event_type` (comment, status_change, escalation), `content`, and `created_at`. The overdue job creates exceptions automatically when SLA thresholds are breached. Staff can also create manual exceptions from the fulfillment detail page. The Templ UI renders the exception as a work-order thread with a chronological event feed. Closing an exception requires a resolution note and triggers a status update on the linked fulfillment if applicable. All exception activity flows into the audit trail. The dashboard surfaces open exception counts and highlights escalated items.

---

## Q7: How should audit trails and immutable history be implemented while still supporting soft delete and recovery?

### Assumption

Audit logs are strictly append-only and can never be edited or deleted by any role, including Administrators. Operational tables (reward tiers, fulfillments, message templates) support soft delete with a 30-day recovery window, after which a cleanup job permanently removes the records. Auditors have read-only access to the full audit history and export capabilities. The audit log captures before and after snapshots for data changes.

### Solution

Create an append-only `audit_logs` table with columns for `table_name`, `record_id`, `operation` (insert, update, delete), `old_values` (JSONB), `new_values` (JSONB), `performed_by`, `performed_at`, and `ip_address`. Use PostgreSQL triggers or application-layer interceptors on key tables to populate this automatically on every insert, update, or soft delete. Soft delete is implemented via `deleted_at` and `deleted_by` columns on operational tables; all application queries filter on `deleted_at IS NULL` by default. Administrators can restore soft-deleted records within the 30-day window via a recovery UI, which itself creates an audit entry. A nightly cleanup job permanently deletes records where `deleted_at` is older than 30 days, logging the purge in the audit trail. The audit_logs table has no `DELETE` or `UPDATE` grants for any application database role, enforcing immutability at the database level. Auditors access a dedicated audit log viewer with search, filtering, and export capabilities.

---

## Q8: How should encryption at rest (AES-256) and masking of sensitive fields be managed with a locally stored key?

### Assumption

The encryption key is stored locally on the server filesystem, outside the database and application directory, and is never exposed through the UI, API responses, or exports. Encryption is handled at the application layer before writing to PostgreSQL. The UI always displays masked values (e.g., last 4 digits of phone, partial address), and exports default to masked values unless an Administrator explicitly requests full decryption with an audited permission grant. Key rotation is a manual administrative operation.

### Solution

On first setup, the application generates a 256-bit random key and writes it to a protected file (e.g., `/etc/fulfillops/encryption.key`) with `0600` filesystem permissions. The key path is referenced via an environment variable (`FULFILLOPS_ENCRYPTION_KEY_PATH`). Sensitive columns (phone numbers, full addresses, voucher codes) are encrypted using AES-256-GCM with a unique IV per value; the IV and ciphertext are stored together in the database column. A Go encryption service handles encrypt/decrypt operations and is the only component that reads the key file. The Templ templates receive pre-masked values from the handler layer (e.g., `***-***-1234`, `*** Oak St, ***, CA 9***2`) — full values are never sent to the browser unless the user has explicit decrypt permission and the request is audit-logged. For exports, the default output contains masked values; an Administrator can toggle "Include Sensitive Data" which triggers an additional permission check and writes an enhanced audit record (who, when, filters, sensitivity level). A CLI admin command (`fulfillops rotate-key`) re-encrypts all sensitive columns in batches within transactions and records the rotation event in the audit log. If the key file is missing at startup, the application refuses to start and logs a critical error.

---

## Q9: What is the process for report exports, file generation, and integrity verification (checksums)?

### Assumption

Reports are generated as CSV files for data portability and compatibility with spreadsheet tools used by offline operations teams. All exports are written to a configurable local directory on the server, not served as direct downloads through the browser. Every export produces a SHA-256 checksum fingerprint for integrity verification. Exports require explicit permission, and every export event is audited with full context (who, when, filters used, file location, checksum). Old exports are retained for 90 days before automatic cleanup.

### Solution

Create a `report_exports` table with columns for `report_type`, `file_name`, `file_path`, `sha256_checksum`, `filters_used` (JSONB), `exported_by`, `exported_at`, `file_size_bytes`, and `expires_at`. The Gin export endpoint validates that the requesting user holds the `export` permission, generates the CSV file into the configured directory (e.g., `/var/fulfillops/exports/`) with `0640` filesystem permissions, computes the SHA-256 checksum over the written file, and writes the audit record atomically. File names include the report type and timestamp to prevent collisions (e.g., `fulfillment_summary_20260416_143022.csv`). The Templ UI provides a report workspace where authorized users configure filters (date range, status, tier), trigger generation with a "Generate Report" button that returns immediate "queued" feedback, and view export history with checksum values. Auditors access a full export history table showing all past exports with verification status. A nightly cleanup job deletes files and database records older than the configured retention period (default 90 days), logging each purge. Checksum verification is available on-demand — the system re-hashes the file on disk and compares against the stored value to detect tampering.
