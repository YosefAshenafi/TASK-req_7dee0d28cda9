package domain

import (
	"time"

	"github.com/google/uuid"
)

// ── Users ─────────────────────────────────────────────────────────────────────

type User struct {
	ID           uuid.UUID  `db:"id"            json:"id"`
	Username     string     `db:"username"      json:"username"`
	Email        string     `db:"email"         json:"email"`
	PasswordHash string     `db:"password_hash" json:"-"`
	Role         UserRole   `db:"role"          json:"role"`
	IsActive     bool       `db:"is_active"     json:"is_active"`
	Version      int        `db:"version"       json:"version"`
	CreatedAt    time.Time  `db:"created_at"    json:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at"    json:"updated_at"`
}

// ── Customers ─────────────────────────────────────────────────────────────────

// Customer holds the DB row with encrypted fields as raw bytes.
type Customer struct {
	ID               uuid.UUID  `db:"id"                json:"id"`
	Name             string     `db:"name"              json:"name"`
	PhoneEncrypted   []byte     `db:"phone_encrypted"   json:"-"`
	EmailEncrypted   []byte     `db:"email_encrypted"   json:"-"`
	AddressEncrypted []byte     `db:"address_encrypted" json:"-"`
	Version          int        `db:"version"           json:"version"`
	CreatedAt        time.Time  `db:"created_at"        json:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at"        json:"updated_at"`
	DeletedAt        *time.Time `db:"deleted_at"        json:"deleted_at,omitempty"`
	DeletedBy        *uuid.UUID `db:"deleted_by"        json:"deleted_by,omitempty"`
}

// CustomerResponse is the API-safe customer shape with masked PII.
type CustomerResponse struct {
	ID           uuid.UUID  `json:"id"`
	Name         string     `json:"name"`
	PhoneMasked  string     `json:"phone_masked"`
	EmailMasked  string     `json:"email_masked"`
	AddressMasked string    `json:"address_masked"`
	Version      int        `json:"version"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// ── Reward Tiers ──────────────────────────────────────────────────────────────

type RewardTier struct {
	ID             uuid.UUID  `db:"id"              json:"id"`
	Name           string     `db:"name"            json:"name"`
	Description    *string    `db:"description"     json:"description"`
	InventoryCount int        `db:"inventory_count" json:"inventory_count"`
	PurchaseLimit  int        `db:"purchase_limit"  json:"purchase_limit"`
	AlertThreshold int        `db:"alert_threshold" json:"alert_threshold"`
	Version        int        `db:"version"         json:"version"`
	CreatedAt      time.Time  `db:"created_at"      json:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"      json:"updated_at"`
	DeletedAt      *time.Time `db:"deleted_at"      json:"deleted_at,omitempty"`
	DeletedBy      *uuid.UUID `db:"deleted_by"      json:"deleted_by,omitempty"`
}

// ── Fulfillments ──────────────────────────────────────────────────────────────

type Fulfillment struct {
	ID                    uuid.UUID         `db:"id"                     json:"id"`
	TierID                uuid.UUID         `db:"tier_id"                json:"tier_id"`
	CustomerID            uuid.UUID         `db:"customer_id"            json:"customer_id"`
	Type                  FulfillmentType   `db:"type"                   json:"type"`
	Status                FulfillmentStatus `db:"status"                 json:"status"`
	CarrierName           *string           `db:"carrier_name"           json:"carrier_name"`
	TrackingNumber        *string           `db:"tracking_number"        json:"tracking_number"`
	VoucherCodeEncrypted  []byte            `db:"voucher_code_encrypted" json:"-"`
	VoucherExpiration     *time.Time        `db:"voucher_expiration"     json:"voucher_expiration"`
	HoldReason            *string           `db:"hold_reason"            json:"hold_reason"`
	CancelReason          *string           `db:"cancel_reason"          json:"cancel_reason"`
	ReadyAt               *time.Time        `db:"ready_at"               json:"ready_at"`
	ShippedAt             *time.Time        `db:"shipped_at"             json:"shipped_at"`
	DeliveredAt           *time.Time        `db:"delivered_at"           json:"delivered_at"`
	CompletedAt           *time.Time        `db:"completed_at"           json:"completed_at"`
	Version               int               `db:"version"                json:"version"`
	CreatedAt             time.Time         `db:"created_at"             json:"created_at"`
	UpdatedAt             time.Time         `db:"updated_at"             json:"updated_at"`
	DeletedAt             *time.Time        `db:"deleted_at"             json:"deleted_at,omitempty"`
	DeletedBy             *uuid.UUID        `db:"deleted_by"             json:"deleted_by,omitempty"`
}

// FulfillmentResponse is the API-safe fulfillment shape with masked voucher.
type FulfillmentResponse struct {
	ID                uuid.UUID         `json:"id"`
	TierID            uuid.UUID         `json:"tier_id"`
	CustomerID        uuid.UUID         `json:"customer_id"`
	Type              FulfillmentType   `json:"type"`
	Status            FulfillmentStatus `json:"status"`
	CarrierName       *string           `json:"carrier_name"`
	TrackingNumber    *string           `json:"tracking_number"`
	VoucherCodeMasked *string           `json:"voucher_code_masked"`
	VoucherExpiration *time.Time        `json:"voucher_expiration"`
	HoldReason        *string           `json:"hold_reason"`
	CancelReason      *string           `json:"cancel_reason"`
	ReadyAt           *time.Time        `json:"ready_at"`
	ShippedAt         *time.Time        `json:"shipped_at"`
	DeliveredAt       *time.Time        `json:"delivered_at"`
	CompletedAt       *time.Time        `json:"completed_at"`
	Version           int               `json:"version"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

// ── Reservations ──────────────────────────────────────────────────────────────

type Reservation struct {
	ID            uuid.UUID         `db:"id"             json:"id"`
	TierID        uuid.UUID         `db:"tier_id"        json:"tier_id"`
	FulfillmentID uuid.UUID         `db:"fulfillment_id" json:"fulfillment_id"`
	Status        ReservationStatus `db:"status"         json:"status"`
	CreatedAt     time.Time         `db:"created_at"     json:"created_at"`
	UpdatedAt     time.Time         `db:"updated_at"     json:"updated_at"`
}

// ── Shipping Addresses ────────────────────────────────────────────────────────

type ShippingAddress struct {
	ID              uuid.UUID `db:"id"               json:"id"`
	FulfillmentID   uuid.UUID `db:"fulfillment_id"   json:"fulfillment_id"`
	Line1Encrypted  []byte    `db:"line_1_encrypted" json:"-"`
	Line2Encrypted  []byte    `db:"line_2_encrypted" json:"-"`
	City            string    `db:"city"             json:"city"`
	State           string    `db:"state"            json:"state"`
	ZipCode         string    `db:"zip_code"         json:"zip_code"`
	CreatedAt       time.Time `db:"created_at"       json:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"       json:"updated_at"`
}

// ShippingAddressResponse is the masked display form of a shipping address.
type ShippingAddressResponse struct {
	Line1   string `json:"line_1"`
	Line2   string `json:"line_2,omitempty"`
	City    string `json:"city"`
	State   string `json:"state"`
	ZipCode string `json:"zip_code"`
}

// ── Fulfillment Timeline ──────────────────────────────────────────────────────

type TimelineEvent struct {
	ID            uuid.UUID         `db:"id"             json:"id"`
	FulfillmentID uuid.UUID         `db:"fulfillment_id" json:"fulfillment_id"`
	FromStatus    *FulfillmentStatus `db:"from_status"   json:"from_status"`
	ToStatus      FulfillmentStatus `db:"to_status"      json:"to_status"`
	Reason        *string           `db:"reason"         json:"reason"`
	Metadata      []byte            `db:"metadata"       json:"metadata"`
	ChangedBy     *uuid.UUID        `db:"changed_by"     json:"changed_by"`
	ChangedAt     time.Time         `db:"changed_at"     json:"changed_at"`
}

// ── Fulfillment Exceptions ────────────────────────────────────────────────────

type FulfillmentException struct {
	ID             uuid.UUID       `db:"id"              json:"id"`
	FulfillmentID  uuid.UUID       `db:"fulfillment_id"  json:"fulfillment_id"`
	Type           ExceptionType   `db:"type"            json:"type"`
	Status         ExceptionStatus `db:"status"          json:"status"`
	ResolutionNote *string         `db:"resolution_note" json:"resolution_note"`
	OpenedBy       *uuid.UUID      `db:"opened_by"       json:"opened_by"`
	ResolvedBy     *uuid.UUID      `db:"resolved_by"     json:"resolved_by"`
	CreatedAt      time.Time       `db:"created_at"      json:"created_at"`
	UpdatedAt      time.Time       `db:"updated_at"      json:"updated_at"`
}

// ExceptionEvent is a threaded work-order note on an exception.
type ExceptionEvent struct {
	ID          uuid.UUID  `db:"id"           json:"id"`
	ExceptionID uuid.UUID  `db:"exception_id" json:"exception_id"`
	EventType   string     `db:"event_type"   json:"event_type"`
	Content     string     `db:"content"      json:"content"`
	CreatedBy   *uuid.UUID `db:"created_by"   json:"created_by"`
	CreatedAt   time.Time  `db:"created_at"   json:"created_at"`
}

// ── Message Templates ─────────────────────────────────────────────────────────

type MessageTemplate struct {
	ID           uuid.UUID        `db:"id"            json:"id"`
	Name         string           `db:"name"          json:"name"`
	Category     TemplateCategory `db:"category"      json:"category"`
	Channel      SendLogChannel   `db:"channel"       json:"channel"`
	BodyTemplate string           `db:"body_template" json:"body_template"`
	Version      int              `db:"version"       json:"version"`
	CreatedAt    time.Time        `db:"created_at"    json:"created_at"`
	UpdatedAt    time.Time        `db:"updated_at"    json:"updated_at"`
	DeletedAt    *time.Time       `db:"deleted_at"    json:"deleted_at,omitempty"`
	DeletedBy    *uuid.UUID       `db:"deleted_by"    json:"deleted_by,omitempty"`
}

// ── Send Logs ─────────────────────────────────────────────────────────────────

type SendLog struct {
	ID           uuid.UUID      `db:"id"            json:"id"`
	TemplateID   *uuid.UUID     `db:"template_id"   json:"template_id"`
	RecipientID  uuid.UUID      `db:"recipient_id"  json:"recipient_id"`
	Channel      SendLogChannel `db:"channel"       json:"channel"`
	Status       SendLogStatus  `db:"status"        json:"status"`
	AttemptCount int            `db:"attempt_count" json:"attempt_count"`
	NextRetryAt  *time.Time     `db:"next_retry_at" json:"next_retry_at"`
	PrintedBy    *uuid.UUID     `db:"printed_by"    json:"printed_by"`
	PrintedAt    *time.Time     `db:"printed_at"    json:"printed_at"`
	Context      []byte         `db:"context"       json:"context"`
	ErrorMessage *string        `db:"error_message" json:"error_message"`
	CreatedAt    time.Time      `db:"created_at"    json:"created_at"`
	UpdatedAt    time.Time      `db:"updated_at"    json:"updated_at"`
}

// ── Notifications ─────────────────────────────────────────────────────────────

type Notification struct {
	ID        uuid.UUID `db:"id"         json:"id"`
	UserID    uuid.UUID `db:"user_id"    json:"user_id"`
	Title     string    `db:"title"      json:"title"`
	Body      *string   `db:"body"       json:"body"`
	IsRead    bool      `db:"is_read"    json:"is_read"`
	Context   []byte    `db:"context"    json:"context"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// ── Report Exports ────────────────────────────────────────────────────────────

type ReportExport struct {
	ID               uuid.UUID    `db:"id"                json:"id"`
	ReportType       string       `db:"report_type"       json:"report_type"`
	Filters          []byte       `db:"filters"           json:"filters"`
	FilePath         *string      `db:"file_path"         json:"file_path"`
	FileSizeBytes    *int64       `db:"file_size_bytes"   json:"file_size_bytes"`
	ChecksumSHA256   *string      `db:"checksum_sha256"   json:"checksum_sha256"`
	IncludeSensitive bool         `db:"include_sensitive" json:"include_sensitive"`
	Status           ExportStatus `db:"status"            json:"status"`
	ExpiresAt        *time.Time   `db:"expires_at"        json:"expires_at"`
	GeneratedBy      *uuid.UUID   `db:"generated_by"      json:"generated_by"`
	CreatedAt        time.Time    `db:"created_at"        json:"created_at"`
	UpdatedAt        time.Time    `db:"updated_at"        json:"updated_at"`
}

// ── System Settings ───────────────────────────────────────────────────────────

type SystemSetting struct {
	ID        uuid.UUID  `db:"id"         json:"id"`
	Key       string     `db:"key"        json:"key"`
	Value     []byte     `db:"value"      json:"value"`
	UpdatedBy *uuid.UUID `db:"updated_by" json:"updated_by"`
	UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
}

// ── Blackout Dates ────────────────────────────────────────────────────────────

type BlackoutDate struct {
	ID          uuid.UUID  `db:"id"          json:"id"`
	Date        time.Time  `db:"date"        json:"date"`
	Description *string    `db:"description" json:"description"`
	CreatedBy   *uuid.UUID `db:"created_by"  json:"created_by"`
	CreatedAt   time.Time  `db:"created_at"  json:"created_at"`
}

// ── Job Run History ───────────────────────────────────────────────────────────

type JobRunHistory struct {
	ID               uuid.UUID  `db:"id"                json:"id"`
	JobName          string     `db:"job_name"          json:"job_name"`
	Status           JobStatus  `db:"status"            json:"status"`
	StartedAt        time.Time  `db:"started_at"        json:"started_at"`
	FinishedAt       *time.Time `db:"finished_at"       json:"finished_at"`
	RecordsProcessed int        `db:"records_processed" json:"records_processed"`
	ErrorStack       *string    `db:"error_stack"       json:"error_stack"`
	CreatedAt        time.Time  `db:"created_at"        json:"created_at"`
}

// ── Audit Logs ────────────────────────────────────────────────────────────────

type AuditLog struct {
	ID          uuid.UUID  `db:"id"           json:"id"`
	TableName   string     `db:"table_name"   json:"table_name"`
	RecordID    *uuid.UUID `db:"record_id"    json:"record_id"`
	Operation   string     `db:"operation"    json:"operation"`
	PerformedBy *uuid.UUID `db:"performed_by" json:"performed_by"`
	BeforeState []byte     `db:"before_state" json:"before_state"`
	AfterState  []byte     `db:"after_state"  json:"after_state"`
	IPAddress   *string    `db:"ip_address"   json:"ip_address"`
	RequestID   *string    `db:"request_id"   json:"request_id"`
	CreatedAt   time.Time  `db:"created_at"   json:"created_at"`
}

// ── Pagination ────────────────────────────────────────────────────────────────

type PageRequest struct {
	Page     int `form:"page"      json:"page"`
	PageSize int `form:"page_size" json:"page_size"`
}

func (p *PageRequest) Normalize() {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 || p.PageSize > 100 {
		p.PageSize = 20
	}
}

func (p PageRequest) Offset() int {
	return (p.Page - 1) * p.PageSize
}

type PageResponse[T any] struct {
	Items    []T `json:"items"`
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}
