package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fulfillops/fulfillops/internal/domain"
)

type TimelineRepository interface {
	Create(ctx context.Context, tx pgx.Tx, event *domain.TimelineEvent) error
	ListByFulfillmentID(ctx context.Context, fulfillmentID uuid.UUID) ([]domain.TimelineEvent, error)
}

type pgTimelineRepo struct{ pool *pgxpool.Pool }

func NewTimelineRepository(pool *pgxpool.Pool) TimelineRepository {
	return &pgTimelineRepo{pool: pool}
}

func (r *pgTimelineRepo) Create(ctx context.Context, tx pgx.Tx, e *domain.TimelineEvent) error {
	e.ID = uuid.New()
	e.ChangedAt = time.Now().UTC()
	if e.Metadata == nil {
		e.Metadata = []byte(`{}`)
	}

	_, err := tx.Exec(ctx,
		`INSERT INTO fulfillment_timeline
		   (id, fulfillment_id, from_status, to_status, reason, metadata, changed_by, changed_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		e.ID, e.FulfillmentID, e.FromStatus, string(e.ToStatus),
		e.Reason, e.Metadata, e.ChangedBy, e.ChangedAt)
	return err
}

func (r *pgTimelineRepo) ListByFulfillmentID(ctx context.Context, fulfillmentID uuid.UUID) ([]domain.TimelineEvent, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, fulfillment_id, from_status, to_status, reason, metadata, changed_by, changed_at
		 FROM fulfillment_timeline WHERE fulfillment_id=$1 ORDER BY changed_at`, fulfillmentID)
	if err != nil {
		return nil, fmt.Errorf("listing timeline: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[domain.TimelineEvent])
}
