package job

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CleanupJob hard-deletes records that were soft-deleted more than retentionDays ago.
// It targets all four soft-delete tables: reward_tiers, customers, fulfillments,
// and message_templates.
type CleanupJob struct {
	pool          *pgxpool.Pool
	retentionDays int
}

func NewCleanupJob(pool *pgxpool.Pool, retentionDays int) *CleanupJob {
	return &CleanupJob{pool: pool, retentionDays: retentionDays}
}

func (j *CleanupJob) Run(ctx context.Context) (int, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -j.retentionDays)
	total := 0

	tables := []string{"reward_tiers", "customers", "fulfillments", "message_templates"}
	for _, table := range tables {
		tag, err := j.pool.Exec(ctx,
			fmt.Sprintf(`DELETE FROM %s WHERE deleted_at IS NOT NULL AND deleted_at < $1`, table),
			cutoff,
		)
		if err != nil {
			return total, fmt.Errorf("hard-deleting %s: %w", table, err)
		}
		total += int(tag.RowsAffected())
	}
	return total, nil
}
