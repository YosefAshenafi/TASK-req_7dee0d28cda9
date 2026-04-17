package service

// Unit tests for the shipping-address maintenance path added to FulfillmentService.
//
// Covers:
//   - Transition to READY_TO_SHIP with an existing address AND new payload
//     updates the row (does not collide with the one-row-per-fulfillment
//     uniqueness constraint).
//   - UpdateShippingAddress maintenance endpoint updates an existing row.
//   - UpdateShippingAddress creates a new row when none exists.
//   - UpdateShippingAddress rejects voucher-type fulfillments.
//   - UpdateShippingAddress rejects DELIVERED/COMPLETED fulfillments.

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/fulfillops/fulfillops/internal/domain"
)

func newTestFulfillSvc(tierRepo *fixedTierRepo, custRepo *fixedCustomerRepo,
	fulfillRepo *zeroFulfillmentRepo, shippingRepo *stubShippingRepo) FulfillmentService {
	return NewFulfillmentService(
		&inlineTxManager{},
		fulfillRepo,
		tierRepo,
		custRepo,
		&stubTimelineRepo{},
		shippingRepo,
		nil,
		NewInventoryService(tierRepo, &stubReservationRepo{}),
		nil,
	)
}

func TestTransition_ReadyToShip_UpdatesExistingAddressInsteadOfColliding(t *testing.T) {
	tier := &domain.RewardTier{ID: uuid.New(), InventoryCount: 5, PurchaseLimit: 3}
	tierRepo := &fixedTierRepo{tier: tier}
	custRepo := &fixedCustomerRepo{customer: &domain.Customer{ID: uuid.New()}}

	fulfillID := uuid.New()
	physical := &domain.Fulfillment{
		ID:      fulfillID,
		TierID:  tier.ID,
		Type:    domain.TypePhysical,
		Status:  domain.StatusDraft,
		Version: 1,
	}
	fulfillRepo := &zeroFulfillmentRepo{created: physical}
	shippingRepo := &stubShippingRepo{
		existing: &domain.ShippingAddress{
			ID: uuid.New(), FulfillmentID: fulfillID,
			Line1Encrypted: []byte("old"), City: "NY", State: "NY", ZipCode: "10001",
		},
	}
	svc := newTestFulfillSvc(tierRepo, custRepo, fulfillRepo, shippingRepo)

	_, err := svc.Transition(context.Background(), TransitionInput{
		FulfillmentID: fulfillID,
		ToStatus:      domain.StatusReadyToShip,
		ShippingAddr: &ShippingAddressEncrypted{
			Line1Encrypted: []byte("new"), City: "SF", State: "CA", ZipCode: "94105",
		},
	})
	if err != nil {
		t.Fatalf("Transition: %v", err)
	}
	if shippingRepo.updated != 1 {
		t.Fatalf("expected Update to be called when address already exists, got updated=%d", shippingRepo.updated)
	}
	if shippingRepo.created != 0 {
		t.Fatalf("expected NO Create when address exists (would collide), got created=%d", shippingRepo.created)
	}
	if shippingRepo.lastSeen == nil || shippingRepo.lastSeen.City != "SF" {
		t.Fatalf("expected city=SF after update, got %+v", shippingRepo.lastSeen)
	}
}

func TestUpdateShippingAddress_Maintenance_UpdatesExisting(t *testing.T) {
	tier := &domain.RewardTier{ID: uuid.New(), InventoryCount: 5, PurchaseLimit: 3}
	tierRepo := &fixedTierRepo{tier: tier}
	custRepo := &fixedCustomerRepo{customer: &domain.Customer{ID: uuid.New()}}

	fulfillID := uuid.New()
	physical := &domain.Fulfillment{
		ID: fulfillID, TierID: tier.ID, Type: domain.TypePhysical, Status: domain.StatusReadyToShip, Version: 1,
	}
	fulfillRepo := &zeroFulfillmentRepo{created: physical}
	shippingRepo := &stubShippingRepo{
		existing: &domain.ShippingAddress{
			ID: uuid.New(), FulfillmentID: fulfillID, City: "OldCity", State: "NY", ZipCode: "10001",
			Line1Encrypted: []byte("enc-old"),
		},
	}
	svc := newTestFulfillSvc(tierRepo, custRepo, fulfillRepo, shippingRepo)

	err := svc.UpdateShippingAddress(context.Background(), fulfillID, 1, &ShippingAddressEncrypted{
		Line1Encrypted: []byte("enc-new"), City: "NewCity", State: "CA", ZipCode: "94105",
	})
	if err != nil {
		t.Fatalf("UpdateShippingAddress: %v", err)
	}
	if shippingRepo.updated != 1 {
		t.Fatalf("expected 1 update, got %d", shippingRepo.updated)
	}
	if shippingRepo.created != 0 {
		t.Fatalf("expected 0 creates, got %d", shippingRepo.created)
	}
	if shippingRepo.lastSeen.City != "NewCity" {
		t.Fatalf("expected city=NewCity, got %q", shippingRepo.lastSeen.City)
	}
}

func TestUpdateShippingAddress_Maintenance_CreatesWhenMissing(t *testing.T) {
	tier := &domain.RewardTier{ID: uuid.New(), InventoryCount: 5, PurchaseLimit: 3}
	tierRepo := &fixedTierRepo{tier: tier}
	custRepo := &fixedCustomerRepo{customer: &domain.Customer{ID: uuid.New()}}

	fulfillID := uuid.New()
	physical := &domain.Fulfillment{
		ID: fulfillID, TierID: tier.ID, Type: domain.TypePhysical, Status: domain.StatusDraft, Version: 1,
	}
	fulfillRepo := &zeroFulfillmentRepo{created: physical}
	shippingRepo := &stubShippingRepo{existing: nil}
	svc := newTestFulfillSvc(tierRepo, custRepo, fulfillRepo, shippingRepo)

	err := svc.UpdateShippingAddress(context.Background(), fulfillID, 1, &ShippingAddressEncrypted{
		Line1Encrypted: []byte("enc"), City: "SF", State: "CA", ZipCode: "94105",
	})
	if err != nil {
		t.Fatalf("UpdateShippingAddress: %v", err)
	}
	if shippingRepo.created != 1 {
		t.Fatalf("expected 1 create, got %d", shippingRepo.created)
	}
}

func TestUpdateShippingAddress_Rejects_VoucherType(t *testing.T) {
	tier := &domain.RewardTier{ID: uuid.New(), InventoryCount: 5, PurchaseLimit: 3}
	tierRepo := &fixedTierRepo{tier: tier}
	custRepo := &fixedCustomerRepo{customer: &domain.Customer{ID: uuid.New()}}

	fulfillID := uuid.New()
	voucher := &domain.Fulfillment{
		ID: fulfillID, TierID: tier.ID, Type: domain.TypeVoucher, Status: domain.StatusDraft, Version: 1,
	}
	svc := newTestFulfillSvc(tierRepo, custRepo, &zeroFulfillmentRepo{created: voucher}, &stubShippingRepo{})

	err := svc.UpdateShippingAddress(context.Background(), fulfillID, 1, &ShippingAddressEncrypted{
		Line1Encrypted: []byte("enc"), City: "SF", State: "CA", ZipCode: "94105",
	})
	if err == nil {
		t.Fatal("expected error for voucher-type fulfillment")
	}
}

func TestUpdateShippingAddress_Rejects_DeliveredFulfillment(t *testing.T) {
	tier := &domain.RewardTier{ID: uuid.New(), InventoryCount: 5, PurchaseLimit: 3}
	tierRepo := &fixedTierRepo{tier: tier}
	custRepo := &fixedCustomerRepo{customer: &domain.Customer{ID: uuid.New()}}

	fulfillID := uuid.New()
	delivered := &domain.Fulfillment{
		ID: fulfillID, TierID: tier.ID, Type: domain.TypePhysical, Status: domain.StatusDelivered, Version: 1,
	}
	svc := newTestFulfillSvc(tierRepo, custRepo, &zeroFulfillmentRepo{created: delivered}, &stubShippingRepo{})

	err := svc.UpdateShippingAddress(context.Background(), fulfillID, 1, &ShippingAddressEncrypted{
		Line1Encrypted: []byte("enc"), City: "SF", State: "CA", ZipCode: "94105",
	})
	if err == nil {
		t.Fatal("expected error: cannot edit shipping address after DELIVERED")
	}
}

func TestUpdateShippingAddress_Rejects_StaleVersion(t *testing.T) {
	tier := &domain.RewardTier{ID: uuid.New(), InventoryCount: 5, PurchaseLimit: 3}
	tierRepo := &fixedTierRepo{tier: tier}
	custRepo := &fixedCustomerRepo{customer: &domain.Customer{ID: uuid.New()}}

	fulfillID := uuid.New()
	physical := &domain.Fulfillment{
		ID: fulfillID, TierID: tier.ID, Type: domain.TypePhysical, Status: domain.StatusReadyToShip, Version: 2,
	}
	fulfillRepo := &zeroFulfillmentRepo{created: physical}
	svc := newTestFulfillSvc(tierRepo, custRepo, fulfillRepo, &stubShippingRepo{})

	err := svc.UpdateShippingAddress(context.Background(), fulfillID, 1, &ShippingAddressEncrypted{
		Line1Encrypted: []byte("enc"), City: "SF", State: "CA", ZipCode: "94105",
	})
	if err == nil {
		t.Fatal("expected stale version conflict")
	}
}
