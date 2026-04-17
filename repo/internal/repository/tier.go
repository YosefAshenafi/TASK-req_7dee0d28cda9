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

type TierRepository interface {
	List(ctx context.Context, q string, includeDeleted bool) ([]domain.RewardTier, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.RewardTier, error)
	GetByIDForUpdate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.RewardTier, error)
	Create(ctx context.Context, tier *domain.RewardTier) (*domain.RewardTier, error)
	Update(ctx context.Context, tier *domain.RewardTier) (*domain.RewardTier, error)
	SoftDelete(ctx context.Context, id uuid.UUID, deletedBy uuid.UUID) error
	Restore(ctx context.Context, id uuid.UUID) error
	DecrementInventory(ctx context.Context, tx pgx.Tx, id uuid.UUID, amount int) error
	IncrementInventory(ctx context.Context, tx pgx.Tx, id uuid.UUID, amount int) error
}

type pgTierRepo struct{ pool *pgxpool.Pool }

func NewTierRepository(pool *pgxpool.Pool) TierRepository {
	return &pgTierRepo{pool: pool}
}

func (r *pgTierRepo) List(ctx context.Context, q string, includeDeleted bool) ([]domain.RewardTier, error) {
	sql := `SELECT id, name, description, inventory_count, purchase_limit, alert_threshold,
	               version, created_at, updated_at, deleted_at, deleted_by
	        FROM reward_tiers
	        WHERE ($1 = '' OR name ILIKE '%' || $1 || '%')`
	if !includeDeleted {
		sql += ` AND deleted_at IS NULL`
	}
	sql += ` ORDER BY name`

	rows, err := r.pool.Query(ctx, sql, q)
	if err != nil {
		return nil, fmt.Errorf("listing tiers: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[domain.RewardTier])
}

func (r *pgTierRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.RewardTier, error) {
	return scanTier(ctx, r.pool, id)
}

func (r *pgTierRepo) GetByIDForUpdate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.RewardTier, error) {
	return scanTierLocked(ctx, tx, id)
}

func (r *pgTierRepo) Create(ctx context.Context, t *domain.RewardTier) (*domain.RewardTier, error) {
	t.ID = uuid.New()
	now := time.Now().UTC()
	t.CreatedAt, t.UpdatedAt = now, now
	t.Version = 1

	_, err := r.pool.Exec(ctx,
		`INSERT INTO reward_tiers (id, name, description, inventory_count, purchase_limit, alert_threshold,
		                           version, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		t.ID, t.Name, t.Description, t.InventoryCount, t.PurchaseLimit, t.AlertThreshold,
		t.Version, t.CreatedAt, t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating tier: %w", err)
	}
	return t, nil
}

func (r *pgTierRepo) Update(ctx context.Context, t *domain.RewardTier) (*domain.RewardTier, error) {
	t.UpdatedAt = time.Now().UTC()
	tag, err := r.pool.Exec(ctx,
		`UPDATE reward_tiers
		 SET name=$1, description=$2, inventory_count=$3, purchase_limit=$4,
		     alert_threshold=$5, updated_at=$6, version=version+1
		 WHERE id=$7 AND version=$8 AND deleted_at IS NULL`,
		t.Name, t.Description, t.InventoryCount, t.PurchaseLimit,
		t.AlertThreshold, t.UpdatedAt, t.ID, t.Version)
	if err != nil {
		return nil, fmt.Errorf("updating tier: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, domain.NewConflictError()
	}
	return r.GetByID(ctx, t.ID)
}

func (r *pgTierRepo) SoftDelete(ctx context.Context, id uuid.UUID, deletedBy uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE reward_tiers SET deleted_at=NOW(), deleted_by=$1 WHERE id=$2 AND deleted_at IS NULL`,
		deletedBy, id)
	if err != nil {
		return fmt.Errorf("soft-deleting tier: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.NewNotFoundError("reward tier")
	}
	return nil
}

func (r *pgTierRepo) Restore(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE reward_tiers
		 SET deleted_at=NULL, deleted_by=NULL, updated_at=NOW()
		 WHERE id=$1 AND deleted_at IS NOT NULL AND deleted_at > NOW() - INTERVAL '30 days'`,
		id)
	if err != nil {
		return fmt.Errorf("restoring tier: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrSoftDeleteExpired
	}
	return nil
}

func (r *pgTierRepo) DecrementInventory(ctx context.Context, tx pgx.Tx, id uuid.UUID, amount int) error {
	tag, err := tx.Exec(ctx,
		`UPDATE reward_tiers SET inventory_count = inventory_count - $1, updated_at=NOW()
		 WHERE id=$2 AND inventory_count >= $1`,
		amount, id)
	if err != nil {
		return fmt.Errorf("decrementing inventory: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.NewInventoryError()
	}
	return nil
}

func (r *pgTierRepo) IncrementInventory(ctx context.Context, tx pgx.Tx, id uuid.UUID, amount int) error {
	_, err := tx.Exec(ctx,
		`UPDATE reward_tiers SET inventory_count = inventory_count + $1, updated_at=NOW() WHERE id=$2`,
		amount, id)
	return err
}

// helpers

func scanTier(ctx context.Context, db DBTX, id uuid.UUID) (*domain.RewardTier, error) {
	rows, err := db.Query(ctx,
		`SELECT id, name, description, inventory_count, purchase_limit, alert_threshold,
		        version, created_at, updated_at, deleted_at, deleted_by
		 FROM reward_tiers WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		return nil, fmt.Errorf("querying tier: %w", err)
	}
	t, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[domain.RewardTier])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.NewNotFoundError("reward tier")
	}
	return &t, err
}

func scanTierLocked(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.RewardTier, error) {
	rows, err := tx.Query(ctx,
		`SELECT id, name, description, inventory_count, purchase_limit, alert_threshold,
		        version, created_at, updated_at, deleted_at, deleted_by
		 FROM reward_tiers WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, id)
	if err != nil {
		return nil, fmt.Errorf("querying tier for update: %w", err)
	}
	t, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[domain.RewardTier])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.NewNotFoundError("reward tier")
	}
	return &t, err
}
