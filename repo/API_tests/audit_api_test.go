package api_tests

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestAuditList(t *testing.T) {
	// Generate an audit entry so the list is non-trivially exercised.
	tierName := fmt.Sprintf("audit-tier-%d", time.Now().UnixNano())
	rr := admin(http.MethodPost, "/api/v1/tiers", map[string]any{
		"name": tierName, "inventory_count": 1, "purchase_limit": 1, "alert_threshold": 0,
	})
	mustStatus(t, rr, http.StatusCreated)

	rr = admin(http.MethodGet, "/api/v1/audit", nil)
	mustStatus(t, rr, http.StatusOK)

	body := decodeJSON(t, rr)
	if body["total"] == nil {
		t.Error("audit list missing 'total' field")
	}
	if body["page"] == nil {
		t.Error("audit list missing 'page' field")
	}
	items, ok := body["items"].([]any)
	if !ok {
		t.Fatal("audit list missing 'items' array")
	}
	if len(items) == 0 {
		t.Error("audit list must have at least one entry after tier creation")
	}
	for _, raw := range items {
		row, _ := raw.(map[string]any)
		if row["table_name"] == nil {
			t.Error("audit entry missing 'table_name'")
		}
		if row["operation"] == nil {
			t.Error("audit entry missing 'operation'")
		}
		break // one item is sufficient to verify shape
	}
}

func TestAuditList_RequiresAuth(t *testing.T) {
	rr := unauthed(http.MethodGet, "/api/v1/audit")
	mustStatus(t, rr, http.StatusUnauthorized)
}

func TestAuditList_ContainsEntries(t *testing.T) {
	admin(http.MethodPost, "/api/v1/tiers", map[string]any{
		"name": "audit-trigger-tier", "inventory_count": 1, "purchase_limit": 1, "alert_threshold": 0,
	})

	rr := admin(http.MethodGet, "/api/v1/audit", nil)
	mustStatus(t, rr, http.StatusOK)
	if rr.Body.Len() == 0 {
		t.Error("audit list response body is empty")
	}
}

// TestAuditList_SpecialistForbidden verifies that a properly authenticated
// fulfillment-specialist session is denied access to the audit log (403).
func TestAuditList_SpecialistForbidden(t *testing.T) {
	specCookie := loginSpecialist(t)
	rr := as(specCookie, http.MethodGet, "/api/v1/audit", nil)
	mustStatus(t, rr, http.StatusForbidden)
}
