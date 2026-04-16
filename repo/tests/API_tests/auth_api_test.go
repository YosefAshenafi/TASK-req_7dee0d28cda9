package api_tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthz(t *testing.T) {
	rr := unauthed(http.MethodGet, "/healthz")
	mustStatus(t, rr, http.StatusOK)
	body := decodeJSON(t, rr)
	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", body["status"])
	}
}

func TestAuthLogin_Success(t *testing.T) {
	rr := admin(http.MethodPost, "/api/v1/auth/login", map[string]string{
		"username": "admin", "password": "Admin@FulfillOps1",
	})
	mustStatus(t, rr, http.StatusOK)
	body := decodeJSON(t, rr)
	if body["username"] == nil {
		t.Error("login response missing 'username' field")
	}
}

func TestAuthLogin_WrongPassword(t *testing.T) {
	b, _ := json.Marshal(map[string]string{"username": "admin", "password": "WrongPassword!"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong password, got %d", rr.Code)
	}
}

func TestAuthLogin_MissingBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, req)
	if rr.Code == http.StatusOK {
		t.Fatal("expected non-200 for empty login body")
	}
}

func TestAuthMe_Authenticated(t *testing.T) {
	rr := admin(http.MethodGet, "/api/v1/auth/me", nil)
	mustStatus(t, rr, http.StatusOK)
	body := decodeJSON(t, rr)
	if body["username"] != "admin" {
		t.Errorf("expected username=admin, got %v", body["username"])
	}
}

func TestAuthMe_Unauthenticated(t *testing.T) {
	rr := unauthed(http.MethodGet, "/api/v1/auth/me")
	mustStatus(t, rr, http.StatusUnauthorized)
}

func TestAuthLogout(t *testing.T) {
	rr := admin(http.MethodPost, "/api/v1/auth/logout", nil)
	if rr.Code != http.StatusOK && rr.Code != http.StatusNoContent {
		t.Fatalf("expected 200/204 from logout, got %d", rr.Code)
	}
}

func TestUnauthenticatedEndpointsReturn401(t *testing.T) {
	endpoints := []string{
		"/api/v1/tiers",
		"/api/v1/customers",
		"/api/v1/fulfillments",
	}
	for _, ep := range endpoints {
		rr := unauthed(http.MethodGet, ep)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("GET %s: expected 401, got %d", ep, rr.Code)
		}
	}
}
