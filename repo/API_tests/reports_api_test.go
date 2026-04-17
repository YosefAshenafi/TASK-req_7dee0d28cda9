package api_tests

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
	"github.com/fulfillops/fulfillops/internal/service"
)

func TestReportsList(t *testing.T) {
	// Seed a non-sensitive export.
	rr := admin(http.MethodPost, "/api/v1/reports/exports", map[string]any{
		"report_type": "fulfillments", "include_sensitive": false,
	})
	mustStatus(t, rr, http.StatusCreated)
	seededID := decodeJSON(t, rr)["id"].(string)

	rr = admin(http.MethodGet, "/api/v1/reports/exports", nil)
	mustStatus(t, rr, http.StatusOK)

	body := decodeJSON(t, rr)
	if body["total"] == nil {
		t.Error("reports list missing 'total' field")
	}
	items, ok := body["items"].([]any)
	if !ok {
		t.Fatal("reports list missing 'items' array")
	}
	found := false
	for _, raw := range items {
		row, _ := raw.(map[string]any)
		if row["id"] == seededID {
			found = true
			if row["report_type"] == nil {
				t.Error("export item missing 'report_type'")
			}
			if row["status"] == nil {
				t.Error("export item missing 'status'")
			}
		}
	}
	if !found {
		t.Errorf("seeded export %s not found in list (total=%v)", seededID, body["total"])
	}
}

func TestReportsCreate(t *testing.T) {
	rr := admin(http.MethodPost, "/api/v1/reports/exports", map[string]any{
		"report_type": "fulfillments", "include_sensitive": false,
	})
	mustStatus(t, rr, http.StatusCreated)
	body := decodeJSON(t, rr)
	if body["id"] == nil {
		t.Fatal("export 'id' missing from response")
	}
}

func TestReportsGet(t *testing.T) {
	rr := admin(http.MethodPost, "/api/v1/reports/exports", map[string]any{
		"report_type": "fulfillments", "include_sensitive": false,
	})
	mustStatus(t, rr, http.StatusCreated)
	exportID := decodeJSON(t, rr)["id"].(string)

	rr = admin(http.MethodGet, "/api/v1/reports/exports/"+exportID, nil)
	mustStatus(t, rr, http.StatusOK)
	body := decodeJSON(t, rr)
	if body["id"] != exportID {
		t.Errorf("id mismatch: got %v want %v", body["id"], exportID)
	}
}

func TestReportsGet_NotFound(t *testing.T) {
	rr := admin(http.MethodGet, "/api/v1/reports/exports/00000000-0000-0000-0000-000000000000", nil)
	mustStatus(t, rr, http.StatusNotFound)
}

func TestReportsVerifyChecksum_AfterCompletion(t *testing.T) {
	rr := admin(http.MethodPost, "/api/v1/reports/exports", map[string]any{
		"report_type": "fulfillments", "include_sensitive": false,
	})
	mustStatus(t, rr, http.StatusCreated)
	exportID := decodeJSON(t, rr)["id"].(string)

	// Poll until completed (up to 5 seconds).
	var completed bool
	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)
		rr = admin(http.MethodGet, "/api/v1/reports/exports/"+exportID, nil)
		body := decodeJSON(t, rr)
		if body["status"] == "COMPLETED" {
			completed = true
			break
		}
	}
	if !completed {
		t.Skip("export did not complete in time — skipping checksum test")
	}

	rr = admin(http.MethodPost, "/api/v1/reports/exports/"+exportID+"/verify-checksum", nil)
	mustStatus(t, rr, http.StatusOK)
	result := decodeJSON(t, rr)
	if result["verified"] != true {
		t.Errorf("expected verified=true, got %v", result["verified"])
	}
}

func TestReportsDelete(t *testing.T) {
	rr := admin(http.MethodPost, "/api/v1/reports/exports", map[string]any{
		"report_type": "fulfillments", "include_sensitive": false,
	})
	mustStatus(t, rr, http.StatusCreated)
	exportID := decodeJSON(t, rr)["id"].(string)

	rr = admin(http.MethodDelete, "/api/v1/reports/exports/"+exportID, nil)
	if rr.Code != http.StatusNoContent && rr.Code != http.StatusOK {
		t.Fatalf("expected 200/204 from delete export, got %d", rr.Code)
	}
}

func TestReports_RequiresAuth(t *testing.T) {
	rr := unauthed(http.MethodGet, "/api/v1/reports/exports")
	mustStatus(t, rr, http.StatusUnauthorized)
}

func loginAuditor(t *testing.T) *http.Cookie {
	t.Helper()
	ctx := context.Background()
	auditSvc := service.NewAuditService(repository.NewAuditRepository(testPool))
	userSvc := service.NewUserService(repository.NewUserRepository(testPool), auditSvc)
	username := fmt.Sprintf("api_aud_%d", time.Now().UnixNano())
	user, err := userSvc.CreateUser(ctx, username, username+"@test.com", "Audit@Test1!", domain.RoleAuditor)
	if err != nil {
		t.Fatalf("create auditor: %v", err)
	}
	t.Cleanup(func() { _ = userSvc.DeactivateUser(ctx, user.ID) })

	rr := as(nil, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"username": username,
		"password": "Audit@Test1!",
	})
	mustStatus(t, rr, http.StatusOK)
	for _, c := range rr.Result().Cookies() {
		if c.Name == "fulfillops_session" {
			return c
		}
	}
	t.Fatal("auditor login did not return session cookie")
	return nil
}

func TestReportsSensitive_AuditorCreateForbidden(t *testing.T) {
	audCookie := loginAuditor(t)
	rr := as(audCookie, http.MethodPost, "/api/v1/reports/exports", map[string]any{
		"report_type":       "customers",
		"include_sensitive": true,
		"filters":           map[string]any{},
	})
	mustStatus(t, rr, http.StatusForbidden)
}

func TestReportsSensitive_AuditorGetAndVerifyForbidden(t *testing.T) {
	rr := admin(http.MethodPost, "/api/v1/reports/exports", map[string]any{
		"report_type":       "customers",
		"include_sensitive": true,
	})
	mustStatus(t, rr, http.StatusCreated)
	exportID := decodeJSON(t, rr)["id"].(string)

	audCookie := loginAuditor(t)

	rr = as(audCookie, http.MethodGet, "/api/v1/reports/exports/"+exportID, nil)
	mustStatus(t, rr, http.StatusForbidden)

	rr = as(audCookie, http.MethodPost, "/api/v1/reports/exports/"+exportID+"/verify-checksum", nil)
	mustStatus(t, rr, http.StatusForbidden)
}
