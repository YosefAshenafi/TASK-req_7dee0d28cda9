package handler

// Tests for Finding #2: auditor-visible fulfillment pages must render masked
// city, state, and ZIP. Admins / specialists must see the unmasked values.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"
	"github.com/jackc/pgx/v5"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
)

// ── minimal stubs ────────────────────────────────────────────────────────────

type stubFulfillRepo struct{ f *domain.Fulfillment }

func (r *stubFulfillRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Fulfillment, error) {
	if r.f == nil {
		return nil, domain.NewNotFoundError("fulfillment")
	}
	cp := *r.f
	cp.ID = id
	return &cp, nil
}
func (r *stubFulfillRepo) List(_ context.Context, _ repository.FulfillmentFilters, _ domain.PageRequest) ([]domain.Fulfillment, int, error) {
	return nil, 0, nil
}
func (r *stubFulfillRepo) GetByIDForUpdate(_ context.Context, _ pgx.Tx, _ uuid.UUID) (*domain.Fulfillment, error) {
	return r.f, nil
}
func (r *stubFulfillRepo) Create(_ context.Context, _ pgx.Tx, f *domain.Fulfillment) (*domain.Fulfillment, error) {
	return f, nil
}
func (r *stubFulfillRepo) Update(_ context.Context, _ pgx.Tx, f *domain.Fulfillment) (*domain.Fulfillment, error) {
	return f, nil
}
func (r *stubFulfillRepo) BumpVersion(_ context.Context, _ pgx.Tx, _ uuid.UUID, _ int) error {
	return nil
}
func (r *stubFulfillRepo) CountByCustomerAndTier(_ context.Context, _ pgx.Tx, _, _ uuid.UUID, _ time.Time) (int, error) {
	return 0, nil
}
func (r *stubFulfillRepo) SoftDelete(_ context.Context, _ uuid.UUID, _ uuid.UUID) error { return nil }
func (r *stubFulfillRepo) Restore(_ context.Context, _ uuid.UUID) error                 { return nil }
func (r *stubFulfillRepo) ListOverdue(_ context.Context) ([]domain.Fulfillment, error)  { return nil, nil }

type stubTierRepo struct{ tier *domain.RewardTier }

func (r *stubTierRepo) GetByID(_ context.Context, _ uuid.UUID) (*domain.RewardTier, error) {
	if r.tier == nil {
		return nil, domain.NewNotFoundError("tier")
	}
	return r.tier, nil
}
func (r *stubTierRepo) List(_ context.Context, _ string, _ bool) ([]domain.RewardTier, error) {
	return nil, nil
}
func (r *stubTierRepo) GetByIDForUpdate(_ context.Context, _ pgx.Tx, _ uuid.UUID) (*domain.RewardTier, error) {
	return r.tier, nil
}
func (r *stubTierRepo) Create(_ context.Context, t *domain.RewardTier) (*domain.RewardTier, error) {
	return t, nil
}
func (r *stubTierRepo) Update(_ context.Context, t *domain.RewardTier) (*domain.RewardTier, error) {
	return t, nil
}
func (r *stubTierRepo) SoftDelete(_ context.Context, _ uuid.UUID, _ uuid.UUID) error { return nil }
func (r *stubTierRepo) Restore(_ context.Context, _ uuid.UUID) error                 { return nil }
func (r *stubTierRepo) DecrementInventory(_ context.Context, _ pgx.Tx, _ uuid.UUID, _ int) error {
	return nil
}
func (r *stubTierRepo) IncrementInventory(_ context.Context, _ pgx.Tx, _ uuid.UUID, _ int) error {
	return nil
}

type stubCustRepo2 struct{ cu *domain.Customer }

func (r *stubCustRepo2) GetByID(_ context.Context, _ uuid.UUID) (*domain.Customer, error) {
	if r.cu == nil {
		return nil, domain.NewNotFoundError("customer")
	}
	return r.cu, nil
}
func (r *stubCustRepo2) List(_ context.Context, _ string, _ domain.PageRequest, _ bool) ([]domain.Customer, int, error) {
	return nil, 0, nil
}
func (r *stubCustRepo2) Create(_ context.Context, c *domain.Customer) (*domain.Customer, error) {
	return c, nil
}
func (r *stubCustRepo2) Update(_ context.Context, c *domain.Customer) (*domain.Customer, error) {
	return c, nil
}
func (r *stubCustRepo2) SoftDelete(_ context.Context, _ uuid.UUID, _ uuid.UUID) error { return nil }
func (r *stubCustRepo2) Restore(_ context.Context, _ uuid.UUID) error                 { return nil }

type stubTimelineRepo struct{}

func (r *stubTimelineRepo) Create(_ context.Context, _ pgx.Tx, _ *domain.TimelineEvent) error {
	return nil
}
func (r *stubTimelineRepo) ListByFulfillmentID(_ context.Context, _ uuid.UUID) ([]domain.TimelineEvent, error) {
	return nil, nil
}

type stubShippingRepo struct{ addr *domain.ShippingAddress }

func (r *stubShippingRepo) GetByFulfillmentID(_ context.Context, _ uuid.UUID) (*domain.ShippingAddress, error) {
	return r.addr, nil
}
func (r *stubShippingRepo) Create(_ context.Context, _ pgx.Tx, a *domain.ShippingAddress) (*domain.ShippingAddress, error) {
	return a, nil
}
func (r *stubShippingRepo) CreateNoTx(_ context.Context, a *domain.ShippingAddress) (*domain.ShippingAddress, error) {
	return a, nil
}
func (r *stubShippingRepo) Update(_ context.Context, _ pgx.Tx, _ *domain.ShippingAddress) error {
	return nil
}

type stubExceptionRepo struct{}

func (r *stubExceptionRepo) List(_ context.Context, _ repository.ExceptionFilters) ([]domain.FulfillmentException, error) {
	return nil, nil
}
func (r *stubExceptionRepo) GetByID(_ context.Context, _ uuid.UUID) (*domain.FulfillmentException, error) {
	return nil, domain.NewNotFoundError("exception")
}
func (r *stubExceptionRepo) Create(_ context.Context, e *domain.FulfillmentException) (*domain.FulfillmentException, error) {
	return e, nil
}
func (r *stubExceptionRepo) UpdateStatus(_ context.Context, _ uuid.UUID, _ domain.ExceptionStatus, _ *string, _ *uuid.UUID) error {
	return nil
}
func (r *stubExceptionRepo) ExistsOpenForFulfillment(_ context.Context, _ uuid.UUID, _ domain.ExceptionType) (bool, error) {
	return false, nil
}

// sessionCookieFor creates a gorilla/sessions cookie with the supplied role.
func sessionCookieFor(t *testing.T, store sessions.Store, role domain.UserRole) *http.Cookie {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	sess, _ := store.Get(req, "fulfillops")
	sess.Values["userID"] = uuid.New().String()
	sess.Values["username"] = "testuser"
	sess.Values["userRole"] = string(role)
	if err := sess.Save(req, rec); err != nil {
		t.Fatalf("saving session: %v", err)
	}
	result := rec.Result()
	cookies := result.Cookies()
	if len(cookies) == 0 {
		t.Fatal("no session cookie produced")
	}
	return cookies[0]
}

func TestFulfillmentDetail_AuditorSeesmaskedAddress(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := sessions.NewCookieStore([]byte("test-secret-32bytes-long-padding!"))
	enc := identityEncryptionSvc{}

	// Build shipping address — city "Boston", state "MA", zip "02110"
	line1Enc, _ := enc.Encrypt([]byte("123 Main St"))
	line2Enc, _ := enc.Encrypt([]byte("Apt 4"))
	addr := &domain.ShippingAddress{
		Line1Encrypted: line1Enc,
		Line2Encrypted: line2Enc,
		City:           "Boston",
		State:          "MA",
		ZipCode:        "02110",
	}

	ff := &domain.Fulfillment{ID: uuid.New(), TierID: uuid.New(), CustomerID: uuid.New(), Type: domain.TypePhysical, Status: domain.StatusDraft}
	h := NewPageFulfillmentHandler(
		store,
		nil, // FulfillmentService not needed for ShowDetail
		&stubFulfillRepo{f: ff},
		&stubTierRepo{},
		&stubCustRepo2{cu: &domain.Customer{ID: ff.CustomerID}},
		&stubTimelineRepo{},
		&stubShippingRepo{addr: addr},
		&stubExceptionRepo{},
		enc,
	)

	r := gin.New()
	r.GET("/fulfillments/:id", h.ShowDetail)

	// ── Auditor: should see masked values ─────────────────────────────────────
	cookie := sessionCookieFor(t, store, domain.RoleAuditor)
	req := httptest.NewRequest(http.MethodGet, "/fulfillments/"+ff.ID.String(), nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("auditor: expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	// Masked values must appear
	for _, masked := range []string{"B***", "**", "021XX"} {
		if !strings.Contains(body, masked) {
			t.Errorf("auditor page missing masked value %q", masked)
		}
	}
	// Real values must NOT appear
	for _, real := range []string{"Boston", "MA", "02110"} {
		if strings.Contains(body, real) {
			t.Errorf("auditor page leaked real value %q", real)
		}
	}
}

func TestFulfillmentDetail_AdminSeesFullAddress(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := sessions.NewCookieStore([]byte("test-secret-32bytes-long-padding!"))
	enc := identityEncryptionSvc{}

	line1Enc, _ := enc.Encrypt([]byte("123 Main St"))
	addr := &domain.ShippingAddress{
		Line1Encrypted: line1Enc,
		City:           "Boston",
		State:          "MA",
		ZipCode:        "02110",
	}

	ff := &domain.Fulfillment{ID: uuid.New(), TierID: uuid.New(), CustomerID: uuid.New(), Type: domain.TypePhysical, Status: domain.StatusDraft}
	h := NewPageFulfillmentHandler(
		store, nil,
		&stubFulfillRepo{f: ff},
		&stubTierRepo{},
		&stubCustRepo2{cu: &domain.Customer{ID: ff.CustomerID}},
		&stubTimelineRepo{},
		&stubShippingRepo{addr: addr},
		&stubExceptionRepo{},
		enc,
	)

	r := gin.New()
	r.GET("/fulfillments/:id", h.ShowDetail)

	cookie := sessionCookieFor(t, store, domain.RoleAdministrator)
	req := httptest.NewRequest(http.MethodGet, "/fulfillments/"+ff.ID.String(), nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("admin: expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	// Full values should appear
	for _, real := range []string{"Boston", "MA", "02110"} {
		if !strings.Contains(body, real) {
			t.Errorf("admin page missing full value %q", real)
		}
	}
}
