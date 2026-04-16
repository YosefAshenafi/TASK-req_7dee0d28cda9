package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fulfillops/fulfillops/internal/domain"
)

type ExceptionFilters struct {
	Status        domain.ExceptionStatus
	Type          domain.ExceptionType
	FulfillmentID *uuid.UUID
	OpenedFrom    *time.Time
	OpenedTo      *time.Time
}

type ExceptionRepository interface {
	List(ctx context.Context, filters ExceptionFilters) ([]domain.FulfillmentException, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.FulfillmentException, error)
	Create(ctx context.Context, e *domain.FulfillmentException) (*domain.FulfillmentException, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.ExceptionStatus, resolutionNote *string, resolvedBy *uuid.UUID) error
	ExistsOpenForFulfillment(ctx context.Context, fulfillmentID uuid.UUID, exType domain.ExceptionType) (bool, error)
}

type pgExceptionRepo struct{ pool *pgxpool.Pool }

func NewExceptionRepository(pool *pgxpool.Pool) ExceptionRepository {
	return &pgExceptionRepo{pool: pool}
}

func (r *pgExceptionRepo) List(ctx context.Context, f ExceptionFilters) ([]domain.FulfillmentException, error) {
	args := []any{}
	where := `WHERE 1=1`
	i := 1

	if f.Status != "" {
		where += fmt.Sprintf(` AND status=$%d`, i)
		args = append(args, string(f.Status))
		i++
	}
	if f.Type != "" {
		where += fmt.Sprintf(` AND type=$%d`, i)
		args = append(args, string(f.Type))
		i++
	}
	if f.FulfillmentID != nil {
		where += fmt.Sprintf(` AND fulfillment_id=$%d`, i)
		args = append(args, *f.FulfillmentID)
		i++
	}
	if f.OpenedFrom != nil {
		where += fmt.Sprintf(` AND created_at >= $%d`, i)
		args = append(args, *f.OpenedFrom)
		i++
	}
	if f.OpenedTo != nil {
		where += fmt.Sprintf(` AND created_at <= $%d`, i)
		args = append(args, *f.OpenedTo)
		i++
	}
	_ = i

	rows, err := r.pool.Query(ctx,
		`SELECT id, fulfillment_id, type, status, resolution_note, opened_by, resolved_by, created_at, updated_at
		 FROM fulfillment_exceptions `+where+` ORDER BY created_at DESC`, args...)
	if err != nil {
		return nil, fmt.Errorf("listing exceptions: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[domain.FulfillmentException])
}

func (r *pgExceptionRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.FulfillmentException, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, fulfillment_id, type, status, resolution_note, opened_by, resolved_by, created_at, updated_at
		 FROM fulfillment_exceptions WHERE id=$1`, id)
	if err != nil {
		return nil, err
	}
	e, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[domain.FulfillmentException])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.NewNotFoundError("exception")
	}
	return &e, err
}

func (r *pgExceptionRepo) Create(ctx context.Context, e *domain.FulfillmentException) (*domain.FulfillmentException, error) {
	e.ID = uuid.New()
	now := time.Now().UTC()
	e.CreatedAt, e.UpdatedAt = now, now
	if e.Status == "" {
		e.Status = domain.ExceptionOpen
	}

	_, err := r.pool.Exec(ctx,
		`INSERT INTO fulfillment_exceptions
		   (id, fulfillment_id, type, status, resolution_note, opened_by, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		e.ID, e.FulfillmentID, string(e.Type), string(e.Status),
		e.ResolutionNote, e.OpenedBy, e.CreatedAt, e.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating exception: %w", err)
	}
	return e, nil
}

func (r *pgExceptionRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.ExceptionStatus, note *string, resolvedBy *uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE fulfillment_exceptions
		 SET status=$1, resolution_note=$2, resolved_by=$3, updated_at=NOW()
		 WHERE id=$4`,
		string(status), note, resolvedBy, id)
	return err
}

func (r *pgExceptionRepo) ExistsOpenForFulfillment(ctx context.Context, fulfillmentID uuid.UUID, exType domain.ExceptionType) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM fulfillment_exceptions
		  WHERE fulfillment_id=$1 AND type=$2 AND status NOT IN ('RESOLVED'))`,
		fulfillmentID, string(exType)).Scan(&exists)
	return exists, err
}
