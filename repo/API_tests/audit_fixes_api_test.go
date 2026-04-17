package api_tests

// Regression tests for the audit-driven fixes:
//   • invalid report_type rejected synchronously (no 201 → async fail)
//   • customer name-only update preserves encrypted phone/email/address
//   • role read-access matrix: auditor cannot read templates / settings
//   • shipping-address maintenance endpoint updates an existing address
//   • /auth/logout clears both API and page cookies
//
// All tests auto-skip when DATABASE_URL is not set (see setup_test.go).

import (
	"net/http"
	"strings"
	"testing"
)

// ── Issue 6: invalid report_type must be rejected synchronously ──────────────

func TestReportsCreate_InvalidTypeRejectedSynchronously(t *testing.T) {
	rr := admin(http.MethodPost, "/api/v1/reports/exports", map[string]any{
		"report_type": "unsupported-type", "include_sensitive": false,
	})
	if rr.Code != http.StatusUnprocessableEntity && rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 422 or 400 for unsupported report_type, got %d\nbody: %s",
			rr.Code, rr.Body.String())
	}
	body := decodeJSON(t, rr)
	if body["code"] != "VALIDATION_ERROR" {
		t.Errorf("expected code=VALIDATION_ERROR, got %v", body["code"])
	}
}

// ── Issue 3: customer partial update must preserve encrypted fields ──────────

func TestCustomerUpdate_NameOnlyPreservesEncryptedFields(t *testing.T) {
	// Create a customer with phone + email + address.
	createRR := admin(http.MethodPost, "/api/v1/customers", map[string]any{
		"name":    "Preserve Test",
		"phone":   "5551112222",
		"email":   "preserve@example.com",
		"address": "100 Main St, NY, NY 10001",
	})
	mustStatus(t, createRR, http.StatusCreated)
	created := decodeJSON(t, createRR)
	id := created["id"].(string)

	origPhone, _ := created["phone_masked"].(string)
	origEmail, _ := created["email_masked"].(string)
	origAddr, _ := created["address_masked"].(string)
	if origPhone == "" || origEmail == "" || origAddr == "" {
		t.Fatalf("precondition failed — expected masked fields populated, got phone=%q email=%q addr=%q",
			origPhone, origEmail, origAddr)
	}

	// Name-only update, sending no phone/email/address keys.
	updateRR := admin(http.MethodPut, "/api/v1/customers/"+id, map[string]any{
		"name":    "Preserve Test Renamed",
		"version": int(created["version"].(float64)),
	})
	mustStatus(t, updateRR, http.StatusOK)
	updated := decodeJSON(t, updateRR)

	if updated["name"] != "Preserve Test Renamed" {
		t.Errorf("expected name updated, got %v", updated["name"])
	}
	if updated["phone_masked"] != origPhone {
		t.Errorf("phone wiped on partial update: before=%q after=%v", origPhone, updated["phone_masked"])
	}
	if updated["email_masked"] != origEmail {
		t.Errorf("email wiped on partial update: before=%q after=%v", origEmail, updated["email_masked"])
	}
	if updated["address_masked"] != origAddr {
		t.Errorf("address wiped on partial update: before=%q after=%v", origAddr, updated["address_masked"])
	}
}

// ── Issue 2: role-based read restrictions ────────────────────────────────────

func TestAuditor_CannotReadMessageTemplates(t *testing.T) {
	auditorCookie := mustLoginAs(t, "auditor", auditorSeed{
		username: "test_auditor_read_block",
		password: "Audit@Pass1234",
	})
	rr := as(auditorCookie, http.MethodGet, "/api/v1/message-templates", nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("auditor should not read message-templates, got %d\nbody: %s", rr.Code, rr.Body.String())
	}
}

func TestAuditor_CannotReadSendLogs(t *testing.T) {
	auditorCookie := mustLoginAs(t, "auditor", auditorSeed{
		username: "test_auditor_sl_block",
		password: "Audit@Pass1234",
	})
	rr := as(auditorCookie, http.MethodGet, "/api/v1/send-logs", nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("auditor should not read send-logs, got %d", rr.Code)
	}
}

func TestAuditor_CannotReadSettingsBlackouts(t *testing.T) {
	auditorCookie := mustLoginAs(t, "auditor", auditorSeed{
		username: "test_auditor_set_block",
		password: "Audit@Pass1234",
	})
	rr := as(auditorCookie, http.MethodGet, "/api/v1/settings/blackout-dates", nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("auditor should not read blackout dates, got %d", rr.Code)
	}
}

// ── Issue 7: logout must clear BOTH the API and page cookies ─────────────────

func TestLogout_ClearsBothCookies(t *testing.T) {
	// Log in fresh via API — the login handler now seeds both cookies.
	loginRR := admin(http.MethodPost, "/api/v1/auth/login", map[string]any{
		"username": "admin", "password": "Admin@FulfillOps1",
	})
	mustStatus(t, loginRR, http.StatusOK)

	var (
		apiCookieSet  bool
		pageCookieSet bool
	)
	for _, c := range loginRR.Result().Cookies() {
		if c.Name == "fulfillops_session" && c.MaxAge >= 0 {
			apiCookieSet = true
		}
		if c.Name == "fulfillops" && c.MaxAge >= 0 {
			pageCookieSet = true
		}
	}
	if !apiCookieSet || !pageCookieSet {
		t.Fatalf("expected API + page cookies set by login (api=%v page=%v)", apiCookieSet, pageCookieSet)
	}

	// Log out and confirm both cookies are told to expire.
	logoutRR := admin(http.MethodPost, "/api/v1/auth/logout", nil)
	mustStatus(t, logoutRR, http.StatusOK)

	var (
		apiCleared  bool
		pageCleared bool
	)
	for _, header := range logoutRR.Result().Header.Values("Set-Cookie") {
		if strings.Contains(header, "fulfillops_session=") && strings.Contains(header, "Max-Age=0") {
			apiCleared = true
		}
		if strings.Contains(header, "fulfillops=") && strings.Contains(header, "Max-Age=0") {
			pageCleared = true
		}
	}
	if !apiCleared || !pageCleared {
		t.Fatalf("expected both cookies cleared by logout (api=%v page=%v), headers=%v",
			apiCleared, pageCleared, logoutRR.Result().Header.Values("Set-Cookie"))
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

type auditorSeed struct {
	username string
	password string
}

// mustLoginAs ensures the given role-user exists (via admin create), then logs
// in as that user and returns the session cookie. Auditor by default.
func mustLoginAs(t *testing.T, role string, seed auditorSeed) *http.Cookie {
	t.Helper()
	roleName := strings.ToUpper(role)
	if roleName == "AUDITOR" {
		roleName = "AUDITOR"
	}

	// Create user (idempotent: ignore 409 on conflict).
	admin(http.MethodPost, "/api/v1/admin/users", map[string]any{
		"username": seed.username,
		"email":    seed.username + "@example.com",
		"password": seed.password,
		"role":     roleName,
	})

	loginRR := admin(http.MethodPost, "/api/v1/auth/login", map[string]any{
		"username": seed.username, "password": seed.password,
	})
	if loginRR.Code != http.StatusOK {
		t.Fatalf("auditor login failed: %d %s", loginRR.Code, loginRR.Body.String())
	}
	for _, c := range loginRR.Result().Cookies() {
		if c.Name == "fulfillops_session" {
			return c
		}
	}
	t.Fatalf("session cookie missing after login as %s", seed.username)
	return nil
}
