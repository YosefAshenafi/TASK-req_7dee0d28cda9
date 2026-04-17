package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
)

// ExceptionService manages fulfillment exceptions.
type ExceptionService interface {
	Create(ctx context.Context, fulfillmentID uuid.UUID, exType domain.ExceptionType, note string) (*domain.FulfillmentException, error)
	// CreateSystem is used by scheduler-originated flows (e.g. overdue-check).
	// It attributes opened_by and audit_logs.performed_by to the system actor
	// so compliance never sees a NULL actor on auto-generated rows.
	CreateSystem(ctx context.Context, fulfillmentID uuid.UUID, exType domain.ExceptionType, note string) (*domain.FulfillmentException, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.ExceptionStatus, resolutionNote string) (*domain.FulfillmentException, error)
	AddEvent(ctx context.Context, exceptionID uuid.UUID, eventType, content string) (*domain.ExceptionEvent, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.FulfillmentException, error)
	List(ctx context.Context, filters repository.ExceptionFilters) ([]domain.FulfillmentException, error)
}

type exceptionService struct {
	exceptionRepo      repository.ExceptionRepository
	exceptionEventRepo repository.ExceptionEventRepository
	auditSvc           AuditService
}

// NewExceptionService creates an ExceptionService.
func NewExceptionService(
	exceptionRepo repository.ExceptionRepository,
	exceptionEventRepo repository.ExceptionEventRepository,
	auditSvc AuditService,
) ExceptionService {
	return &exceptionService{
		exceptionRepo:      exceptionRepo,
		exceptionEventRepo: exceptionEventRepo,
		auditSvc:           auditSvc,
	}
}

func (s *exceptionService) Create(ctx context.Context, fulfillmentID uuid.UUID, exType domain.ExceptionType, note string) (*domain.FulfillmentException, error) {
	if !exType.IsValid() {
		return nil, domain.NewValidationError("invalid exception type", map[string]string{
			"type": "must be OVERDUE_SHIPMENT, OVERDUE_VOUCHER, or MANUAL",
		})
	}

	ex := &domain.FulfillmentException{
		FulfillmentID: fulfillmentID,
		Type:          exType,
		Status:        domain.ExceptionOpen,
	}
	if note != "" {
		ex.ResolutionNote = &note
	}

	actorID, ok := UserIDFromContext(ctx)
	if ok {
		ex.OpenedBy = &actorID
	}

	created, err := s.exceptionRepo.Create(ctx, ex)
	if err != nil {
		return nil, err
	}

	if s.auditSvc != nil {
		_ = s.auditSvc.Log(ctx, "fulfillment_exceptions", created.ID, "CREATE", nil, created)
	}
	return created, nil
}

// CreateSystem records an exception originated by an automated job. It stamps
// opened_by with the seeded system user and attaches a SYSTEM audit entry so
// every scheduler-created row satisfies the compliance audit-attribution rule.
func (s *exceptionService) CreateSystem(ctx context.Context, fulfillmentID uuid.UUID, exType domain.ExceptionType, note string) (*domain.FulfillmentException, error) {
	if !exType.IsValid() {
		return nil, domain.NewValidationError("invalid exception type", map[string]string{
			"type": "must be OVERDUE_SHIPMENT, OVERDUE_VOUCHER, or MANUAL",
		})
	}

	systemID := domain.SystemActorID
	ex := &domain.FulfillmentException{
		FulfillmentID: fulfillmentID,
		Type:          exType,
		Status:        domain.ExceptionOpen,
		OpenedBy:      &systemID,
	}
	if note != "" {
		ex.ResolutionNote = &note
	}

	created, err := s.exceptionRepo.Create(ctx, ex)
	if err != nil {
		return nil, err
	}

	if s.auditSvc != nil {
		// Ensure the audit row is attributed to the system actor even when no
		// userID is present on the scheduler context.
		auditCtx := WithUserID(ctx, systemID)
		_ = s.auditSvc.Log(auditCtx, "fulfillment_exceptions", created.ID, "SYSTEM_CREATE", nil, created)
	}
	return created, nil
}

func (s *exceptionService) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.ExceptionStatus, resolutionNote string) (*domain.FulfillmentException, error) {
	if !status.IsValid() {
		return nil, domain.NewValidationError("invalid exception status", map[string]string{
			"status": "must be OPEN, INVESTIGATING, ESCALATED, or RESOLVED",
		})
	}
	if status == domain.ExceptionResolved && resolutionNote == "" {
		return nil, domain.NewValidationError("resolution note required", map[string]string{
			"resolution_note": "required when resolving an exception",
		})
	}

	ex, err := s.exceptionRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	var note *string
	if resolutionNote != "" {
		note = &resolutionNote
	}
	var resolvedBy *uuid.UUID
	if status == domain.ExceptionResolved {
		actorID, ok := UserIDFromContext(ctx)
		if ok {
			resolvedBy = &actorID
		}
	}

	if err := s.exceptionRepo.UpdateStatus(ctx, id, status, note, resolvedBy); err != nil {
		return nil, err
	}

	// Re-fetch to return updated state.
	updated, err := s.exceptionRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if s.auditSvc != nil {
		_ = s.auditSvc.Log(ctx, "fulfillment_exceptions", updated.ID, "UPDATE", ex, updated)
	}
	return updated, nil
}

func (s *exceptionService) AddEvent(ctx context.Context, exceptionID uuid.UUID, eventType, content string) (*domain.ExceptionEvent, error) {
	if eventType == "" {
		return nil, domain.NewValidationError("missing field", map[string]string{
			"event_type": "required",
		})
	}
	if content == "" {
		return nil, domain.NewValidationError("missing field", map[string]string{
			"content": "required",
		})
	}

	// Verify exception exists.
	if _, err := s.exceptionRepo.GetByID(ctx, exceptionID); err != nil {
		return nil, err
	}

	ev := &domain.ExceptionEvent{
		ExceptionID: exceptionID,
		EventType:   eventType,
		Content:     content,
	}
	actorID, ok := UserIDFromContext(ctx)
	if ok {
		ev.CreatedBy = &actorID
	}

	if err := s.exceptionEventRepo.Create(ctx, ev); err != nil {
		return nil, err
	}
	return ev, nil
}

func (s *exceptionService) GetByID(ctx context.Context, id uuid.UUID) (*domain.FulfillmentException, error) {
	ex, err := s.exceptionRepo.GetByID(ctx, id)
	if err != nil && errors.Is(err, domain.ErrNotFound) {
		return nil, domain.NewNotFoundError("exception")
	}
	return ex, err
}

func (s *exceptionService) List(ctx context.Context, filters repository.ExceptionFilters) ([]domain.FulfillmentException, error) {
	return s.exceptionRepo.List(ctx, filters)
}
