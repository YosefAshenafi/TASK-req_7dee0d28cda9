# FulfillOps Rewards and Compliance Console - API Specification

> This document describes the HTTP APIs exposed by FulfillOps under the `/api/v1` prefix.
> It reflects the routes registered in `repo/internal/handler/router.go` and the request
> structs declared in `repo/internal/handler/*.go`.
>
> Server-rendered Templ pages (mounted at `/`, `/tiers`, `/customers`, `/admin/...`, etc.)
> are **not** covered in this document. See `repo/README.md` for UI routes.

## 1. Conventions

Base path:

- `/api/v1`

Content type:

- `application/json` for request and response bodies

Authentication:

- Session cookie (server-managed). The `POST /auth/login` endpoint sets
  `fulfillops_session` (API) and `fulfillops` (page) cookies. Subsequent
  requests must send the session cookie — the session middleware revalidates
  the user against the `users` table on every request.
- Unauthenticated endpoints: `POST /auth/login` and `POST /auth/logout`.

Authorization:

- Role-based via `middleware.RequireRole`. The three roles are
  `ADMINISTRATOR`, `FULFILLMENT_SPECIALIST`, `AUDITOR`. Each endpoint section
  below declares the allowed roles.

Optimistic locking:

- Mutable resources (tiers, customers, fulfillments, message templates, job
  schedules) return a `version` integer.
- Update requests must submit the current `version`.
- A mismatched `version` returns `409 Conflict`.

Standard error shape:

```json
{
  "code": "VALIDATION_ERROR",
  "message": "Human-readable message",
  "details": {
    "field": "tracking_number"
  }
}
```

Status codes in use:

- `200 OK`, `201 Created`, `202 Accepted`, `204 No Content`
- `400 Bad Request`, `401 Unauthorized`, `403 Forbidden`, `404 Not Found`,
  `409 Conflict`, `422 Unprocessable Entity`
- `500 Internal Server Error`, `503 Service Unavailable`

## 2. Enums

Fulfillment status:

- `DRAFT`
- `READY_TO_SHIP`
- `SHIPPED`
- `DELIVERED`
- `VOUCHER_ISSUED`
- `COMPLETED`
- `ON_HOLD`
- `CANCELED`

Fulfillment type:

- `PHYSICAL`
- `VOUCHER`

Send log channel:

- `IN_APP`
- `SMS`
- `EMAIL`

Send log status:

- `QUEUED`
- `SENT`
- `PRINTED`
- `FAILED`

Exception type:

- `OVERDUE_SHIPMENT`
- `OVERDUE_VOUCHER`
- `MANUAL`

Exception status:

- `OPEN`
- `INVESTIGATING`
- `ESCALATED`
- `RESOLVED`

Template category:

- `BOOKING_RESULT`
- `BOOKING_CHANGE`
- `EXPIRATION`
- `FULFILLMENT_PROGRESS`

Report type (supported values for `POST /reports/exports`):

- `fulfillments`
- `customers`
- `audit`

User role:

- `ADMINISTRATOR`
- `FULFILLMENT_SPECIALIST`
- `AUDITOR`

Job keys (used by `POST /admin/jobs/{jobName}/run` and `GET/PUT /admin/job-schedules`):

- `overdue-check`
- `notify-retry`
- `backup`
- `stats`
- `scheduled-reports`
- `cleanup`
- `export-cleanup`

## 3. Core Schemas

### 3.1 Reward Tier

```json
{
  "id": "uuid",
  "name": "Gold Mug",
  "description": "Limited campaign item",
  "inventory_count": 120,
  "purchase_limit": 2,
  "alert_threshold": 12,
  "version": 4,
  "created_at": "2026-04-16T09:00:00Z",
  "updated_at": "2026-04-16T10:00:00Z",
  "deleted_at": null
}
```

### 3.2 Customer (response shape)

Phone, email, and address are encrypted at rest. Responses always return
masked values on read; the plain strings are never returned.

```json
{
  "id": "uuid",
  "name": "Jane Doe",
  "phone_masked": "***-***-4567",
  "email_masked": "j***@example.com",
  "address_masked": "*** Oak St, ***, CA 9***0",
  "version": 1,
  "created_at": "2026-04-16T08:00:00Z",
  "updated_at": "2026-04-16T08:00:00Z"
}
```

### 3.3 Fulfillment

```json
{
  "id": "uuid",
  "tier_id": "uuid",
  "customer_id": "uuid",
  "type": "PHYSICAL",
  "status": "READY_TO_SHIP",
  "carrier_name": null,
  "tracking_number": null,
  "voucher_code_masked": null,
  "voucher_expiration": null,
  "hold_reason": null,
  "cancel_reason": null,
  "ready_at": "2026-04-16T09:10:00Z",
  "shipped_at": null,
  "delivered_at": null,
  "completed_at": null,
  "version": 2,
  "created_at": "2026-04-16T09:10:00Z",
  "updated_at": "2026-04-16T09:10:00Z"
}
```

### 3.4 Transition Event (timeline entry)

```json
{
  "id": "uuid",
  "fulfillment_id": "uuid",
  "from_status": "DRAFT",
  "to_status": "READY_TO_SHIP",
  "reason": null,
  "changed_by": "uuid",
  "changed_at": "2026-04-16T09:12:00Z",
  "metadata": {}
}
```

### 3.5 Paginated list response

All list endpoints that paginate return:

```json
{
  "items": [ /* ... */ ],
  "total": 123,
  "page": 1,
  "page_size": 20
}
```

## 4. Auth and Session

### POST `/auth/login`

Authenticates user and creates a session cookie. Unauthenticated.

Request:

```json
{
  "username": "string",
  "password": "string"
}
```

Response `200 OK`:

```json
{
  "id": "uuid",
  "username": "admin",
  "role": "ADMINISTRATOR"
}
```

### POST `/auth/logout`

Clears both API and page session cookies. Unauthenticated (idempotent).
Response `200 OK` with empty body.

### GET `/auth/me`

Returns the current authenticated user. All authenticated roles.

Response `200 OK`:

```json
{
  "id": "uuid",
  "username": "admin",
  "role": "ADMINISTRATOR"
}
```

## 5. Reward Tier Endpoints

### GET `/tiers`

List tiers. All authenticated roles.

Query params:

- `q` (optional, substring match on name)
- `include_deleted` (admin/auditor only)
- `page`, `page_size`

### POST `/tiers`

Create tier. `ADMINISTRATOR` only.

Request:

```json
{
  "name": "Gold Mug",
  "description": "Limited campaign item",
  "inventory_count": 200,
  "purchase_limit": 2,
  "alert_threshold": 20
}
```

- When `purchase_limit` is omitted or `<= 0`, the default of `2 per 30 days`
  is applied.

Response `201 Created` with the tier schema (Section 3.1).

### GET `/tiers/{tierId}`

Get a tier by id. All authenticated roles. `404` if not found.

### PUT `/tiers/{tierId}`

Replace editable fields. `ADMINISTRATOR` only. Requires `version`.

Request:

```json
{
  "name": "Gold Mug",
  "description": "Limited campaign item",
  "inventory_count": 180,
  "purchase_limit": 2,
  "alert_threshold": 20,
  "version": 4
}
```

Response `200 OK` with the updated tier. `409 Conflict` on version mismatch.

### DELETE `/tiers/{tierId}`

Soft-delete a tier. `ADMINISTRATOR` only. 30-day recovery window enforced by
the nightly `cleanup` job.

### POST `/tiers/{tierId}/restore`

Restore a soft-deleted tier within its 30-day window. `ADMINISTRATOR` only.

## 6. Customer Endpoints

### GET `/customers`

List customers. All authenticated roles.

Query params:

- `name` (optional, substring match on name)
- `page`, `page_size`
- `include_deleted` (admin/auditor only)

### POST `/customers`

Create customer. `ADMINISTRATOR` and `FULFILLMENT_SPECIALIST`. Phone, email,
and address are encrypted at rest. Responses return masked PII (Section 3.2).

Request:

```json
{
  "name": "Jane Doe",
  "phone": "5551234567",
  "email": "jane@example.com",
  "address": "123 Oak St, Apt 4, Springfield, CA 90210"
}
```

Response `201 Created` with the masked customer schema (Section 3.2).

### GET `/customers/{customerId}`

Get a customer. All authenticated roles. PII is masked.

### PUT `/customers/{customerId}`

Update customer. `ADMINISTRATOR` and `FULFILLMENT_SPECIALIST`. Requires `version`.

Request:

```json
{
  "name": "Jane Doe",
  "phone": "5551234567",
  "email": "jane@example.com",
  "address": "123 Oak St, Apt 4, Springfield, CA 90210",
  "version": 1
}
```

Notes:

- `phone`, `email`, `address` are optional pointers. Omitted fields preserve
  the encrypted value already on the row; a field set to `""` clears the
  stored ciphertext.

### DELETE `/customers/{customerId}`

Soft-delete a customer. `ADMINISTRATOR` only. 30-day recovery window.

### POST `/customers/{customerId}/restore`

Restore a soft-deleted customer within its 30-day window. `ADMINISTRATOR` only.

## 7. Fulfillment Endpoints

### GET `/fulfillments`

List fulfillments. All authenticated roles.

Filters:

- `status`
- `tier_id`
- `customer_id`
- `type`
- `date_from`, `date_to`
- `page`, `page_size`

Response is paginated (Section 3.5) with fulfillment schema items (Section 3.3).

### POST `/fulfillments`

Create fulfillment. `ADMINISTRATOR` and `FULFILLMENT_SPECIALIST`. Atomic
inventory check and customer purchase-limit check.

Request:

```json
{
  "tier_id": "uuid",
  "customer_id": "uuid",
  "type": "PHYSICAL"
}
```

Validation:

- Tier must have `inventory_count > 0` (no backorders).
- Customer must not have exceeded the tier's rolling-30-day `purchase_limit`
  (default 2). `CANCELED` records are excluded.

Error bodies (examples):

- `422 Unprocessable Entity` — `code: "INVENTORY_UNAVAILABLE"`
- `422 Unprocessable Entity` — `code: "PURCHASE_LIMIT_REACHED"`
- `400 Bad Request` — `code: "VALIDATION_ERROR"` for missing fields

### GET `/fulfillments/{fulfillmentId}`

Get a fulfillment. All authenticated roles.

### POST `/fulfillments/{fulfillmentId}/transition`

Perform a validated status transition. `ADMINISTRATOR` and
`FULFILLMENT_SPECIALIST`. The transition, timeline insert, inventory delta,
and notification enqueue all commit in one DB transaction or roll back
together.

Request:

```json
{
  "to_status": "SHIPPED",
  "reason": null,
  "version": 2,
  "carrier_name": "UPS",
  "tracking_number": "1ZABC12345",
  "voucher_code": null,
  "voucher_expiration": null,
  "shipping_address": {
    "line_1": "123 Oak St",
    "line_2": "Apt 4",
    "city": "Springfield",
    "state": "CA",
    "zip_code": "90210"
  }
}
```

Rules:

- `to_status` must be reachable from current status per
  `domain.AllowedTransitions` (see `repo/internal/domain/enums.go`).
- `ON_HOLD` and `CANCELED` require `reason`.
- `SHIPPED` requires a `tracking_number` matching `[A-Za-z0-9]{8,30}` and a
  `carrier_name`.
- `VOUCHER_ISSUED` requires `voucher_code`; `voucher_expiration` is optional.
- PHYSICAL fulfillments transitioning to `READY_TO_SHIP` require a
  `shipping_address` if none is stored. Address lines are encrypted at rest.
- `409 Conflict` on `version` mismatch.
- `422 Unprocessable Entity` with `code: "INVALID_TRANSITION"` when the
  transition is not in the allowed graph.

### PUT `/fulfillments/{fulfillmentId}/shipping-address`

Update or set the shipping address on an existing fulfillment (does not
change status). `ADMINISTRATOR` and `FULFILLMENT_SPECIALIST`. Requires
`version`.

Request:

```json
{
  "line_1": "123 Oak St",
  "line_2": "Apt 4",
  "city": "Springfield",
  "state": "CA",
  "zip_code": "90210",
  "version": 2
}
```

### GET `/fulfillments/{fulfillmentId}/timeline`

Returns the append-only transition timeline. All authenticated roles.

Response `200 OK`:

```json
{
  "items": [ /* Transition Event schema (Section 3.4) */ ]
}
```

### DELETE `/fulfillments/{fulfillmentId}`

Soft-delete a fulfillment. `ADMINISTRATOR` only. 30-day recovery window.

### POST `/fulfillments/{fulfillmentId}/restore`

Restore a soft-deleted fulfillment within its 30-day window.
`ADMINISTRATOR` only.

## 8. Exception Endpoints

All exception endpoints require `ADMINISTRATOR` or `FULFILLMENT_SPECIALIST`.
Auditors are excluded.

### GET `/exceptions`

Filters:

- `status`
- `type`
- `fulfillment_id`

### POST `/exceptions`

Manual exception creation.

Request:

```json
{
  "fulfillment_id": "uuid",
  "type": "MANUAL",
  "note": "Customer reported wrong address"
}
```

Response `201 Created`.

### GET `/exceptions/{exceptionId}`

### PUT `/exceptions/{exceptionId}/status`

Update exception status.

Request:

```json
{
  "status": "RESOLVED",
  "resolution_note": "Replacement shipped"
}
```

Rules:

- `resolution_note` is required when `status` is `RESOLVED`.

### POST `/exceptions/{exceptionId}/events`

Append a thread event.

Request:

```json
{
  "event_type": "COMMENT",
  "content": "Carrier contacted, awaiting response"
}
```

## 9. Messaging Endpoints

### GET `/message-templates`

List templates. `ADMINISTRATOR` and `FULFILLMENT_SPECIALIST`.

Query params:

- `category`
- `channel`
- `include_deleted` (admin only)

### POST `/message-templates`

Create template. `ADMINISTRATOR` only.

Request:

```json
{
  "name": "Shipment delivered",
  "category": "FULFILLMENT_PROGRESS",
  "channel": "EMAIL",
  "body_template": "Hello {{customer}}, your {{tier}} has been delivered."
}
```

### GET `/message-templates/{templateId}`

`ADMINISTRATOR` and `FULFILLMENT_SPECIALIST`.

### PUT `/message-templates/{templateId}`

Update template. `ADMINISTRATOR` only. Requires `version`.

Request:

```json
{
  "name": "Shipment delivered",
  "category": "FULFILLMENT_PROGRESS",
  "channel": "EMAIL",
  "body_template": "Hello {{customer}}, your {{tier}} is on its way.",
  "version": 3
}
```

### DELETE `/message-templates/{templateId}`

Soft-delete a template. `ADMINISTRATOR` only.

> **Note:** Template restore is exposed only through the server-rendered page
> route `POST /messages/templates/{id}/restore`, not on `/api/v1`.

### POST `/dispatch`

Dispatches a template to a recipient. In-app notifications are always
enqueued; SMS/EMAIL are queued to the offline handoff send log.
`ADMINISTRATOR` and `FULFILLMENT_SPECIALIST`.

Request:

```json
{
  "template_id": "uuid",
  "recipient_id": "uuid",
  "extra_channels": ["SMS", "EMAIL"],
  "context": {
    "fulfillment_id": "uuid"
  }
}
```

Behavior:

- `IN_APP` notification is always created for `recipient_id`.
- Each entry in `extra_channels` creates a `QUEUED` row in `send_logs` for
  offline handoff (printed/marked-failed flows below).
- Failed sends are retried up to 3 times over 30 minutes by the
  `notify-retry` scheduled job.

### GET `/send-logs`

List send logs. `ADMINISTRATOR` and `FULFILLMENT_SPECIALIST`.

Filters:

- `recipient_id`
- `channel`
- `status`
- `date_from`, `date_to`
- `page`, `page_size`

### PUT `/send-logs/{sendLogId}/printed`

Mark an offline handoff as printed. `ADMINISTRATOR` and
`FULFILLMENT_SPECIALIST`.

Response `200 OK`:

```json
{
  "id": "uuid",
  "channel": "SMS",
  "status": "PRINTED",
  "printed_by": "uuid",
  "printed_at": "2026-04-16T10:30:00Z"
}
```

### PUT `/send-logs/{sendLogId}/failed`

Mark an offline handoff as failed. Optional body supplies a reason.
`ADMINISTRATOR` and `FULFILLMENT_SPECIALIST`.

Request (optional):

```json
{
  "reason": "Carrier rejected tracking number"
}
```

Response `200 OK` with the updated send log.

### GET `/notifications`

List the calling user's in-app notifications. All authenticated roles.

Query params:

- `is_read` (`true` / `false`)
- `page`, `page_size`

### PUT `/notifications/{notificationId}/read`

Mark the notification as read. All authenticated roles. Users can only mark
their own notifications; cross-user writes return `404`.

Response `200 OK` with the updated notification.

## 10. Reports and Exports

### GET `/reports/exports`

List export history. `ADMINISTRATOR` and `AUDITOR`.

Visibility rule: non-admin callers never see rows marked
`include_sensitive=true`. The filter is applied by the repository so the
`total` count and the page slice agree (no phantom "missing" rows).

Query params:

- `page`, `page_size`

### POST `/reports/exports`

Generate a report export file. `ADMINISTRATOR` and `AUDITOR`. Auditors
cannot create sensitive exports (403).

Request:

```json
{
  "report_type": "fulfillments",
  "filters": {
    "date_from": "2026-04-01",
    "date_to": "2026-04-16",
    "status": ["READY_TO_SHIP", "SHIPPED"]
  },
  "include_sensitive": false
}
```

Unsupported `report_type` values return `422 Unprocessable Entity` with
`details.report_type: "must be one of: fulfillments, customers, audit"`.

Response `202 Accepted`:

```json
{
  "id": "uuid",
  "status": "QUEUED",
  "report_type": "fulfillments",
  "include_sensitive": false,
  "created_at": "2026-04-16T10:30:00Z"
}
```

### GET `/reports/exports/{exportId}`

Fetch a single export record. `ADMINISTRATOR` and `AUDITOR` (auditor cannot
fetch sensitive exports → `403`).

### POST `/reports/exports/{exportId}/verify-checksum`

Re-hashes the export file on disk and compares against the stored SHA-256.
`ADMINISTRATOR` and `AUDITOR`.

Response `200 OK`:

```json
{
  "export_id": "uuid",
  "verified": true,
  "checksum": "hex"
}
```

### DELETE `/reports/exports/{exportId}`

Permanently delete an export record and its on-disk file. `ADMINISTRATOR` only.

## 11. Settings and SLA Configuration

Business hours, timezone, blackout calendar, and other operational policies
are stored as rows in the `system_settings` table and are read/written via
the generic key/value endpoints below.

### GET `/settings`

Returns all known settings. `ADMINISTRATOR` and `FULFILLMENT_SPECIALIST`.

Response `200 OK`:

```json
{
  "items": [
    { "id": "uuid", "key": "business_hours_start", "value": "08:00", "updated_by": "uuid", "updated_at": "..." },
    { "id": "uuid", "key": "business_hours_end",   "value": "18:00", "updated_by": "uuid", "updated_at": "..." },
    { "id": "uuid", "key": "business_days",        "value": "[1,2,3,4,5]", "updated_by": "uuid", "updated_at": "..." },
    { "id": "uuid", "key": "timezone",             "value": "America/New_York", "updated_by": "uuid", "updated_at": "..." }
  ]
}
```

### PUT `/settings/{key}`

Write a single setting. `ADMINISTRATOR` only.

Request:

```json
{ "value": "America/New_York" }
```

Notes:

- `value` is stored as JSON. If the supplied string is already valid JSON,
  it is stored verbatim (for example `"[1,2,3,4,5]"`). Otherwise the server
  JSON-encodes it as a string.
- Well-known keys include `business_hours_start`, `business_hours_end`,
  `business_days`, `timezone`.

### GET `/settings/blackout-dates`

List blackout dates. `ADMINISTRATOR` and `FULFILLMENT_SPECIALIST`.

### POST `/settings/blackout-dates`

Create a blackout date. `ADMINISTRATOR` only.

Request:

```json
{
  "date": "2026-12-25",
  "description": "Christmas Day"
}
```

- `date` accepts either RFC3339 (`2026-12-25T00:00:00Z`) or `YYYY-MM-DD`.

### DELETE `/settings/blackout-dates/{dateId}`

Remove a blackout date. `ADMINISTRATOR` only.

## 12. Admin — Health, Jobs, Schedules, DR Drills

### GET `/admin/health`

System readiness. `ADMINISTRATOR` only.

Response `200 OK`:

```json
{
  "status": "ok",
  "checks": {
    "database":   "ok",
    "encryption": "ok",
    "dirs":       "ok",
    "scheduler":  "ok"
  }
}
```

`status` is `"degraded"` when any check starts with `"error"`.

### GET `/admin/jobs/runs`

Returns job run history. `ADMINISTRATOR` only.

Filters:

- `job_name`
- `status`
- `page`, `page_size`

### POST `/admin/jobs/{jobName}/run`

Manually triggers a scheduled job (one-shot). `ADMINISTRATOR` only.
Returns `202 Accepted` with `{ "message": "job triggered", "job": "<name>" }`.
Unknown job name returns `404`.

Allowed `jobName` values are the job keys listed in Section 2.

### GET `/admin/job-schedules`

List current job cadences. `ADMINISTRATOR` only.

### PUT `/admin/job-schedules/{key}`

Update a job cadence. `ADMINISTRATOR` only. Requires `version`.

Request (exactly one of `interval_seconds` or `daily_hour`+`daily_minute`):

```json
{
  "interval_seconds": 900,
  "daily_hour": null,
  "daily_minute": null,
  "enabled": true,
  "version": 1
}
```

Version mismatch returns `409 Conflict`.

### GET `/admin/dr-drills`

List DR drill records. `ADMINISTRATOR` only. Paginated.

### POST `/admin/dr-drills`

Record a scheduled DR drill. `ADMINISTRATOR` only.

Request:

```json
{
  "scheduled_for": "2026-06-15",
  "notes": "Quarterly restore exercise"
}
```

- `scheduled_for` accepts RFC3339 or `YYYY-MM-DD`.

### PUT `/admin/dr-drills/{drillId}`

Record the outcome of a DR drill. `ADMINISTRATOR` only.

Request:

```json
{
  "outcome": "PASS",
  "notes": "Restore verified referential integrity",
  "artifact_path": "/app/backups/dr-2026-06-15.dump"
}
```

## 13. User Management

All endpoints in this section are `ADMINISTRATOR` only.

### GET `/admin/users`

List users.

Query params:

- `role` (optional)
- `is_active` (optional, boolean)

### POST `/admin/users`

Create user. Passwords are hashed with bcrypt; minimum length 8.

Request:

```json
{
  "username": "specialist",
  "email": "specialist@fulfillops.local",
  "password": "Spec@Demo1!",
  "role": "FULFILLMENT_SPECIALIST"
}
```

### GET `/admin/users/{userId}`

### PUT `/admin/users/{userId}`

Update a user's email and role.

Request:

```json
{
  "email": "specialist.new@fulfillops.local",
  "role": "FULFILLMENT_SPECIALIST"
}
```

### DELETE `/admin/users/{userId}`

Deactivate user (sets `is_active=false`). The row is preserved for audit.

## 14. Audit Log

### GET `/audit`

List audit log entries. `ADMINISTRATOR` and `AUDITOR` only. The `audit_logs`
table is append-only at the DB layer; there are no write endpoints.

Filters:

- `table_name`
- `record_id`
- `operation` (e.g. `CREATE`, `UPDATE`, `DELETE`, `BOOTSTRAP`, `CHANGE_PASSWORD`,
  `EXPORT`, `RESTORE`, `DEACTIVATE`)
- `performed_by`
- `date_from`, `date_to`
- `page`, `page_size`

Response is paginated (Section 3.5) with audit-log rows:

```json
{
  "id": "uuid",
  "table_name": "fulfillments",
  "record_id": "uuid",
  "operation": "UPDATE",
  "performed_by": "uuid",
  "performed_at": "2026-04-16T09:12:00Z",
  "before": { /* previous row or metadata */ },
  "after":  { /* new row or metadata */ }
}
```

## 15. Health (unauthenticated)

### GET `/healthz`

Liveness + basic DB reachability probe. No session required.

Response `200 OK`:

```json
{ "status": "ok", "db": "connected" }
```

`503 Service Unavailable` with `{"status":"error","db":"unreachable"}` when
the DB ping fails.

## 16. Backup and Restore

Backup and restore are exposed **only** via the server-rendered admin pages,
not on `/api/v1`:

- `GET  /admin/backups` — list page.
- `POST /admin/backups/run` — trigger a `pg_dump` backup; writes an audit row.
- `POST /admin/backups/{backupId}/restore` — one-click restore workflow; the
  `BackupService` verifies referential integrity before committing and writes
  an audit row.

All three require `ADMINISTRATOR` and are mounted on the session-cookie page
surface (`PageSessionAuth` + `PageRequireRole`). There are no equivalent
REST endpoints.

## 17. Validation Rules Summary

- Tracking number: alphanumeric, 8–30 characters.
- Purchase limit: per (customer, tier), rolling 30-day window, excludes
  `CANCELED`. Default is 2 if the tier's `purchase_limit` is `<= 0`.
- No backorders: create/transition fails when `inventory_count` is 0.
- `ON_HOLD` and `CANCELED` transitions require `reason`.
- `SHIPPED` transitions require `carrier_name` and an 8–30-char alphanumeric
  `tracking_number`.
- `VOUCHER_ISSUED` transitions require `voucher_code`.
- PHYSICAL fulfillments transitioning to `READY_TO_SHIP` require a
  US-formatted `shipping_address` if none is already stored.
- Soft-deleted rows are restorable only within 30 days.
- Customer PII is masked by default; plain values are never returned.
- Report exports with `include_sensitive=true` are visible to
  `ADMINISTRATOR` only.

## 18. Idempotency and Observability

Recommended request headers:

- `X-Request-Id` — trace correlation. The `middleware.RequestID` middleware
  echoes this header on the response when supplied, or generates one.
- `Idempotency-Key` — recommended on create/transition/export endpoints.

Audit trail (written server-side, not client-supplied):

- Actor user id
- Table + record id
- Operation
- Before / after values
- Timestamp
