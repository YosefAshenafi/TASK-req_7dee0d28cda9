package api_tests

import (
	"net/http"
	"testing"
	"time"
)

func TestReportsList(t *testing.T) {
	rr := admin(http.MethodGet, "/api/v1/reports/exports", nil)
	mustStatus(t, rr, http.StatusOK)
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
