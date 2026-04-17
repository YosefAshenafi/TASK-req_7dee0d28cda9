package service_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
	"github.com/fulfillops/fulfillops/internal/service"
)

var svcPool *pgxpool.Pool

func TestMain(m *testing.M) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Println("DATABASE_URL not set, skipping service tests")
		os.Exit(0)
	}
	ctx := context.Background()
	var err error
	svcPool, err = pgxpool.New(ctx, dbURL)
	if err != nil {
		fmt.Printf("connecting: %v\n", err)
		os.Exit(1)
	}
	if err := svcPool.Ping(ctx); err != nil {
		fmt.Printf("ping: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	svcPool.Close()
	os.Exit(code)
}

func adminUserID(t *testing.T) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := svcPool.QueryRow(context.Background(), `SELECT id FROM users WHERE username='admin' LIMIT 1`).Scan(&id); err != nil {
		t.Fatalf("get admin: %v", err)
	}
	return id
}

func buildFulfillmentService(t *testing.T) service.FulfillmentService {
	t.Helper()
	txMgr := repository.NewTxManager(svcPool)
	tierRepo := repository.NewTierRepository(svcPool)
	fulfillRepo := repository.NewFulfillmentRepository(svcPool)
	timelineRepo := repository.NewTimelineRepository(svcPool)
	reservationRepo := repository.NewReservationRepository()
	invSvc := service.NewInventoryService(tierRepo, reservationRepo)
	auditRepo := repository.NewAuditRepository(svcPool)
	auditSvc := service.NewAuditService(auditRepo)
	shippingRepo := repository.NewShippingAddressRepository(svcPool)
	notifRepo := repository.NewNotificationRepository(svcPool)
	return service.NewFulfillmentService(txMgr, fulfillRepo, tierRepo, timelineRepo, shippingRepo, notifRepo, invSvc, auditSvc)
}

// ── Transition Tests ──────────────────────────────────────────────────────────

func TestFulfillmentAllTransitions(t *testing.T) {
	ctx := context.Background()
	ctx = service.WithUserID(ctx, adminUserID(t))
	svc := buildFulfillmentService(t)
	tierRepo := repository.NewTierRepository(svcPool)
	custRepo := repository.NewCustomerRepository(svcPool)

	tier, _ := tierRepo.Create(ctx, &domain.RewardTier{
		Name: "Trans Tier " + uuid.New().String()[:8], InventoryCount: 10, PurchaseLimit: 5, AlertThreshold: 1,
	})
	cust, _ := custRepo.Create(ctx, &domain.Customer{Name: "Trans Cust " + uuid.New().String()[:8]})

	f, err := svc.Create(ctx, service.CreateFulfillmentInput{
		TierID: tier.ID, CustomerID: cust.ID, Type: domain.TypePhysical,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if f.Status != domain.StatusDraft {
		t.Fatalf("expected DRAFT, got %s", f.Status)
	}

	// DRAFT → READY_TO_SHIP
	f, err = svc.Transition(ctx, service.TransitionInput{
		FulfillmentID: f.ID, ToStatus: domain.StatusReadyToShip,
	})
	if err != nil {
		t.Fatalf("READY_TO_SHIP: %v", err)
	}

	// READY_TO_SHIP → SHIPPED (carrier + tracking required)
	carrier := "FedEx"
	tracking := "TRACK12345678"
	f, err = svc.Transition(ctx, service.TransitionInput{
		FulfillmentID: f.ID, ToStatus: domain.StatusShipped,
		CarrierName: &carrier, TrackingNumber: &tracking,
	})
	if err != nil {
		t.Fatalf("SHIPPED: %v", err)
	}

	// SHIPPED → DELIVERED
	f, err = svc.Transition(ctx, service.TransitionInput{
		FulfillmentID: f.ID, ToStatus: domain.StatusDelivered,
	})
	if err != nil {
		t.Fatalf("DELIVERED: %v", err)
	}

	// DELIVERED → COMPLETED
	f, err = svc.Transition(ctx, service.TransitionInput{
		FulfillmentID: f.ID, ToStatus: domain.StatusCompleted,
	})
	if err != nil {
		t.Fatalf("COMPLETED: %v", err)
	}
	if f.Status != domain.StatusCompleted {
		t.Errorf("expected COMPLETED, got %s", f.Status)
	}
}

func TestTransitionTerminalReturnsError(t *testing.T) {
	ctx := context.Background()
	ctx = service.WithUserID(ctx, adminUserID(t))
	svc := buildFulfillmentService(t)
	tierRepo := repository.NewTierRepository(svcPool)
	custRepo := repository.NewCustomerRepository(svcPool)

	tier, _ := tierRepo.Create(ctx, &domain.RewardTier{
		Name: "Terminal Tier " + uuid.New().String()[:8], InventoryCount: 10, PurchaseLimit: 5, AlertThreshold: 1,
	})
	cust, _ := custRepo.Create(ctx, &domain.Customer{Name: "Terminal Cust " + uuid.New().String()[:8]})

	f, _ := svc.Create(ctx, service.CreateFulfillmentInput{
		TierID: tier.ID, CustomerID: cust.ID, Type: domain.TypePhysical,
	})

	// Cancel it.
	reason := "test cancel"
	_, _ = svc.Transition(ctx, service.TransitionInput{
		FulfillmentID: f.ID, ToStatus: domain.StatusReadyToShip,
	})
	f2, err := svc.Transition(ctx, service.TransitionInput{
		FulfillmentID: f.ID, ToStatus: domain.StatusCanceled, Reason: &reason,
	})
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	_ = f2

	// Any further transition from CANCELED should fail.
	_, err = svc.Transition(ctx, service.TransitionInput{
		FulfillmentID: f.ID, ToStatus: domain.StatusDraft,
	})
	if !errors.Is(err, domain.ErrInvalidTransition) {
		t.Errorf("expected ErrInvalidTransition from CANCELED, got %v", err)
	}
}

func TestTransitionShippedWithoutTracking(t *testing.T) {
	ctx := context.Background()
	ctx = service.WithUserID(ctx, adminUserID(t))
	svc := buildFulfillmentService(t)
	tierRepo := repository.NewTierRepository(svcPool)
	custRepo := repository.NewCustomerRepository(svcPool)

	tier, _ := tierRepo.Create(ctx, &domain.RewardTier{
		Name: "Track Tier " + uuid.New().String()[:8], InventoryCount: 5, PurchaseLimit: 5, AlertThreshold: 1,
	})
	cust, _ := custRepo.Create(ctx, &domain.Customer{Name: "Track Cust " + uuid.New().String()[:8]})

	f, _ := svc.Create(ctx, service.CreateFulfillmentInput{
		TierID: tier.ID, CustomerID: cust.ID, Type: domain.TypePhysical,
	})
	_, _ = svc.Transition(ctx, service.TransitionInput{FulfillmentID: f.ID, ToStatus: domain.StatusReadyToShip})

	// Try to ship without tracking number.
	carrier := "UPS"
	_, err := svc.Transition(ctx, service.TransitionInput{
		FulfillmentID: f.ID, ToStatus: domain.StatusShipped,
		CarrierName: &carrier, // no tracking
	})
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("expected ErrValidation for missing tracking, got %v", err)
	}
}

func TestInventoryDecrementedOnCreate(t *testing.T) {
	ctx := context.Background()
	ctx = service.WithUserID(ctx, adminUserID(t))
	svc := buildFulfillmentService(t)
	tierRepo := repository.NewTierRepository(svcPool)
	custRepo := repository.NewCustomerRepository(svcPool)

	tier, _ := tierRepo.Create(ctx, &domain.RewardTier{
		Name: "Inv Tier " + uuid.New().String()[:8], InventoryCount: 3, PurchaseLimit: 5, AlertThreshold: 1,
	})
	cust, _ := custRepo.Create(ctx, &domain.Customer{Name: "Inv Cust " + uuid.New().String()[:8]})

	_, err := svc.Create(ctx, service.CreateFulfillmentInput{
		TierID: tier.ID, CustomerID: cust.ID, Type: domain.TypePhysical,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	updated, _ := tierRepo.GetByID(ctx, tier.ID)
	if updated.InventoryCount != 2 {
		t.Errorf("expected inventory 2, got %d", updated.InventoryCount)
	}
}

func TestInventoryRestoredOnCancel(t *testing.T) {
	ctx := context.Background()
	ctx = service.WithUserID(ctx, adminUserID(t))
	svc := buildFulfillmentService(t)
	tierRepo := repository.NewTierRepository(svcPool)
	custRepo := repository.NewCustomerRepository(svcPool)

	tier, _ := tierRepo.Create(ctx, &domain.RewardTier{
		Name: "Cancel Tier " + uuid.New().String()[:8], InventoryCount: 2, PurchaseLimit: 5, AlertThreshold: 1,
	})
	cust, _ := custRepo.Create(ctx, &domain.Customer{Name: "Cancel Cust " + uuid.New().String()[:8]})

	f, _ := svc.Create(ctx, service.CreateFulfillmentInput{
		TierID: tier.ID, CustomerID: cust.ID, Type: domain.TypePhysical,
	})
	// inventory now at 1

	_, _ = svc.Transition(ctx, service.TransitionInput{FulfillmentID: f.ID, ToStatus: domain.StatusReadyToShip})
	reason := "changed mind"
	_, err := svc.Transition(ctx, service.TransitionInput{
		FulfillmentID: f.ID, ToStatus: domain.StatusCanceled, Reason: &reason,
	})
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}

	restored, _ := tierRepo.GetByID(ctx, tier.ID)
	if restored.InventoryCount != 2 {
		t.Errorf("expected inventory restored to 2, got %d", restored.InventoryCount)
	}
}

func TestPurchaseLimitEnforced(t *testing.T) {
	ctx := context.Background()
	ctx = service.WithUserID(ctx, adminUserID(t))
	svc := buildFulfillmentService(t)
	tierRepo := repository.NewTierRepository(svcPool)
	custRepo := repository.NewCustomerRepository(svcPool)

	tier, _ := tierRepo.Create(ctx, &domain.RewardTier{
		Name: "Limit Tier " + uuid.New().String()[:8], InventoryCount: 10, PurchaseLimit: 2, AlertThreshold: 1,
	})
	cust, _ := custRepo.Create(ctx, &domain.Customer{Name: "Limit Cust " + uuid.New().String()[:8]})

	// First two succeed.
	for i := 0; i < 2; i++ {
		_, err := svc.Create(ctx, service.CreateFulfillmentInput{
			TierID: tier.ID, CustomerID: cust.ID, Type: domain.TypePhysical,
		})
		if err != nil {
			t.Fatalf("create %d: %v", i+1, err)
		}
	}

	// Third should fail.
	_, err := svc.Create(ctx, service.CreateFulfillmentInput{
		TierID: tier.ID, CustomerID: cust.ID, Type: domain.TypePhysical,
	})
	if !errors.Is(err, domain.ErrPurchaseLimitReached) {
		t.Errorf("expected ErrPurchaseLimitReached, got %v", err)
	}
}

func TestCanceledNotCountedTowardLimit(t *testing.T) {
	ctx := context.Background()
	ctx = service.WithUserID(ctx, adminUserID(t))
	svc := buildFulfillmentService(t)
	tierRepo := repository.NewTierRepository(svcPool)
	custRepo := repository.NewCustomerRepository(svcPool)

	tier, _ := tierRepo.Create(ctx, &domain.RewardTier{
		Name: "CancelLimit Tier " + uuid.New().String()[:8], InventoryCount: 10, PurchaseLimit: 2, AlertThreshold: 1,
	})
	cust, _ := custRepo.Create(ctx, &domain.Customer{Name: "CancelLimit Cust " + uuid.New().String()[:8]})

	// Create 2 and cancel both.
	for i := 0; i < 2; i++ {
		f, err := svc.Create(ctx, service.CreateFulfillmentInput{
			TierID: tier.ID, CustomerID: cust.ID, Type: domain.TypePhysical,
		})
		if err != nil {
			t.Fatalf("create %d: %v", i+1, err)
		}
		_, _ = svc.Transition(ctx, service.TransitionInput{FulfillmentID: f.ID, ToStatus: domain.StatusReadyToShip})
		reason := "canceled for test"
		_, _ = svc.Transition(ctx, service.TransitionInput{
			FulfillmentID: f.ID, ToStatus: domain.StatusCanceled, Reason: &reason,
		})
	}

	// Third should succeed since canceled don't count.
	_, err := svc.Create(ctx, service.CreateFulfillmentInput{
		TierID: tier.ID, CustomerID: cust.ID, Type: domain.TypePhysical,
	})
	if err != nil {
		t.Errorf("expected success after cancellations, got %v", err)
	}
}

func TestCreateWhenInventoryZero(t *testing.T) {
	ctx := context.Background()
	ctx = service.WithUserID(ctx, adminUserID(t))
	svc := buildFulfillmentService(t)
	tierRepo := repository.NewTierRepository(svcPool)
	custRepo := repository.NewCustomerRepository(svcPool)

	tier, _ := tierRepo.Create(ctx, &domain.RewardTier{
		Name: "Zero Tier " + uuid.New().String()[:8], InventoryCount: 0, PurchaseLimit: 5, AlertThreshold: 1,
	})
	cust, _ := custRepo.Create(ctx, &domain.Customer{Name: "Zero Cust " + uuid.New().String()[:8]})

	_, err := svc.Create(ctx, service.CreateFulfillmentInput{
		TierID: tier.ID, CustomerID: cust.ID, Type: domain.TypePhysical,
	})
	if !errors.Is(err, domain.ErrInventoryUnavailable) {
		t.Errorf("expected ErrInventoryUnavailable, got %v", err)
	}
}

func TestStaleVersionConflict(t *testing.T) {
	ctx := context.Background()
	ctx = service.WithUserID(ctx, adminUserID(t))
	svc := buildFulfillmentService(t)
	tierRepo := repository.NewTierRepository(svcPool)
	custRepo := repository.NewCustomerRepository(svcPool)
	fulfillRepo := repository.NewFulfillmentRepository(svcPool)

	tier, _ := tierRepo.Create(ctx, &domain.RewardTier{
		Name: "Stale Tier " + uuid.New().String()[:8], InventoryCount: 10, PurchaseLimit: 5, AlertThreshold: 1,
	})
	cust, _ := custRepo.Create(ctx, &domain.Customer{Name: "Stale Cust " + uuid.New().String()[:8]})

	f, _ := svc.Create(ctx, service.CreateFulfillmentInput{
		TierID: tier.ID, CustomerID: cust.ID, Type: domain.TypePhysical,
	})

	// Advance version by making a real transition.
	_, _ = svc.Transition(ctx, service.TransitionInput{
		FulfillmentID: f.ID, ToStatus: domain.StatusReadyToShip,
	})

	// Force stale version on f (version 1 was already consumed).
	stale, _ := fulfillRepo.GetByID(ctx, f.ID)
	stale.Version = 1 // intentionally stale

	tx, _ := svcPool.Begin(ctx)
	_, err := fulfillRepo.Update(ctx, tx, stale)
	tx.Rollback(ctx)
	if !errors.Is(err, domain.ErrConflict) {
		t.Errorf("expected ErrConflict, got %v", err)
	}
}

// ── SLA Tests ─────────────────────────────────────────────────────────────────

func TestSLAPhysical(t *testing.T) {
	ctx := context.Background()
	settingRepo := repository.NewSystemSettingRepository(svcPool)
	blackoutRepo := repository.NewBlackoutDateRepository(svcPool)
	slaSvc := service.NewSLAService(settingRepo, blackoutRepo)

	now := time.Now().UTC()
	deadline, err := slaSvc.CalculateDeadline(ctx, domain.TypePhysical, now)
	if err != nil {
		t.Fatalf("CalculateDeadline: %v", err)
	}
	expected := now.Add(48 * time.Hour)
	if diff := deadline.Sub(expected); diff > time.Second || diff < -time.Second {
		t.Errorf("physical deadline: expected %v got %v (diff %v)", expected, deadline, diff)
	}
}

func TestSLAVoucher(t *testing.T) {
	ctx := context.Background()
	settingRepo := repository.NewSystemSettingRepository(svcPool)
	blackoutRepo := repository.NewBlackoutDateRepository(svcPool)
	slaSvc := service.NewSLAService(settingRepo, blackoutRepo)

	// Friday 17:00 ET → deadline should be Monday 11:00 ET (skipping weekend)
	// Use a fixed timezone for the test
	loc, _ := time.LoadLocation("America/New_York")
	// Find the next Friday
	now := time.Now().In(loc)
	daysUntilFriday := (5 - int(now.Weekday()) + 7) % 7
	if daysUntilFriday == 0 {
		daysUntilFriday = 7
	}
	friday17 := time.Date(now.Year(), now.Month(), now.Day()+daysUntilFriday, 17, 0, 0, 0, loc)

	deadline, err := slaSvc.CalculateDeadline(ctx, domain.TypeVoucher, friday17)
	if err != nil {
		t.Fatalf("CalculateDeadline voucher: %v", err)
	}

	// Expected: Monday 11:00 (Fri 17:00 → close at 18:00 = 1h, need 3 more = Mon 08:00+3h = 11:00)
	monday11 := time.Date(friday17.Year(), friday17.Month(), friday17.Day()+3, 11, 0, 0, 0, loc)
	if diff := deadline.Sub(monday11.UTC()); diff > time.Minute || diff < -time.Minute {
		t.Errorf("voucher deadline: expected %v got %v (diff %v)", monday11.UTC(), deadline, diff)
	}
}
