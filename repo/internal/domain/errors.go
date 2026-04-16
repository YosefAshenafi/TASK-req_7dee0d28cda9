package domain

import "errors"

// Sentinel errors — use errors.Is for matching.
var (
	ErrNotFound            = errors.New("not found")
	ErrConflict            = errors.New("version conflict")
	ErrInventoryUnavailable = errors.New("inventory unavailable")
	ErrPurchaseLimitReached = errors.New("purchase limit reached")
	ErrInvalidTransition   = errors.New("invalid status transition")
	ErrValidation          = errors.New("validation error")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrForbidden           = errors.New("forbidden")
	ErrSoftDeleteExpired   = errors.New("soft-delete recovery window expired")
	ErrAlreadyExists       = errors.New("already exists")
)

// DomainError carries structured error info matching the API error shape.
type DomainError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
	Cause   error             `json:"-"`
}

func (e *DomainError) Error() string { return e.Message }

func (e *DomainError) Unwrap() error { return e.Cause }

// NewValidationError creates a validation DomainError with field details.
func NewValidationError(message string, fields map[string]string) *DomainError {
	return &DomainError{
		Code:    "VALIDATION_ERROR",
		Message: message,
		Details: fields,
		Cause:   ErrValidation,
	}
}

// NewNotFoundError creates a not-found DomainError.
func NewNotFoundError(resource string) *DomainError {
	return &DomainError{
		Code:    "NOT_FOUND",
		Message: resource + " not found",
		Cause:   ErrNotFound,
	}
}

// NewConflictError creates a version-conflict DomainError.
func NewConflictError() *DomainError {
	return &DomainError{
		Code:    "CONFLICT",
		Message: "resource was modified by another request; reload and retry",
		Cause:   ErrConflict,
	}
}

// NewInventoryError creates an inventory-unavailable DomainError.
func NewInventoryError() *DomainError {
	return &DomainError{
		Code:    "INVENTORY_UNAVAILABLE",
		Message: "no inventory available for this reward tier",
		Cause:   ErrInventoryUnavailable,
	}
}

// NewPurchaseLimitError creates a purchase-limit DomainError.
func NewPurchaseLimitError() *DomainError {
	return &DomainError{
		Code:    "PURCHASE_LIMIT_REACHED",
		Message: "customer has reached the purchase limit for this tier in the last 30 days",
		Cause:   ErrPurchaseLimitReached,
	}
}

// NewTransitionError creates an invalid-transition DomainError.
func NewTransitionError(from, to FulfillmentStatus) *DomainError {
	return &DomainError{
		Code:    "INVALID_TRANSITION",
		Message: "transition from " + string(from) + " to " + string(to) + " is not allowed",
		Cause:   ErrInvalidTransition,
	}
}
