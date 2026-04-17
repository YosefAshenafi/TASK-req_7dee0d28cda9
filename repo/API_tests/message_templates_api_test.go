package api_tests

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// createTemplate is a shared helper that seeds a message template and returns
// the full decoded response (including id and version).
func createTemplate(t *testing.T) map[string]any {
	t.Helper()
	rr := admin(http.MethodPost, "/api/v1/message-templates", map[string]any{
		"name":          fmt.Sprintf("tmpl-%d", time.Now().UnixNano()),
		"category":      "BOOKING_RESULT",
		"channel":       "SMS",
		"body_template": "Hi {{customer}}, your order {{id}} is ready.",
	})
	mustStatus(t, rr, http.StatusCreated)
	return decodeJSON(t, rr)
}

// ── GET /api/v1/message-templates ────────────────────────────────────────────

func TestMessageTemplatesList(t *testing.T) {
	// Seed at least one template so the list is non-trivially exercised.
	tmpl := createTemplate(t)
	seededID := tmpl["id"].(string)

	rr := admin(http.MethodGet, "/api/v1/message-templates", nil)
	mustStatus(t, rr, http.StatusOK)

	body := decodeJSON(t, rr)
	items, ok := body["items"].([]any)
	if !ok {
		t.Fatal("response missing 'items' array")
	}

	found := false
	for _, raw := range items {
		row, _ := raw.(map[string]any)
		if row["id"] == seededID {
			found = true
			if row["name"] == nil {
				t.Error("template 'name' missing from list item")
			}
			if row["channel"] == nil {
				t.Error("template 'channel' missing from list item")
			}
		}
	}
	if !found {
		t.Errorf("seeded template %s not found in list", seededID)
	}
}

// ── GET /api/v1/message-templates/:id ────────────────────────────────────────

func TestMessageTemplatesGet(t *testing.T) {
	tmpl := createTemplate(t)
	id := tmpl["id"].(string)

	rr := admin(http.MethodGet, "/api/v1/message-templates/"+id, nil)
	mustStatus(t, rr, http.StatusOK)

	body := decodeJSON(t, rr)
	if body["id"] != id {
		t.Errorf("id mismatch: got %v want %v", body["id"], id)
	}
	if body["channel"] != "SMS" {
		t.Errorf("channel mismatch: got %v want SMS", body["channel"])
	}
	if body["category"] != "BOOKING_RESULT" {
		t.Errorf("category mismatch: got %v", body["category"])
	}
}

func TestMessageTemplatesGet_NotFound(t *testing.T) {
	rr := admin(http.MethodGet, "/api/v1/message-templates/00000000-0000-0000-0000-000000000000", nil)
	mustStatus(t, rr, http.StatusNotFound)
}

// ── PUT /api/v1/message-templates/:id ────────────────────────────────────────

func TestMessageTemplatesUpdate(t *testing.T) {
	tmpl := createTemplate(t)
	id := tmpl["id"].(string)
	version := int(tmpl["version"].(float64))

	updated := fmt.Sprintf("updated-tmpl-%d", time.Now().UnixNano())
	rr := admin(http.MethodPut, "/api/v1/message-templates/"+id, map[string]any{
		"name":          updated,
		"category":      "BOOKING_RESULT",
		"channel":       "SMS",
		"body_template": "Updated: {{customer}}",
		"version":       version,
	})
	mustStatus(t, rr, http.StatusOK)

	body := decodeJSON(t, rr)
	if body["name"] != updated {
		t.Errorf("name not updated: got %v want %v", body["name"], updated)
	}
	// Version must increment after a successful update.
	newVersion := int(body["version"].(float64))
	if newVersion <= version {
		t.Errorf("version must increment after update: old=%d new=%d", version, newVersion)
	}
}

func TestMessageTemplatesUpdate_VersionConflict(t *testing.T) {
	tmpl := createTemplate(t)
	id := tmpl["id"].(string)
	version := int(tmpl["version"].(float64))

	payload := map[string]any{
		"name":          "first-update",
		"category":      "BOOKING_RESULT",
		"channel":       "SMS",
		"body_template": "v2 body",
		"version":       version,
	}
	rr := admin(http.MethodPut, "/api/v1/message-templates/"+id, payload)
	mustStatus(t, rr, http.StatusOK)

	// Re-submit with the now-stale version — must conflict.
	rr = admin(http.MethodPut, "/api/v1/message-templates/"+id, payload)
	mustStatus(t, rr, http.StatusConflict)
}

// ── DELETE /api/v1/message-templates/:id ─────────────────────────────────────

func TestMessageTemplatesDelete(t *testing.T) {
	tmpl := createTemplate(t)
	id := tmpl["id"].(string)

	rr := admin(http.MethodDelete, "/api/v1/message-templates/"+id, nil)
	mustStatus(t, rr, http.StatusNoContent)

	// A soft-deleted template should no longer be visible in the default list.
	rr = admin(http.MethodGet, "/api/v1/message-templates", nil)
	mustStatus(t, rr, http.StatusOK)
	body := decodeJSON(t, rr)
	for _, raw := range body["items"].([]any) {
		row, _ := raw.(map[string]any)
		if row["id"] == id {
			t.Errorf("soft-deleted template %s should not appear in default list", id)
		}
	}
}

// ── RBAC: auditor must not reach message-template endpoints ──────────────────

func TestMessageTemplates_AuditorForbidden(t *testing.T) {
	audCookie := loginAuditor(t)

	rr := as(audCookie, http.MethodGet, "/api/v1/message-templates", nil)
	mustStatus(t, rr, http.StatusForbidden)
}

func TestMessageTemplates_SpecialistCanRead(t *testing.T) {
	// Fulfillment specialists have read-only access to message templates.
	tmpl := createTemplate(t)
	id := tmpl["id"].(string)

	specCookie := loginSpecialist(t)

	rr := as(specCookie, http.MethodGet, "/api/v1/message-templates", nil)
	mustStatus(t, rr, http.StatusOK)

	rr = as(specCookie, http.MethodGet, "/api/v1/message-templates/"+id, nil)
	mustStatus(t, rr, http.StatusOK)
}

func TestMessageTemplates_SpecialistCannotWrite(t *testing.T) {
	tmpl := createTemplate(t)
	id := tmpl["id"].(string)
	version := int(tmpl["version"].(float64))

	specCookie := loginSpecialist(t)

	// PUT is admin-only.
	rr := as(specCookie, http.MethodPut, "/api/v1/message-templates/"+id, map[string]any{
		"name": "specialist-attempt", "category": "BOOKING_RESULT",
		"channel": "SMS", "body_template": "attempt", "version": version,
	})
	mustStatus(t, rr, http.StatusForbidden)

	// DELETE is admin-only.
	rr = as(specCookie, http.MethodDelete, "/api/v1/message-templates/"+id, nil)
	mustStatus(t, rr, http.StatusForbidden)
}
