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

type ShippingAddressRepository interface {
	Create(ctx context.Context, tx pgx.Tx, addr *domain.ShippingAddress) (*domain.ShippingAddress, error)
	CreateNoTx(ctx context.Context, addr *domain.ShippingAddress) (*domain.ShippingAddress, error)
	GetByFulfillmentID(ctx context.Context, fulfillmentID uuid.UUID) (*domain.ShippingAddress, error)
	Update(ctx context.Context, tx pgx.Tx, addr *domain.ShippingAddress) error
}

type pgShippingAddressRepo struct{ pool *pgxpool.Pool }

func NewShippingAddressRepository(pool *pgxpool.Pool) ShippingAddressRepository {
	return &pgShippingAddressRepo{pool: pool}
}

func (r *pgShippingAddressRepo) Create(ctx context.Context, tx pgx.Tx, addr *domain.ShippingAddress) (*domain.ShippingAddress, error) {
	addr.ID = uuid.New()
	now := time.Now().UTC()
	addr.CreatedAt, addr.UpdatedAt = now, now

	_, err := tx.Exec(ctx,
		`INSERT INTO shipping_addresses
		   (id, fulfillment_id, line_1_encrypted, line_2_encrypted, city, state, zip_code, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		addr.ID, addr.FulfillmentID, addr.Line1Encrypted, addr.Line2Encrypted,
		addr.City, addr.State, addr.ZipCode, addr.CreatedAt, addr.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating shipping address: %w", err)
	}
	return addr, nil
}

func (r *pgShippingAddressRepo) CreateNoTx(ctx context.Context, addr *domain.ShippingAddress) (*domain.ShippingAddress, error) {
	addr.ID = uuid.New()
	now := time.Now().UTC()
	addr.CreatedAt, addr.UpdatedAt = now, now

	_, err := r.pool.Exec(ctx,
		`INSERT INTO shipping_addresses
		   (id, fulfillment_id, line_1_encrypted, line_2_encrypted, city, state, zip_code, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		addr.ID, addr.FulfillmentID, addr.Line1Encrypted, addr.Line2Encrypted,
		addr.City, addr.State, addr.ZipCode, addr.CreatedAt, addr.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating shipping address: %w", err)
	}
	return addr, nil
}

func (r *pgShippingAddressRepo) GetByFulfillmentID(ctx context.Context, fulfillmentID uuid.UUID) (*domain.ShippingAddress, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, fulfillment_id, line_1_encrypted, line_2_encrypted, city, state, zip_code, created_at, updated_at
		 FROM shipping_addresses WHERE fulfillment_id=$1`, fulfillmentID)
	if err != nil {
		return nil, err
	}
	addr, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[domain.ShippingAddress])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &addr, err
}

func (r *pgShippingAddressRepo) Update(ctx context.Context, tx pgx.Tx, addr *domain.ShippingAddress) error {
	addr.UpdatedAt = time.Now().UTC()
	_, err := tx.Exec(ctx,
		`UPDATE shipping_addresses
		 SET line_1_encrypted=$1, line_2_encrypted=$2, city=$3, state=$4, zip_code=$5, updated_at=$6
		 WHERE fulfillment_id=$7`,
		addr.Line1Encrypted, addr.Line2Encrypted, addr.City, addr.State, addr.ZipCode, addr.UpdatedAt, addr.FulfillmentID)
	return err
}
