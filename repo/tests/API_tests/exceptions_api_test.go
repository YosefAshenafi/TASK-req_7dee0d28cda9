package api_tests

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func setupExceptionFixture(t *testing.T) (tierID, custID, ffID string) {
	t.Helper()
	rr := admin(http.MethodPost, "/api/v1/tiers", map[string]any{
		"name": fmt.Sprintf("exc-tier-%d", time.Now().UnixNano()),
		"inventory_count": 5, "purchase_limit": 3, "alert_threshold": 1,
	})
	mustStatus(t, rr, http.StatusCreated)
	tierID = decodeJSON(t, rr)["id"].(string)

	rr = admin(http.MethodPost, "/api/v1/customers", map[string]any{
		"name": fmt.Sprintf("exc-cust-%d", time.Now().UnixNano()),
	})
	mustStatus(t, rr, http.StatusCreated)
	custID = decodeJSON(t, rr)["id"].(string)

	rr = admin(http.MethodPost, "/api/v1/fulfillments", map[string]any{
		"tier_id": tierID, "customer_id": custID, "type": "VOUCHER",
	})
	mustStatus(t, rr, http.StatusCreated)
	ffID = decodeJSON(t, rr)["id"].(string)
	return
}

func TestExceptionsCreate(t *testing.T) {
	_, _, ffID := setupExceptionFixture(t)

	rr := admin(http.MethodPost, "/api/v1/exceptions", map[string]any{
		"fulfillment_id": ffID, "type": "MANUAL",
	})
	mustStatus(t, rr, http.StatusCreated)
	body := decodeJSON(t, rr)
	if body["id"] == nil {
		t.Fatal("exception 'id' missing from response")
	}
	if body["status"] != "OPEN" {
		t.Errorf("expected OPEN status, got %v", body["status"])
	}
}

func TestExceptionsList(t *testing.T) {
	rr := admin(http.MethodGet, "/api/v1/exceptions", nil)
	mustStatus(t, rr, http.StatusOK)
}

func TestExceptionsGet(t *testing.T) {
	_, _, ffID := setupExceptionFixture(t)

	rr := admin(http.MethodPost, "/api/v1/exceptions", map[string]any{
		"fulfillment_id": ffID, "type": "MANUAL",
	})
	mustStatus(t, rr, http.StatusCreated)
	excID := decodeJSON(t, rr)["id"].(string)

	rr = admin(http.MethodGet, "/api/v1/exceptions/"+excID, nil)
	mustStatus(t, rr, http.StatusOK)
	body := decodeJSON(t, rr)
	if body["id"] != excID {
		t.Errorf("id mismatch: got %v want %v", body["id"], excID)
	}
}

func TestExceptionsGet_NotFound(t *testing.T) {
	rr := admin(http.MethodGet, "/api/v1/exceptions/00000000-0000-0000-0000-000000000000", nil)
	mustStatus(t, rr, http.StatusNotFound)
}

func TestExceptionsAddEvent(t *testing.T) {
	_, _, ffID := setupExceptionFixture(t)

	rr := admin(http.MethodPost, "/api/v1/exceptions", map[string]any{
		"fulfillment_id": ffID, "type": "MANUAL",
	})
	mustStatus(t, rr, http.StatusCreated)
	excID := decodeJSON(t, rr)["id"].(string)

	rr = admin(http.MethodPost, "/api/v1/exceptions/"+excID+"/events", map[string]any{
		"event_type": "NOTE", "content": "Investigating the issue",
	})
	mustStatus(t, rr, http.StatusCreated)
}

func TestExceptionsUpdateStatus_Investigating(t *testing.T) {
	_, _, ffID := setupExceptionFixture(t)

	rr := admin(http.MethodPost, "/api/v1/exceptions", map[string]any{
		"fulfillment_id": ffID, "type": "MANUAL",
	})
	mustStatus(t, rr, http.StatusCreated)
	excID := decodeJSON(t, rr)["id"].(string)

	rr = admin(http.MethodPut, "/api/v1/exceptions/"+excID+"/status", map[string]any{
		"status": "INVESTIGATING",
	})
	mustStatus(t, rr, http.StatusOK)
	body := decodeJSON(t, rr)
	if body["status"] != "INVESTIGATING" {
		t.Errorf("expected INVESTIGATING, got %v", body["status"])
	}
}

func TestExceptionsUpdateStatus_Resolved(t *testing.T) {
	_, _, ffID := setupExceptionFixture(t)

	rr := admin(http.MethodPost, "/api/v1/exceptions", map[string]any{
		"fulfillment_id": ffID, "type": "MANUAL",
	})
	mustStatus(t, rr, http.StatusCreated)
	excID := decodeJSON(t, rr)["id"].(string)

	rr = admin(http.MethodPut, "/api/v1/exceptions/"+excID+"/status", map[string]any{
		"status": "RESOLVED", "resolution_note": "Issue resolved",
	})
	mustStatus(t, rr, http.StatusOK)
	body := decodeJSON(t, rr)
	if body["status"] != "RESOLVED" {
		t.Errorf("expected RESOLVED, got %v", body["status"])
	}
}
