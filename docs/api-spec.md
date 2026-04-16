# FulfillOps Rewards and Compliance Console - API Specification

## 1. Conventions

Base path:

- `/api/v1`

Content type:

- `application/json`

Authentication:

- Session cookie (server-managed)

Authorization:

- Role/permission based in middleware

Optimistic locking:

- Mutable resources return `version`
- Client must submit `version` on update
- Version mismatch returns `409 Conflict`

Standard error shape:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Human-readable message",
    "details": {
      "field": "tracking_number"
    }
  }
}
```

Status codes:

- `200 OK`, `201 Created`, `202 Accepted`
- `400 Bad Request`, `401 Unauthorized`, `403 Forbidden`, `404 Not Found`, `409 Conflict`, `422 Unprocessable Entity`
- `500 Internal Server Error`

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

User role:

- `ADMINISTRATOR`
- `FULFILLMENT_SPECIALIST`
- `AUDITOR`

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
  "updated_at": "2026-04-16T10:00:00Z"
}
```

### 3.2 Customer

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
  "version": 2,
  "created_at": "2026-04-16T09:10:00Z",
  "updated_at": "2026-04-16T09:10:00Z"
}
```

### 3.4 Transition Event

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

## 4. Auth and Session

### POST `/auth/login`

Authenticates user and creates a session.

Request:

```json
{
  "username": "string",
  "password": "string"
}
```

### GET `/auth/me`

Returns current authenticated user.

### POST `/auth/logout`

Invalidates active session.

## 5. Reward Tier Endpoints

### GET `/tiers`

Query params:

- `q` (optional)
- `include_deleted` (admin/auditor only)

### POST `/tiers`

Creates tier.

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

### GET `/tiers/{tierId}`

### PATCH `/tiers/{tierId}`

Requires `version`.

### DELETE `/tiers/{tierId}`

Soft-delete tier.

### POST `/tiers/{tierId}/restore`

Restore soft-deleted tier if within 30 days.

## 6. Customer Endpoints

### GET `/customers`

Query params:

- `q` (optional, searches name/email)
- `page`, `page_size`
- `include_deleted` (admin/auditor only)

### POST `/customers`

Request:

```json
{
  "name": "Jane Doe",
  "phone": "5551234567",
  "email": "jane@example.com",
  "address_line_1": "123 Oak St",
  "address_line_2": "Apt 4",
  "city": "Springfield",
  "state": "CA",
  "zip_code": "90210"
}
```

Phone, email, and address fields are encrypted at rest. Responses return masked values by default.

### GET `/customers/{customerId}`

Returns customer with masked PII. Includes purchase limit status per tier.

### PATCH `/customers/{customerId}`

Requires `version`.

### DELETE `/customers/{customerId}`

Soft-delete.

### POST `/customers/{customerId}/restore`

Restore soft-deleted customer if within 30 days.

## 7. Fulfillment Endpoints

### GET `/fulfillments`

Filters:

- `status`
- `tier_id`
- `customer_id`
- `type`
- `date_from`
- `date_to`
- `page`, `page_size`

### POST `/fulfillments`

Creates fulfillment and performs atomic checks/reservation.

Request:

```json
{
  "tier_id": "uuid",
  "customer_id": "uuid",
  "type": "PHYSICAL",
  "initial_status": "DRAFT"
}
```

Validation:

- Inventory available
- Purchase limit not exceeded in rolling 30-day window

Error examples:

- `422 PURCHASE_LIMIT_REACHED`
- `422 INVENTORY_UNAVAILABLE`

### GET `/fulfillments/{fulfillmentId}`

### PATCH `/fulfillments/{fulfillmentId}`

Non-status editable fields only, requires `version`.

### POST `/fulfillments/{fulfillmentId}/transition`

Performs validated status transition.

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

`shipping_address` is required when transitioning a PHYSICAL fulfillment to READY_TO_SHIP (if not already set). US address format enforced. Address lines encrypted at rest.

Rules:

- Enforce allowed state graph
- `ON_HOLD`/`CANCELED` require `reason`
- `SHIPPED` requires tracking number regex `[A-Za-z0-9]{8,30}`
- `VOUCHER_ISSUED` requires voucher code
- Transition + side effects + timeline + notifications are atomic

### GET `/fulfillments/{fulfillmentId}/timeline`

Returns append-only transition timeline.

### POST `/fulfillments/{fulfillmentId}/restore`

Restore soft-deleted fulfillment if within 30 days. Admin only.

## 8. Exception Endpoints

### GET `/exceptions`

Filters:

- `status`
- `type`
- `fulfillment_id`
- `opened_from`
- `opened_to`

### POST `/exceptions`

Manual exception creation.

Request:

```json
{
  "fulfillment_id": "uuid",
  "type": "MANUAL",
  "content": "Customer reported wrong address"
}
```

### GET `/exceptions/{exceptionId}`

### POST `/exceptions/{exceptionId}/events`

Append thread event.

Request:

```json
{
  "event_type": "COMMENT",
  "content": "Carrier contacted, awaiting response"
}
```

### POST `/exceptions/{exceptionId}/status`

Update exception status.

Request:

```json
{
  "status": "RESOLVED",
  "resolution_note": "Replacement shipped"
}
```

`resolution_note` required when moving to `RESOLVED`.

## 9. Messaging Endpoints

### GET `/message-templates`

### POST `/message-templates`

### PATCH `/message-templates/{templateId}`

Requires `version`.

### DELETE `/message-templates/{templateId}`

Soft-delete.

### POST `/message-templates/{templateId}/restore`

Restore soft-deleted template if within 30 days. Admin only.

### GET `/notifications`

Returns current user's in-app notification inbox.

Query params:

- `is_read` (optional, boolean filter)
- `page`, `page_size`

### PATCH `/notifications/{notificationId}`

Mark notification as read.

Request:

```json
{
  "is_read": true
}
```

### POST `/notifications/dispatch`

Queues/sends notifications for configured channels.

Request:

```json
{
  "template_id": "uuid",
  "recipient_id": "uuid",
  "channels": ["IN_APP", "SMS", "EMAIL"],
  "context": {
    "fulfillment_id": "uuid"
  }
}
```

Behavior:

- IN_APP attempts immediate write, retries up to 3 times over 30 minutes
- SMS/EMAIL create `QUEUED` handoff entries

### GET `/send-logs`

Filters:

- `recipient_id`
- `channel`
- `status`
- `date_from`
- `date_to`

### POST `/send-logs/{sendLogId}/print`

Marks offline handoff as printed.

Response:

```json
{
  "id": "uuid",
  "status": "PRINTED",
  "printed_by": "uuid",
  "printed_at": "2026-04-16T10:30:00Z"
}
```

## 10. Reports and Exports

### POST `/reports/exports`

Generate report file to local path; returns queued/created result.

Request:

```json
{
  "report_type": "FULFILLMENT_SUMMARY",
  "filters": {
    "date_from": "2026-04-01",
    "date_to": "2026-04-16",
    "status": ["READY_TO_SHIP", "SHIPPED"]
  },
  "include_sensitive": false
}
```

Response:

```json
{
  "export_id": "uuid",
  "status": "QUEUED"
}
```

### GET `/reports/exports`

List export history with checksum and expiry.

### GET `/reports/exports/{exportId}`

### POST `/reports/exports/{exportId}/verify-checksum`

Re-hash file and compare to stored SHA-256.

Response:

```json
{
  "export_id": "uuid",
  "verified": true,
  "checksum": "hex"
}
```

## 11. Settings and SLA Configuration

### GET `/settings/business-hours`

### PUT `/settings/business-hours`

Request:

```json
{
  "business_hours_start": "08:00",
  "business_hours_end": "18:00",
  "business_days": [1, 2, 3, 4, 5],
  "timezone": "America/New_York"
}
```

### GET `/settings/blackout-dates`

### POST `/settings/blackout-dates`

### DELETE `/settings/blackout-dates/{dateId}`

## 12. Jobs and Health

### GET `/admin/health`

Returns system readiness and dependencies (db, key file, scheduler).

### GET `/admin/jobs/runs`

Returns job run history.

Filters:

- `job_name`
- `status`
- `started_from`
- `started_to`

### POST `/admin/jobs/{jobName}/run`

Manual trigger for allowed jobs.

## 13. User Management

### GET `/admin/users`

List all users. Admin only.

Query params:

- `role` (optional)
- `is_active` (optional)

### POST `/admin/users`

Create user. Admin only.

Request:

```json
{
  "username": "string",
  "email": "string",
  "password": "string",
  "role": "ADMINISTRATOR | FULFILLMENT_SPECIALIST | AUDITOR"
}
```

### GET `/admin/users/{userId}`

### PATCH `/admin/users/{userId}`

Update user details or role. Requires `version`. Admin only.

### DELETE `/admin/users/{userId}`

Deactivate user (sets `is_active = false`). Admin only.

## 14. Audit and Compliance

### GET `/audit/logs`

Filters:

- `table_name`
- `record_id`
- `operation`
- `performed_by`
- `date_from`
- `date_to`

Auditor/Admin only.

### GET `/audit/exports`

Returns export audit events and metadata.

## 15. Backup and Restore Operations

### POST `/admin/backups/run`

Starts local backup (db + media path).

### GET `/admin/backups`

Lists available backups.

### POST `/admin/restore`

Triggers one-click restore workflow; requires admin elevated permission and writes full audit trail.

Request:

```json
{
  "backup_id": "uuid",
  "verify_referential_integrity": true
}
```

## 16. Validation Rules Summary

- Tracking number: alphanumeric length 8-30
- Purchase limit: per customer+tier, rolling 30 days, excludes canceled
- No backorders: creation/ready transition fails when inventory is 0
- Hold/cancel transitions require reason
- Soft-delete restore allowed only within 30 days
- Default export output is masked sensitive values

## 17. Idempotency and Observability

Recommended headers:

- `X-Request-Id` for trace correlation
- `Idempotency-Key` for create/transition/export endpoints

Audit and job logs should capture:

- actor
- endpoint/action
- outcome
- timestamps
- error stacks where applicable
