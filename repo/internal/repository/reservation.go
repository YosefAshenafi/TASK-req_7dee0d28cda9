package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fulfillops/fulfillops/internal/domain"
)

type ReservationRepository interface {
	Create(ctx context.Context, tx pgx.Tx, r *domain.Reservation) (*domain.Reservation, error)
	VoidByFulfillmentID(ctx context.Context, tx pgx.Tx, fulfillmentID uuid.UUID) error
	GetActiveByFulfillmentID(ctx context.Context, tx pgx.Tx, fulfillmentID uuid.UUID) (*domain.Reservation, error)
}

type pgReservationRepo struct{}

func NewReservationRepository() ReservationRepository { return &pgReservationRepo{} }

func (r *pgReservationRepo) Create(ctx context.Context, tx pgx.Tx, res *domain.Reservation) (*domain.Reservation, error) {
	res.ID = uuid.New()
	now := time.Now().UTC()
	res.CreatedAt, res.UpdatedAt = now, now
	res.Status = domain.ReservationActive

	_, err := tx.Exec(ctx,
		`INSERT INTO reservations (id, tier_id, fulfillment_id, status, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		res.ID, res.TierID, res.FulfillmentID, string(res.Status), res.CreatedAt, res.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating reservation: %w", err)
	}
	return res, nil
}

func (r *pgReservationRepo) VoidByFulfillmentID(ctx context.Context, tx pgx.Tx, fulfillmentID uuid.UUID) error {
	_, err := tx.Exec(ctx,
		`UPDATE reservations SET status='VOIDED', updated_at=NOW()
		 WHERE fulfillment_id=$1 AND status='ACTIVE'`, fulfillmentID)
	return err
}

func (r *pgReservationRepo) GetActiveByFulfillmentID(ctx context.Context, tx pgx.Tx, fulfillmentID uuid.UUID) (*domain.Reservation, error) {
	rows, err := tx.Query(ctx,
		`SELECT id, tier_id, fulfillment_id, status, created_at, updated_at
		 FROM reservations WHERE fulfillment_id=$1 AND status='ACTIVE'`, fulfillmentID)
	if err != nil {
		return nil, err
	}
	res, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[domain.Reservation])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &res, err
}
