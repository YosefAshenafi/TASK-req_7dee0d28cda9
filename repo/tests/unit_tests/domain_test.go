// Package unit_tests covers pure-logic packages that need no database.
package unit_tests

import (
	"errors"
	"testing"

	"github.com/fulfillops/fulfillops/internal/domain"
)

// ── FulfillmentStatus ─────────────────────────────────────────────────────────

func TestFulfillmentStatusIsValid(t *testing.T) {
	valid := []domain.FulfillmentStatus{
		domain.StatusDraft, domain.StatusReadyToShip, domain.StatusShipped,
		domain.StatusDelivered, domain.StatusVoucherIssued, domain.StatusCompleted,
		domain.StatusOnHold, domain.StatusCanceled,
	}
	for _, s := range valid {
		if !s.IsValid() {
			t.Errorf("expected %q to be valid", s)
		}
	}
	for _, s := range []domain.FulfillmentStatus{"", "PENDING", "draft", "UNKNOWN"} {
		if s.IsValid() {
			t.Errorf("expected %q to be invalid", s)
		}
	}
}

func TestFulfillmentStatusIsTerminal(t *testing.T) {
	for _, s := range []domain.FulfillmentStatus{domain.StatusCompleted, domain.StatusCanceled} {
		if !s.IsTerminal() {
			t.Errorf("expected %q to be terminal", s)
		}
	}
	for _, s := range []domain.FulfillmentStatus{
		domain.StatusDraft, domain.StatusReadyToShip, domain.StatusShipped,
		domain.StatusDelivered, domain.StatusVoucherIssued, domain.StatusOnHold,
	} {
		if s.IsTerminal() {
			t.Errorf("expected %q to be non-terminal", s)
		}
	}
}

func TestIsTransitionAllowed_Allowed(t *testing.T) {
	cases := []struct{ from, to domain.FulfillmentStatus }{
		{domain.StatusDraft, domain.StatusReadyToShip},
		{domain.StatusReadyToShip, domain.StatusShipped},
		{domain.StatusReadyToShip, domain.StatusVoucherIssued},
		{domain.StatusReadyToShip, domain.StatusOnHold},
		{domain.StatusReadyToShip, domain.StatusCanceled},
		{domain.StatusShipped, domain.StatusDelivered},
		{domain.StatusShipped, domain.StatusOnHold},
		{domain.StatusShipped, domain.StatusCanceled},
		{domain.StatusVoucherIssued, domain.StatusCompleted},
		{domain.StatusVoucherIssued, domain.StatusOnHold},
		{domain.StatusVoucherIssued, domain.StatusCanceled},
		{domain.StatusDelivered, domain.StatusCompleted},
		{domain.StatusOnHold, domain.StatusReadyToShip},
	}
	for _, tc := range cases {
		if !domain.IsTransitionAllowed(tc.from, tc.to) {
			t.Errorf("expected %s → %s to be allowed", tc.from, tc.to)
		}
	}
}

func TestIsTransitionAllowed_NotAllowed(t *testing.T) {
	cases := []struct{ from, to domain.FulfillmentStatus }{
		{domain.StatusDraft, domain.StatusShipped},
		{domain.StatusDraft, domain.StatusCompleted},
		{domain.StatusCompleted, domain.StatusDraft},
		{domain.StatusCanceled, domain.StatusDraft},
		{domain.StatusCanceled, domain.StatusReadyToShip},
		{domain.StatusDelivered, domain.StatusShipped},
		{domain.FulfillmentStatus("UNKNOWN"), domain.StatusDraft},
	}
	for _, tc := range cases {
		if domain.IsTransitionAllowed(tc.from, tc.to) {
			t.Errorf("expected %s → %s to NOT be allowed", tc.from, tc.to)
		}
	}
}

func TestAllowedTransitionsMapCompleteness(t *testing.T) {
	for _, s := range []domain.FulfillmentStatus{
		domain.StatusDraft, domain.StatusReadyToShip, domain.StatusShipped,
		domain.StatusDelivered, domain.StatusVoucherIssued, domain.StatusCompleted,
		domain.StatusOnHold, domain.StatusCanceled,
	} {
		if _, ok := domain.AllowedTransitions[s]; !ok {
			t.Errorf("AllowedTransitions missing entry for %q", s)
		}
	}
}

// ── FulfillmentType ───────────────────────────────────────────────────────────

func TestFulfillmentTypeIsValid(t *testing.T) {
	if !domain.TypePhysical.IsValid() {
		t.Error("TypePhysical should be valid")
	}
	if !domain.TypeVoucher.IsValid() {
		t.Error("TypeVoucher should be valid")
	}
	for _, bad := range []domain.FulfillmentType{"", "DIGITAL", "physical"} {
		if bad.IsValid() {
			t.Errorf("expected %q to be invalid FulfillmentType", bad)
		}
	}
}

// ── SendLogChannel ────────────────────────────────────────────────────────────

func TestSendLogChannelIsValid(t *testing.T) {
	for _, c := range []domain.SendLogChannel{domain.ChannelInApp, domain.ChannelSMS, domain.ChannelEmail} {
		if !c.IsValid() {
			t.Errorf("expected channel %q to be valid", c)
		}
	}
	for _, bad := range []domain.SendLogChannel{"", "PUSH", "WHATSAPP"} {
		if bad.IsValid() {
			t.Errorf("expected channel %q to be invalid", bad)
		}
	}
}

// ── SendLogStatus ─────────────────────────────────────────────────────────────

func TestSendLogStatusIsValid(t *testing.T) {
	for _, s := range []domain.SendLogStatus{domain.SendQueued, domain.SendSent, domain.SendPrinted, domain.SendFailed} {
		if !s.IsValid() {
			t.Errorf("expected send status %q to be valid", s)
		}
	}
	if domain.SendLogStatus("DELIVERED").IsValid() {
		t.Error("DELIVERED should be invalid send status")
	}
}

// ── ExceptionType ─────────────────────────────────────────────────────────────

func TestExceptionTypeIsValid(t *testing.T) {
	for _, e := range []domain.ExceptionType{
		domain.ExceptionOverdueShipment, domain.ExceptionOverdueVoucher, domain.ExceptionManual,
	} {
		if !e.IsValid() {
			t.Errorf("expected exception type %q to be valid", e)
		}
	}
	for _, bad := range []domain.ExceptionType{"", "OTHER", "FRAUD"} {
		if bad.IsValid() {
			t.Errorf("expected exception type %q to be invalid", bad)
		}
	}
}

// ── ExceptionStatus ───────────────────────────────────────────────────────────

func TestExceptionStatusIsValid(t *testing.T) {
	for _, s := range []domain.ExceptionStatus{
		domain.ExceptionOpen, domain.ExceptionInvestigating,
		domain.ExceptionEscalated, domain.ExceptionResolved,
	} {
		if !s.IsValid() {
			t.Errorf("expected exception status %q to be valid", s)
		}
	}
	if domain.ExceptionStatus("CLOSED").IsValid() {
		t.Error("CLOSED should be invalid exception status")
	}
}

// ── UserRole ──────────────────────────────────────────────────────────────────

func TestUserRoleIsValid(t *testing.T) {
	for _, r := range []domain.UserRole{domain.RoleAdministrator, domain.RoleFulfillmentSpecialist, domain.RoleAuditor} {
		if !r.IsValid() {
			t.Errorf("expected role %q to be valid", r)
		}
	}
	for _, bad := range []domain.UserRole{"", "SUPERADMIN", "GUEST"} {
		if bad.IsValid() {
			t.Errorf("expected role %q to be invalid", bad)
		}
	}
}

// ── ReservationStatus ─────────────────────────────────────────────────────────

func TestReservationStatusIsValid(t *testing.T) {
	if !domain.ReservationActive.IsValid() {
		t.Error("ReservationActive should be valid")
	}
	if !domain.ReservationVoided.IsValid() {
		t.Error("ReservationVoided should be valid")
	}
	if domain.ReservationStatus("PENDING").IsValid() {
		t.Error("PENDING should be invalid reservation status")
	}
}

// ── TemplateCategory ──────────────────────────────────────────────────────────

func TestTemplateCategoryIsValid(t *testing.T) {
	for _, c := range []domain.TemplateCategory{
		domain.CategoryBookingResult, domain.CategoryBookingChange,
		domain.CategoryExpiration, domain.CategoryFulfillmentProgress,
	} {
		if !c.IsValid() {
			t.Errorf("expected category %q to be valid", c)
		}
	}
	if domain.TemplateCategory("REMINDER").IsValid() {
		t.Error("REMINDER should be invalid template category")
	}
}

// ── Sentinel Errors ───────────────────────────────────────────────────────────

func TestSentinelErrorsAreNonNilAndHaveMessages(t *testing.T) {
	for _, e := range []error{
		domain.ErrNotFound, domain.ErrConflict, domain.ErrInventoryUnavailable,
		domain.ErrPurchaseLimitReached, domain.ErrInvalidTransition, domain.ErrValidation,
		domain.ErrUnauthorized, domain.ErrForbidden, domain.ErrSoftDeleteExpired,
		domain.ErrAlreadyExists,
	} {
		if e == nil {
			t.Error("sentinel error should not be nil")
		}
		if e.Error() == "" {
			t.Errorf("sentinel error %T has empty message", e)
		}
	}
}

// ── DomainError ───────────────────────────────────────────────────────────────

func TestDomainErrorMessageAndUnwrap(t *testing.T) {
	de := &domain.DomainError{Code: "TEST", Message: "test message", Cause: domain.ErrNotFound}
	if de.Error() != "test message" {
		t.Errorf("Error() = %q; want %q", de.Error(), "test message")
	}
	if !errors.Is(de, domain.ErrNotFound) {
		t.Error("Unwrap should expose Cause via errors.Is")
	}
}

func TestNewValidationError(t *testing.T) {
	e := domain.NewValidationError("bad input", map[string]string{"name": "required"})
	if e.Code != "VALIDATION_ERROR" {
		t.Errorf("Code = %q; want VALIDATION_ERROR", e.Code)
	}
	if !errors.Is(e, domain.ErrValidation) {
		t.Error("should wrap ErrValidation")
	}
	if e.Details["name"] != "required" {
		t.Error("details field 'name' missing or wrong")
	}
}

func TestNewNotFoundError(t *testing.T) {
	e := domain.NewNotFoundError("tier")
	if e.Code != "NOT_FOUND" {
		t.Errorf("Code = %q; want NOT_FOUND", e.Code)
	}
	if !errors.Is(e, domain.ErrNotFound) {
		t.Error("should wrap ErrNotFound")
	}
	if e.Message != "tier not found" {
		t.Errorf("Message = %q; want %q", e.Message, "tier not found")
	}
}

func TestNewConflictError(t *testing.T) {
	e := domain.NewConflictError()
	if e.Code != "CONFLICT" {
		t.Errorf("Code = %q; want CONFLICT", e.Code)
	}
	if !errors.Is(e, domain.ErrConflict) {
		t.Error("should wrap ErrConflict")
	}
}

func TestNewInventoryError(t *testing.T) {
	e := domain.NewInventoryError()
	if e.Code != "INVENTORY_UNAVAILABLE" {
		t.Errorf("Code = %q; want INVENTORY_UNAVAILABLE", e.Code)
	}
	if !errors.Is(e, domain.ErrInventoryUnavailable) {
		t.Error("should wrap ErrInventoryUnavailable")
	}
}

func TestNewPurchaseLimitError(t *testing.T) {
	e := domain.NewPurchaseLimitError()
	if e.Code != "PURCHASE_LIMIT_REACHED" {
		t.Errorf("Code = %q; want PURCHASE_LIMIT_REACHED", e.Code)
	}
	if !errors.Is(e, domain.ErrPurchaseLimitReached) {
		t.Error("should wrap ErrPurchaseLimitReached")
	}
}

func TestNewTransitionError(t *testing.T) {
	e := domain.NewTransitionError(domain.StatusDraft, domain.StatusCompleted)
	if e.Code != "INVALID_TRANSITION" {
		t.Errorf("Code = %q; want INVALID_TRANSITION", e.Code)
	}
	if !errors.Is(e, domain.ErrInvalidTransition) {
		t.Error("should wrap ErrInvalidTransition")
	}
	if e.Message == "" {
		t.Error("transition error message should not be empty")
	}
}
