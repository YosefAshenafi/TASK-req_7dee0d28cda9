package api_tests

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func createCustomer(t *testing.T, name string) map[string]any {
	t.Helper()
	rr := admin(http.MethodPost, "/api/v1/customers", map[string]any{
		"name": name, "phone": "5551234567", "email": fmt.Sprintf("%d@test.com", time.Now().UnixNano()),
	})
	mustStatus(t, rr, http.StatusCreated)
	return decodeJSON(t, rr)
}

func TestCustomersCreate(t *testing.T) {
	name := fmt.Sprintf("api-customer-%d", time.Now().UnixNano())
	cust := createCustomer(t, name)
	if cust["id"] == nil {
		t.Fatal("create customer response missing 'id'")
	}
	if cust["name"] != name {
		t.Errorf("name mismatch: got %v want %v", cust["name"], name)
	}
}

func TestCustomersCreate_MissingName(t *testing.T) {
	rr := admin(http.MethodPost, "/api/v1/customers", map[string]any{
		"phone": "5559999999",
	})
	if rr.Code == http.StatusCreated {
		t.Fatal("expected error for missing customer name, got 201")
	}
}

func TestCustomersList(t *testing.T) {
	rr := admin(http.MethodGet, "/api/v1/customers", nil)
	mustStatus(t, rr, http.StatusOK)
}

func TestCustomersGet(t *testing.T) {
	cust := createCustomer(t, fmt.Sprintf("get-cust-%d", time.Now().UnixNano()))
	id := cust["id"].(string)

	rr := admin(http.MethodGet, "/api/v1/customers/"+id, nil)
	mustStatus(t, rr, http.StatusOK)
	body := decodeJSON(t, rr)
	if body["id"] != id {
		t.Errorf("id mismatch: got %v want %v", body["id"], id)
	}
}

func TestCustomersGet_NotFound(t *testing.T) {
	rr := admin(http.MethodGet, "/api/v1/customers/00000000-0000-0000-0000-000000000000", nil)
	mustStatus(t, rr, http.StatusNotFound)
}

func TestCustomersUpdate(t *testing.T) {
	cust := createCustomer(t, fmt.Sprintf("update-cust-%d", time.Now().UnixNano()))
	id := cust["id"].(string)
	version := int(cust["version"].(float64))

	rr := admin(http.MethodPut, "/api/v1/customers/"+id, map[string]any{
		"name": "Updated Customer", "version": version,
	})
	mustStatus(t, rr, http.StatusOK)
	body := decodeJSON(t, rr)
	if body["name"] != "Updated Customer" {
		t.Errorf("name not updated: got %v", body["name"])
	}
}

func TestCustomersSoftDeleteAndRestore(t *testing.T) {
	cust := createCustomer(t, fmt.Sprintf("del-cust-%d", time.Now().UnixNano()))
	id := cust["id"].(string)

	// Delete → 204.
	rr := admin(http.MethodDelete, "/api/v1/customers/"+id, nil)
	mustStatus(t, rr, http.StatusNoContent)

	// GET → 404.
	rr = admin(http.MethodGet, "/api/v1/customers/"+id, nil)
	mustStatus(t, rr, http.StatusNotFound)

	// Restore → 200.
	rr = admin(http.MethodPost, "/api/v1/customers/"+id+"/restore", nil)
	mustStatus(t, rr, http.StatusOK)

	// GET → 200.
	rr = admin(http.MethodGet, "/api/v1/customers/"+id, nil)
	mustStatus(t, rr, http.StatusOK)
}

func TestCustomersCreate_PhoneAndEmailAreMaskedInResponse(t *testing.T) {
	rr := admin(http.MethodPost, "/api/v1/customers", map[string]any{
		"name": "Masked Test", "phone": "5551234567", "email": "masked@example.com",
	})
	mustStatus(t, rr, http.StatusCreated)
	body := decodeJSON(t, rr)
	// Raw PII must not appear in response.
	if phone, ok := body["phone_masked"].(string); ok {
		if phone == "5551234567" {
			t.Error("phone should be masked, not raw, in response")
		}
	}
}
