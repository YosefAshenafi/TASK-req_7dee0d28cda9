package api_tests

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// helpers -------------------------------------------------------------------

func apiCreateTier(t *testing.T) string {
	t.Helper()
	rr := admin(http.MethodPost, "/api/v1/tiers", map[string]any{
		"name": fmt.Sprintf("ff-tier-%d", time.Now().UnixNano()),
		"inventory_count": 10, "purchase_limit": 5, "alert_threshold": 1,
	})
	mustStatus(t, rr, http.StatusCreated)
	return decodeJSON(t, rr)["id"].(string)
}

func apiCreateCustomer(t *testing.T) string {
	t.Helper()
	rr := admin(http.MethodPost, "/api/v1/customers", map[string]any{
		"name": fmt.Sprintf("ff-cust-%d", time.Now().UnixNano()),
	})
	mustStatus(t, rr, http.StatusCreated)
	return decodeJSON(t, rr)["id"].(string)
}

func apiCreateFulfillment(t *testing.T, tierID, custID, ffType string) map[string]any {
	t.Helper()
	rr := admin(http.MethodPost, "/api/v1/fulfillments", map[string]any{
		"tier_id": tierID, "customer_id": custID, "type": ffType,
	})
	mustStatus(t, rr, http.StatusCreated)
	return decodeJSON(t, rr)
}

// tests ---------------------------------------------------------------------

func TestFulfillmentsCreate(t *testing.T) {
	tierID := apiCreateTier(t)
	custID := apiCreateCustomer(t)

	rr := admin(http.MethodPost, "/api/v1/fulfillments", map[string]any{
		"tier_id": tierID, "customer_id": custID, "type": "PHYSICAL",
	})
	mustStatus(t, rr, http.StatusCreated)
	body := decodeJSON(t, rr)
	if body["status"] != "DRAFT" {
		t.Errorf("expected DRAFT status, got %v", body["status"])
	}
	if body["id"] == nil {
		t.Fatal("fulfillment 'id' missing from response")
	}
}

func TestFulfillmentsCreate_MissingTierID(t *testing.T) {
	custID := apiCreateCustomer(t)
	rr := admin(http.MethodPost, "/api/v1/fulfillments", map[string]any{
		"customer_id": custID, "type": "PHYSICAL",
	})
	if rr.Code == http.StatusCreated {
		t.Fatal("expected error for missing tier_id, got 201")
	}
}

func TestFulfillmentsCreate_InventoryUnavailable(t *testing.T) {
	rr := admin(http.MethodPost, "/api/v1/tiers", map[string]any{
		"name": fmt.Sprintf("zero-inv-%d", time.Now().UnixNano()),
		"inventory_count": 0, "purchase_limit": 5, "alert_threshold": 0,
	})
	mustStatus(t, rr, http.StatusCreated)
	tierID := decodeJSON(t, rr)["id"].(string)
	custID := apiCreateCustomer(t)

	rr = admin(http.MethodPost, "/api/v1/fulfillments", map[string]any{
		"tier_id": tierID, "customer_id": custID, "type": "PHYSICAL",
	})
	mustStatus(t, rr, http.StatusUnprocessableEntity)
}

func TestFulfillmentsList(t *testing.T) {
	rr := admin(http.MethodGet, "/api/v1/fulfillments", nil)
	mustStatus(t, rr, http.StatusOK)
}

func TestFulfillmentsGet(t *testing.T) {
	tierID := apiCreateTier(t)
	custID := apiCreateCustomer(t)
	ff := apiCreateFulfillment(t, tierID, custID, "PHYSICAL")
	id := ff["id"].(string)

	rr := admin(http.MethodGet, "/api/v1/fulfillments/"+id, nil)
	mustStatus(t, rr, http.StatusOK)
	body := decodeJSON(t, rr)
	if body["id"] != id {
		t.Errorf("id mismatch: got %v want %v", body["id"], id)
	}
}

func TestFulfillmentsGet_NotFound(t *testing.T) {
	rr := admin(http.MethodGet, "/api/v1/fulfillments/00000000-0000-0000-0000-000000000000", nil)
	mustStatus(t, rr, http.StatusNotFound)
}

func TestFulfillmentsTransition_DraftToReadyToShip(t *testing.T) {
	tierID := apiCreateTier(t)
	custID := apiCreateCustomer(t)
	ff := apiCreateFulfillment(t, tierID, custID, "PHYSICAL")
	id := ff["id"].(string)

	rr := admin(http.MethodPost, "/api/v1/fulfillments/"+id+"/transition",
		map[string]any{"to_status": "READY_TO_SHIP"})
	mustStatus(t, rr, http.StatusOK)
	body := decodeJSON(t, rr)
	if body["status"] != "READY_TO_SHIP" {
		t.Errorf("expected READY_TO_SHIP, got %v", body["status"])
	}
}

func TestFulfillmentsTransition_ShippedRequiresTracking(t *testing.T) {
	tierID := apiCreateTier(t)
	custID := apiCreateCustomer(t)
	ff := apiCreateFulfillment(t, tierID, custID, "PHYSICAL")
	id := ff["id"].(string)

	// advance to READY_TO_SHIP
	rr := admin(http.MethodPost, "/api/v1/fulfillments/"+id+"/transition",
		map[string]any{"to_status": "READY_TO_SHIP"})
	mustStatus(t, rr, http.StatusOK)

	// try SHIPPED without tracking → should fail
	rr = admin(http.MethodPost, "/api/v1/fulfillments/"+id+"/transition",
		map[string]any{"to_status": "SHIPPED", "carrier_name": "FedEx"})
	if rr.Code == http.StatusOK {
		t.Fatal("expected error when shipping without tracking number")
	}
}

func TestFulfillmentsTransition_InvalidTransition(t *testing.T) {
	tierID := apiCreateTier(t)
	custID := apiCreateCustomer(t)
	ff := apiCreateFulfillment(t, tierID, custID, "PHYSICAL")
	id := ff["id"].(string)

	// DRAFT → COMPLETED is not allowed
	rr := admin(http.MethodPost, "/api/v1/fulfillments/"+id+"/transition",
		map[string]any{"to_status": "COMPLETED"})
	if rr.Code == http.StatusOK {
		t.Fatal("expected error for invalid DRAFT→COMPLETED transition")
	}
}

func TestFulfillmentsTransition_Cancel(t *testing.T) {
	tierID := apiCreateTier(t)
	custID := apiCreateCustomer(t)
	ff := apiCreateFulfillment(t, tierID, custID, "VOUCHER")
	id := ff["id"].(string)

	rr := admin(http.MethodPost, "/api/v1/fulfillments/"+id+"/transition",
		map[string]any{"to_status": "READY_TO_SHIP"})
	mustStatus(t, rr, http.StatusOK)

	rr = admin(http.MethodPost, "/api/v1/fulfillments/"+id+"/transition",
		map[string]any{"to_status": "CANCELED", "reason": "test cancel"})
	mustStatus(t, rr, http.StatusOK)
	body := decodeJSON(t, rr)
	if body["status"] != "CANCELED" {
		t.Errorf("expected CANCELED, got %v", body["status"])
	}
}

func TestFulfillmentsTimeline(t *testing.T) {
	tierID := apiCreateTier(t)
	custID := apiCreateCustomer(t)
	ff := apiCreateFulfillment(t, tierID, custID, "VOUCHER")
	id := ff["id"].(string)

	// Make a transition to generate timeline entries.
	admin(http.MethodPost, "/api/v1/fulfillments/"+id+"/transition",
		map[string]any{"to_status": "READY_TO_SHIP"})

	rr := admin(http.MethodGet, "/api/v1/fulfillments/"+id+"/timeline", nil)
	mustStatus(t, rr, http.StatusOK)
}

func TestFulfillmentsSoftDelete(t *testing.T) {
	tierID := apiCreateTier(t)
	custID := apiCreateCustomer(t)
	ff := apiCreateFulfillment(t, tierID, custID, "VOUCHER")
	id := ff["id"].(string)

	rr := admin(http.MethodDelete, "/api/v1/fulfillments/"+id, nil)
	mustStatus(t, rr, http.StatusNoContent)
}
