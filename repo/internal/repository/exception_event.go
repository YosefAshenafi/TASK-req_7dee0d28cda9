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

type ExceptionEventRepository interface {
	Create(ctx context.Context, event *domain.ExceptionEvent) error
	ListByExceptionID(ctx context.Context, exceptionID uuid.UUID) ([]domain.ExceptionEvent, error)
}

type pgExceptionEventRepo struct{ pool *pgxpool.Pool }

func NewExceptionEventRepository(pool *pgxpool.Pool) ExceptionEventRepository {
	return &pgExceptionEventRepo{pool: pool}
}

func (r *pgExceptionEventRepo) Create(ctx context.Context, e *domain.ExceptionEvent) error {
	e.ID = uuid.New()
	e.CreatedAt = time.Now().UTC()

	_, err := r.pool.Exec(ctx,
		`INSERT INTO exception_events (id, exception_id, event_type, content, created_by, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		e.ID, e.ExceptionID, e.EventType, e.Content, e.CreatedBy, e.CreatedAt)
	return err
}

func (r *pgExceptionEventRepo) ListByExceptionID(ctx context.Context, exceptionID uuid.UUID) ([]domain.ExceptionEvent, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, exception_id, event_type, content, created_by, created_at
		 FROM exception_events WHERE exception_id=$1 ORDER BY created_at`, exceptionID)
	if err != nil {
		return nil, fmt.Errorf("listing exception events: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[domain.ExceptionEvent])
}
