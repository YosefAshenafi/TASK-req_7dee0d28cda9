package job

// Fix 1 — FK-safe cleanup.
// Verifies that the cleanup job handles the FK dependency chain correctly by
// running against a live database and inserting a fulfillment with related rows.

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCleanupJob_PurgesFulfillmentWithRelatedRows(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set — skipping FK cleanup test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	defer pool.Close()

	// Resolve or create a usable admin user.
	var adminID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM users LIMIT 1`).Scan(&adminID); err != nil {
		t.Fatalf("no users in DB: %v", err)
	}

	// Create a tier.
	tierID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO reward_tiers (id,name,inventory_count,purchase_limit,alert_threshold,version,created_at,updated_at)
		 VALUES ($1,'CleanupTestTier',10,5,1,1,NOW(),NOW())`, tierID); err != nil {
		t.Fatalf("insert tier: %v", err)
	}

	// Create a customer.
	custID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO customers (id,name,version,created_at,updated_at)
		 VALUES ($1,'CleanupCust',1,NOW(),NOW())`, custID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}

	// Create a fulfillment.
	fulfillID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO fulfillments (id,tier_id,customer_id,type,status,version,created_at,updated_at)
		 VALUES ($1,$2,$3,'PHYSICAL','DRAFT',1,NOW(),NOW())`, fulfillID, tierID, custID); err != nil {
		t.Fatalf("insert fulfillment: %v", err)
	}

	// Create a timeline event (FK → fulfillments).
	if _, err := pool.Exec(ctx,
		`INSERT INTO fulfillment_timeline (id,fulfillment_id,to_status,changed_at)
		 VALUES ($1,$2,'DRAFT',NOW())`, uuid.New(), fulfillID); err != nil {
		t.Fatalf("insert timeline: %v", err)
	}

	// Create a fulfillment exception (FK → fulfillments).
	exID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO fulfillment_exceptions (id,fulfillment_id,type,status,created_at,updated_at)
		 VALUES ($1,$2,'MANUAL','OPEN',NOW(),NOW())`, exID, fulfillID); err != nil {
		t.Fatalf("insert exception: %v", err)
	}

	// Create an exception event (FK → fulfillment_exceptions).
	if _, err := pool.Exec(ctx,
		`INSERT INTO exception_events (id,exception_id,event_type,content,created_at)
		 VALUES ($1,$2,'NOTE','test',NOW())`, uuid.New(), exID); err != nil {
		t.Fatalf("insert exception_event: %v", err)
	}

	// Soft-delete the fulfillment and age its deleted_at beyond the retention window.
	cutoff := time.Now().UTC().Add(-31 * 24 * time.Hour)
	if _, err := pool.Exec(ctx,
		`UPDATE fulfillments SET deleted_at=$1, deleted_by=$2 WHERE id=$3`, cutoff, adminID, fulfillID); err != nil {
		t.Fatalf("soft-delete fulfillment: %v", err)
	}

	// Run the cleanup job with a 30-day retention window.
	j := NewCleanupJob(pool, 30)
	n, err := j.Run(ctx)
	if err != nil {
		t.Fatalf("CleanupJob.Run: %v — FK dependency ordering failure", err)
	}
	if n < 1 {
		t.Fatalf("expected at least 1 deleted record, got %d", n)
	}

	// Confirm the fulfillment is gone.
	var count int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM fulfillments WHERE id=$1`, fulfillID).Scan(&count); err != nil {
		t.Fatalf("count after cleanup: %v", err)
	}
	if count != 0 {
		t.Fatalf("fulfillment still present after cleanup")
	}
	fmt.Printf("TestCleanupJob_PurgesFulfillmentWithRelatedRows: deleted %d records\n", n)
}
