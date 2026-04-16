# FulfillOps Rewards and Compliance Console - Design

## 1. Scope and Goals

FulfillOps is an offline-first web application for end-to-end reward fulfillment operations. The stack is:

- Frontend: Go Templ templates, HTML, CSS, JavaScript
- Backend: Go + Gin
- Database: PostgreSQL

Core goals:

- Prevent inventory oversell under concurrent fulfillment operations
- Enforce purchase limits per customer and tier
- Enforce a strict fulfillment state machine
- Support offline messaging workflows for SMS and email handoff
- Provide immutable audit history with soft-delete recovery on operational data
- Protect sensitive data with AES-256 encryption at rest and masked display

## 2. Actors and Permissions

- Administrator
- Manage reward tiers, limits, templates, settings, blackout dates, exports, recovery, and job ops
- Fulfillment Specialist
- Create and process fulfillments, transitions, handoff/print queued messages, manage exceptions
- Auditor
- Read-only access to immutable history, job history, export records, and reports

## 3. Architecture

Logical layers:

1. Templ UI pages and forms
2. Gin REST handlers
3. Services (inventory, fulfillment workflow, SLA calculator, messaging, exports, auditing)
4. Repositories and SQL transactions
5. PostgreSQL as source of truth

Scheduled jobs run in-process and write execution results to `job_run_history`.

## 4. Domain Model

Primary entities:

- `users`
- `customers`
- `reward_tiers` (inventory_count, purchase_limit default 2, alert_threshold, version)
- `fulfillments` (status, type physical|voucher, version, ready_at, shipped/delivered/completed timestamps)
- `reservations` (active|voided, tied to tier + fulfillment)
- `shipping_addresses` (fulfillment-linked, US format: line_1, line_2, city, state, zip_code; address lines encrypted)
- `fulfillment_timeline` (append-only transition events)
- `fulfillment_exceptions`
- `exception_events`
- `message_templates`
- `send_logs`
- `notifications` (in-app inbox)
- `report_exports`
- `system_settings`
- `blackout_dates`
- `job_run_history`
- `audit_logs` (append-only)

Soft-delete columns (`deleted_at`, `deleted_by`) apply to operational tables where recovery is required.

## 5. Transaction and Concurrency Design

Atomic transaction boundary for fulfillment creation and lifecycle updates:

1. Lock target tier row using `SELECT ... FOR UPDATE`
2. Validate inventory > 0
3. Validate rolling purchase limit (30-day, non-canceled fulfillments)
4. Decrement tier inventory
5. Create/maintain reservation row
6. Insert or update fulfillment
7. Append timeline event
8. Enqueue send_logs/notifications as required
9. Commit or rollback as one unit

Optimistic locking:

- Mutable rows include `version` integer
- Updates require matching `version`
- On mismatch return `409 Conflict`

## 6. Fulfillment Lifecycle

States:

- Draft
- Ready to Ship
- Shipped
- Delivered
- Voucher Issued
- Completed
- On Hold
- Canceled

Allowed transitions:

- Draft -> Ready to Ship
- Ready to Ship -> Shipped | Voucher Issued | On Hold | Canceled
- Shipped -> Delivered | On Hold | Canceled
- Voucher Issued -> Completed | On Hold | Canceled
- Delivered -> Completed
- On Hold -> Ready to Ship
- Canceled -> terminal
- Completed -> terminal

Rules:

- `On Hold` and `Canceled` require `reason`
- Physical shipment requires `carrier_name` and tracking number `[A-Za-z0-9]{8,30}`
- Voucher path requires `voucher_code`, optional expiration
- Every attempted transition (success or reject) is audit-visible

## 7. Inventory and Purchase Limit Rules

Inventory:

- Reserve on fulfillment creation or move to Ready to Ship
- Decrement occurs under locked transaction
- Cancel/void returns stock by voiding reservation and incrementing tier inventory
- No backorders

Purchase limit:

- Default `purchase_limit = 2` per tier
- Window is rolling 30 days by fulfillment `created_at`
- Exclude canceled fulfillments from count

## 8. SLA and Exception Model

SLAs:

- Physical: 48 calendar hours from `ready_at`
- Voucher: 4 business hours from `ready_at`

Business time config:

- `business_hours_start`
- `business_hours_end`
- `business_days`
- `timezone`
- `blackout_dates`

Overdue job:

- Runs every 15 minutes
- Creates `fulfillment_exceptions` for overdue items
- Avoid duplicate open exceptions for same breach

Exception workflow:

- Types: `overdue_shipment`, `overdue_voucher`, `manual`
- Status: `Open`, `Investigating`, `Escalated`, `Resolved`
- Threaded events in `exception_events`
- Resolution requires `resolution_note`

## 9. Messaging and Offline Delivery

Template categories:

- booking_result
- booking_change
- expiration
- fulfillment_progress

Channels:

- in_app (always enabled)
- sms (offline queued/printed)
- email (offline queued/printed)

Retry behavior:

- In-app only: up to 3 retries over 30 minutes (10-minute interval)
- SMS/email are queued for manual handoff and become `printed` on staff acknowledgment

`send_logs` stores channel, status, attempt_count, next_retry_at, print metadata.

## 10. Security, Privacy, and Compliance

Encryption at rest:

- AES-256-GCM at application layer
- Key file outside app/db path via `FULFILLOPS_ENCRYPTION_KEY_PATH`
- Strict file permissions (0600)
- Startup fails fast if key is missing

Masking:

- UI/API default returns masked phone/address/voucher fields
- Full sensitive export requires explicit permission and enhanced audit record

Audit:

- `audit_logs` append-only with actor, operation, before/after JSON, timestamp, IP
- No UPDATE/DELETE grant for app role on audit table

Soft-delete recovery:

- 30-day recovery window for operational records
- Nightly purge of expired soft-deleted rows (with purge audit)

## 11. Reports and Export Integrity

- CSV exports written to local directory
- Per-export checksum SHA-256 persisted in `report_exports`
- Export records store who/when/filters/path/size/expires_at
- Retention default: 90 days
- Nightly cleanup removes expired files and rows, then audits purge
- On-demand checksum verification re-hashes file and compares to stored fingerprint

## 12. Scheduled Jobs and Operability

Default jobs:

- Overdue reminder dispatch: every 15 minutes
- Notification retry worker: every 10 minutes
- Nightly stats refresh: 2:00 AM
- Soft-delete cleanup: nightly
- Export cleanup: nightly
- Scheduled reports: configurable cadence

All jobs write:

- `started_at`, `finished_at`, status
- `records_processed`
- `error_stack` on failure

Admin health screen shows current status, last run, next run, and failure details.

## 13. Operations and Disaster Recovery

Health checks:

- On-demand endpoint verifying database connectivity, encryption key file access, export directory writability, and scheduler status

Backups:

- Scheduled PostgreSQL backups (pg_dump) to configurable offline path
- Media asset backups to same offline path
- Schedule configurable by Administrator

Disaster recovery:

- Quarterly DR drills as operational requirement
- One-click restore: rehydrates PostgreSQL from backup, verifies referential integrity, reopens system only after passing verification

Configuration:

- `DATABASE_URL` — PostgreSQL connection string
- `FULFILLOPS_ENCRYPTION_KEY_PATH` — path to AES-256 key file
- `FULFILLOPS_EXPORT_DIR` — report export directory
- `FULFILLOPS_BACKUP_DIR` — backup output directory
- `FULFILLOPS_PORT` — HTTP listen port

## 14. UI Structure

| Page | Route | Description |
|---|---|---|
| Dashboard | `/` | Pending fulfillments, overdue exceptions, threshold alerts, stats |
| Reward Tiers | `/tiers` | List, create, edit, soft-delete tiers with inventory and limits |
| Tier Detail | `/tiers/:id` | Single tier with fulfillment list and inventory history |
| Fulfillments | `/fulfillments` | Searchable, filterable fulfillment list |
| Fulfillment Detail | `/fulfillments/:id` | Status timeline, shipping/voucher info, exception threads |
| Customers | `/customers` | Customer list with masked PII |
| Customer Detail | `/customers/:id` | Customer info, fulfillment history, purchase limit status |
| Message Center | `/messages` | Template management, send logs, pending handoff queue |
| Notifications | `/notifications` | In-app notification inbox for the current user |
| Reports | `/reports` | Report workspace: configure filters, generate, view history |
| Exceptions | `/exceptions` | Open exception list with work-order threads |
| Audit Log | `/audit` | Auditor-facing log viewer with search and export |
| System Settings | `/settings` | Business hours, blackout dates, job configuration |
| Admin Health | `/admin/health` | Job status, health checks, backup status |
| User Management | `/admin/users` | Administrator user/role management |
| Recovery | `/admin/recovery` | Soft-deleted record recovery (30-day window) |

## 15. Non-Functional Requirements

- Offline operation with no required external gateways
- Transactional consistency on critical write paths
- Concurrency safety via row locks + optimistic versioning
- Immutable compliance trails
- Data minimization in UI and exports through masking-by-default
