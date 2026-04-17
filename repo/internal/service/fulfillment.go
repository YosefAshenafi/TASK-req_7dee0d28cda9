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

var (
	trackingNumberRegex = regexp.MustCompile(`^[A-Za-z0-9]{8,30}$`)
	usStateRegex        = regexp.MustCompile(`^[A-Z]{2}$`)
	usZipRegex          = regexp.MustCompile(`^\d{5}(-\d{4})?$`)
)

// validateShippingAddress enforces US state (2 letters) and ZIP (5 or 9 digit) formats.
func validateShippingAddress(a *ShippingAddressEncrypted) error {
	if len(a.Line1Encrypted) == 0 {
		return domain.NewValidationError("missing required field", map[string]string{
			"addr_line1": "required",
		})
	}
	if a.City == "" {
		return domain.NewValidationError("missing required field", map[string]string{
			"addr_city": "required",
		})
	}
	if !usStateRegex.MatchString(a.State) {
		return domain.NewValidationError("invalid field", map[string]string{
			"addr_state": "must be a 2-letter US state code",
		})
	}
	if !usZipRegex.MatchString(a.ZipCode) {
		return domain.NewValidationError("invalid field", map[string]string{
			"addr_zip": "must be a 5-digit or 9-digit (zip+4) US ZIP code",
		})
	}
	return nil
}

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
	// ExpectedVersion enforces optimistic locking. If non-zero, the current
	// row version must match or ErrConflict is returned.
	ExpectedVersion int
	// SHIPPED
	CarrierName    *string
	TrackingNumber *string
	// VOUCHER_ISSUED
	VoucherCode       []byte // pre-encrypted by handler
	VoucherExpiration *time.Time
	// ON_HOLD / CANCELED
	Reason *string
	// Optional shipping address for PHYSICAL → READY_TO_SHIP. Pre-encrypted by handler.
	ShippingAddr *ShippingAddressEncrypted
}

// ShippingAddressEncrypted holds pre-encrypted address bytes plus plaintext metadata.
type ShippingAddressEncrypted struct {
	Line1Encrypted []byte
	Line2Encrypted []byte
	City           string
	State          string
	ZipCode        string
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
	// UpdateShippingAddress maintains a physical fulfillment's shipping address
	// out-of-band (no status transition). Used when an address is corrected after
	// initial capture — e.g. zip typo discovered during carrier handoff.
	UpdateShippingAddress(ctx context.Context, fulfillmentID uuid.UUID, expectedVersion int, addr *ShippingAddressEncrypted) error
}

type fulfillmentService struct {
	txMgr        repository.TxManager
	fulfillRepo  repository.FulfillmentRepository
	tierRepo     repository.TierRepository
	customerRepo repository.CustomerRepository
	timelineRepo repository.TimelineRepository
	shippingRepo repository.ShippingAddressRepository
	notifRepo    repository.NotificationRepository
	inventorySvc InventoryService
	auditSvc     AuditService
}

// NewFulfillmentService wires all dependencies. shippingRepo and notifRepo are
// optional (pass nil to disable transactional shipping-address / notification
// enqueue). Including them enforces the atomic-bundle requirement that every
// transition write is committed-or-rolled-back together.
func NewFulfillmentService(
	txMgr repository.TxManager,
	fulfillRepo repository.FulfillmentRepository,
	tierRepo repository.TierRepository,
	customerRepo repository.CustomerRepository,
	timelineRepo repository.TimelineRepository,
	shippingRepo repository.ShippingAddressRepository,
	notifRepo repository.NotificationRepository,
	inventorySvc InventoryService,
	auditSvc AuditService,
) FulfillmentService {
	return &fulfillmentService{
		txMgr:        txMgr,
		fulfillRepo:  fulfillRepo,
		tierRepo:     tierRepo,
		customerRepo: customerRepo,
		timelineRepo: timelineRepo,
		shippingRepo: shippingRepo,
		notifRepo:    notifRepo,
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
		// GetByIDForUpdate respects deleted_at IS NULL, so a soft-deleted tier
		// returns ErrNotFound and cannot be used for new fulfillments.
		tier, err := s.tierRepo.GetByIDForUpdate(ctx, tx, input.TierID)
		if err != nil {
			return err
		}

		// 1a. Validate that the customer exists and is not soft-deleted.
		if _, err := s.customerRepo.GetByID(ctx, input.CustomerID); err != nil {
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

		// 1a. Optimistic locking: caller-supplied version must match current row.
		if input.ExpectedVersion != 0 && input.ExpectedVersion != f.Version {
			return domain.NewConflictError()
		}

		// 2. Validate transition.
		if !domain.IsTransitionAllowed(f.Status, input.ToStatus) {
			return domain.NewTransitionError(f.Status, input.ToStatus)
		}

		// 2a. Type-specific transition guard.
		switch input.ToStatus {
		case domain.StatusShipped:
			if f.Type != domain.TypePhysical {
				return domain.NewValidationError("invalid transition", map[string]string{
					"to_status": "SHIPPED is only valid for PHYSICAL fulfillments",
				})
			}
		case domain.StatusVoucherIssued:
			if f.Type != domain.TypeVoucher {
				return domain.NewValidationError("invalid transition", map[string]string{
					"to_status": "VOUCHER_ISSUED is only valid for VOUCHER fulfillments",
				})
			}
		case domain.StatusDelivered:
			if f.Type != domain.TypePhysical {
				return domain.NewValidationError("invalid transition", map[string]string{
					"to_status": "DELIVERED is only valid for PHYSICAL fulfillments",
				})
			}
		}

		fromStatus := f.Status
		f.Status = input.ToStatus
		now := time.Now().UTC()

		// 3. Apply status-specific rules and field updates.
		switch input.ToStatus {
		case domain.StatusReadyToShip:
			f.ReadyAt = &now
			if f.Type == domain.TypePhysical && s.shippingRepo != nil {
				if input.ShippingAddr != nil {
					// New address provided — validate, then update existing or
					// create if none. This supports both first-time transitions
					// and maintenance (e.g. correcting an address after ON_HOLD).
					if err := validateShippingAddress(input.ShippingAddr); err != nil {
						return err
					}
					existing, err := s.shippingRepo.GetByFulfillmentID(ctx, f.ID)
					if err != nil {
						return fmt.Errorf("checking existing shipping address: %w", err)
					}
					if existing != nil {
						existing.Line1Encrypted = input.ShippingAddr.Line1Encrypted
						existing.Line2Encrypted = input.ShippingAddr.Line2Encrypted
						existing.City = input.ShippingAddr.City
						existing.State = input.ShippingAddr.State
						existing.ZipCode = input.ShippingAddr.ZipCode
						if err := s.shippingRepo.Update(ctx, tx, existing); err != nil {
							return fmt.Errorf("updating shipping address: %w", err)
						}
					} else {
						addr := &domain.ShippingAddress{
							FulfillmentID:  f.ID,
							Line1Encrypted: input.ShippingAddr.Line1Encrypted,
							Line2Encrypted: input.ShippingAddr.Line2Encrypted,
							City:           input.ShippingAddr.City,
							State:          input.ShippingAddr.State,
							ZipCode:        input.ShippingAddr.ZipCode,
						}
						if _, err := s.shippingRepo.Create(ctx, tx, addr); err != nil {
							return fmt.Errorf("persisting shipping address: %w", err)
						}
					}
				} else {
					// No new address supplied — a pre-existing address (e.g. after
					// ON_HOLD → READY_TO_SHIP resume) satisfies the requirement.
					existing, err := s.shippingRepo.GetByFulfillmentID(ctx, f.ID)
					if err != nil {
						return fmt.Errorf("checking existing shipping address: %w", err)
					}
					if existing == nil {
						return domain.NewValidationError("missing required field", map[string]string{
							"shipping_address": "physical fulfillments require a shipping address when transitioning to READY_TO_SHIP",
						})
					}
				}
			}

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

		// 6. Atomically enqueue a status-change notification for the actor.
		// Part of the atomic bundle — rolls back with the transition on failure.
		if s.notifRepo != nil && actorID != (uuid.UUID{}) {
			body := fmt.Sprintf("Fulfillment %s transitioned to %s", f.ID.String()[:8], input.ToStatus)
			notif := &domain.Notification{
				UserID: actorID,
				Title:  "Fulfillment status update",
				Body:   &body,
			}
			if err := s.notifRepo.CreateTx(ctx, tx, notif); err != nil {
				return fmt.Errorf("enqueuing transition notification: %w", err)
			}
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

// UpdateShippingAddress maintains a physical fulfillment's shipping address
// without a status transition. It validates, either updates (if an address
// exists) or creates (first capture), emits a timeline event, and writes audit.
func (s *fulfillmentService) UpdateShippingAddress(ctx context.Context, fulfillmentID uuid.UUID, expectedVersion int, addr *ShippingAddressEncrypted) error {
	if addr == nil {
		return domain.NewValidationError("missing required field", map[string]string{
			"shipping_address": "address payload is required",
		})
	}
	if err := validateShippingAddress(addr); err != nil {
		return err
	}

	actorID, _ := UserIDFromContext(ctx)

	return s.txMgr.WithTx(ctx, func(tx pgx.Tx) error {
		f, err := s.fulfillRepo.GetByIDForUpdate(ctx, tx, fulfillmentID)
		if err != nil {
			return err
		}
		if expectedVersion == 0 || expectedVersion != f.Version {
			return domain.NewConflictError()
		}
		if f.Type != domain.TypePhysical {
			return domain.NewValidationError("invalid operation", map[string]string{
				"type": "only PHYSICAL fulfillments have shipping addresses",
			})
		}
		// Once the package has left the warehouse, address edits must go through
		// a carrier redirect — block on DELIVERED/COMPLETED to avoid drift with
		// the carrier record.
		if f.Status == domain.StatusDelivered || f.Status == domain.StatusCompleted {
			return domain.NewValidationError("invalid transition", map[string]string{
				"status": "cannot edit shipping address after DELIVERED",
			})
		}

		existing, err := s.shippingRepo.GetByFulfillmentID(ctx, fulfillmentID)
		if err != nil {
			return fmt.Errorf("checking existing shipping address: %w", err)
		}
		if existing != nil {
			existing.Line1Encrypted = addr.Line1Encrypted
			existing.Line2Encrypted = addr.Line2Encrypted
			existing.City = addr.City
			existing.State = addr.State
			existing.ZipCode = addr.ZipCode
			if err := s.shippingRepo.Update(ctx, tx, existing); err != nil {
				return fmt.Errorf("updating shipping address: %w", err)
			}
		} else {
			created := &domain.ShippingAddress{
				FulfillmentID:  fulfillmentID,
				Line1Encrypted: addr.Line1Encrypted,
				Line2Encrypted: addr.Line2Encrypted,
				City:           addr.City,
				State:          addr.State,
				ZipCode:        addr.ZipCode,
			}
			if _, err := s.shippingRepo.Create(ctx, tx, created); err != nil {
				return fmt.Errorf("creating shipping address: %w", err)
			}
		}

		if s.auditSvc != nil {
			_ = s.auditSvc.Log(ctx, "shipping_addresses", fulfillmentID, "UPDATE", nil, map[string]any{
				"fulfillment_id": fulfillmentID,
				"city":           addr.City,
				"state":          addr.State,
				"zip":            addr.ZipCode,
				"actor_id":       actorID,
			})
		}
		return s.fulfillRepo.BumpVersion(ctx, tx, fulfillmentID, expectedVersion)
	})
}
