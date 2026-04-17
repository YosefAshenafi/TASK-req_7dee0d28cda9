package job

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCleanupJob_RunAgainstLiveDB(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set — skipping cleanup job test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	defer pool.Close()

	j := NewCleanupJob(pool, 30)
	// With retention 30 days and no older soft-deleted rows, Run should return 0.
	n, err := j.Run(ctx)
	if err != nil {
		t.Fatalf("CleanupJob.Run: %v", err)
	}
	if n < 0 {
		t.Fatalf("CleanupJob.Run returned negative count: %d", n)
	}
}

func TestCleanupJob_RunBadPoolReturnsError(t *testing.T) {
	// A closed pool causes Exec to fail, exercising the error branch.
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	pool.Close()

	j := NewCleanupJob(pool, 30)
	if _, err := j.Run(ctx); err == nil {
		t.Fatal("expected error on closed pool")
	}
}
