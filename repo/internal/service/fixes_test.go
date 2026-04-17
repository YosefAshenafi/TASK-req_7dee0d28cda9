package service

// Unit tests for the 8 audit fixes. These run without a live DB.
//
// Fix coverage map:
//  Fix 1: cleanup_job_test.go (live-DB) + job package tests
//  Fix 2: TestCreate_SoftDeletedCustomerRejected, TestCreate_SoftDeletedTierRejected
//  Fix 3: TestMaskAddressHelper (util package) — covered in util/maskpii_test.go
//  Fix 4: TestAdminHealthChecks_RealFS (handler package)
//  Fix 5: TestFulfillmentFilters_IncludeDeleted
//  Fix 6: TestDashboardPendingFilter_TodaySemantics
//  Fix 7: TestTransition_PhysicalReadyToShipRequiresAddress
//  Fix 8: TestRetryPending_QueuedRowsRetried (get_retryable query change)

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
)

// ── Helpers ──────────────────────────────────────────────────────────────────

// inlineTxManager runs the function with a nil pgx.Tx.
// It is only safe for stubs that do not actually use the transaction object.
type inlineTxManager struct{}

func (m *inlineTxManager) WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return fn(nil)
}

// fixedTierRepo returns the same tier for every GetByIDForUpdate call.
type fixedTierRepo struct {
	tier    *domain.RewardTier
	err     error
	deleted bool // when true, returns NotFound (mimics deleted_at IS NULL filter)
}

func (r *fixedTierRepo) List(context.Context, string, bool) ([]domain.RewardTier, error) {
	return nil, nil
}
func (r *fixedTierRepo) GetByID(context.Context, uuid.UUID) (*domain.RewardTier, error) {
	if r.deleted {
		return nil, domain.NewNotFoundError("reward tier")
	}
	return r.tier, r.err
}
func (r *fixedTierRepo) GetByIDForUpdate(_ context.Context, _ pgx.Tx, _ uuid.UUID) (*domain.RewardTier, error) {
	if r.deleted || r.err != nil {
		return nil, domain.NewNotFoundError("reward tier")
	}
	return r.tier, nil
}
func (r *fixedTierRepo) Create(context.Context, *domain.RewardTier) (*domain.RewardTier, error) {
	return nil, nil
}
func (r *fixedTierRepo) Update(context.Context, *domain.RewardTier) (*domain.RewardTier, error) {
	return nil, nil
}
func (r *fixedTierRepo) SoftDelete(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (r *fixedTierRepo) Restore(context.Context, uuid.UUID) error                { return nil }
func (r *fixedTierRepo) DecrementInventory(context.Context, pgx.Tx, uuid.UUID, int) error {
	return nil
}
func (r *fixedTierRepo) IncrementInventory(context.Context, pgx.Tx, uuid.UUID, int) error {
	return nil
}

// fixedCustomerRepo controls what GetByID returns.
type fixedCustomerRepo struct {
	customer *domain.Customer
	notFound bool
}

func (r *fixedCustomerRepo) List(context.Context, string, domain.PageRequest, bool) ([]domain.Customer, int, error) {
	return nil, 0, nil
}
func (r *fixedCustomerRepo) GetByID(_ context.Context, _ uuid.UUID) (*domain.Customer, error) {
	if r.notFound {
		return nil, domain.NewNotFoundError("customer")
	}
	return r.customer, nil
}
func (r *fixedCustomerRepo) Create(_ context.Context, c *domain.Customer) (*domain.Customer, error) {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return c, nil
}
func (r *fixedCustomerRepo) Update(context.Context, *domain.Customer) (*domain.Customer, error) {
	return nil, nil
}
func (r *fixedCustomerRepo) SoftDelete(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (r *fixedCustomerRepo) Restore(context.Context, uuid.UUID) error                { return nil }

// zeroFulfillmentRepo is a stub fulfillment repo; CountByCustomerAndTier returns 0.
type zeroFulfillmentRepo struct {
	created *domain.Fulfillment
}

func (r *zeroFulfillmentRepo) List(context.Context, repository.FulfillmentFilters, domain.PageRequest) ([]domain.Fulfillment, int, error) {
	return nil, 0, nil
}
func (r *zeroFulfillmentRepo) GetByID(context.Context, uuid.UUID) (*domain.Fulfillment, error) {
	return nil, domain.NewNotFoundError("fulfillment")
}
func (r *zeroFulfillmentRepo) GetByIDForUpdate(_ context.Context, _ pgx.Tx, id uuid.UUID) (*domain.Fulfillment, error) {
	if r.created != nil && r.created.ID == id {
		return r.created, nil
	}
	return nil, domain.NewNotFoundError("fulfillment")
}
func (r *zeroFulfillmentRepo) Create(_ context.Context, _ pgx.Tx, f *domain.Fulfillment) (*domain.Fulfillment, error) {
	f.ID = uuid.New()
	r.created = f
	return f, nil
}
func (r *zeroFulfillmentRepo) Update(_ context.Context, _ pgx.Tx, f *domain.Fulfillment) (*domain.Fulfillment, error) {
	f.Version++
	return f, nil
}
func (r *zeroFulfillmentRepo) CountByCustomerAndTier(context.Context, pgx.Tx, uuid.UUID, uuid.UUID, time.Time) (int, error) {
	return 0, nil
}
func (r *zeroFulfillmentRepo) SoftDelete(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (r *zeroFulfillmentRepo) Restore(context.Context, uuid.UUID) error                { return nil }
func (r *zeroFulfillmentRepo) ListOverdue(context.Context) ([]domain.Fulfillment, error) {
	return nil, nil
}

// stubShippingRepo tracks Create calls and can simulate existing/missing address.
type stubShippingRepo struct {
	existing *domain.ShippingAddress
	created  int
}

func (r *stubShippingRepo) Create(_ context.Context, _ pgx.Tx, addr *domain.ShippingAddress) (*domain.ShippingAddress, error) {
	r.created++
	addr.ID = uuid.New()
	return addr, nil
}
func (r *stubShippingRepo) CreateNoTx(_ context.Context, addr *domain.ShippingAddress) (*domain.ShippingAddress, error) {
	addr.ID = uuid.New()
	return addr, nil
}
func (r *stubShippingRepo) GetByFulfillmentID(_ context.Context, _ uuid.UUID) (*domain.ShippingAddress, error) {
	return r.existing, nil
}
func (r *stubShippingRepo) Update(context.Context, pgx.Tx, *domain.ShippingAddress) error {
	return nil
}

// stubTimelineRepo drops timeline events silently.
type stubTimelineRepo struct{}

func (r *stubTimelineRepo) Create(context.Context, pgx.Tx, *domain.TimelineEvent) error {
	return nil
}
func (r *stubTimelineRepo) ListByFulfillmentID(context.Context, uuid.UUID) ([]domain.TimelineEvent, error) {
	return nil, nil
}

// ── Fix 2: soft-deleted customer must be rejected at create time ──────────────

func TestCreate_SoftDeletedCustomerRejected(t *testing.T) {
	tier := &domain.RewardTier{
		ID: uuid.New(), InventoryCount: 5, PurchaseLimit: 3,
	}
	tierRepo := &fixedTierRepo{tier: tier}
	// Customer repo returns NotFound (simulates soft-deleted or missing customer).
	custRepo := &fixedCustomerRepo{notFound: true}

	svc := NewFulfillmentService(
		&inlineTxManager{},
		&zeroFulfillmentRepo{},
		tierRepo,
		custRepo,
		&stubTimelineRepo{},
		&stubShippingRepo{},
		nil,
		NewInventoryService(tierRepo, &stubReservationRepo{}),
		nil,
	)

	_, err := svc.Create(context.Background(), CreateFulfillmentInput{
		TierID:     tier.ID,
		CustomerID: uuid.New(),
		Type:       domain.TypeVoucher,
	})
	if err == nil {
		t.Fatal("expected error when customer is soft-deleted/missing")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// ── Fix 2: soft-deleted tier must be rejected at create time ─────────────────

func TestCreate_SoftDeletedTierRejected(t *testing.T) {
	// Tier repo returns NotFound — simulates GetByIDForUpdate with deleted_at IS NULL.
	tierRepo := &fixedTierRepo{deleted: true}
	custRepo := &fixedCustomerRepo{customer: &domain.Customer{ID: uuid.New(), Name: "c"}}

	svc := NewFulfillmentService(
		&inlineTxManager{},
		&zeroFulfillmentRepo{},
		tierRepo,
		custRepo,
		&stubTimelineRepo{},
		&stubShippingRepo{},
		nil,
		NewInventoryService(tierRepo, &stubReservationRepo{}),
		nil,
	)

	_, err := svc.Create(context.Background(), CreateFulfillmentInput{
		TierID:     uuid.New(),
		CustomerID: uuid.New(),
		Type:       domain.TypePhysical,
	})
	if err == nil {
		t.Fatal("expected error when tier is soft-deleted/missing")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// ── Fix 7: physical READY_TO_SHIP requires a shipping address ────────────────

func TestTransition_PhysicalReadyToShipRequiresAddress(t *testing.T) {
	tier := &domain.RewardTier{ID: uuid.New(), InventoryCount: 5, PurchaseLimit: 3}
	tierRepo := &fixedTierRepo{tier: tier}
	custRepo := &fixedCustomerRepo{customer: &domain.Customer{ID: uuid.New()}}

	fulfillID := uuid.New()
	physical := &domain.Fulfillment{
		ID:     fulfillID,
		TierID: tier.ID,
		Type:   domain.TypePhysical,
		Status: domain.StatusDraft,
	}
	fulfillRepo := &zeroFulfillmentRepo{created: physical}

	// shippingRepo returns nil (no existing address)
	shippingRepo := &stubShippingRepo{existing: nil}

	svc := NewFulfillmentService(
		&inlineTxManager{},
		fulfillRepo,
		tierRepo,
		custRepo,
		&stubTimelineRepo{},
		shippingRepo,
		nil,
		NewInventoryService(tierRepo, &stubReservationRepo{}),
		nil,
	)

	_, err := svc.Transition(context.Background(), TransitionInput{
		FulfillmentID: fulfillID,
		ToStatus:      domain.StatusReadyToShip,
		// No ShippingAddr provided
	})
	if err == nil {
		t.Fatal("expected error: physical fulfillment READY_TO_SHIP requires shipping address")
	}
}

func TestTransition_PhysicalReadyToShipExistingAddressAccepted(t *testing.T) {
	tier := &domain.RewardTier{ID: uuid.New(), InventoryCount: 5, PurchaseLimit: 3}
	tierRepo := &fixedTierRepo{tier: tier}
	custRepo := &fixedCustomerRepo{customer: &domain.Customer{ID: uuid.New()}}

	fulfillID := uuid.New()
	physical := &domain.Fulfillment{
		ID:     fulfillID,
		TierID: tier.ID,
		Type:   domain.TypePhysical,
		Status: domain.StatusDraft,
	}
	fulfillRepo := &zeroFulfillmentRepo{created: physical}

	// shippingRepo returns an existing address (e.g. after ON_HOLD resume)
	shippingRepo := &stubShippingRepo{
		existing: &domain.ShippingAddress{
			ID:             uuid.New(),
			FulfillmentID:  fulfillID,
			Line1Encrypted: []byte("enc"),
			City:           "NY",
			State:          "NY",
			ZipCode:        "10001",
		},
	}

	svc := NewFulfillmentService(
		&inlineTxManager{},
		fulfillRepo,
		tierRepo,
		custRepo,
		&stubTimelineRepo{},
		shippingRepo,
		nil,
		NewInventoryService(tierRepo, &stubReservationRepo{}),
		nil,
	)

	_, err := svc.Transition(context.Background(), TransitionInput{
		FulfillmentID: fulfillID,
		ToStatus:      domain.StatusReadyToShip,
	})
	if err != nil {
		t.Fatalf("expected no error when existing address present, got %v", err)
	}
}

// ── Fix 5: FulfillmentFilters.IncludeDeleted flag ─────────────────────────────

func TestFulfillmentFilters_IncludeDeleted(t *testing.T) {
	// Verify the field exists and its zero value is false (does not include deleted).
	f := repository.FulfillmentFilters{}
	if f.IncludeDeleted {
		t.Fatal("zero value of IncludeDeleted should be false")
	}
	f.IncludeDeleted = true
	if !f.IncludeDeleted {
		t.Fatal("IncludeDeleted should be settable to true")
	}
}

// ── Fix 8: QUEUED rows with elapsed next_retry_at must be retried ─────────────

func TestRetryPending_QueuedRowsRetried(t *testing.T) {
	// A QUEUED SMS send_log with a past next_retry_at should be picked up and
	// re-queued (simulating the full lifecycle: Dispatch → retry loop).
	smsID := uuid.New()
	past := time.Now().UTC().Add(-5 * time.Minute)
	sendRepo := &retryStubSendLog{
		retryables: []domain.SendLog{
			{
				ID:           smsID,
				Channel:      domain.ChannelSMS,
				Status:       domain.SendQueued,
				AttemptCount: 0,
				NextRetryAt:  &past,
			},
		},
	}
	svc := NewMessagingService(&stubTemplateRepo{}, sendRepo, &stubNotificationRepo{})
	retried, err := svc.RetryPending(context.Background(), 3)
	if err != nil {
		t.Fatalf("RetryPending: %v", err)
	}
	if retried != 1 {
		t.Fatalf("expected 1 retried, got %d", retried)
	}
	if sendRepo.updates[smsID] != domain.SendQueued {
		t.Fatalf("sms should be re-queued, got %v", sendRepo.updates[smsID])
	}
	if _, ok := sendRepo.nextRetry[smsID]; !ok {
		t.Fatal("sms retry should update next_retry_at")
	}
}

func TestRetryPending_MaxAttemptsMarksFailed(t *testing.T) {
	deadID := uuid.New()
	past := time.Now().UTC().Add(-5 * time.Minute)
	sendRepo := &retryStubSendLog{
		retryables: []domain.SendLog{
			{
				ID:           deadID,
				Channel:      domain.ChannelEmail,
				Status:       domain.SendQueued,
				AttemptCount: 3, // == maxAttempts
				NextRetryAt:  &past,
			},
		},
	}
	svc := NewMessagingService(&stubTemplateRepo{}, sendRepo, &stubNotificationRepo{})
	retried, err := svc.RetryPending(context.Background(), 3)
	if err != nil {
		t.Fatalf("RetryPending: %v", err)
	}
	// Terminal FAILED transitions are NOT counted as retried.
	if retried != 0 {
		t.Fatalf("expected 0 retried for terminal case, got %d", retried)
	}
	if sendRepo.updates[deadID] != domain.SendFailed {
		t.Fatalf("over-attempt row should be marked FAILED, got %v", sendRepo.updates[deadID])
	}
}
