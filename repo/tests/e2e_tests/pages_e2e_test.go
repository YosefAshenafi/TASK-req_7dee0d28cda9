package e2e_tests

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func pageReq(t *testing.T, cookie *http.Cookie, method, path string, form url.Values) *httptest.ResponseRecorder {
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

func mustPageCookie(t *testing.T) *http.Cookie {
	t.Helper()
	rr := pageReq(t, nil, http.MethodPost, "/auth/login", url.Values{
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
	t.Fatal("page cookie not found")
	return nil
}

func TestPageRoutesRender(t *testing.T) {
	pageCookie := mustPageCookie(t)
	suffix := uuid.New().String()[:8]

	tierID := decode(t, do(http.MethodPost, "/api/v1/tiers", map[string]any{
		"name": "e2e-page-tier", "inventory_count": 4, "purchase_limit": 2, "alert_threshold": 1,
	}))["id"].(string)
	customerID := decode(t, do(http.MethodPost, "/api/v1/customers", map[string]any{
		"name": "E2E Page Customer",
	}))["id"].(string)
	fulfillmentID := decode(t, do(http.MethodPost, "/api/v1/fulfillments", map[string]any{
		"tier_id": tierID, "customer_id": customerID, "type": "PHYSICAL",
	}))["id"].(string)
	exceptionID := decode(t, do(http.MethodPost, "/api/v1/exceptions", map[string]any{
		"fulfillment_id": fulfillmentID, "type": "MANUAL", "note": "page coverage",
	}))["id"].(string)
	templateID := decode(t, do(http.MethodPost, "/api/v1/message-templates", map[string]any{
		"name": "E2E Page Template " + suffix, "category": "BOOKING_CHANGE", "channel": "SMS", "body_template": "hello",
	}))["id"].(string)
	userID := decode(t, do(http.MethodPost, "/api/v1/admin/users", map[string]any{
		"username": "page_user_e2e_" + suffix, "email": "page_user_e2e_" + suffix + "@example.com", "password": "Password123", "role": "AUDITOR",
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
		rr := pageReq(t, pageCookie, http.MethodGet, path, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("GET %s => %d %s", path, rr.Code, rr.Body.String())
		}
		if !strings.Contains(rr.Header().Get("Content-Type"), "text/html") {
			t.Fatalf("GET %s should render html, got %q", path, rr.Header().Get("Content-Type"))
		}
	}

	rr := pageReq(t, pageCookie, http.MethodPost, "/auth/logout", nil)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("logout => %d %s", rr.Code, rr.Body.String())
	}
}
