package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
)

// InventoryService manages tier inventory via reservations.
type InventoryService interface {
	// Reserve decrements inventory and creates an ACTIVE reservation (must be called inside a tx).
	Reserve(ctx context.Context, tx pgx.Tx, tierID, fulfillmentID uuid.UUID) error
	// Release voids the reservation and increments inventory (must be called inside a tx).
	Release(ctx context.Context, tx pgx.Tx, tierID, fulfillmentID uuid.UUID) error
}

type inventoryService struct {
	tierRepo        repository.TierRepository
	reservationRepo repository.ReservationRepository
}

// NewInventoryService creates an InventoryService.
func NewInventoryService(tierRepo repository.TierRepository, reservationRepo repository.ReservationRepository) InventoryService {
	return &inventoryService{tierRepo: tierRepo, reservationRepo: reservationRepo}
}

func (s *inventoryService) Reserve(ctx context.Context, tx pgx.Tx, tierID, fulfillmentID uuid.UUID) error {
	if err := s.tierRepo.DecrementInventory(ctx, tx, tierID, 1); err != nil {
		return err
	}
	_, err := s.reservationRepo.Create(ctx, tx, &domain.Reservation{
		TierID:        tierID,
		FulfillmentID: fulfillmentID,
	})
	return err
}

func (s *inventoryService) Release(ctx context.Context, tx pgx.Tx, tierID, fulfillmentID uuid.UUID) error {
	if err := s.reservationRepo.VoidByFulfillmentID(ctx, tx, fulfillmentID); err != nil {
		return err
	}
	return s.tierRepo.IncrementInventory(ctx, tx, tierID, 1)
}
