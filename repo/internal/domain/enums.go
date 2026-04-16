package domain

// FulfillmentStatus represents the lifecycle state of a fulfillment.
type FulfillmentStatus string

const (
	StatusDraft        FulfillmentStatus = "DRAFT"
	StatusReadyToShip  FulfillmentStatus = "READY_TO_SHIP"
	StatusShipped      FulfillmentStatus = "SHIPPED"
	StatusDelivered    FulfillmentStatus = "DELIVERED"
	StatusVoucherIssued FulfillmentStatus = "VOUCHER_ISSUED"
	StatusCompleted    FulfillmentStatus = "COMPLETED"
	StatusOnHold       FulfillmentStatus = "ON_HOLD"
	StatusCanceled     FulfillmentStatus = "CANCELED"
)

func (s FulfillmentStatus) IsValid() bool {
	switch s {
	case StatusDraft, StatusReadyToShip, StatusShipped, StatusDelivered,
		StatusVoucherIssued, StatusCompleted, StatusOnHold, StatusCanceled:
		return true
	}
	return false
}

func (s FulfillmentStatus) IsTerminal() bool {
	return s == StatusCompleted || s == StatusCanceled
}

// AllowedTransitions defines the valid state graph for fulfillments.
var AllowedTransitions = map[FulfillmentStatus][]FulfillmentStatus{
	StatusDraft:         {StatusReadyToShip},
	StatusReadyToShip:   {StatusShipped, StatusVoucherIssued, StatusOnHold, StatusCanceled},
	StatusShipped:       {StatusDelivered, StatusOnHold, StatusCanceled},
	StatusVoucherIssued: {StatusCompleted, StatusOnHold, StatusCanceled},
	StatusDelivered:     {StatusCompleted},
	StatusOnHold:        {StatusReadyToShip},
	StatusCanceled:      {},
	StatusCompleted:     {},
}

// IsTransitionAllowed returns true if the from→to transition is in the state graph.
func IsTransitionAllowed(from, to FulfillmentStatus) bool {
	allowed, ok := AllowedTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// FulfillmentType distinguishes physical shipments from e-vouchers.
type FulfillmentType string

const (
	TypePhysical FulfillmentType = "PHYSICAL"
	TypeVoucher  FulfillmentType = "VOUCHER"
)

func (t FulfillmentType) IsValid() bool {
	return t == TypePhysical || t == TypeVoucher
}

// SendLogChannel is the delivery channel for a message.
type SendLogChannel string

const (
	ChannelInApp SendLogChannel = "IN_APP"
	ChannelSMS   SendLogChannel = "SMS"
	ChannelEmail SendLogChannel = "EMAIL"
)

func (c SendLogChannel) IsValid() bool {
	switch c {
	case ChannelInApp, ChannelSMS, ChannelEmail:
		return true
	}
	return false
}

// SendLogStatus tracks the delivery state of a send_log entry.
type SendLogStatus string

const (
	SendQueued  SendLogStatus = "QUEUED"
	SendSent    SendLogStatus = "SENT"
	SendPrinted SendLogStatus = "PRINTED"
	SendFailed  SendLogStatus = "FAILED"
)

func (s SendLogStatus) IsValid() bool {
	switch s {
	case SendQueued, SendSent, SendPrinted, SendFailed:
		return true
	}
	return false
}

// ExceptionType categorises a fulfillment exception.
type ExceptionType string

const (
	ExceptionOverdueShipment ExceptionType = "OVERDUE_SHIPMENT"
	ExceptionOverdueVoucher  ExceptionType = "OVERDUE_VOUCHER"
	ExceptionManual          ExceptionType = "MANUAL"
)

func (e ExceptionType) IsValid() bool {
	switch e {
	case ExceptionOverdueShipment, ExceptionOverdueVoucher, ExceptionManual:
		return true
	}
	return false
}

// ExceptionStatus tracks the workflow state of an exception.
type ExceptionStatus string

const (
	ExceptionOpen         ExceptionStatus = "OPEN"
	ExceptionInvestigating ExceptionStatus = "INVESTIGATING"
	ExceptionEscalated    ExceptionStatus = "ESCALATED"
	ExceptionResolved     ExceptionStatus = "RESOLVED"
)

func (s ExceptionStatus) IsValid() bool {
	switch s {
	case ExceptionOpen, ExceptionInvestigating, ExceptionEscalated, ExceptionResolved:
		return true
	}
	return false
}

// UserRole defines access level for application users.
type UserRole string

const (
	RoleAdministrator        UserRole = "ADMINISTRATOR"
	RoleFulfillmentSpecialist UserRole = "FULFILLMENT_SPECIALIST"
	RoleAuditor              UserRole = "AUDITOR"
)

func (r UserRole) IsValid() bool {
	switch r {
	case RoleAdministrator, RoleFulfillmentSpecialist, RoleAuditor:
		return true
	}
	return false
}

// ReservationStatus tracks whether a tier inventory slot is held or released.
type ReservationStatus string

const (
	ReservationActive ReservationStatus = "ACTIVE"
	ReservationVoided ReservationStatus = "VOIDED"
)

func (s ReservationStatus) IsValid() bool {
	return s == ReservationActive || s == ReservationVoided
}

// ExportStatus tracks the generation state of a report export.
type ExportStatus string

const (
	ExportQueued     ExportStatus = "QUEUED"
	ExportProcessing ExportStatus = "PROCESSING"
	ExportCompleted  ExportStatus = "COMPLETED"
	ExportFailed     ExportStatus = "FAILED"
)

// TemplateCategory classifies message templates.
type TemplateCategory string

const (
	CategoryBookingResult      TemplateCategory = "BOOKING_RESULT"
	CategoryBookingChange      TemplateCategory = "BOOKING_CHANGE"
	CategoryExpiration         TemplateCategory = "EXPIRATION"
	CategoryFulfillmentProgress TemplateCategory = "FULFILLMENT_PROGRESS"
)

func (c TemplateCategory) IsValid() bool {
	switch c {
	case CategoryBookingResult, CategoryBookingChange, CategoryExpiration, CategoryFulfillmentProgress:
		return true
	}
	return false
}

// JobStatus is the run state of a scheduled job execution.
type JobStatus string

const (
	JobRunning   JobStatus = "RUNNING"
	JobCompleted JobStatus = "COMPLETED"
	JobFailed    JobStatus = "FAILED"
)
