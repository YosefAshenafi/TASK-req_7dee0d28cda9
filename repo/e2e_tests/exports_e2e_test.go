package e2e_tests

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// ── Report Export Workflow ────────────────────────────────────────────────────

func TestExportWorkflow_CreateAndVerifyChecksum(t *testing.T) {
	rr := do(http.MethodPost, "/api/v1/reports/exports", map[string]any{
		"report_type": "fulfillments", "include_sensitive": false,
	})
	mustStatus(t, rr, http.StatusCreated)
	exp := decode(t, rr)
	exportID := exp["id"].(string)

	// Poll until COMPLETED.
	var completed bool
	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)
		rr = do(http.MethodGet, "/api/v1/reports/exports/"+exportID, nil)
		if decode(t, rr)["status"] == "COMPLETED" {
			completed = true
			break
		}
	}
	if !completed {
		t.Fatal("export did not reach COMPLETED within timeout")
	}

	// Checksum must be present.
	rr = do(http.MethodGet, "/api/v1/reports/exports/"+exportID, nil)
	mustStatus(t, rr, http.StatusOK)
	exp = decode(t, rr)
	if exp["checksum_sha256"] == nil {
		t.Fatal("completed export is missing checksum_sha256")
	}

	// Verify checksum → verified=true.
	rr = do(http.MethodPost, "/api/v1/reports/exports/"+exportID+"/verify-checksum", nil)
	mustStatus(t, rr, http.StatusOK)
	result := decode(t, rr)
	if result["verified"] != true {
		t.Fatalf("expected verified=true, got %v", result["verified"])
	}
}

func TestExportWorkflow_MultipleExportsAreListed(t *testing.T) {
	// Create two exports.
	for i := 0; i < 2; i++ {
		rr := do(http.MethodPost, "/api/v1/reports/exports", map[string]any{
			"report_type": "fulfillments", "include_sensitive": false,
		})
		mustStatus(t, rr, http.StatusCreated)
	}

	rr := do(http.MethodGet, "/api/v1/reports/exports", nil)
	mustStatus(t, rr, http.StatusOK)
	if rr.Body.Len() == 0 {
		t.Error("exports list response body should not be empty")
	}
}

// ── Exception E2E Workflow ────────────────────────────────────────────────────

func TestExceptionWorkflow(t *testing.T) {
	rr := do(http.MethodPost, "/api/v1/tiers", map[string]any{
		"name": fmt.Sprintf("e2e-exc-%d", time.Now().UnixNano()),
		"inventory_count": 5, "purchase_limit": 2, "alert_threshold": 1,
	})
	mustStatus(t, rr, http.StatusCreated)
	tierID := decode(t, rr)["id"].(string)

	rr = do(http.MethodPost, "/api/v1/customers", map[string]any{"name": "Exception E2E"})
	mustStatus(t, rr, http.StatusCreated)
	custID := decode(t, rr)["id"].(string)

	rr = do(http.MethodPost, "/api/v1/fulfillments", map[string]any{
		"tier_id": tierID, "customer_id": custID, "type": "VOUCHER",
	})
	mustStatus(t, rr, http.StatusCreated)
	ffID := decode(t, rr)["id"].(string)

	// 1. Create exception.
	rr = do(http.MethodPost, "/api/v1/exceptions", map[string]any{
		"fulfillment_id": ffID, "type": "MANUAL",
	})
	mustStatus(t, rr, http.StatusCreated)
	exc := decode(t, rr)
	excID := exc["id"].(string)
	if exc["status"] != "OPEN" {
		t.Errorf("expected OPEN, got %v", exc["status"])
	}

	// 2. Add a NOTE event.
	rr = do(http.MethodPost, "/api/v1/exceptions/"+excID+"/events", map[string]any{
		"event_type": "NOTE", "content": "Investigating the delay",
	})
	mustStatus(t, rr, http.StatusCreated)

	// 3. Move to INVESTIGATING.
	rr = do(http.MethodPut, "/api/v1/exceptions/"+excID+"/status", map[string]any{
		"status": "INVESTIGATING",
	})
	mustStatus(t, rr, http.StatusOK)
	if decode(t, rr)["status"] != "INVESTIGATING" {
		t.Fatal("expected INVESTIGATING status")
	}

	// 4. Escalate.
	rr = do(http.MethodPut, "/api/v1/exceptions/"+excID+"/status", map[string]any{
		"status": "ESCALATED",
	})
	mustStatus(t, rr, http.StatusOK)

	// 5. Resolve with note.
	rr = do(http.MethodPut, "/api/v1/exceptions/"+excID+"/status", map[string]any{
		"status": "RESOLVED", "resolution_note": "Replacement item shipped",
	})
	mustStatus(t, rr, http.StatusOK)
	if decode(t, rr)["status"] != "RESOLVED" {
		t.Fatal("expected RESOLVED status")
	}
}

// ── Settings API ──────────────────────────────────────────────────────────────

func TestSettingsGetAll(t *testing.T) {
	rr := do(http.MethodGet, "/api/v1/settings", nil)
	mustStatus(t, rr, http.StatusOK)
}

func TestBlackoutDatesCreateAndDelete(t *testing.T) {
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")

	rr := do(http.MethodPost, "/api/v1/settings/blackout-dates", map[string]any{
		"date":   tomorrow,
		"reason": "E2E test blackout",
	})
	mustStatus(t, rr, http.StatusCreated)
	body := decode(t, rr)
	id := body["id"].(string)

	rr = do(http.MethodDelete, "/api/v1/settings/blackout-dates/"+id, nil)
	if rr.Code != http.StatusNoContent && rr.Code != http.StatusOK {
		t.Fatalf("expected 200/204 deleting blackout date, got %d", rr.Code)
	}
}
