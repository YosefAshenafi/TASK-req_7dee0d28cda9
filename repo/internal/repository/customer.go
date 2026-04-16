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

type CustomerRepository interface {
	List(ctx context.Context, q string, page domain.PageRequest, includeDeleted bool) ([]domain.Customer, int, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Customer, error)
	Create(ctx context.Context, c *domain.Customer) (*domain.Customer, error)
	Update(ctx context.Context, c *domain.Customer) (*domain.Customer, error)
	SoftDelete(ctx context.Context, id uuid.UUID, deletedBy uuid.UUID) error
	Restore(ctx context.Context, id uuid.UUID) error
}

type pgCustomerRepo struct{ pool *pgxpool.Pool }

func NewCustomerRepository(pool *pgxpool.Pool) CustomerRepository {
	return &pgCustomerRepo{pool: pool}
}

func (r *pgCustomerRepo) List(ctx context.Context, q string, page domain.PageRequest, includeDeleted bool) ([]domain.Customer, int, error) {
	page.Normalize()

	whereClause := `WHERE ($1 = '' OR name ILIKE '%' || $1 || '%')`
	if !includeDeleted {
		whereClause += ` AND deleted_at IS NULL`
	}

	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM customers `+whereClause, q).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting customers: %w", err)
	}

	rows, err := r.pool.Query(ctx,
		`SELECT id, name, phone_encrypted, email_encrypted, address_encrypted,
		        version, created_at, updated_at, deleted_at, deleted_by
		 FROM customers `+whereClause+`
		 ORDER BY name LIMIT $2 OFFSET $3`,
		q, page.PageSize, page.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("listing customers: %w", err)
	}
	customers, err := pgx.CollectRows(rows, pgx.RowToStructByName[domain.Customer])
	return customers, total, err
}

func (r *pgCustomerRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Customer, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, phone_encrypted, email_encrypted, address_encrypted,
		        version, created_at, updated_at, deleted_at, deleted_by
		 FROM customers WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		return nil, fmt.Errorf("querying customer: %w", err)
	}
	c, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[domain.Customer])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.NewNotFoundError("customer")
	}
	return &c, err
}

func (r *pgCustomerRepo) Create(ctx context.Context, c *domain.Customer) (*domain.Customer, error) {
	c.ID = uuid.New()
	now := time.Now().UTC()
	c.CreatedAt, c.UpdatedAt = now, now
	c.Version = 1

	_, err := r.pool.Exec(ctx,
		`INSERT INTO customers (id, name, phone_encrypted, email_encrypted, address_encrypted,
		                        version, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		c.ID, c.Name, c.PhoneEncrypted, c.EmailEncrypted, c.AddressEncrypted,
		c.Version, c.CreatedAt, c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating customer: %w", err)
	}
	return c, nil
}

func (r *pgCustomerRepo) Update(ctx context.Context, c *domain.Customer) (*domain.Customer, error) {
	c.UpdatedAt = time.Now().UTC()
	tag, err := r.pool.Exec(ctx,
		`UPDATE customers
		 SET name=$1, phone_encrypted=$2, email_encrypted=$3, address_encrypted=$4,
		     updated_at=$5, version=version+1
		 WHERE id=$6 AND version=$7 AND deleted_at IS NULL`,
		c.Name, c.PhoneEncrypted, c.EmailEncrypted, c.AddressEncrypted,
		c.UpdatedAt, c.ID, c.Version)
	if err != nil {
		return nil, fmt.Errorf("updating customer: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, domain.NewConflictError()
	}
	return r.GetByID(ctx, c.ID)
}

func (r *pgCustomerRepo) SoftDelete(ctx context.Context, id uuid.UUID, deletedBy uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE customers SET deleted_at=NOW(), deleted_by=$1 WHERE id=$2 AND deleted_at IS NULL`,
		deletedBy, id)
	if err != nil {
		return fmt.Errorf("soft-deleting customer: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.NewNotFoundError("customer")
	}
	return nil
}

func (r *pgCustomerRepo) Restore(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE customers SET deleted_at=NULL, deleted_by=NULL, updated_at=NOW()
		 WHERE id=$1 AND deleted_at IS NOT NULL AND deleted_at > NOW() - INTERVAL '30 days'`, id)
	if err != nil {
		return fmt.Errorf("restoring customer: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrSoftDeleteExpired
	}
	return nil
}
