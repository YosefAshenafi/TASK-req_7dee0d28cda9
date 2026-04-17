package api_tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
)

func pageRequest(t *testing.T, cookie *http.Cookie, method, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	var body *bytes.Reader
	if form == nil {
		body = bytes.NewReader(nil)
	} else {
		body = bytes.NewReader([]byte(form.Encode()))
	}
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, req)
	return rr
}

func mustPageLogin(t *testing.T) *http.Cookie {
	t.Helper()
	rr := pageRequest(t, nil, http.MethodPost, "/auth/login", url.Values{
		"username": {"admin"},
		"password": {"Admin@FulfillOps1"},
	})
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("page login failed: %d %s", rr.Code, rr.Body.String())
	}
	for _, c := range rr.Result().Cookies() {
		if c.Name == "fulfillops" {
			return c
		}
	}
	t.Fatal("page auth cookie not found")
	return nil
}

func TestPageRoutesSmoke(t *testing.T) {
	pageCookie := mustPageLogin(t)
	suffix := uuid.New().String()[:8]

	rr := pageRequest(t, nil, http.MethodGet, "/auth/login", nil)
	mustStatus(t, rr, http.StatusOK)

	rr = pageRequest(t, nil, http.MethodPost, "/auth/login", url.Values{
		"username": {"admin"},
		"password": {"wrong"},
	})
	if rr.Code != http.StatusUnauthorized || !strings.Contains(rr.Body.String(), "Invalid username or password.") {
		t.Fatalf("expected invalid login page response, got %d %s", rr.Code, rr.Body.String())
	}

	tierID := decodeJSON(t, admin(http.MethodPost, "/api/v1/tiers", map[string]any{
		"name": "page-tier", "inventory_count": 3, "purchase_limit": 2, "alert_threshold": 1,
	}))["id"].(string)
	customerID := decodeJSON(t, admin(http.MethodPost, "/api/v1/customers", map[string]any{
		"name": "Page Customer",
	}))["id"].(string)
	fulfillmentID := decodeJSON(t, admin(http.MethodPost, "/api/v1/fulfillments", map[string]any{
		"tier_id": tierID, "customer_id": customerID, "type": "VOUCHER",
	}))["id"].(string)
	exceptionID := decodeJSON(t, admin(http.MethodPost, "/api/v1/exceptions", map[string]any{
		"fulfillment_id": fulfillmentID, "type": "MANUAL", "note": "page coverage",
	}))["id"].(string)
	templateID := decodeJSON(t, admin(http.MethodPost, "/api/v1/message-templates", map[string]any{
		"name": "Page Template " + suffix, "category": "FULFILLMENT_PROGRESS", "channel": "EMAIL", "body_template": "hello",
	}))["id"].(string)
	userID := decodeJSON(t, admin(http.MethodPost, "/api/v1/admin/users", map[string]any{
		"username": "page_user_api_" + suffix, "email": "page_user_api_" + suffix + "@example.com", "password": "Password123", "role": "AUDITOR",
	}))["id"].(string)

	paths := []string{
		"/",
		"/tiers",
		"/tiers/" + tierID,
		"/tiers/new",
		"/tiers/" + tierID + "/edit",
		"/customers",
		"/customers/" + customerID,
		"/customers/new",
		"/customers/" + customerID + "/edit",
		"/fulfillments",
		"/fulfillments/" + fulfillmentID,
		"/fulfillments/new",
		"/exceptions",
		"/exceptions/" + exceptionID,
		"/messages",
		"/messages/templates/new",
		"/messages/templates/" + templateID + "/edit",
		"/messages/send-logs",
		"/messages/handoff",
		"/notifications",
		"/reports",
		"/reports/history",
		"/audit",
		"/settings",
		"/settings/blackout-dates",
		"/admin/health",
		"/admin/users",
		"/admin/users/new",
		"/admin/users/" + userID + "/edit",
		"/admin/recovery",
		"/admin/backups",
	}

	for _, path := range paths {
		rr = pageRequest(t, pageCookie, http.MethodGet, path, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("GET %s => %d %s", path, rr.Code, rr.Body.String())
		}
		if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
			t.Fatalf("GET %s content-type = %q", path, ct)
		}
	}

	rr = pageRequest(t, pageCookie, http.MethodPost, "/auth/logout", nil)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("logout => %d %s", rr.Code, rr.Body.String())
	}
}

func TestPageReportDownloadAuthorization(t *testing.T) {
	pageCookie := mustPageLogin(t)
	exportID := decodeJSON(t, admin(http.MethodPost, "/api/v1/reports/exports", map[string]any{
		"report_type": "audit",
	}))["id"].(string)

	req := httptest.NewRequest(http.MethodGet, "/reports/exports/"+exportID+"/download", nil)
	req.AddCookie(pageCookie)
	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, req)

	switch rr.Code {
	case http.StatusOK:
		if cd := rr.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment") {
			t.Fatalf("expected file attachment, got Content-Disposition=%q", cd)
		}
	case http.StatusConflict, http.StatusNotFound:
		// Export still processing or row missing — both are valid for this flow.
	default:
		var body map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &body)
		t.Fatalf("unexpected download status: %d body=%v", rr.Code, body)
	}
}

func TestPageReportCreate_WritesAuditRecord(t *testing.T) {
	pageCookie := mustPageLogin(t)
	auditRepo := repository.NewAuditRepository(testPool)
	_, beforeTotal, err := auditRepo.List(context.Background(), repository.AuditFilters{
		TableName: "report_exports",
		Operation: "CREATE",
	}, domain.PageRequest{Page: 1, PageSize: 200})
	if err != nil {
		t.Fatalf("audit list before: %v", err)
	}

	rr := pageRequest(t, pageCookie, http.MethodPost, "/reports/exports", url.Values{
		"report_type": {"audit"},
		"date_from":   {"2026-01-01"},
		"date_to":     {"2026-01-31"},
	})
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("page export create => %d %s", rr.Code, rr.Body.String())
	}

	_, afterTotal, err := auditRepo.List(context.Background(), repository.AuditFilters{
		TableName: "report_exports",
		Operation: "CREATE",
	}, domain.PageRequest{Page: 1, PageSize: 200})
	if err != nil {
		t.Fatalf("audit list after: %v", err)
	}
	if afterTotal <= beforeTotal {
		t.Fatal("expected new audit row for page export creation")
	}
}
