package e2e_tests

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"
)

// ── Concurrent Inventory ──────────────────────────────────────────────────────

func TestConcurrentInventory_ExactlyOneSucceeds(t *testing.T) {
	rr := do(http.MethodPost, "/api/v1/tiers", map[string]any{
		"name": fmt.Sprintf("e2e-concurrent-%d", time.Now().UnixNano()),
		"inventory_count": 1, "purchase_limit": 5, "alert_threshold": 0,
	})
	mustStatus(t, rr, http.StatusCreated)
	tierID := decode(t, rr)["id"].(string)

	// Create two customers.
	custIDs := make([]string, 2)
	for i := range custIDs {
		rr = do(http.MethodPost, "/api/v1/customers", map[string]any{
			"name": fmt.Sprintf("Concurrent Cust %d %d", i, time.Now().UnixNano()),
		})
		mustStatus(t, rr, http.StatusCreated)
		custIDs[i] = decode(t, rr)["id"].(string)
	}

	type result struct {
		code int
		body string
	}
	results := make([]result, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			rr := do(http.MethodPost, "/api/v1/fulfillments", map[string]any{
				"tier_id": tierID, "customer_id": custIDs[idx], "type": "PHYSICAL",
			})
			results[idx] = result{code: rr.Code, body: rr.Body.String()}
		}(i)
	}
	wg.Wait()

	successes, failures := 0, 0
	for _, r := range results {
		switch r.code {
		case http.StatusCreated:
			successes++
		case http.StatusUnprocessableEntity:
			failures++
		}
	}
	if successes != 1 || failures != 1 {
		t.Fatalf("expected exactly 1 success + 1 INVENTORY_UNAVAILABLE, got %d/%d\n%v", successes, failures, results)
	}
}

// ── Purchase Limit Enforcement ────────────────────────────────────────────────

func TestPurchaseLimitEnforced(t *testing.T) {
	rr := do(http.MethodPost, "/api/v1/tiers", map[string]any{
		"name": fmt.Sprintf("e2e-limit-%d", time.Now().UnixNano()),
		"inventory_count": 10, "purchase_limit": 2, "alert_threshold": 0,
	})
	mustStatus(t, rr, http.StatusCreated)
	tierID := decode(t, rr)["id"].(string)

	rr = do(http.MethodPost, "/api/v1/customers", map[string]any{"name": "Limit E2E Customer"})
	mustStatus(t, rr, http.StatusCreated)
	custID := decode(t, rr)["id"].(string)

	create := func() int {
		return do(http.MethodPost, "/api/v1/fulfillments", map[string]any{
			"tier_id": tierID, "customer_id": custID, "type": "VOUCHER",
		}).Code
	}

	if c := create(); c != http.StatusCreated {
		t.Fatalf("1st create: expected 201, got %d", c)
	}
	if c := create(); c != http.StatusCreated {
		t.Fatalf("2nd create: expected 201, got %d", c)
	}
	if c := create(); c != http.StatusUnprocessableEntity {
		t.Fatalf("3rd create: expected 422 (purchase limit), got %d", c)
	}
}

func TestCanceledFulfillmentsDoNotCountTowardLimit(t *testing.T) {
	rr := do(http.MethodPost, "/api/v1/tiers", map[string]any{
		"name": fmt.Sprintf("e2e-cancel-limit-%d", time.Now().UnixNano()),
		"inventory_count": 10, "purchase_limit": 2, "alert_threshold": 0,
	})
	mustStatus(t, rr, http.StatusCreated)
	tierID := decode(t, rr)["id"].(string)

	rr = do(http.MethodPost, "/api/v1/customers", map[string]any{"name": "Cancel Limit E2E"})
	mustStatus(t, rr, http.StatusCreated)
	custID := decode(t, rr)["id"].(string)

	// Create 2 and cancel both.
	for i := 0; i < 2; i++ {
		rr = do(http.MethodPost, "/api/v1/fulfillments", map[string]any{
			"tier_id": tierID, "customer_id": custID, "type": "PHYSICAL",
		})
		mustStatus(t, rr, http.StatusCreated)
		ffID := decode(t, rr)["id"].(string)

		do(http.MethodPost, "/api/v1/fulfillments/"+ffID+"/transition",
			map[string]any{"to_status": "READY_TO_SHIP"})
		do(http.MethodPost, "/api/v1/fulfillments/"+ffID+"/transition",
			map[string]any{"to_status": "CANCELED", "reason": "cancel for test"})
	}

	// A third creation should succeed since canceled ones don't count.
	rr = do(http.MethodPost, "/api/v1/fulfillments", map[string]any{
		"tier_id": tierID, "customer_id": custID, "type": "PHYSICAL",
	})
	mustStatus(t, rr, http.StatusCreated)
}

// ── Version Conflict (Optimistic Locking) ────────────────────────────────────

func TestVersionConflict_Tier(t *testing.T) {
	rr := do(http.MethodPost, "/api/v1/tiers", map[string]any{
		"name": fmt.Sprintf("e2e-version-%d", time.Now().UnixNano()),
		"inventory_count": 5, "purchase_limit": 2, "alert_threshold": 1,
	})
	mustStatus(t, rr, http.StatusCreated)
	tier := decode(t, rr)
	tierID := tier["id"].(string)

	// First update with version=1 succeeds.
	rr = do(http.MethodPut, "/api/v1/tiers/"+tierID, map[string]any{
		"name": "Updated Name", "inventory_count": 5, "purchase_limit": 2, "alert_threshold": 1, "version": 1,
	})
	mustStatus(t, rr, http.StatusOK)

	// Second update with same stale version=1 must conflict.
	rr = do(http.MethodPut, "/api/v1/tiers/"+tierID, map[string]any{
		"name": "Stale Update", "inventory_count": 5, "purchase_limit": 2, "alert_threshold": 1, "version": 1,
	})
	mustStatus(t, rr, http.StatusConflict)
}
