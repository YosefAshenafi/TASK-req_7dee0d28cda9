package api_tests

import (
	"net/http"
	"testing"
)

// ── PUT /api/v1/settings/:key ─────────────────────────────────────────────────

func TestSettingsSet(t *testing.T) {
	// Update a known setting seeded by migration 002.
	rr := admin(http.MethodPut, "/api/v1/settings/timezone", map[string]any{
		"value": "America/Chicago",
	})
	mustStatus(t, rr, http.StatusOK)

	body := decodeJSON(t, rr)
	if body["key"] != "timezone" {
		t.Errorf("key mismatch: got %v want timezone", body["key"])
	}
	// The returned value is JSON-encoded by the handler; verify the id/key fields.
	if body["id"] == nil {
		t.Error("response missing 'id' field")
	}

	// Restore the original value so tests don't interfere with each other.
	rr = admin(http.MethodPut, "/api/v1/settings/timezone", map[string]any{
		"value": "America/New_York",
	})
	mustStatus(t, rr, http.StatusOK)
}

func TestSettingsSet_SpecialistForbidden(t *testing.T) {
	specCookie := loginSpecialist(t)
	rr := as(specCookie, http.MethodPut, "/api/v1/settings/timezone", map[string]any{
		"value": "Europe/London",
	})
	mustStatus(t, rr, http.StatusForbidden)
}

func TestSettingsSet_AuditorForbidden(t *testing.T) {
	audCookie := loginAuditor(t)
	rr := as(audCookie, http.MethodPut, "/api/v1/settings/timezone", map[string]any{
		"value": "Europe/London",
	})
	mustStatus(t, rr, http.StatusForbidden)
}

func TestSettingsSet_MissingValue(t *testing.T) {
	rr := admin(http.MethodPut, "/api/v1/settings/timezone", map[string]any{})
	if rr.Code == http.StatusOK {
		t.Fatal("expected error for missing value, got 200")
	}
}

// ── GET /api/v1/settings/blackout-dates ──────────────────────────────────────

func TestSettingsBlackoutDatesList(t *testing.T) {
	// Seed a blackout date first so the list is non-trivially exercised.
	rr := admin(http.MethodPost, "/api/v1/settings/blackout-dates", map[string]any{
		"date":        "2099-01-15",
		"description": "test blackout",
	})
	mustStatus(t, rr, http.StatusCreated)
	created := decodeJSON(t, rr)
	createdID := created["id"].(string)

	rr = admin(http.MethodGet, "/api/v1/settings/blackout-dates", nil)
	mustStatus(t, rr, http.StatusOK)

	body := decodeJSON(t, rr)
	items, ok := body["items"].([]any)
	if !ok {
		t.Fatal("response missing 'items' array")
	}

	found := false
	for _, raw := range items {
		row, _ := raw.(map[string]any)
		if row["id"] == createdID {
			found = true
			if row["date"] == nil {
				t.Error("blackout date 'date' missing from list item")
			}
		}
	}
	if !found {
		t.Errorf("seeded blackout date %s not found in list", createdID)
	}

	// Cleanup: delete the seeded date.
	rr = admin(http.MethodDelete, "/api/v1/settings/blackout-dates/"+createdID, nil)
	mustStatus(t, rr, http.StatusNoContent)
}

func TestSettingsBlackoutDatesList_SpecialistAllowed(t *testing.T) {
	specCookie := loginSpecialist(t)
	rr := as(specCookie, http.MethodGet, "/api/v1/settings/blackout-dates", nil)
	mustStatus(t, rr, http.StatusOK)
}

func TestSettingsBlackoutDatesList_AuditorForbidden(t *testing.T) {
	audCookie := loginAuditor(t)
	rr := as(audCookie, http.MethodGet, "/api/v1/settings/blackout-dates", nil)
	mustStatus(t, rr, http.StatusForbidden)
}
