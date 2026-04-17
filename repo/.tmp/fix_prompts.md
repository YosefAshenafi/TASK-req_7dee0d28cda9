# FulfillOps Fix Prompts (from Architecture Audit)

Each section below is a self-contained prompt. Copy the prompt text and run it directly in Claude Code.

---

## BLOCKER 1 — In-app messaging: customer ID written into `notifications.user_id` (FK violation)

**Files:** `internal/service/messaging.go:68-73`

In internal/service/messaging.go, the Dispatch method signature is:
  Dispatch(ctx, templateID, recipientID uuid.UUID, contextData map[string]any)

The problem: for IN_APP channel, `recipientID` is used directly as `notification.UserID`, but
`notifications.user_id` is a FK to `users(id)` — not `customers(id)`. If a caller passes a
customer UUID, this will panic with a FK violation.

Fix by splitting the recipient model:
1. Update the MessagingService interface and messagingService.Dispatch to accept both
   a `userID uuid.UUID` (the system user who gets the in-app notification, may be nil/zero)
   and a `recipientID uuid.UUID` (the logical recipient for SMS/EMAIL send_log tracking).
   - Alternatively, add a separate `RecipientUserID *uuid.UUID` field to the call signature or
     a small input struct, so IN_APP path always uses a valid users.id.
2. In the IN_APP switch case (line 66-77), use `userID` for `notification.UserID` rather than
   the generic `recipientID`.
3. Update all callers of Dispatch (grep for `.Dispatch(`) to pass the correct user ID for
   in-app notifications and the customer/recipient ID for the send_log.
4. Update the SendLog `recipient_id` to reflect the customer ID (the logical recipient) —
   that column has no FK constraint so it is fine.
5. Update any affected tests in tests/API_tests/ and tests/integration/.

---

## BLOCKER 2 — Page report workflow broken: invalid status, stub verify, missing download route, unsupported types

**Files:** `internal/handler/page_report.go`, `internal/view/reports/workspace.templ`, `internal/view/reports/history.templ`, `internal/handler/router.go`

There are four broken things in the page report flow. Fix all of them:

1. INVALID STATUS — internal/handler/page_report.go:39
   The line `Status: "PENDING"` sets an invalid status. The DB CHECK constraint
   (migrations/001_init.up.sql:184) only allows QUEUED/PROCESSING/COMPLETED/FAILED.
   Change it to `Status: domain.ExportQueued`.

2. BYPASS SERVICE — page_report.go:31-48 (PostGenerateExport)
   PostGenerateExport creates the export record directly via reportRepo.Create then
   redirects. It must also kick off generation. Inject ExportService into
   PageReportHandler and after creating the record, launch:
     go func(id uuid.UUID) { _ = h.exportSvc.GenerateExport(context.Background(), id) }(created.ID)
   Also add `ExportService service.ExportService` field to PageReportHandler and wire it in
   the constructor in internal/handler/router.go.

3. STUB VERIFY — page_report.go:61-64 (PostVerifyExport)
   PostVerifyExport is a no-op stub. Replace it with a real call to
   h.exportSvc.VerifyChecksum(ctx, id) where id comes from c.Param("id").
   On failure, redirect with error flash. On success, redirect with the verified result.

4. MISSING DOWNLOAD ROUTE — internal/view/reports/history.templ:50 links to
   `/reports/exports/:id/download` but no route or handler for that path exists
   (internal/handler/router.go has no /download route).
   Add a PageReportHandler.DownloadExport handler that:
   - Parses the export ID from c.Param("id")
   - Calls h.reportRepo.GetByID to get the export record
   - Verifies Status == COMPLETED and FilePath is set
   - Serves the file using c.FileAttachment(filePath, filename)
   Register the route in router.go:
     adminOrAuditPage.GET("/reports/exports/:id/download", pageReport.DownloadExport)

5. UNSUPPORTED REPORT TYPES — workspace.templ:30 offers "audit_log", "send_logs",
   "exceptions" but internal/service/export.go only handles "fulfillments", "customers",
   "audit" (maps to writeAuditCSV).
   In workspace.templ, remove the "send_logs" and "exceptions" options, and rename the
   "audit_log" option value to "audit" so it matches what export.go expects.

---

## HIGH 3 — Sensitive export: auditor can bypass explicit-permission check

**Files:** `internal/handler/report.go:57-100`, `internal/handler/router.go:247`

In internal/handler/report.go, the Create handler (POST /api/v1/reports/exports) does not
check whether the requesting user has permission to set include_sensitive=true.
The route is accessible to both ADMINISTRATOR and AUDITOR roles (router.go:247).

Fix:
In the Create handler, after binding the request (line ~60), if req.IncludeSensitive is true,
check the user's role from context:
  roleRaw, _ := c.Get("userRole")
  role, _ := roleRaw.(domain.UserRole)
  if req.IncludeSensitive && role != domain.RoleAdministrator {
      c.JSON(http.StatusForbidden, middleware.ErrorResponse{
          Code: "FORBIDDEN",
          Message: "include_sensitive requires Administrator role",
      })
      return
  }
Add the same check to PageReportHandler.PostGenerateExport in internal/handler/page_report.go.
Ensure userRole is set in the auth middleware context (grep for c.Set("userRole") to confirm
it is already set, or add it in internal/middleware/auth.go alongside "userID").

---

## HIGH 4 — Voucher code stored as plaintext in page transition flow

**File:** `internal/handler/page_fulfillment.go:200`

In internal/handler/page_fulfillment.go, the PostTransition handler at line 200 does:
  if vc := c.PostForm("voucher_code"); vc != "" { input.VoucherCode = []byte(vc) }

This stores the raw plaintext voucher code. The service (internal/service/fulfillment.go:33)
expects VoucherCode to be *pre-encrypted bytes* (the comment says "pre-encrypted by handler").
The API handler path does encrypt before calling the service — the page handler must match.

Fix:
Replace line 200 with:
  if vc := c.PostForm("voucher_code"); vc != "" {
      encrypted, err := h.encSvc.Encrypt([]byte(vc))
      if err != nil {
          redirectWithFlash(c, h.store, "/fulfillments/"+id.String(), "error", "Encryption failed.")
          return
      }
      input.VoucherCode = encrypted
  }

Ensure PageFulfillmentHandler already has an encSvc service.EncryptionService field (check
the struct definition). If not, inject it in the constructor and wire it in router.go.

---

## HIGH 5 — Cross-type fulfillment transitions allowed (physical→VOUCHER_ISSUED, voucher→SHIPPED)

**Files:** `internal/service/fulfillment.go:182-237`, `internal/domain/enums.go:31-40`

The fulfillment Transition service (internal/service/fulfillment.go) validates transitions
using the generic AllowedTransitions map in domain/enums.go but does NOT enforce
type-specific rules. A VOUCHER-type fulfillment can currently be transitioned to SHIPPED
and a PHYSICAL-type fulfillment can be transitioned to VOUCHER_ISSUED — both corrupt
business semantics.

Fix in internal/service/fulfillment.go, inside the Transition method, after the generic
IsTransitionAllowed check (around line 173), add type-specific validation:

  // Type-specific transition guard
  switch input.ToStatus {
  case domain.StatusShipped:
      if f.Type != domain.TypePhysical {
          return domain.NewValidationError("invalid transition", map[string]string{
              "to_status": "SHIPPED is only valid for PHYSICAL fulfillments",
          })
      }
  case domain.StatusVoucherIssued:
      if f.Type != domain.TypeVoucher {
          return domain.NewValidationError("invalid transition", map[string]string{
              "to_status": "VOUCHER_ISSUED is only valid for VOUCHER fulfillments",
          })
      }
  case domain.StatusDelivered:
      if f.Type != domain.TypePhysical {
          return domain.NewValidationError("invalid transition", map[string]string{
              "to_status": "DELIVERED is only valid for PHYSICAL fulfillments",
          })
      }
  }

Add corresponding tests in tests/API_tests/fulfillments_api_test.go or
internal/service/fulfillment_test.go proving:
- A VOUCHER-type fulfillment returns 422/ErrValidation when transitioning to SHIPPED
- A PHYSICAL-type fulfillment returns 422/ErrValidation when transitioning to VOUCHER_ISSUED

---

## HIGH 6 — Overdue detection wrongly includes SHIPPED / VOUCHER_ISSUED fulfillments

**File:** `internal/repository/fulfillment.go:194-205`

The ListOverdue query in internal/repository/fulfillment.go:198 is:
  WHERE status IN ('READY_TO_SHIP','SHIPPED','VOUCHER_ISSUED')

SHIPPED and VOUCHER_ISSUED are in-progress but the action for those statuses has already
happened — they should not be flagged as overdue for the next unmet action. Including them
causes false overdue exceptions to be opened.

Fix:
Change the WHERE clause to only select READY_TO_SHIP fulfillments (the ones waiting
for the fulfillment action to begin):
  WHERE status = 'READY_TO_SHIP'
    AND ready_at IS NOT NULL AND deleted_at IS NULL

The SLA service (internal/service/sla.go) is responsible for computing whether the
deadline has been exceeded — keep that logic there. The repository should only return
candidates that have not yet had their primary fulfillment action taken.

Also verify that internal/job/overdue_job.go (or wherever ListOverdue is called) handles
PHYSICAL vs VOUCHER type correctly when creating exception records (ExceptionOverdueShipment
vs ExceptionOverdueVoucher based on fulfillment.Type).

---

## HIGH 7 — Message template form submits lowercase enum values (DB constraint mismatch)

**File:** `internal/view/messages/template_form.templ:49-62`

In internal/view/messages/template_form.templ, the category and channel select option
values use lowercase strings (e.g. "booking_result", "in_app") but the domain enums
(internal/domain/enums.go:181) and DB CHECK constraints use uppercase
("BOOKING_RESULT", "IN_APP", etc.). Submitting the form will fail DB validation or
silently store invalid values.

Fix — update option values to match domain constants:

Category options (change value attributes only, keep display text):
  "booking_result"       → "BOOKING_RESULT"
  "booking_change"       → "BOOKING_CHANGE"
  "expiration"           → "EXPIRATION"
  "fulfillment_progress" → "FULFILLMENT_PROGRESS"

Channel options:
  "in_app"  → "IN_APP"
  "sms"     → "SMS"
  "email"   → "EMAIL"

Also update the helper functions tmplCategory() and tmplChannel() at the bottom of the
file (or wherever they are defined in that package) to compare against uppercase values
for the selected?= check to work correctly.

Apply the same fix to any other template form that sets category/channel values (grep for
value="booking_result" and value="in_app" across internal/view/).

---

## HIGH 8 — Handoff queue "Mark as Printed" posts to nonexistent `/print` route

**File:** `internal/view/messages/handoff_queue.templ:43`

In internal/view/messages/handoff_queue.templ line 43, the form action is:
  action={ templ.SafeURL("/messages/send-logs/" + item.ID.String() + "/print") }

But the registered route in internal/handler/router.go:137 is:
  adminOrSpecPage.POST("/messages/send-logs/:id/printed", pageMessage.PostMarkPrinted)

The trailing path segment is "/printed" not "/print". Every "Mark as Printed" button
in the handoff queue posts to a 404 URL.

Fix:
In handoff_queue.templ line 43, change "/print" to "/printed":
  action={ templ.SafeURL("/messages/send-logs/" + item.ID.String() + "/printed") }

No router or handler changes are needed — the handler already exists and is correctly named.

---

## HIGH 9 — Backup restore "verify integrity" checkbox is outside the `<form>` element

**File:** `internal/view/admin/backups.templ:63-81`

In internal/view/admin/backups.templ, the modal for restore contains:
  - Lines 68-71: a checkbox `<input name="verify_integrity">` inside the modal-body div
  - Lines 73-79: a `<form method="POST">` that wraps only the hidden input and buttons

The checkbox is OUTSIDE the form, so its value is never submitted with the POST request.
The server-side handler will never receive verify_integrity=on.

Fix:
Move the checkbox inside the <form> element. The corrected structure should be:
  <form method="POST" action="/admin/backups/BACKUP_ID/restore">
      <input type="hidden" name="backup_id" value="..."/>
      <div class="form-check" style="margin-top:12px">
          <input type="checkbox" id="integrity-ID" name="verify_integrity"/>
          <label for="integrity-ID">Verify referential integrity after restore</label>
      </div>
      <div class="modal-footer">
          <button type="button" ...>Cancel</button>
          <button type="submit" ...>Restore Database</button>
      </div>
  </form>

Also in internal/handler/page_admin.go PostRestoreBackup, ensure the handler reads
c.PostForm("verify_integrity") and passes it to the backup service — default behavior
should be verify=true even if the checkbox is unchecked (fail-safe default).

---

## HIGH 10 — Scheduler uses interval-only; stats job must run at explicit 2:00 AM wall-clock

**Files:** `internal/job/scheduler.go`, `cmd/server/main.go:88-97`

The current Scheduler in internal/job/scheduler.go uses fixed-interval tickers (line 38).
The "stats" job must run at 2:00 AM local time, and the "cleanup" job should also respect
a nightly schedule rather than an arbitrary 24h offset from startup.

Fix by adding a cron-like "daily at time" registration to the Scheduler:

1. Add a new method to Scheduler:
   func (s *Scheduler) RegisterDaily(name string, hour, minute int, fn JobFunc)
   This method registers a job that runs at the specified wall-clock time each day.
   Implementation: compute the duration until the next occurrence of hour:minute in UTC,
   use time.AfterFunc for the first firing, then re-arm with a 24h ticker.

   Example initial delay calculation:
     now := time.Now().UTC()
     next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, time.UTC)
     if !next.After(now) { next = next.Add(24 * time.Hour) }
     initialDelay := next.Sub(now)

2. In cmd/server/main.go, change the "stats" job registration from:
     sched.Register("stats", 24*time.Hour, job.NewStatsJob(fulfillRepo, tierRepo).Run)
   to:
     sched.RegisterDaily("stats", 2, 0, job.NewStatsJob(fulfillRepo, tierRepo).Run)

3. Similarly register "cleanup" and "export-cleanup" as daily jobs at a fixed time
   (e.g. 3:00 AM) so they don't run at arbitrary times based on server start.

4. Ensure RunOnce still works for all registered jobs (both interval and daily).

---

## HIGH 11 — Customer edit silently drops encrypted address (dead `addrField` helper)

**Files:** `internal/repository/customer.go:91-107`, `internal/handler/page_customer.go`

When a customer is edited via the page form, the handler builds a domain.Customer and calls
customerRepo.Update(). If address form fields are empty or not submitted, the handler sets
AddressEncrypted to nil/empty — which overwrites the existing encrypted address with NULL,
silently deleting it.

Fix the page customer update handler (internal/handler/page_customer.go, PostUpdate or
similar):

1. Before building the updated customer, fetch the existing record:
     existing, err := h.customerRepo.GetByID(ctx, id)
     if err != nil { /* handle */ }

2. Only re-encrypt and update address fields if the form values are non-empty:
     if line1 := c.PostForm("addr_line1"); line1 != "" {
         addr := line1 + "|" + c.PostForm("addr_line2") + "|" +
                 c.PostForm("addr_city") + "|" + c.PostForm("addr_state") + "|" +
                 c.PostForm("addr_zip")
         encrypted, err := h.encSvc.Encrypt([]byte(addr))
         if err != nil { /* handle */ }
         customer.AddressEncrypted = encrypted
     } else {
         customer.AddressEncrypted = existing.AddressEncrypted  // preserve
     }

3. Apply the same preserve-if-empty logic for phone and email encrypted fields.

4. This ensures editing a customer's name does not wipe their encrypted PII fields.

---

## HIGH 12 — Template restore route is missing from router

**Files:** `internal/handler/router.go:129-134`, `internal/handler/page_message.go:107-115`

In internal/view/admin/recovery.templ:74, the restore button for soft-deleted message
templates POSTs to `/messages/templates/:id/restore`.

The handler PostRestoreTemplate already exists in internal/handler/page_message.go:107.
However, the route is NOT registered in internal/handler/router.go.

Fix — add this line in router.go in the Messages page section (after the delete route):
  adminOnlyPage.POST("/messages/templates/:id/restore", pageMessage.PostRestoreTemplate)

No handler changes are needed. Verify that pageMessage is the *PageMessageHandler variable
already in scope at that point in RegisterRoutes.

---

## MEDIUM 13 — Nil pointer panic on exception detail page when `OpenedBy` is nil

**File:** `internal/handler/page_exception.go:100`

In internal/handler/page_exception.go line 100:
  OpenedByName: ex.OpenedBy.String()[:8],

ex.OpenedBy is of type *uuid.UUID (the column is nullable per migrations/001_init.up.sql:115).
If OpenedBy is nil, calling .String() panics with a nil pointer dereference.

Fix:
  var openedByName string
  if ex.OpenedBy != nil {
      s := ex.OpenedBy.String()
      if len(s) >= 8 {
          openedByName = s[:8]
      } else {
          openedByName = s
      }
  }
  // then use openedByName in the render call

---

## MEDIUM 14 — Exception status page bypasses service validation; "CLOSED" is an invalid status

**Files:** `internal/view/exceptions/detail.templ:72`, `internal/handler/page_exception.go:105-121`, `internal/service/exception.go:73`

Two problems in the exception update flow:

1. INVALID STATUS OPTION — detail.templ:72 offers a "CLOSED" option in the status
   dropdown. "CLOSED" is not a valid ExceptionStatus in domain/enums.go (valid values:
   OPEN, INVESTIGATING, ESCALATED, RESOLVED). Remove the CLOSED option from the select.
   Also add ESCALATED if it is not already present.

2. SERVICE BYPASS — page_exception.go:116 calls h.exRepo.UpdateStatus() directly,
   bypassing ExceptionService.UpdateStatus(). This means the service-layer rule
   "resolution note is required when status = RESOLVED" (exception.go:79) is never
   enforced on the page path.

   Fix: inject ExceptionService into PageExceptionHandler and call it instead of the repo:
     if _, err := h.exceptionSvc.UpdateStatus(ctx, id, status, note); err != nil {
   
   Update the PageExceptionHandler struct to hold `exceptionSvc service.ExceptionService`,
   update NewPageExceptionHandler constructor, and wire it in router.go.
   
   The service.UpdateStatus signature takes (ctx, id, status, resolutionNote string) —
   pass c.PostForm("resolution_note") directly. The service handles the nil/empty check.

---

## MEDIUM 15 — Retry policy only covers IN_APP; doesn't match "3 retries over 30 min"

**File:** `internal/service/messaging.go:93-134`

The current RetryPending in internal/service/messaging.go:93 only processes IN_APP records
from GetRetryable. SMS/EMAIL channels in FAILED status are never retried. The retry
backoff uses `attemptCount * 5 minutes` which can exceed 30 minutes after 7 attempts.

Fix to match the spec ("failed sends retried up to 3 times, spread over 30 minutes"):

1. Change the retry logic to also attempt SMS/EMAIL retries. For SMS/EMAIL, a "retry"
   means re-queuing the send_log to QUEUED status so it appears in the handoff queue again.

2. Cap at exactly 3 retry attempts (maxAttempts = 3 at the call site in main.go — already
   correct at job.NewNotifyJob(messagingSvc, 3)).

3. Space retries at 10-minute intervals (not exponential backoff) to spread 3 attempts over
   ~30 minutes:
     next := now.Add(10 * time.Minute)  // fixed 10-min window per retry

4. Update the retry loop to handle all channels:
   for _, l := range logs {
       if l.AttemptCount >= maxAttempts {
           _ = s.sendLogRepo.UpdateStatus(ctx, l.ID, domain.SendFailed, nil)
           continue
       }
       switch l.Channel {
       case domain.ChannelInApp:
           // existing IN_APP retry logic (create notification)
       case domain.ChannelSMS, domain.ChannelEmail:
           // re-queue for handoff
           next := now.Add(10 * time.Minute)
           _ = s.sendLogRepo.UpdateStatus(ctx, l.ID, domain.SendQueued, nil)
           _ = s.sendLogRepo.UpdateNextRetry(ctx, l.ID, next)
       }
       retried++
   }

5. Ensure GetRetryable in internal/repository/send_log.go returns FAILED records for
   SMS/EMAIL (not just QUEUED IN_APP) when next_retry_at <= now and attempt_count < maxAttempts.

---

## MEDIUM 16 — Send logs page ignores "recipient" filter

**File:** `internal/handler/page_message.go:117-148`

In internal/handler/page_message.go ShowSendLogs (line 117-148), the handler parses
`c.Query("recipient")` and passes it to the view (line 145) for display but NEVER adds it
to the `repository.SendLogFilters` struct before querying. The filter is silently ignored.

Fix:

1. Check if repository.SendLogFilters has a RecipientID or Recipient field. If not, add one:
   In internal/repository/send_log.go (wherever SendLogFilters is defined):
     RecipientID *uuid.UUID  // filter by recipient_id exact match

2. In page_message.go ShowSendLogs, parse the recipient query param and add it to filters:
   if r := c.Query("recipient"); r != "" {
       if rid, err := uuid.Parse(r); err == nil {
           filters.RecipientID = &rid
       }
   }

3. In the repository List method for send_logs, apply the filter if set:
   if f.RecipientID != nil {
       // append AND recipient_id = $N to the WHERE clause
   }

4. Apply the same fix to the API handler at internal/handler/message.go:167 — check if
   the recipient query param is parsed and applied there as well.

---

## MEDIUM 17 — Session cookies missing `Secure` flag

**File:** `cmd/server/main.go:100-106`

In cmd/server/main.go, the session cookie store options (lines 101-106) do not set
Secure: true, meaning session cookies can be transmitted over plain HTTP, risking
session theft if deployed without TLS enforcement at the load balancer.

Fix:
1. Add a FULFILLOPS_SECURE_COOKIES env var (bool, default "true" for production) to
   internal/config/config.go alongside the other env vars.

2. In cmd/server/main.go, update the store options:
   store.Options = &sessions.Options{
       Path:     "/",
       MaxAge:   86400 * 7,
       HttpOnly: true,
       Secure:   cfg.SecureCookies,  // true in prod, can disable for local HTTP dev
       SameSite: http.SameSiteStrictMode,
   }

3. Set the default to true (opt-out for dev). In config, parse:
   SecureCookies: os.Getenv("FULFILLOPS_SECURE_COOKIES") != "false"
   This means Secure=true unless the operator explicitly sets the env var to "false".

---

## MEDIUM 18 — Settings API stores raw string bytes; SLA service expects JSON-encoded values

**Files:** `internal/handler/settings.go:65`, `internal/service/sla.go:47-62`

In internal/handler/settings.go line 65:
  h.settingRepo.Set(ctx, key, []byte(req.Value), &actorID)

This stores the raw string bytes (e.g. the bytes of "08:00") directly.

But internal/service/sla.go:49 reads the setting and does:
  json.Unmarshal(setting.Value, &v)  // expects JSON-encoded string: `"08:00"`

Raw string bytes are not valid JSON — json.Unmarshal will fail silently and the SLA
service will always use its hardcoded defaults, ignoring API-updated settings.

Fix in internal/handler/settings.go, the Set handler, before calling settingRepo.Set:

  // JSON-encode the value so it can be decoded by consumers expecting JSON
  jsonValue, err := json.Marshal(req.Value)
  if err != nil {
      c.JSON(http.StatusBadRequest, middleware.ErrorResponse{Code: "VALIDATION_ERROR", Message: "invalid value"})
      return
  }
  if err := h.settingRepo.Set(ctx, key, jsonValue, &actorID); err != nil { ... }

For array-type settings like "business_days" where req.Value is a JSON array string,
callers should send the raw JSON value and it should be stored as-is. Consider accepting
`json.RawMessage` for the value field in setSettingRequest, or document that string
settings must be double-JSON-encoded (i.e. the value field itself is a JSON string).
The simplest fix: if req.Value is already valid JSON (json.Valid([]byte(req.Value))),
store it as-is; otherwise JSON-encode it as a string.

---

## LOW 19 — Health endpoint leaks raw DB error message publicly

**File:** `cmd/server/main.go:115-121`

In cmd/server/main.go the /healthz handler at line 115-121:
  c.JSON(http.StatusServiceUnavailable, gin.H{
      "status": "error",
      "db":     "unreachable",
      "error":  err.Error(),   // exposes raw driver error
  })

This can expose internal DB hostnames, connection strings, or driver version info to
unauthenticated callers.

Fix: remove the raw error from the public response:
  c.JSON(http.StatusServiceUnavailable, gin.H{
      "status": "error",
      "db":     "unreachable",
  })

Optionally log the full error server-side:
  log.Printf("healthz: db ping failed: %v", err)

---

## Issue Index

| # | Severity | Issue | Key File(s) |
|---|----------|-------|-------------|
| 1 | Blocker | Notification FK violation (customer ID → users table) | `service/messaging.go:69` |
| 2 | Blocker | Report page: invalid status, stub verify, missing download, unsupported types | `handler/page_report.go`, `view/reports/workspace.templ` |
| 3 | High | Auditor can request sensitive export without permission | `handler/report.go:78` |
| 4 | High | Voucher code written as plaintext via page path | `handler/page_fulfillment.go:200` |
| 5 | High | Cross-type transitions (voucher→SHIPPED, physical→VOUCHER_ISSUED) | `service/fulfillment.go:182` |
| 6 | High | Overdue query includes SHIPPED/VOUCHER_ISSUED | `repository/fulfillment.go:198` |
| 7 | High | Template form submits lowercase enum values | `view/messages/template_form.templ:49` |
| 8 | High | Handoff `/print` vs `/printed` route mismatch | `view/messages/handoff_queue.templ:43` |
| 9 | High | Restore checkbox outside form element | `view/admin/backups.templ:69` |
| 10 | High | Stats job uses 24h interval instead of 2:00 AM cron | `job/scheduler.go`, `cmd/server/main.go:96` |
| 11 | High | Customer edit silently clears encrypted address | `handler/page_customer.go` PostUpdate |
| 12 | High | Template restore route not registered | `handler/router.go:129` |
| 13 | Medium | Nil panic on `ex.OpenedBy.String()[:8]` | `handler/page_exception.go:100` |
| 14 | Medium | Exception page bypasses service; "CLOSED" is invalid status | `handler/page_exception.go:116`, `view/exceptions/detail.templ:72` |
| 15 | Medium | Retry only covers IN_APP; wrong backoff interval | `service/messaging.go:93` |
| 16 | Medium | Recipient filter parsed but not applied to query | `handler/page_message.go:118` |
| 17 | Medium | Session cookie missing `Secure: true` | `cmd/server/main.go:101` |
| 18 | Medium | Settings stored as raw bytes, SLA expects JSON | `handler/settings.go:65` |
| 19 | Low | Health endpoint exposes raw DB error | `cmd/server/main.go:119` |
