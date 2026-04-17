package e2e_tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// ── Full Physical Fulfillment Lifecycle ───────────────────────────────────────

func TestPhysicalFulfillmentLifecycle(t *testing.T) {
	// 1. Create tier.
	rr := do(http.MethodPost, "/api/v1/tiers", map[string]any{
		"name":            fmt.Sprintf("e2e-lifecycle-%d", time.Now().UnixNano()),
		"inventory_count": 5, "purchase_limit": 2, "alert_threshold": 1,
	})
	mustStatus(t, rr, http.StatusCreated)
	tier := decode(t, rr)
	tierID := tier["id"].(string)

	// 2. Create customer.
	rr = do(http.MethodPost, "/api/v1/customers", map[string]any{
		"name": "E2E Lifecycle Customer", "phone": "5551112222", "email": "lifecycle@e2e.test",
	})
	mustStatus(t, rr, http.StatusCreated)
	custID := decode(t, rr)["id"].(string)

	// 3. Create fulfillment → DRAFT.
	rr = do(http.MethodPost, "/api/v1/fulfillments", map[string]any{
		"tier_id": tierID, "customer_id": custID, "type": "PHYSICAL",
	})
	mustStatus(t, rr, http.StatusCreated)
	ff := decode(t, rr)
	ffID := ff["id"].(string)
	if ff["status"] != "DRAFT" {
		t.Fatalf("expected DRAFT, got %v", ff["status"])
	}

	// Inventory must decrease by 1.
	rr = do(http.MethodGet, "/api/v1/tiers/"+tierID, nil)
	if inv := int(decode(t, rr)["inventory_count"].(float64)); inv != 4 {
		t.Fatalf("inventory after create: want 4, got %d", inv)
	}

	// 4. DRAFT → READY_TO_SHIP (shipping address required for PHYSICAL).
	rr = transition(t, ffID, map[string]any{
		"to_status": "READY_TO_SHIP",
		"shipping_address": map[string]any{
			"line_1":   "123 Main St",
			"city":     "Springfield",
			"state":    "MA",
			"zip_code": "01101",
		},
	})
	mustStatus(t, rr, http.StatusOK)

	// 5. READY_TO_SHIP → SHIPPED (tracking required).
	rr = transition(t, ffID, map[string]any{
		"to_status": "SHIPPED", "carrier_name": "FedEx", "tracking_number": "1Z999AA10123456784",
	})
	mustStatus(t, rr, http.StatusOK)

	// 6. SHIPPED → DELIVERED.
	rr = transition(t, ffID, map[string]any{"to_status": "DELIVERED"})
	mustStatus(t, rr, http.StatusOK)

	// 7. DELIVERED → COMPLETED.
	rr = transition(t, ffID, map[string]any{"to_status": "COMPLETED"})
	mustStatus(t, rr, http.StatusOK)
	if decode(t, rr)["status"] != "COMPLETED" {
		t.Fatal("expected final status COMPLETED")
	}

	// 8. Timeline must have ≥ 4 entries.
	rr = do(http.MethodGet, "/api/v1/fulfillments/"+ffID+"/timeline", nil)
	mustStatus(t, rr, http.StatusOK)
	var tlResp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &tlResp)
	if items, _ := tlResp["items"].([]any); len(items) < 4 {
		t.Fatalf("expected ≥4 timeline entries, got %d", len(items))
	}
}

// ── Voucher Lifecycle ─────────────────────────────────────────────────────────

func TestVoucherFulfillmentLifecycle(t *testing.T) {
	rr := do(http.MethodPost, "/api/v1/tiers", map[string]any{
		"name":            fmt.Sprintf("e2e-voucher-%d", time.Now().UnixNano()),
		"inventory_count": 3, "purchase_limit": 2, "alert_threshold": 1,
	})
	mustStatus(t, rr, http.StatusCreated)
	tierID := decode(t, rr)["id"].(string)

	rr = do(http.MethodPost, "/api/v1/customers", map[string]any{"name": "Voucher E2E Customer"})
	mustStatus(t, rr, http.StatusCreated)
	custID := decode(t, rr)["id"].(string)

	rr = do(http.MethodPost, "/api/v1/fulfillments", map[string]any{
		"tier_id": tierID, "customer_id": custID, "type": "VOUCHER",
	})
	mustStatus(t, rr, http.StatusCreated)
	ffID := decode(t, rr)["id"].(string)

	// DRAFT → READY_TO_SHIP → VOUCHER_ISSUED → COMPLETED.
	steps := []map[string]any{
		{"to_status": "READY_TO_SHIP"},
		{"to_status": "VOUCHER_ISSUED", "voucher_code": "E2E-VOUCHER-001"},
		{"to_status": "COMPLETED"},
	}
	for _, step := range steps {
		rr = transition(t, ffID, step)
		mustStatus(t, rr, http.StatusOK)
	}
	if decode(t, rr)["status"] != "COMPLETED" {
		t.Fatal("voucher lifecycle should end at COMPLETED")
	}
}

// ── Cancel and Inventory Restore ─────────────────────────────────────────────

func TestCancelRestoresInventory(t *testing.T) {
	rr := do(http.MethodPost, "/api/v1/tiers", map[string]any{
		"name":            fmt.Sprintf("e2e-cancel-%d", time.Now().UnixNano()),
		"inventory_count": 3, "purchase_limit": 2, "alert_threshold": 1,
	})
	mustStatus(t, rr, http.StatusCreated)
	tierID := decode(t, rr)["id"].(string)

	rr = do(http.MethodPost, "/api/v1/customers", map[string]any{"name": "Cancel E2E"})
	mustStatus(t, rr, http.StatusCreated)
	custID := decode(t, rr)["id"].(string)

	rr = do(http.MethodPost, "/api/v1/fulfillments", map[string]any{
		"tier_id": tierID, "customer_id": custID, "type": "PHYSICAL",
	})
	mustStatus(t, rr, http.StatusCreated)
	ffID := decode(t, rr)["id"].(string)

	// Inventory should now be 2.
	rr = do(http.MethodGet, "/api/v1/tiers/"+tierID, nil)
	if inv := int(decode(t, rr)["inventory_count"].(float64)); inv != 2 {
		t.Fatalf("inventory after create: want 2, got %d", inv)
	}

	_ = transition(t, ffID, map[string]any{
		"to_status":        "READY_TO_SHIP",
		"shipping_address": map[string]any{"line_1": "123 Main St", "city": "Springfield", "state": "IL", "zip_code": "62701"},
	})

	// Cancel from READY_TO_SHIP.
	rr = transition(t, ffID, map[string]any{"to_status": "CANCELED", "reason": "E2E cancel test"})
	mustStatus(t, rr, http.StatusOK)

	// Inventory must be restored to 3.
	rr = do(http.MethodGet, "/api/v1/tiers/"+tierID, nil)
	if inv := int(decode(t, rr)["inventory_count"].(float64)); inv != 3 {
		t.Fatalf("inventory after cancel: want 3, got %d", inv)
	}
}

// ── On-Hold Flow ──────────────────────────────────────────────────────────────

func TestOnHoldAndResume(t *testing.T) {
	rr := do(http.MethodPost, "/api/v1/tiers", map[string]any{
		"name":            fmt.Sprintf("e2e-hold-%d", time.Now().UnixNano()),
		"inventory_count": 5, "purchase_limit": 2, "alert_threshold": 1,
	})
	mustStatus(t, rr, http.StatusCreated)
	tierID := decode(t, rr)["id"].(string)

	rr = do(http.MethodPost, "/api/v1/customers", map[string]any{"name": "Hold E2E"})
	mustStatus(t, rr, http.StatusCreated)
	custID := decode(t, rr)["id"].(string)

	rr = do(http.MethodPost, "/api/v1/fulfillments", map[string]any{
		"tier_id": tierID, "customer_id": custID, "type": "PHYSICAL",
	})
	mustStatus(t, rr, http.StatusCreated)
	ffID := decode(t, rr)["id"].(string)

	// DRAFT → READY_TO_SHIP → ON_HOLD → READY_TO_SHIP.
	_ = transition(t, ffID, map[string]any{
		"to_status":        "READY_TO_SHIP",
		"shipping_address": map[string]any{"line_1": "456 Elm Ave", "city": "Chicago", "state": "IL", "zip_code": "60601"},
	})

	rr = transition(t, ffID, map[string]any{"to_status": "ON_HOLD", "reason": "e2e hold"})
	mustStatus(t, rr, http.StatusOK)
	if decode(t, rr)["status"] != "ON_HOLD" {
		t.Fatal("expected ON_HOLD status")
	}

	rr = transition(t, ffID, map[string]any{"to_status": "READY_TO_SHIP"})
	mustStatus(t, rr, http.StatusOK)
	if decode(t, rr)["status"] != "READY_TO_SHIP" {
		t.Fatal("expected READY_TO_SHIP after resuming from hold")
	}
}

// ── Soft-Delete and Restore ───────────────────────────────────────────────────

func TestTierSoftDeleteAndRestore(t *testing.T) {
	rr := do(http.MethodPost, "/api/v1/tiers", map[string]any{
		"name":            fmt.Sprintf("e2e-delrestore-%d", time.Now().UnixNano()),
		"inventory_count": 5, "purchase_limit": 2, "alert_threshold": 1,
	})
	mustStatus(t, rr, http.StatusCreated)
	tierID := decode(t, rr)["id"].(string)

	// Soft-delete → 204.
	rr = do(http.MethodDelete, "/api/v1/tiers/"+tierID, nil)
	mustStatus(t, rr, http.StatusNoContent)

	// Deleted tier should return 404.
	rr = do(http.MethodGet, "/api/v1/tiers/"+tierID, nil)
	mustStatus(t, rr, http.StatusNotFound)

	// Restore → 200.
	rr = do(http.MethodPost, "/api/v1/tiers/"+tierID+"/restore", nil)
	mustStatus(t, rr, http.StatusOK)

	// Tier reachable again.
	rr = do(http.MethodGet, "/api/v1/tiers/"+tierID, nil)
	mustStatus(t, rr, http.StatusOK)
}
