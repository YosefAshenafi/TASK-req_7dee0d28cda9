package api_tests

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// ── GET /api/v1/admin/health ─────────────────────────────────────────────────

func TestAdminHealth(t *testing.T) {
	rr := admin(http.MethodGet, "/api/v1/admin/health", nil)
	mustStatus(t, rr, http.StatusOK)

	body := decodeJSON(t, rr)
	if body["status"] == nil {
		t.Error("health response missing 'status' field")
	}
	checks, ok := body["checks"].(map[string]any)
	if !ok {
		t.Fatal("health response missing 'checks' object")
	}
	for _, key := range []string{"database", "encryption", "dirs", "scheduler"} {
		if checks[key] == nil {
			t.Errorf("health checks missing key %q", key)
		}
	}
	// Database must be reachable when tests are running.
	if checks["database"] != "ok" {
		t.Errorf("expected database=ok, got %v", checks["database"])
	}
}

func TestAdminHealth_SpecialistForbidden(t *testing.T) {
	specCookie := loginSpecialist(t)
	rr := as(specCookie, http.MethodGet, "/api/v1/admin/health", nil)
	mustStatus(t, rr, http.StatusForbidden)
}

func TestAdminHealth_AuditorForbidden(t *testing.T) {
	audCookie := loginAuditor(t)
	rr := as(audCookie, http.MethodGet, "/api/v1/admin/health", nil)
	mustStatus(t, rr, http.StatusForbidden)
}

// ── POST /api/v1/admin/jobs/:name/run ────────────────────────────────────────

func TestAdminTriggerJob(t *testing.T) {
	// The test router is built without a scheduler, so the handler returns 202
	// Accepted with a confirmation payload rather than actually running the job.
	rr := admin(http.MethodPost, "/api/v1/admin/jobs/stats/run", nil)
	mustStatus(t, rr, http.StatusAccepted)

	body := decodeJSON(t, rr)
	if body["job"] != "stats" {
		t.Errorf("expected job=stats, got %v", body["job"])
	}
	if body["message"] == nil {
		t.Error("trigger response missing 'message' field")
	}
}

func TestAdminTriggerJob_ForbiddenForSpecialist(t *testing.T) {
	specCookie := loginSpecialist(t)
	rr := as(specCookie, http.MethodPost, "/api/v1/admin/jobs/stats/run", nil)
	mustStatus(t, rr, http.StatusForbidden)
}

// ── GET /api/v1/admin/job-schedules ──────────────────────────────────────────

func TestAdminJobSchedulesList(t *testing.T) {
	rr := admin(http.MethodGet, "/api/v1/admin/job-schedules", nil)
	mustStatus(t, rr, http.StatusOK)

	body := decodeJSON(t, rr)
	items, ok := body["items"].([]any)
	if !ok {
		t.Fatal("response missing 'items' array")
	}
	// Migration 006 seeds 7 schedules; at minimum the two interval-based ones
	// must always be present.
	if len(items) == 0 {
		t.Error("job-schedules list must not be empty after migration 006 runs")
	}
	// Each item must have job_key and enabled fields.
	for _, raw := range items {
		row, _ := raw.(map[string]any)
		if row["job_key"] == nil {
			t.Errorf("job schedule missing 'job_key': %+v", row)
		}
		if row["enabled"] == nil {
			t.Errorf("job schedule missing 'enabled': %+v", row)
		}
	}
}

// ── PUT /api/v1/admin/job-schedules/:key ─────────────────────────────────────

func TestAdminJobScheduleUpdate(t *testing.T) {
	// List first to get a valid key and current version.
	rr := admin(http.MethodGet, "/api/v1/admin/job-schedules", nil)
	mustStatus(t, rr, http.StatusOK)

	body := decodeJSON(t, rr)
	items := body["items"].([]any)
	if len(items) == 0 {
		t.Skip("no job schedules found — skipping update test")
	}

	// Pick the first seeded interval-based schedule (overdue-check).
	var schedule map[string]any
	for _, raw := range items {
		row, _ := raw.(map[string]any)
		if row["job_key"] == "overdue-check" {
			schedule = row
			break
		}
	}
	if schedule == nil {
		schedule = items[0].(map[string]any)
	}

	jobKey := schedule["job_key"].(string)
	version := int(schedule["version"].(float64))
	intervalSecs := 900

	rr = admin(http.MethodPut, "/api/v1/admin/job-schedules/"+jobKey, map[string]any{
		"interval_seconds": intervalSecs,
		"enabled":          true,
		"version":          version,
	})
	mustStatus(t, rr, http.StatusOK)

	updated := decodeJSON(t, rr)
	if updated["job_key"] != jobKey {
		t.Errorf("job_key mismatch: got %v want %v", updated["job_key"], jobKey)
	}
	if updated["enabled"] != true {
		t.Errorf("expected enabled=true, got %v", updated["enabled"])
	}

	// Restore original version to avoid leaving stale state for other tests.
	newVersion := int(updated["version"].(float64))
	rr = admin(http.MethodPut, "/api/v1/admin/job-schedules/"+jobKey, map[string]any{
		"interval_seconds": intervalSecs,
		"enabled":          true,
		"version":          newVersion,
	})
	mustStatus(t, rr, http.StatusOK)
}

func TestAdminJobScheduleUpdate_VersionConflict(t *testing.T) {
	rr := admin(http.MethodGet, "/api/v1/admin/job-schedules", nil)
	mustStatus(t, rr, http.StatusOK)
	body := decodeJSON(t, rr)
	items := body["items"].([]any)
	if len(items) == 0 {
		t.Skip("no schedules — skipping conflict test")
	}
	schedule := items[0].(map[string]any)
	jobKey := schedule["job_key"].(string)

	// Submit with version 0 — guaranteed to be stale unless the row has never
	// been updated and the DB version column starts at 0.  Using version=-1 is
	// unambiguously stale.
	rr = admin(http.MethodPut, "/api/v1/admin/job-schedules/"+jobKey, map[string]any{
		"interval_seconds": 60,
		"enabled":          true,
		"version":          -1,
	})
	mustStatus(t, rr, http.StatusConflict)
}

// ── GET /api/v1/admin/dr-drills ──────────────────────────────────────────────

func TestAdminDRDrillsList(t *testing.T) {
	rr := admin(http.MethodGet, "/api/v1/admin/dr-drills", nil)
	mustStatus(t, rr, http.StatusOK)

	body := decodeJSON(t, rr)
	if body["items"] == nil {
		t.Error("dr-drills list response missing 'items' field")
	}
	if body["total"] == nil {
		t.Error("dr-drills list response missing 'total' field")
	}
}

// ── POST /api/v1/admin/dr-drills ─────────────────────────────────────────────

func TestAdminDRDrillCreate(t *testing.T) {
	scheduledFor := "2099-06-01"

	rr := admin(http.MethodPost, "/api/v1/admin/dr-drills", map[string]any{
		"scheduled_for": scheduledFor,
		"notes":         "quarterly DR test",
	})
	mustStatus(t, rr, http.StatusCreated)

	body := decodeJSON(t, rr)
	if body["id"] == nil {
		t.Error("dr-drill create response missing 'id'")
	}
	if body["scheduled_for"] == nil {
		t.Error("dr-drill create response missing 'scheduled_for'")
	}
	// Newly created drill should have a PENDING outcome.
	if body["outcome"] != "PENDING" {
		t.Errorf("expected outcome=PENDING, got %v", body["outcome"])
	}
}

func TestAdminDRDrillCreate_MissingScheduledFor(t *testing.T) {
	rr := admin(http.MethodPost, "/api/v1/admin/dr-drills", map[string]any{
		"notes": "missing date",
	})
	if rr.Code == http.StatusCreated {
		t.Fatal("expected error for missing scheduled_for, got 201")
	}
}

// ── PUT /api/v1/admin/dr-drills/:id ──────────────────────────────────────────

func TestAdminDRDrillUpdate(t *testing.T) {
	// Create a drill first.
	rr := admin(http.MethodPost, "/api/v1/admin/dr-drills", map[string]any{
		"scheduled_for": fmt.Sprintf("2099-%02d-15", (time.Now().Month()%12)+1),
	})
	mustStatus(t, rr, http.StatusCreated)
	drill := decodeJSON(t, rr)
	drillID := drill["id"].(string)

	// Record its outcome.
	rr = admin(http.MethodPut, "/api/v1/admin/dr-drills/"+drillID, map[string]any{
		"outcome": "PASS",
		"notes":   "all systems restored in < 4h",
	})
	mustStatus(t, rr, http.StatusOK)

	updated := decodeJSON(t, rr)
	if updated["outcome"] != "PASS" {
		t.Errorf("expected outcome=PASS, got %v", updated["outcome"])
	}
	if updated["executed_at"] == nil {
		t.Error("executed_at must be stamped when outcome is recorded")
	}
}

func TestAdminDRDrillUpdate_InvalidOutcome(t *testing.T) {
	rr := admin(http.MethodPost, "/api/v1/admin/dr-drills", map[string]any{
		"scheduled_for": "2099-07-20",
	})
	mustStatus(t, rr, http.StatusCreated)
	drillID := decodeJSON(t, rr)["id"].(string)

	// "UNKNOWN" is not in the DB CHECK constraint (PASS | FAIL | PENDING).
	rr = admin(http.MethodPut, "/api/v1/admin/dr-drills/"+drillID, map[string]any{
		"outcome": "UNKNOWN",
	})
	if rr.Code == http.StatusOK {
		t.Fatal("expected error for invalid outcome UNKNOWN, got 200")
	}
}

func TestAdminDRDrillUpdate_NotFound(t *testing.T) {
	rr := admin(http.MethodPut, "/api/v1/admin/dr-drills/00000000-0000-0000-0000-000000000000", map[string]any{
		"outcome": "PASS",
	})
	mustStatus(t, rr, http.StatusNotFound)
}
