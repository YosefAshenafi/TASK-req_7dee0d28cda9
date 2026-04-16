package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fulfillops/fulfillops/internal/domain"
)

type BlackoutDateRepository interface {
	List(ctx context.Context) ([]domain.BlackoutDate, error)
	Create(ctx context.Context, d *domain.BlackoutDate) (*domain.BlackoutDate, error)
	Delete(ctx context.Context, id uuid.UUID) error
	GetBetween(ctx context.Context, start, end time.Time) ([]domain.BlackoutDate, error)
}

type pgBlackoutDateRepo struct{ pool *pgxpool.Pool }

func NewBlackoutDateRepository(pool *pgxpool.Pool) BlackoutDateRepository {
	return &pgBlackoutDateRepo{pool: pool}
}

func (r *pgBlackoutDateRepo) List(ctx context.Context) ([]domain.BlackoutDate, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, date, description, created_by, created_at FROM blackout_dates ORDER BY date`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[domain.BlackoutDate])
}

func (r *pgBlackoutDateRepo) Create(ctx context.Context, d *domain.BlackoutDate) (*domain.BlackoutDate, error) {
	d.ID = uuid.New()
	d.CreatedAt = time.Now().UTC()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO blackout_dates (id, date, description, created_by, created_at)
		 VALUES ($1,$2,$3,$4,$5)`,
		d.ID, d.Date, d.Description, d.CreatedBy, d.CreatedAt)
	return d, err
}

func (r *pgBlackoutDateRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM blackout_dates WHERE id=$1`, id)
	return err
}

func (r *pgBlackoutDateRepo) GetBetween(ctx context.Context, start, end time.Time) ([]domain.BlackoutDate, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, date, description, created_by, created_at FROM blackout_dates
		 WHERE date >= $1 AND date <= $2 ORDER BY date`, start, end)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[domain.BlackoutDate])
}
