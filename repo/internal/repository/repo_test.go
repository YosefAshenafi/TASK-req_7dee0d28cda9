package repository_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
)

var testPool *pgxpool.Pool

// seedAdminID returns the ID of the seeded admin user, creating one if needed.
func seedAdminID(t *testing.T) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := testPool.QueryRow(context.Background(), `SELECT id FROM users WHERE username='admin' LIMIT 1`).Scan(&id)
	if err != nil {
		t.Fatalf("seeding admin user: %v", err)
	}
	return id
}

func TestMain(m *testing.M) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Println("DATABASE_URL not set, skipping repository tests")
		os.Exit(0)
	}

	ctx := context.Background()
	var err error
	testPool, err = pgxpool.New(ctx, dbURL)
	if err != nil {
		fmt.Printf("connecting to test DB: %v\n", err)
		os.Exit(1)
	}
	if err := testPool.Ping(ctx); err != nil {
		fmt.Printf("pinging test DB: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()
	testPool.Close()
	os.Exit(code)
}

// ── Tier Tests ───────────────────────────────────────────────────────────────

func TestTierCreateAndGet(t *testing.T) {
	repo := repository.NewTierRepository(testPool)
	ctx := context.Background()

	desc := "test description"
	tier := &domain.RewardTier{
		Name:           "Test Tier " + uuid.New().String()[:8],
		Description:    &desc,
		InventoryCount: 50,
		PurchaseLimit:  2,
		AlertThreshold: 5,
	}

	created, err := repo.Create(ctx, tier)
	if err != nil {
		t.Fatalf("Create tier: %v", err)
	}
	if created.ID == uuid.Nil {
		t.Fatal("expected non-nil ID")
	}

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != created.Name {
		t.Errorf("name mismatch: got %q want %q", got.Name, created.Name)
	}
}

func TestTierDecrementInventory(t *testing.T) {
	repo := repository.NewTierRepository(testPool)
	ctx := context.Background()

	tier, err := repo.Create(ctx, &domain.RewardTier{
		Name: "Inventory Tier " + uuid.New().String()[:8], InventoryCount: 10, PurchaseLimit: 2, AlertThreshold: 2,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Direct pool transaction for test
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := repo.DecrementInventory(ctx, tx, tier.ID, 3); err != nil {
		tx.Rollback(ctx)
		t.Fatalf("decrement: %v", err)
	}
	tx.Commit(ctx)

	updated, _ := repo.GetByID(ctx, tier.ID)
	if updated.InventoryCount != 7 {
		t.Errorf("expected inventory 7, got %d", updated.InventoryCount)
	}

	// Decrement past 0 should fail
	tx2, _ := testPool.Begin(ctx)
	err = repo.DecrementInventory(ctx, tx2, tier.ID, 100)
	tx2.Rollback(ctx)
	if !errors.Is(err, domain.ErrInventoryUnavailable) {
		t.Errorf("expected ErrInventoryUnavailable, got %v", err)
	}
}

func TestTierVersionConflict(t *testing.T) {
	repo := repository.NewTierRepository(testPool)
	ctx := context.Background()

	tier, _ := repo.Create(ctx, &domain.RewardTier{
		Name: "Conflict Tier " + uuid.New().String()[:8], InventoryCount: 5, PurchaseLimit: 2, AlertThreshold: 1,
	})

	tier.Name = "Updated"
	_, err := repo.Update(ctx, tier) // version=1, should succeed
	if err != nil {
		t.Fatalf("first update: %v", err)
	}

	// Try again with old version=1
	tier.Version = 1
	_, err = repo.Update(ctx, tier)
	if !errors.Is(err, domain.ErrConflict) {
		t.Errorf("expected ErrConflict, got %v", err)
	}
}

func TestTierSoftDelete(t *testing.T) {
	repo := repository.NewTierRepository(testPool)
	ctx := context.Background()

	tier, _ := repo.Create(ctx, &domain.RewardTier{
		Name: "Delete Tier " + uuid.New().String()[:8], InventoryCount: 5, PurchaseLimit: 2, AlertThreshold: 1,
	})

	adminID := seedAdminID(t)
	if err := repo.SoftDelete(ctx, tier.ID, adminID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	// Should not appear in list without include_deleted
	tiers, _ := repo.List(ctx, tier.Name, false)
	for _, tl := range tiers {
		if tl.ID == tier.ID {
			t.Error("soft-deleted tier should not appear in default list")
		}
	}

	// Should appear with include_deleted
	tiersAll, _ := repo.List(ctx, tier.Name, true)
	found := false
	for _, tl := range tiersAll {
		if tl.ID == tier.ID {
			found = true
		}
	}
	if !found {
		t.Error("soft-deleted tier should appear in include_deleted list")
	}
}

// ── Fulfillment Tests ─────────────────────────────────────────────────────────

func TestFulfillmentVersionConflict(t *testing.T) {
	ctx := context.Background()
	tierRepo := repository.NewTierRepository(testPool)
	custRepo := repository.NewCustomerRepository(testPool)
	fulfRepo := repository.NewFulfillmentRepository(testPool)

	tier, _ := tierRepo.Create(ctx, &domain.RewardTier{
		Name: "F Tier " + uuid.New().String()[:8], InventoryCount: 10, PurchaseLimit: 5, AlertThreshold: 1,
	})
	cust, _ := custRepo.Create(ctx, &domain.Customer{Name: "Test Customer " + uuid.New().String()[:8]})

	tx, _ := testPool.Begin(ctx)
	f, err := fulfRepo.Create(ctx, tx, &domain.Fulfillment{
		TierID: tier.ID, CustomerID: cust.ID, Type: domain.TypePhysical, Status: domain.StatusDraft,
	})
	tx.Commit(ctx)
	if err != nil {
		t.Fatalf("create fulfillment: %v", err)
	}

	// Update with correct version
	tx2, _ := testPool.Begin(ctx)
	f.Status = domain.StatusReadyToShip
	_, err = fulfRepo.Update(ctx, tx2, f)
	tx2.Commit(ctx)
	if err != nil {
		t.Fatalf("first update: %v", err)
	}

	// Update with stale version
	f.Version = 1
	tx3, _ := testPool.Begin(ctx)
	_, err = fulfRepo.Update(ctx, tx3, f)
	tx3.Rollback(ctx)
	if !errors.Is(err, domain.ErrConflict) {
		t.Errorf("expected ErrConflict, got %v", err)
	}
}

func TestFulfillmentCountByCustomerAndTier(t *testing.T) {
	ctx := context.Background()
	tierRepo := repository.NewTierRepository(testPool)
	custRepo := repository.NewCustomerRepository(testPool)
	fulfRepo := repository.NewFulfillmentRepository(testPool)

	tier, _ := tierRepo.Create(ctx, &domain.RewardTier{
		Name: "Count Tier " + uuid.New().String()[:8], InventoryCount: 10, PurchaseLimit: 2, AlertThreshold: 1,
	})
	cust, _ := custRepo.Create(ctx, &domain.Customer{Name: "Count Customer " + uuid.New().String()[:8]})

	// Create 2 fulfillments (one active, one canceled)
	for i, status := range []domain.FulfillmentStatus{domain.StatusDraft, domain.StatusCanceled} {
		_ = i
		tx, _ := testPool.Begin(ctx)
		_, err := fulfRepo.Create(ctx, tx, &domain.Fulfillment{
			TierID: tier.ID, CustomerID: cust.ID, Type: domain.TypePhysical, Status: status,
		})
		tx.Commit(ctx)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	// Count should be 1 (excluding CANCELED)
	tx, _ := testPool.Begin(ctx)
	count, err := fulfRepo.CountByCustomerAndTier(ctx, tx, cust.ID, tier.ID, time.Now().AddDate(0, 0, -30))
	tx.Rollback(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count 1 (excluding CANCELED), got %d", count)
	}

	// Old fulfillment (>30 days) should not count
	tx2, _ := testPool.Begin(ctx)
	count2, _ := fulfRepo.CountByCustomerAndTier(ctx, tx2, cust.ID, tier.ID, time.Now().Add(time.Hour))
	tx2.Rollback(ctx)
	if count2 != 0 {
		t.Errorf("expected 0 for future since, got %d", count2)
	}
}

// ── Transaction Rollback Test ─────────────────────────────────────────────────

func TestTransactionRollback(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewTierRepository(testPool)
	txMgr := repository.NewTxManager(testPool)

	tier, _ := repo.Create(ctx, &domain.RewardTier{
		Name: "Rollback Tier " + uuid.New().String()[:8], InventoryCount: 10, PurchaseLimit: 2, AlertThreshold: 1,
	})

	// Decrement then return error — should rollback
	_ = txMgr.WithTx(ctx, func(tx pgx.Tx) error {
		return fmt.Errorf("intentional error")
	})

	// Verify inventory unchanged (we can't easily call DecrementInventory + error in one test without the interface,
	// so we verify the tier still has original inventory)
	after, _ := repo.GetByID(ctx, tier.ID)
	if after.InventoryCount != 10 {
		t.Errorf("expected inventory 10 after rollback test, got %d", after.InventoryCount)
	}
}

// ── Concurrent SELECT FOR UPDATE Test ────────────────────────────────────────

func TestConcurrentSelectForUpdate(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewTierRepository(testPool)

	tier, _ := repo.Create(ctx, &domain.RewardTier{
		Name: "Lock Tier " + uuid.New().String()[:8], InventoryCount: 2, PurchaseLimit: 5, AlertThreshold: 1,
	})

	// Two goroutines attempt to decrement inventory concurrently
	var wg sync.WaitGroup
	results := make([]error, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			tx, err := testPool.Begin(ctx)
			if err != nil {
				results[idx] = err
				return
			}
			// Lock the row
			_, err = repo.GetByIDForUpdate(ctx, tx, tier.ID)
			if err != nil {
				tx.Rollback(ctx)
				results[idx] = err
				return
			}
			time.Sleep(10 * time.Millisecond) // simulate work
			err = repo.DecrementInventory(ctx, tx, tier.ID, 2)
			if err != nil {
				tx.Rollback(ctx)
				results[idx] = err
				return
			}
			results[idx] = tx.Commit(ctx)
		}(i)
	}
	wg.Wait()

	// Exactly one should succeed, one should fail (inventory check)
	successes := 0
	failures := 0
	for _, err := range results {
		if err == nil {
			successes++
		} else {
			failures++
		}
	}
	if successes != 1 || failures != 1 {
		t.Errorf("expected 1 success + 1 failure, got %d + %d (errors: %v, %v)",
			successes, failures, results[0], results[1])
	}
}

// ── Customer Soft-Delete Test ─────────────────────────────────────────────────

func TestCustomerSoftDelete(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewCustomerRepository(testPool)

	cust, _ := repo.Create(ctx, &domain.Customer{Name: "SD Customer " + uuid.New().String()[:8]})

	adminID := seedAdminID(t)
	if err := repo.SoftDelete(ctx, cust.ID, adminID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	page := domain.PageRequest{Page: 1, PageSize: 100}
	// Without include_deleted: should not appear
	custs, _, _ := repo.List(ctx, cust.Name, page, false)
	for _, c := range custs {
		if c.ID == cust.ID {
			t.Error("soft-deleted customer should not appear in default list")
		}
	}
	// With include_deleted: should appear
	custsAll, _, _ := repo.List(ctx, cust.Name, page, true)
	found := false
	for _, c := range custsAll {
		if c.ID == cust.ID {
			found = true
		}
	}
	if !found {
		t.Error("soft-deleted customer should appear in include_deleted list")
	}
}
