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

type FulfillmentFilters struct {
	Status         domain.FulfillmentStatus
	TierID         *uuid.UUID
	CustomerID     *uuid.UUID
	Type           domain.FulfillmentType
	DateFrom       *time.Time
	DateTo         *time.Time
	IncludeDeleted bool
}

type FulfillmentRepository interface {
	List(ctx context.Context, filters FulfillmentFilters, page domain.PageRequest) ([]domain.Fulfillment, int, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Fulfillment, error)
	GetByIDForUpdate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.Fulfillment, error)
	Create(ctx context.Context, tx pgx.Tx, f *domain.Fulfillment) (*domain.Fulfillment, error)
	Update(ctx context.Context, tx pgx.Tx, f *domain.Fulfillment) (*domain.Fulfillment, error)
	BumpVersion(ctx context.Context, tx pgx.Tx, id uuid.UUID, expectedVersion int) error
	CountByCustomerAndTier(ctx context.Context, tx pgx.Tx, customerID, tierID uuid.UUID, since time.Time) (int, error)
	SoftDelete(ctx context.Context, id uuid.UUID, deletedBy uuid.UUID) error
	Restore(ctx context.Context, id uuid.UUID) error
	ListOverdue(ctx context.Context) ([]domain.Fulfillment, error)
}

type pgFulfillmentRepo struct{ pool *pgxpool.Pool }

func NewFulfillmentRepository(pool *pgxpool.Pool) FulfillmentRepository {
	return &pgFulfillmentRepo{pool: pool}
}

const fulfillmentCols = `id, tier_id, customer_id, type, status,
	carrier_name, tracking_number, voucher_code_encrypted, voucher_expiration,
	hold_reason, cancel_reason, ready_at, shipped_at, delivered_at, completed_at,
	version, created_at, updated_at, deleted_at, deleted_by`

func (r *pgFulfillmentRepo) List(ctx context.Context, filters FulfillmentFilters, page domain.PageRequest) ([]domain.Fulfillment, int, error) {
	page.Normalize()
	args := []any{}
	where := `WHERE 1=1`
	if !filters.IncludeDeleted {
		where += ` AND deleted_at IS NULL`
	}
	i := 1

	if filters.Status != "" {
		where += fmt.Sprintf(` AND status=$%d`, i)
		args = append(args, string(filters.Status))
		i++
	}
	if filters.TierID != nil {
		where += fmt.Sprintf(` AND tier_id=$%d`, i)
		args = append(args, *filters.TierID)
		i++
	}
	if filters.CustomerID != nil {
		where += fmt.Sprintf(` AND customer_id=$%d`, i)
		args = append(args, *filters.CustomerID)
		i++
	}
	if filters.Type != "" {
		where += fmt.Sprintf(` AND type=$%d`, i)
		args = append(args, string(filters.Type))
		i++
	}
	if filters.DateFrom != nil {
		where += fmt.Sprintf(` AND created_at >= $%d`, i)
		args = append(args, *filters.DateFrom)
		i++
	}
	if filters.DateTo != nil {
		where += fmt.Sprintf(` AND created_at <= $%d`, i)
		args = append(args, *filters.DateTo)
		i++
	}

	countArgs := make([]any, len(args))
	copy(countArgs, args)
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM fulfillments `+where, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting fulfillments: %w", err)
	}

	args = append(args, page.PageSize, page.Offset())
	rows, err := r.pool.Query(ctx,
		`SELECT `+fulfillmentCols+` FROM fulfillments `+where+
			fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, i, i+1),
		args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing fulfillments: %w", err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[domain.Fulfillment])
	return items, total, err
}

func (r *pgFulfillmentRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Fulfillment, error) {
	return scanFulfillment(ctx, r.pool, id, false)
}

func (r *pgFulfillmentRepo) GetByIDForUpdate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.Fulfillment, error) {
	return scanFulfillment(ctx, tx, id, true)
}

func (r *pgFulfillmentRepo) Create(ctx context.Context, tx pgx.Tx, f *domain.Fulfillment) (*domain.Fulfillment, error) {
	f.ID = uuid.New()
	now := time.Now().UTC()
	f.CreatedAt, f.UpdatedAt = now, now
	f.Version = 1

	_, err := tx.Exec(ctx,
		`INSERT INTO fulfillments (id, tier_id, customer_id, type, status,
		                           carrier_name, tracking_number, voucher_code_encrypted,
		                           voucher_expiration, hold_reason, cancel_reason,
		                           ready_at, shipped_at, delivered_at, completed_at,
		                           version, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`,
		f.ID, f.TierID, f.CustomerID, string(f.Type), string(f.Status),
		f.CarrierName, f.TrackingNumber, f.VoucherCodeEncrypted,
		f.VoucherExpiration, f.HoldReason, f.CancelReason,
		f.ReadyAt, f.ShippedAt, f.DeliveredAt, f.CompletedAt,
		f.Version, f.CreatedAt, f.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating fulfillment: %w", err)
	}
	return f, nil
}

func (r *pgFulfillmentRepo) Update(ctx context.Context, tx pgx.Tx, f *domain.Fulfillment) (*domain.Fulfillment, error) {
	f.UpdatedAt = time.Now().UTC()
	tag, err := tx.Exec(ctx,
		`UPDATE fulfillments
		 SET status=$1, carrier_name=$2, tracking_number=$3, voucher_code_encrypted=$4,
		     voucher_expiration=$5, hold_reason=$6, cancel_reason=$7,
		     ready_at=$8, shipped_at=$9, delivered_at=$10, completed_at=$11,
		     updated_at=$12, version=version+1
		 WHERE id=$13 AND version=$14 AND deleted_at IS NULL`,
		string(f.Status), f.CarrierName, f.TrackingNumber, f.VoucherCodeEncrypted,
		f.VoucherExpiration, f.HoldReason, f.CancelReason,
		f.ReadyAt, f.ShippedAt, f.DeliveredAt, f.CompletedAt,
		f.UpdatedAt, f.ID, f.Version)
	if err != nil {
		return nil, fmt.Errorf("updating fulfillment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, domain.NewConflictError()
	}
	f.Version++
	return f, nil
}

func (r *pgFulfillmentRepo) BumpVersion(ctx context.Context, tx pgx.Tx, id uuid.UUID, expectedVersion int) error {
	tag, err := tx.Exec(ctx,
		`UPDATE fulfillments
		 SET updated_at=NOW(), version=version+1
		 WHERE id=$1 AND version=$2 AND deleted_at IS NULL`,
		id, expectedVersion)
	if err != nil {
		return fmt.Errorf("bumping fulfillment version: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.NewConflictError()
	}
	return nil
}

func (r *pgFulfillmentRepo) CountByCustomerAndTier(ctx context.Context, tx pgx.Tx, customerID, tierID uuid.UUID, since time.Time) (int, error) {
	var count int
	err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM fulfillments
		 WHERE customer_id=$1 AND tier_id=$2 AND status != 'CANCELED' AND created_at >= $3`,
		customerID, tierID, since).Scan(&count)
	return count, err
}

func (r *pgFulfillmentRepo) SoftDelete(ctx context.Context, id uuid.UUID, deletedBy uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE fulfillments SET deleted_at=NOW(), deleted_by=$1 WHERE id=$2 AND deleted_at IS NULL`,
		deletedBy, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.NewNotFoundError("fulfillment")
	}
	return nil
}

func (r *pgFulfillmentRepo) Restore(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE fulfillments SET deleted_at=NULL, deleted_by=NULL, updated_at=NOW()
		 WHERE id=$1 AND deleted_at IS NOT NULL AND deleted_at > NOW() - INTERVAL '30 days'`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrSoftDeleteExpired
	}
	return nil
}

func (r *pgFulfillmentRepo) ListOverdue(ctx context.Context) ([]domain.Fulfillment, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+fulfillmentCols+`
		 FROM fulfillments
		 WHERE status = 'READY_TO_SHIP'
		   AND ready_at IS NOT NULL AND deleted_at IS NULL
		 ORDER BY ready_at`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[domain.Fulfillment])
}

func scanFulfillment(ctx context.Context, db DBTX, id uuid.UUID, forUpdate bool) (*domain.Fulfillment, error) {
	sql := `SELECT ` + fulfillmentCols + ` FROM fulfillments WHERE id=$1 AND deleted_at IS NULL`
	if forUpdate {
		sql += ` FOR UPDATE`
	}
	rows, err := db.Query(ctx, sql, id)
	if err != nil {
		return nil, fmt.Errorf("querying fulfillment: %w", err)
	}
	f, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[domain.Fulfillment])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.NewNotFoundError("fulfillment")
	}
	return &f, err
}
