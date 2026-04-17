package api_tests

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func createTier(t *testing.T, name string, inventory int) map[string]any {
	t.Helper()
	rr := admin(http.MethodPost, "/api/v1/tiers", map[string]any{
		"name": name, "inventory_count": inventory, "purchase_limit": 2, "alert_threshold": 1,
	})
	mustStatus(t, rr, http.StatusCreated)
	return decodeJSON(t, rr)
}

func TestTiersCreate(t *testing.T) {
	name := fmt.Sprintf("api-tier-%d", time.Now().UnixNano())
	tier := createTier(t, name, 10)
	if tier["id"] == nil {
		t.Fatal("create tier response missing 'id'")
	}
	if tier["name"] != name {
		t.Errorf("name mismatch: got %v want %v", tier["name"], name)
	}
	if inv := int(tier["inventory_count"].(float64)); inv != 10 {
		t.Errorf("inventory_count: got %d want 10", inv)
	}
}

func TestTiersCreate_MissingName(t *testing.T) {
	rr := admin(http.MethodPost, "/api/v1/tiers", map[string]any{
		"inventory_count": 5, "purchase_limit": 2, "alert_threshold": 1,
	})
	if rr.Code == http.StatusCreated {
		t.Fatal("expected error for missing name, got 201")
	}
}

func TestTiersCreate_RequiresAdmin(t *testing.T) {
	rr := unauthed(http.MethodPost, "/api/v1/tiers")
	mustStatus(t, rr, http.StatusUnauthorized)
}

func TestTiersList(t *testing.T) {
	seeded := createTier(t, fmt.Sprintf("list-tier-%d", time.Now().UnixNano()), 5)
	seededID := seeded["id"].(string)

	rr := admin(http.MethodGet, "/api/v1/tiers", nil)
	mustStatus(t, rr, http.StatusOK)

	body := decodeJSON(t, rr)
	items, ok := body["items"].([]any)
	if !ok {
		t.Fatal("tiers list missing 'items' array")
	}
	found := false
	for _, raw := range items {
		row, _ := raw.(map[string]any)
		if row["id"] == seededID {
			found = true
			if row["name"] == nil {
				t.Error("tier item missing 'name'")
			}
			if row["inventory_count"] == nil {
				t.Error("tier item missing 'inventory_count'")
			}
		}
	}
	if !found {
		t.Errorf("seeded tier %s not found in list", seededID)
	}
}

func TestTiersGet(t *testing.T) {
	tier := createTier(t, fmt.Sprintf("get-tier-%d", time.Now().UnixNano()), 5)
	id := tier["id"].(string)

	rr := admin(http.MethodGet, "/api/v1/tiers/"+id, nil)
	mustStatus(t, rr, http.StatusOK)
	body := decodeJSON(t, rr)
	if body["id"] != id {
		t.Errorf("id mismatch: got %v want %v", body["id"], id)
	}
}

func TestTiersGet_NotFound(t *testing.T) {
	rr := admin(http.MethodGet, "/api/v1/tiers/00000000-0000-0000-0000-000000000000", nil)
	mustStatus(t, rr, http.StatusNotFound)
}

func TestTiersUpdate(t *testing.T) {
	tier := createTier(t, fmt.Sprintf("update-tier-%d", time.Now().UnixNano()), 5)
	id := tier["id"].(string)

	rr := admin(http.MethodPut, "/api/v1/tiers/"+id, map[string]any{
		"name": "Updated Tier Name", "inventory_count": 8, "purchase_limit": 3, "alert_threshold": 2,
		"version": 1,
	})
	mustStatus(t, rr, http.StatusOK)
	body := decodeJSON(t, rr)
	if body["name"] != "Updated Tier Name" {
		t.Errorf("name not updated: got %v", body["name"])
	}
}

func TestTiersUpdate_VersionConflict(t *testing.T) {
	tier := createTier(t, fmt.Sprintf("conflict-tier-%d", time.Now().UnixNano()), 5)
	id := tier["id"].(string)

	// First update succeeds.
	rr := admin(http.MethodPut, "/api/v1/tiers/"+id, map[string]any{
		"name": "First Update", "inventory_count": 5, "purchase_limit": 2, "alert_threshold": 1, "version": 1,
	})
	mustStatus(t, rr, http.StatusOK)

	// Second update with same stale version should conflict.
	rr = admin(http.MethodPut, "/api/v1/tiers/"+id, map[string]any{
		"name": "Stale Update", "inventory_count": 5, "purchase_limit": 2, "alert_threshold": 1, "version": 1,
	})
	mustStatus(t, rr, http.StatusConflict)
}

func TestTiersSoftDeleteAndRestore(t *testing.T) {
	tier := createTier(t, fmt.Sprintf("del-restore-%d", time.Now().UnixNano()), 3)
	id := tier["id"].(string)

	// Delete → 204.
	rr := admin(http.MethodDelete, "/api/v1/tiers/"+id, nil)
	mustStatus(t, rr, http.StatusNoContent)

	// GET → 404.
	rr = admin(http.MethodGet, "/api/v1/tiers/"+id, nil)
	mustStatus(t, rr, http.StatusNotFound)

	// Restore → 200.
	rr = admin(http.MethodPost, "/api/v1/tiers/"+id+"/restore", nil)
	mustStatus(t, rr, http.StatusOK)

	// GET → 200.
	rr = admin(http.MethodGet, "/api/v1/tiers/"+id, nil)
	mustStatus(t, rr, http.StatusOK)
}

func TestTiersDelete_NotFound(t *testing.T) {
	rr := admin(http.MethodDelete, "/api/v1/tiers/00000000-0000-0000-0000-000000000000", nil)
	mustStatus(t, rr, http.StatusNotFound)
}
