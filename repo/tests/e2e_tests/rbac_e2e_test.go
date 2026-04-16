package e2e_tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
	"github.com/fulfillops/fulfillops/internal/service"
)

// loginAs logs in with the given credentials and returns the session cookie.
func loginAs(t *testing.T, username, password string) *http.Cookie {
	t.Helper()
	b, _ := json.Marshal(map[string]string{"username": username, "password": password})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, r)
	if rr.Code != http.StatusOK {
		t.Fatalf("login as %s failed: %d %s", username, rr.Code, rr.Body.String())
	}
	for _, c := range rr.Result().Cookies() {
		if c.Name == "fulfillops_session" {
			return c
		}
	}
	t.Fatalf("no session cookie after login as %s", username)
	return nil
}

func TestRBAC_SpecialistCanReadButNotDeleteTiers(t *testing.T) {
	ctx := context.Background()
	userRepo := repository.NewUserRepository(testPool)
	auditSvc := service.NewAuditService(repository.NewAuditRepository(testPool))
	userSvc := service.NewUserService(userRepo, auditSvc)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	specName := "spec_" + suffix
	specialist, err := userSvc.CreateUser(ctx, specName, specName+"@test.com", "Spec@Test1!", domain.RoleFulfillmentSpecialist)
	if err != nil {
		t.Fatalf("create specialist: %v", err)
	}
	defer func() { _ = userSvc.DeactivateUser(ctx, specialist.ID) }()

	cookie := loginAs(t, specName, "Spec@Test1!")

	// Specialist CAN list tiers.
	rr := req(cookie, http.MethodGet, "/api/v1/tiers", nil)
	mustStatus(t, rr, http.StatusOK)

	// Specialist CANNOT delete a tier.
	rr = req(cookie, http.MethodDelete, "/api/v1/tiers/00000000-0000-0000-0000-000000000001", nil)
	mustStatus(t, rr, http.StatusForbidden)

	// Specialist CANNOT access admin endpoints.
	rr = req(cookie, http.MethodGet, "/api/v1/admin/health", nil)
	mustStatus(t, rr, http.StatusForbidden)
}

func TestRBAC_AuditorCanReadAuditButNotCreateTier(t *testing.T) {
	ctx := context.Background()
	userRepo := repository.NewUserRepository(testPool)
	auditSvc := service.NewAuditService(repository.NewAuditRepository(testPool))
	userSvc := service.NewUserService(userRepo, auditSvc)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	audName := "aud_" + suffix
	auditor, err := userSvc.CreateUser(ctx, audName, audName+"@test.com", "Audit@Test1!", domain.RoleAuditor)
	if err != nil {
		t.Fatalf("create auditor: %v", err)
	}
	defer func() { _ = userSvc.DeactivateUser(ctx, auditor.ID) }()

	cookie := loginAs(t, audName, "Audit@Test1!")

	// Auditor CAN list audit logs.
	rr := req(cookie, http.MethodGet, "/api/v1/audit", nil)
	mustStatus(t, rr, http.StatusOK)

	// Auditor CANNOT create a tier.
	rr = req(cookie, http.MethodPost, "/api/v1/tiers", map[string]any{
		"name": "auditor-tier", "inventory_count": 1, "purchase_limit": 1, "alert_threshold": 0,
	})
	mustStatus(t, rr, http.StatusForbidden)

	// Auditor CANNOT create a fulfillment.
	rr = req(cookie, http.MethodPost, "/api/v1/fulfillments", map[string]any{
		"tier_id": "00000000-0000-0000-0000-000000000001",
		"customer_id": "00000000-0000-0000-0000-000000000002",
		"type": "PHYSICAL",
	})
	mustStatus(t, rr, http.StatusForbidden)
}

func TestRBAC_UnauthenticatedReturns401(t *testing.T) {
	for _, path := range []string{
		"/api/v1/tiers",
		"/api/v1/customers",
		"/api/v1/fulfillments",
		"/api/v1/audit",
		"/api/v1/auth/me",
	} {
		rr := req(nil, http.MethodGet, path, nil)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("GET %s: expected 401, got %d", path, rr.Code)
		}
	}
}

func TestRBAC_SpecialistCanCreateFulfillments(t *testing.T) {
	ctx := context.Background()
	userRepo := repository.NewUserRepository(testPool)
	auditSvc := service.NewAuditService(repository.NewAuditRepository(testPool))
	userSvc := service.NewUserService(userRepo, auditSvc)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	specName := "spec2_" + suffix
	specialist, err := userSvc.CreateUser(ctx, specName, specName+"@test.com", "Spec@Test2!", domain.RoleFulfillmentSpecialist)
	if err != nil {
		t.Fatalf("create specialist: %v", err)
	}
	defer func() { _ = userSvc.DeactivateUser(ctx, specialist.ID) }()

	// Create tier + customer as admin.
	rr := do(http.MethodPost, "/api/v1/tiers", map[string]any{
		"name": fmt.Sprintf("rbac-spec-tier-%d", time.Now().UnixNano()),
		"inventory_count": 5, "purchase_limit": 3, "alert_threshold": 1,
	})
	mustStatus(t, rr, http.StatusCreated)
	tierID := decode(t, rr)["id"].(string)

	rr = do(http.MethodPost, "/api/v1/customers", map[string]any{"name": "RBAC Spec Customer"})
	mustStatus(t, rr, http.StatusCreated)
	custID := decode(t, rr)["id"].(string)

	// Specialist CAN create a fulfillment.
	cookie := loginAs(t, specName, "Spec@Test2!")
	rr = req(cookie, http.MethodPost, "/api/v1/fulfillments", map[string]any{
		"tier_id": tierID, "customer_id": custID, "type": "VOUCHER",
	})
	mustStatus(t, rr, http.StatusCreated)
}
