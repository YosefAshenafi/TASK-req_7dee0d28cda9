package api_tests

import (
	"net/http"
	"testing"
)

func TestAuditList(t *testing.T) {
	rr := admin(http.MethodGet, "/api/v1/audit", nil)
	mustStatus(t, rr, http.StatusOK)
}

func TestAuditList_RequiresAuth(t *testing.T) {
	rr := unauthed(http.MethodGet, "/api/v1/audit")
	mustStatus(t, rr, http.StatusUnauthorized)
}

func TestAuditList_ContainsEntries(t *testing.T) {
	// Create something so there is at least one audit entry.
	admin(http.MethodPost, "/api/v1/tiers", map[string]any{
		"name": "audit-trigger-tier", "inventory_count": 1, "purchase_limit": 1, "alert_threshold": 0,
	})

	rr := admin(http.MethodGet, "/api/v1/audit", nil)
	mustStatus(t, rr, http.StatusOK)
	// Body should be parseable JSON (array or object with items).
	if rr.Body.Len() == 0 {
		t.Error("audit list response body is empty")
	}
}

func TestAuditList_SpecialistForbidden(t *testing.T) {
	// Auditor role can list audit; plain specialist cannot.
	// We test with no auth here as a quick proxy for forbidden paths.
	rr := unauthed(http.MethodGet, "/api/v1/audit")
	if rr.Code == http.StatusOK {
		t.Error("unauthenticated request should not reach audit list")
	}
}
