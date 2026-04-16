package service

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
)

// contextKey is an unexported type for context keys in this package.
type contextKey int

const (
	contextKeyUserID    contextKey = iota
	contextKeyIPAddress contextKey = iota
	contextKeyRequestID contextKey = iota
)

// WithUserID returns a context carrying the authenticated user's ID.
func WithUserID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, contextKeyUserID, id)
}

// WithIPAddress returns a context carrying the client's IP address.
func WithIPAddress(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, contextKeyIPAddress, ip)
}

// WithRequestID returns a context carrying an X-Request-Id trace token.
func WithRequestID(ctx context.Context, reqID string) context.Context {
	return context.WithValue(ctx, contextKeyRequestID, reqID)
}

// UserIDFromContext extracts the user ID stored by WithUserID.
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(contextKeyUserID).(uuid.UUID)
	return v, ok
}

// IPFromContext extracts the IP address stored by WithIPAddress.
func IPFromContext(ctx context.Context) string {
	v, _ := ctx.Value(contextKeyIPAddress).(string)
	return v
}

// RequestIDFromContext extracts the request ID stored by WithRequestID.
func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(contextKeyRequestID).(string)
	return v
}

// AuditService records audit trail entries.
type AuditService interface {
	// Log records a single audit entry. before/after may be nil (e.g. for CREATE/DELETE).
	Log(ctx context.Context, tableName string, recordID uuid.UUID, operation string, before, after any) error
}

type auditService struct {
	repo repository.AuditRepository
}

// NewAuditService creates an AuditService backed by the given repository.
func NewAuditService(repo repository.AuditRepository) AuditService {
	return &auditService{repo: repo}
}

func (s *auditService) Log(ctx context.Context, tableName string, recordID uuid.UUID, operation string, before, after any) error {
	entry := &domain.AuditLog{
		TableName: tableName,
		RecordID:  &recordID,
		Operation: operation,
	}

	if uid, ok := UserIDFromContext(ctx); ok {
		entry.PerformedBy = &uid
	}
	if ip := IPFromContext(ctx); ip != "" {
		entry.IPAddress = &ip
	}
	if rid := RequestIDFromContext(ctx); rid != "" {
		entry.RequestID = &rid
	}

	if before != nil {
		b, err := json.Marshal(before)
		if err != nil {
			return err
		}
		entry.BeforeState = b
	}
	if after != nil {
		b, err := json.Marshal(after)
		if err != nil {
			return err
		}
		entry.AfterState = b
	}

	return s.repo.Create(ctx, entry)
}
