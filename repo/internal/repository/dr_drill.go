package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fulfillops/fulfillops/internal/domain"
)

type DRDrillRepository interface {
	Create(ctx context.Context, d *domain.DRDrill) (*domain.DRDrill, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.DRDrill, error)
	List(ctx context.Context, page domain.PageRequest) ([]domain.DRDrill, int, error)
	Update(ctx context.Context, d *domain.DRDrill) (*domain.DRDrill, error)
}

type pgDRDrillRepo struct{ pool *pgxpool.Pool }

func NewDRDrillRepository(pool *pgxpool.Pool) DRDrillRepository {
	return &pgDRDrillRepo{pool: pool}
}

const drDrillCols = `id, scheduled_for, executed_at, executed_by,
	outcome, notes, artifact_path, created_at, updated_at`

func (r *pgDRDrillRepo) Create(ctx context.Context, d *domain.DRDrill) (*domain.DRDrill, error) {
	d.ID = uuid.New()
	now := time.Now().UTC()
	d.CreatedAt, d.UpdatedAt = now, now

	_, err := r.pool.Exec(ctx,
		`INSERT INTO dr_drills (id, scheduled_for, executed_at, executed_by,
		                        outcome, notes, artifact_path, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		d.ID, d.ScheduledFor, d.ExecutedAt, d.ExecutedBy,
		d.Outcome, d.Notes, d.ArtifactPath, d.CreatedAt, d.UpdatedAt)
	return d, err
}

func (r *pgDRDrillRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.DRDrill, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+drDrillCols+` FROM dr_drills WHERE id=$1`, id)
	if err != nil {
		return nil, err
	}
	d, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[domain.DRDrill])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.NewNotFoundError("dr drill")
	}
	return &d, err
}

func (r *pgDRDrillRepo) List(ctx context.Context, page domain.PageRequest) ([]domain.DRDrill, int, error) {
	page.Normalize()
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM dr_drills`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+drDrillCols+` FROM dr_drills ORDER BY scheduled_for DESC LIMIT $1 OFFSET $2`,
		page.PageSize, page.Offset())
	if err != nil {
		return nil, 0, err
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[domain.DRDrill])
	return items, total, err
}

func (r *pgDRDrillRepo) Update(ctx context.Context, d *domain.DRDrill) (*domain.DRDrill, error) {
	d.UpdatedAt = time.Now().UTC()
	_, err := r.pool.Exec(ctx,
		`UPDATE dr_drills
		 SET scheduled_for=$1, executed_at=$2, executed_by=$3,
		     outcome=$4, notes=$5, artifact_path=$6, updated_at=$7
		 WHERE id=$8`,
		d.ScheduledFor, d.ExecutedAt, d.ExecutedBy,
		d.Outcome, d.Notes, d.ArtifactPath, d.UpdatedAt, d.ID)
	return d, err
}
