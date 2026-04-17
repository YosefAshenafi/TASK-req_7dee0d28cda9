package job

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CleanupJob hard-deletes records that were soft-deleted more than retentionDays ago.
// Deletion is performed inside a single transaction in reverse FK-dependency order so
// that no foreign-key constraint can fire against an already-deleted parent row.
//
// Dependency topology (→ means "references"):
//   exception_events → fulfillment_exceptions → fulfillments → reward_tiers / customers
//   fulfillment_timeline → fulfillments
//   shipping_addresses   → fulfillments
//   reservations         → fulfillments / reward_tiers
//   send_logs            → customers / message_templates
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

	tx, err := j.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("beginning cleanup transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// ── Step 1: purge soft-deleted fulfillments and their child rows ──────────
	rows, err := tx.Query(ctx,
		`SELECT id FROM fulfillments WHERE deleted_at IS NOT NULL AND deleted_at < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("querying fulfillments to purge: %w", err)
	}
	var fulfillIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if scanErr := rows.Scan(&id); scanErr != nil {
			rows.Close()
			return 0, scanErr
		}
		fulfillIDs = append(fulfillIDs, id)
	}
	rows.Close()

	for _, id := range fulfillIDs {
		if _, err := tx.Exec(ctx,
			`DELETE FROM exception_events WHERE exception_id IN
			   (SELECT id FROM fulfillment_exceptions WHERE fulfillment_id=$1)`, id); err != nil {
			return total, fmt.Errorf("deleting exception_events for fulfillment %s: %w", id, err)
		}
		if _, err := tx.Exec(ctx,
			`DELETE FROM fulfillment_exceptions WHERE fulfillment_id=$1`, id); err != nil {
			return total, fmt.Errorf("deleting exceptions for fulfillment %s: %w", id, err)
		}
		if _, err := tx.Exec(ctx,
			`DELETE FROM fulfillment_timeline WHERE fulfillment_id=$1`, id); err != nil {
			return total, fmt.Errorf("deleting timeline for fulfillment %s: %w", id, err)
		}
		if _, err := tx.Exec(ctx,
			`DELETE FROM shipping_addresses WHERE fulfillment_id=$1`, id); err != nil {
			return total, fmt.Errorf("deleting shipping_address for fulfillment %s: %w", id, err)
		}
		if _, err := tx.Exec(ctx,
			`DELETE FROM reservations WHERE fulfillment_id=$1`, id); err != nil {
			return total, fmt.Errorf("deleting reservations for fulfillment %s: %w", id, err)
		}
		tag, err := tx.Exec(ctx, `DELETE FROM fulfillments WHERE id=$1`, id)
		if err != nil {
			return total, fmt.Errorf("deleting fulfillment %s: %w", id, err)
		}
		total += int(tag.RowsAffected())
	}

	// ── Step 2: purge soft-deleted customers and their send_logs ──────────────
	rows, err = tx.Query(ctx,
		`SELECT id FROM customers WHERE deleted_at IS NOT NULL AND deleted_at < $1`, cutoff)
	if err != nil {
		return total, fmt.Errorf("querying customers to purge: %w", err)
	}
	var custIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if scanErr := rows.Scan(&id); scanErr != nil {
			rows.Close()
			return total, scanErr
		}
		custIDs = append(custIDs, id)
	}
	rows.Close()

	for _, id := range custIDs {
		if _, err := tx.Exec(ctx,
			`DELETE FROM send_logs WHERE recipient_id=$1`, id); err != nil {
			return total, fmt.Errorf("deleting send_logs for customer %s: %w", id, err)
		}
		tag, err := tx.Exec(ctx, `DELETE FROM customers WHERE id=$1`, id)
		if err != nil {
			return total, fmt.Errorf("deleting customer %s: %w", id, err)
		}
		total += int(tag.RowsAffected())
	}

	// ── Step 3: purge soft-deleted reward_tiers ───────────────────────────────
	// Any live fulfillments referencing these tiers were already cleaned in step 1
	// (deleted fulfillments); live non-deleted fulfillments prevent the purge,
	// which is the desired behaviour — the tier cannot be permanently removed while
	// active orders still reference it.
	tag, err := tx.Exec(ctx,
		`DELETE FROM reward_tiers WHERE deleted_at IS NOT NULL AND deleted_at < $1`, cutoff)
	if err != nil {
		return total, fmt.Errorf("hard-deleting reward_tiers: %w", err)
	}
	total += int(tag.RowsAffected())

	// ── Step 4: purge soft-deleted message_templates and their send_logs ──────
	rows, err = tx.Query(ctx,
		`SELECT id FROM message_templates WHERE deleted_at IS NOT NULL AND deleted_at < $1`, cutoff)
	if err != nil {
		return total, fmt.Errorf("querying message_templates to purge: %w", err)
	}
	var tmplIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if scanErr := rows.Scan(&id); scanErr != nil {
			rows.Close()
			return total, scanErr
		}
		tmplIDs = append(tmplIDs, id)
	}
	rows.Close()

	for _, id := range tmplIDs {
		if _, err := tx.Exec(ctx,
			`DELETE FROM send_logs WHERE template_id=$1`, id); err != nil {
			return total, fmt.Errorf("deleting send_logs for template %s: %w", id, err)
		}
		tag, err := tx.Exec(ctx, `DELETE FROM message_templates WHERE id=$1`, id)
		if err != nil {
			return total, fmt.Errorf("deleting template %s: %w", id, err)
		}
		total += int(tag.RowsAffected())
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("committing cleanup transaction: %w", err)
	}
	return total, nil
}
