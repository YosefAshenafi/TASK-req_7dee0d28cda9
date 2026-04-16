package api_tests

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestUsersList(t *testing.T) {
	rr := admin(http.MethodGet, "/api/v1/admin/users", nil)
	mustStatus(t, rr, http.StatusOK)
}

func TestUsersCreate(t *testing.T) {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	rr := admin(http.MethodPost, "/api/v1/admin/users", map[string]any{
		"username": "user_" + suffix,
		"email":    "user_" + suffix + "@test.com",
		"password": "TestPass@1!",
		"role":     "FULFILLMENT_SPECIALIST",
	})
	mustStatus(t, rr, http.StatusCreated)
	body := decodeJSON(t, rr)
	if body["id"] == nil {
		t.Fatal("user 'id' missing from create response")
	}
	if body["role"] != "FULFILLMENT_SPECIALIST" {
		t.Errorf("role mismatch: got %v", body["role"])
	}
}

func TestUsersCreate_DuplicateUsername(t *testing.T) {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	payload := map[string]any{
		"username": "dup_" + suffix,
		"email":    "dup_" + suffix + "@test.com",
		"password": "TestPass@1!",
		"role":     "AUDITOR",
	}
	rr := admin(http.MethodPost, "/api/v1/admin/users", payload)
	mustStatus(t, rr, http.StatusCreated)

	// Second creation with same username → conflict or validation error.
	rr = admin(http.MethodPost, "/api/v1/admin/users", payload)
	if rr.Code == http.StatusCreated {
		t.Fatal("expected error for duplicate username, got 201")
	}
}

func TestUsersCreate_InvalidRole(t *testing.T) {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	rr := admin(http.MethodPost, "/api/v1/admin/users", map[string]any{
		"username": "badrole_" + suffix,
		"email":    "badrole_" + suffix + "@test.com",
		"password": "TestPass@1!",
		"role":     "SUPERADMIN",
	})
	if rr.Code == http.StatusCreated {
		t.Fatal("expected error for invalid role SUPERADMIN, got 201")
	}
}

func TestUsersGet(t *testing.T) {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	rr := admin(http.MethodPost, "/api/v1/admin/users", map[string]any{
		"username": "getuser_" + suffix,
		"email":    "getuser_" + suffix + "@test.com",
		"password": "TestPass@1!",
		"role":     "AUDITOR",
	})
	mustStatus(t, rr, http.StatusCreated)
	userID := decodeJSON(t, rr)["id"].(string)

	rr = admin(http.MethodGet, "/api/v1/admin/users/"+userID, nil)
	mustStatus(t, rr, http.StatusOK)
	body := decodeJSON(t, rr)
	if body["id"] != userID {
		t.Errorf("id mismatch: got %v want %v", body["id"], userID)
	}
}

func TestUsersUpdate(t *testing.T) {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	rr := admin(http.MethodPost, "/api/v1/admin/users", map[string]any{
		"username": "upd_" + suffix,
		"email":    "upd_" + suffix + "@test.com",
		"password": "TestPass@1!",
		"role":     "AUDITOR",
	})
	mustStatus(t, rr, http.StatusCreated)
	body := decodeJSON(t, rr)
	userID := body["id"].(string)
	version := int(body["version"].(float64))

	rr = admin(http.MethodPut, "/api/v1/admin/users/"+userID, map[string]any{
		"username": "upd_" + suffix,
		"email":    "upd2_" + suffix + "@test.com",
		"role":     "FULFILLMENT_SPECIALIST",
		"version":  version,
	})
	mustStatus(t, rr, http.StatusOK)
	updated := decodeJSON(t, rr)
	if updated["role"] != "FULFILLMENT_SPECIALIST" {
		t.Errorf("role not updated: got %v", updated["role"])
	}
}

func TestUsersDeactivate(t *testing.T) {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	rr := admin(http.MethodPost, "/api/v1/admin/users", map[string]any{
		"username": "deact_" + suffix,
		"email":    "deact_" + suffix + "@test.com",
		"password": "TestPass@1!",
		"role":     "AUDITOR",
	})
	mustStatus(t, rr, http.StatusCreated)
	userID := decodeJSON(t, rr)["id"].(string)

	rr = admin(http.MethodDelete, "/api/v1/admin/users/"+userID, nil)
	if rr.Code != http.StatusOK && rr.Code != http.StatusNoContent {
		t.Fatalf("expected 200/204 from deactivate, got %d", rr.Code)
	}
}

func TestUsers_RequiresAdminRole(t *testing.T) {
	rr := unauthed(http.MethodGet, "/api/v1/admin/users")
	mustStatus(t, rr, http.StatusUnauthorized)
}
