package api_tests

// Tests added to close audit-report-2 coverage gaps:
//   - failed-send retry path surfaces QUEUED→FAILED in production flow
//   - reports list visibility filter is applied with the total count pre-paged
//   - admin-only routes reject non-admin callers (schedules, dr-drills)

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
	"github.com/fulfillops/fulfillops/internal/service"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func loginSpecialist(t *testing.T) *http.Cookie {
	t.Helper()
	ctx := context.Background()
	auditSvc := service.NewAuditService(repository.NewAuditRepository(testPool))
	userSvc := service.NewUserService(repository.NewUserRepository(testPool), auditSvc)
	username := fmt.Sprintf("api_spec_%d", time.Now().UnixNano())
	user, err := userSvc.CreateUser(ctx, username, username+"@test.com", "Spec@Test1!", domain.RoleFulfillmentSpecialist)
	if err != nil {
		t.Fatalf("create specialist: %v", err)
	}
	t.Cleanup(func() { _ = userSvc.DeactivateUser(ctx, user.ID) })
	rr := as(nil, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"username": username,
		"password": "Spec@Test1!",
	})
	mustStatus(t, rr, http.StatusOK)
	for _, c := range rr.Result().Cookies() {
		if c.Name == "fulfillops_session" {
			return c
		}
	}
	t.Fatal("specialist login did not return session cookie")
	return nil
}

func createDispatchableTemplate(t *testing.T) string {
	t.Helper()
	rr := admin(http.MethodPost, "/api/v1/message-templates", map[string]any{
		"name":          fmt.Sprintf("t_%d", time.Now().UnixNano()),
		"category":      "BOOKING_RESULT",
		"channel":       "SMS",
		"body_template": "hi {{customer}}",
	})
	mustStatus(t, rr, http.StatusCreated)
	return decodeJSON(t, rr)["id"].(string)
}

func ensureCustomer(t *testing.T) string {
	t.Helper()
	rr := admin(http.MethodPost, "/api/v1/customers", map[string]any{
		"name":  fmt.Sprintf("Recipient %d", time.Now().UnixNano()),
		"phone": "555-123-9999",
		"email": "r@example.com",
	})
	mustStatus(t, rr, http.StatusCreated)
	return decodeJSON(t, rr)["id"].(string)
}

// ── (6) Dispatch queued → MarkFailed → scheduler retry path ──────────────────

func TestSendLog_Queued_MarkFailed_RetryCycle(t *testing.T) {
	tmplID := createDispatchableTemplate(t)
	recID := ensureCustomer(t)

	rr := admin(http.MethodPost, "/api/v1/dispatch", map[string]any{
		"template_id":    tmplID,
		"recipient_id":   recID,
		"extra_channels": []string{"SMS"},
		"context":        map[string]any{"customer": "R"},
	})
	mustStatus(t, rr, http.StatusCreated)

	// Locate the QUEUED SMS row created for the same recipient.
	rr = admin(http.MethodGet, "/api/v1/send-logs?recipient_id="+recID+"&status=QUEUED&channel=SMS", nil)
	mustStatus(t, rr, http.StatusOK)
	body := decodeJSON(t, rr)
	items, _ := body["items"].([]any)
	if len(items) == 0 {
		t.Fatal("expected a QUEUED SMS send_log to be created by dispatch")
	}
	sendLog, _ := items[0].(map[string]any)
	sendLogID, _ := sendLog["id"].(string)
	if sendLogID == "" {
		t.Fatal("QUEUED send_log missing id")
	}

	// Transition QUEUED → FAILED via the new API path.
	rr = admin(http.MethodPut, "/api/v1/send-logs/"+sendLogID+"/failed", map[string]any{
		"reason": "sms gateway unreachable",
	})
	mustStatus(t, rr, http.StatusNoContent)

	// The row should now be FAILED with first_failed_at and next_retry_at set
	// so the retry scheduler can pick it up.
	rr = admin(http.MethodGet, "/api/v1/send-logs?recipient_id="+recID+"&channel=SMS&status=FAILED", nil)
	mustStatus(t, rr, http.StatusOK)
	body = decodeJSON(t, rr)
	items, _ = body["items"].([]any)
	if len(items) == 0 {
		t.Fatal("expected send_log to be reachable in FAILED state after MarkFailed")
	}
	failedRow, _ := items[0].(map[string]any)
	if failedRow["first_failed_at"] == nil {
		t.Error("first_failed_at must be stamped on MarkFailed")
	}
	if failedRow["next_retry_at"] == nil {
		t.Error("next_retry_at must be scheduled so retry job picks up the row")
	}

	// Drive the retry scheduler directly. next_retry_at is T+10 in the future,
	// so we shift it into the past to prove the transition logic is reachable.
	ctx := context.Background()
	sendLogRepo := repository.NewSendLogRepository(testPool)
	msgSvc := service.NewMessagingService(
		repository.NewMessageTemplateRepository(testPool),
		sendLogRepo,
		repository.NewNotificationRepository(testPool),
		service.NewAuditService(repository.NewAuditRepository(testPool)),
	)
	parsedID, err := uuid.Parse(sendLogID)
	if err != nil {
		t.Fatalf("parsing send log ID: %v", err)
	}
	if err := sendLogRepo.UpdateNextRetry(ctx, parsedID, time.Now().UTC().Add(-time.Minute)); err != nil {
		t.Fatalf("forcing next_retry_at: %v", err)
	}
	retried, err := msgSvc.RetryPending(ctx, 3)
	if err != nil {
		t.Fatalf("RetryPending: %v", err)
	}
	if retried != 1 {
		t.Fatalf("expected retry scheduler to transition 1 row, got %d", retried)
	}
	// After a retry cycle the row should be back in QUEUED handoff state.
	rr = admin(http.MethodGet, "/api/v1/send-logs?recipient_id="+recID+"&channel=SMS&status=QUEUED", nil)
	mustStatus(t, rr, http.StatusOK)
	body = decodeJSON(t, rr)
	if items, _ := body["items"].([]any); len(items) == 0 {
		t.Fatal("expected re-queued send_log after retry cycle")
	}
}

// ── (8) Reports list role-filter pagination correctness ──────────────────────

func TestReportsList_AuditorFilterAppliedBeforePagination(t *testing.T) {
	// Seed: one sensitive + one non-sensitive export. Both exist, but the
	// auditor must see only the non-sensitive one and the total must reflect
	// that filtered set — not the unfiltered row count that paginated out the
	// non-sensitive row behind a sensitive one.
	for i := 0; i < 2; i++ {
		rr := admin(http.MethodPost, "/api/v1/reports/exports", map[string]any{
			"report_type":       "fulfillments",
			"include_sensitive": true,
		})
		mustStatus(t, rr, http.StatusCreated)
	}
	rr := admin(http.MethodPost, "/api/v1/reports/exports", map[string]any{
		"report_type":       "fulfillments",
		"include_sensitive": false,
	})
	mustStatus(t, rr, http.StatusCreated)

	audCookie := loginAuditor(t)

	// Page size 1 exaggerates the pagination-correctness issue: if sensitive
	// rows are filtered after the page is cut, page 1 can come back empty for
	// the auditor while total still says >= 1.
	rr = as(audCookie, http.MethodGet, "/api/v1/reports/exports?page=1&page_size=1", nil)
	mustStatus(t, rr, http.StatusOK)
	body := decodeJSON(t, rr)
	items, _ := body["items"].([]any)
	if len(items) == 0 {
		t.Fatal("auditor should always see non-sensitive exports on page 1 when any exist")
	}
	for _, raw := range items {
		row, _ := raw.(map[string]any)
		if sens, _ := row["include_sensitive"].(bool); sens {
			t.Fatalf("auditor must not receive sensitive row: %+v", row)
		}
	}
	if total, _ := body["total"].(float64); int(total) < 1 {
		t.Fatalf("expected auditor total >= 1 after filter, got %v", body["total"])
	}
}

// ── (7) Scheduler failed-run history surfacing ───────────────────────────────

func TestAdminJobHistory_SurfacesFailedRunWithError(t *testing.T) {
	// A failed job_run row created directly in the repo should show up in the
	// admin history listing with the error_message preserved for the operator.
	ctx := context.Background()
	runRepo := repository.NewJobRunRepository(testPool)
	created, err := runRepo.Create(ctx, &domain.JobRunHistory{
		JobName: fmt.Sprintf("job-fail-%d", time.Now().UnixNano()),
		Status:  domain.JobRunning,
	})
	if err != nil {
		t.Fatalf("seed job run: %v", err)
	}
	msg := "simulated failure stack: boom"
	if err := runRepo.Finish(ctx, created.ID, domain.JobFailed, 0, &msg); err != nil {
		t.Fatalf("finish job run: %v", err)
	}

	rr := admin(http.MethodGet, "/api/v1/admin/jobs/runs?status=FAILED&page_size=100", nil)
	mustStatus(t, rr, http.StatusOK)
	body := decodeJSON(t, rr)
	items, _ := body["items"].([]any)
	found := false
	for _, raw := range items {
		row, _ := raw.(map[string]any)
		if id, _ := row["id"].(string); id == created.ID.String() {
			found = true
			if es, _ := row["error_stack"].(string); es != msg {
				t.Fatalf("expected error_stack=%q, got %q", msg, es)
			}
		}
	}
	if !found {
		t.Fatalf("failed run %s not present in admin history", created.ID)
	}
}

// ── (9) 403 coverage: message handoff fail + admin-only schedule/dr-drill ────

func TestSendLog_MarkFailed_ForbiddenForAuditor(t *testing.T) {
	audCookie := loginAuditor(t)
	rr := as(audCookie, http.MethodPut,
		"/api/v1/send-logs/00000000-0000-0000-0000-000000000001/failed", map[string]any{})
	mustStatus(t, rr, http.StatusForbidden)
}

func TestAdminSchedules_ForbiddenForSpecialist(t *testing.T) {
	specCookie := loginSpecialist(t)
	rr := as(specCookie, http.MethodGet, "/api/v1/admin/job-schedules", nil)
	mustStatus(t, rr, http.StatusForbidden)
}

func TestAdminSchedules_ForbiddenForAuditor(t *testing.T) {
	audCookie := loginAuditor(t)
	rr := as(audCookie, http.MethodGet, "/api/v1/admin/job-schedules", nil)
	mustStatus(t, rr, http.StatusForbidden)
}

func TestAdminDRDrills_ForbiddenForSpecialist(t *testing.T) {
	specCookie := loginSpecialist(t)
	rr := as(specCookie, http.MethodGet, "/api/v1/admin/dr-drills", nil)
	mustStatus(t, rr, http.StatusForbidden)
}

func TestAdminDRDrills_ForbiddenForAuditor(t *testing.T) {
	audCookie := loginAuditor(t)
	rr := as(audCookie, http.MethodGet, "/api/v1/admin/dr-drills", nil)
	mustStatus(t, rr, http.StatusForbidden)
}
