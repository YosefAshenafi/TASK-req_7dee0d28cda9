package service

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
)

var trackingNumberRegex = regexp.MustCompile(`^[A-Za-z0-9]{8,30}$`)

// CreateFulfillmentInput holds all data needed to create a fulfillment.
type CreateFulfillmentInput struct {
	TierID     uuid.UUID
	CustomerID uuid.UUID
	Type       domain.FulfillmentType
}

// TransitionInput holds the data for a state transition.
type TransitionInput struct {
	FulfillmentID uuid.UUID
	ToStatus      domain.FulfillmentStatus
	// SHIPPED
	CarrierName    *string
	TrackingNumber *string
	// VOUCHER_ISSUED
	VoucherCode       []byte // pre-encrypted by handler
	VoucherExpiration *time.Time
	// ON_HOLD / CANCELED
	Reason *string
}

// ShippingAddressInput is the decrypted form used at the service boundary.
type ShippingAddressInput struct {
	Line1   string
	Line2   string
	City    string
	State   string
	ZipCode string
}

// FulfillmentService encapsulates business logic for fulfillment lifecycle.
type FulfillmentService interface {
	Create(ctx context.Context, input CreateFulfillmentInput) (*domain.Fulfillment, error)
	Transition(ctx context.Context, input TransitionInput) (*domain.Fulfillment, error)
}

type fulfillmentService struct {
	txMgr        repository.TxManager
	fulfillRepo  repository.FulfillmentRepository
	tierRepo     repository.TierRepository
	timelineRepo repository.TimelineRepository
	inventorySvc InventoryService
	auditSvc     AuditService
}

// NewFulfillmentService wires all dependencies.
func NewFulfillmentService(
	txMgr repository.TxManager,
	fulfillRepo repository.FulfillmentRepository,
	tierRepo repository.TierRepository,
	timelineRepo repository.TimelineRepository,
	inventorySvc InventoryService,
	auditSvc AuditService,
) FulfillmentService {
	return &fulfillmentService{
		txMgr:        txMgr,
		fulfillRepo:  fulfillRepo,
		tierRepo:     tierRepo,
		timelineRepo: timelineRepo,
		inventorySvc: inventorySvc,
		auditSvc:     auditSvc,
	}
}

// purchaseWindow is how far back we count prior purchases for the limit check.
const purchaseWindow = 30 * 24 * time.Hour

// Create atomically: lock tier → check purchase limit → decrement inventory
// → create fulfillment → create reservation → append timeline → audit.
func (s *fulfillmentService) Create(ctx context.Context, input CreateFulfillmentInput) (*domain.Fulfillment, error) {
	if !input.Type.IsValid() {
		return nil, domain.NewValidationError("invalid fulfillment type", map[string]string{
			"type": "must be PHYSICAL or VOUCHER",
		})
	}

	var created *domain.Fulfillment

	err := s.txMgr.WithTx(ctx, func(tx pgx.Tx) error {
		// 1. Lock the tier row to prevent concurrent modifications.
		tier, err := s.tierRepo.GetByIDForUpdate(ctx, tx, input.TierID)
		if err != nil {
			return err
		}

		// 2. Check purchase limit (rolling 30-day window, excluding CANCELED).
		since := time.Now().UTC().Add(-purchaseWindow)
		count, err := s.fulfillRepo.CountByCustomerAndTier(ctx, tx, input.CustomerID, input.TierID, since)
		if err != nil {
			return fmt.Errorf("checking purchase limit: %w", err)
		}
		if count >= tier.PurchaseLimit {
			return domain.NewPurchaseLimitError()
		}

		// 3. Create fulfillment row first (need its ID for the reservation).
		f := &domain.Fulfillment{
			TierID:     input.TierID,
			CustomerID: input.CustomerID,
			Type:       input.Type,
			Status:     domain.StatusDraft,
		}
		created, err = s.fulfillRepo.Create(ctx, tx, f)
		if err != nil {
			return fmt.Errorf("creating fulfillment: %w", err)
		}

		// 4. Reserve inventory (decrement + create reservation).
		if err := s.inventorySvc.Reserve(ctx, tx, input.TierID, created.ID); err != nil {
			return err
		}

		// 5. Append CREATED timeline event.
		actorID, _ := UserIDFromContext(ctx)
		if err := s.timelineRepo.Create(ctx, tx, &domain.TimelineEvent{
			FulfillmentID: created.ID,
			ToStatus:      domain.StatusDraft,
			ChangedBy:     &actorID,
		}); err != nil {
			return fmt.Errorf("creating timeline event: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// 6. Audit (outside tx — best-effort).
	if s.auditSvc != nil {
		_ = s.auditSvc.Log(ctx, "fulfillments", created.ID, "CREATE", nil, created)
	}

	return created, nil
}

// Transition atomically: lock fulfillment row → validate transition
// → apply status-specific rules → update → append timeline → audit.
func (s *fulfillmentService) Transition(ctx context.Context, input TransitionInput) (*domain.Fulfillment, error) {
	if !input.ToStatus.IsValid() {
		return nil, domain.NewValidationError("invalid status", map[string]string{
			"to_status": "unrecognized status value",
		})
	}

	var updated *domain.Fulfillment

	err := s.txMgr.WithTx(ctx, func(tx pgx.Tx) error {
		// 1. Lock current row.
		f, err := s.fulfillRepo.GetByIDForUpdate(ctx, tx, input.FulfillmentID)
		if err != nil {
			return err
		}

		// 2. Validate transition.
		if !domain.IsTransitionAllowed(f.Status, input.ToStatus) {
			return domain.NewTransitionError(f.Status, input.ToStatus)
		}

		fromStatus := f.Status
		f.Status = input.ToStatus
		now := time.Now().UTC()

		// 3. Apply status-specific rules and field updates.
		switch input.ToStatus {
		case domain.StatusReadyToShip:
			f.ReadyAt = &now

		case domain.StatusShipped:
			if f.Type == domain.TypePhysical {
				if input.CarrierName == nil || *input.CarrierName == "" {
					return domain.NewValidationError("missing required field", map[string]string{
						"carrier_name": "required when transitioning to SHIPPED",
					})
				}
				if input.TrackingNumber == nil || !trackingNumberRegex.MatchString(*input.TrackingNumber) {
					return domain.NewValidationError("invalid field", map[string]string{
						"tracking_number": "must be 8-30 alphanumeric characters",
					})
				}
				f.CarrierName = input.CarrierName
				f.TrackingNumber = input.TrackingNumber
			}
			f.ShippedAt = &now

		case domain.StatusVoucherIssued:
			if len(input.VoucherCode) == 0 {
				return domain.NewValidationError("missing required field", map[string]string{
					"voucher_code": "required for VOUCHER_ISSUED",
				})
			}
			f.VoucherCodeEncrypted = input.VoucherCode
			f.VoucherExpiration = input.VoucherExpiration

		case domain.StatusDelivered:
			f.DeliveredAt = &now

		case domain.StatusCompleted:
			f.CompletedAt = &now

		case domain.StatusOnHold:
			if input.Reason == nil || *input.Reason == "" {
				return domain.NewValidationError("missing required field", map[string]string{
					"reason": "required when placing on hold",
				})
			}
			f.HoldReason = input.Reason

		case domain.StatusCanceled:
			if input.Reason == nil || *input.Reason == "" {
				return domain.NewValidationError("missing required field", map[string]string{
					"reason": "required for cancellation",
				})
			}
			f.CancelReason = input.Reason
			// Release reserved inventory back to the tier.
			if releaseErr := s.inventorySvc.Release(ctx, tx, f.TierID, f.ID); releaseErr != nil {
				return fmt.Errorf("releasing inventory on cancel: %w", releaseErr)
			}
		}

		// 4. Persist update (version check inside Update).
		updated, err = s.fulfillRepo.Update(ctx, tx, f)
		if err != nil {
			return err
		}

		// 5. Append timeline event.
		actorID, _ := UserIDFromContext(ctx)
		var reason *string
		if input.Reason != nil {
			reason = input.Reason
		}
		if err := s.timelineRepo.Create(ctx, tx, &domain.TimelineEvent{
			FulfillmentID: f.ID,
			FromStatus:    &fromStatus,
			ToStatus:      input.ToStatus,
			Reason:        reason,
			ChangedBy:     &actorID,
		}); err != nil {
			return fmt.Errorf("creating timeline event: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if s.auditSvc != nil {
		_ = s.auditSvc.Log(ctx, "fulfillments", updated.ID, "UPDATE", nil, updated)
	}

	return updated, nil
}
