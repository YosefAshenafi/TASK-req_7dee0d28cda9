package service

// Integration-style coverage for the operator "mark failed" transition and the
// subsequent scheduler retry cycle. This is the production flow missing from
// the original spec: QUEUED handoff → MarkFailed → retry picks up.

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
)

// markFailedRepo is a send-log repository stub that captures every state
// transition so the test can observe the QUEUED→FAILED→QUEUED cycle.
type markFailedRepo struct {
	row       *domain.SendLog
	updates   []domain.SendLogStatus
	nextRetry map[uuid.UUID]time.Time
}

func newMarkFailedRepo(row *domain.SendLog) *markFailedRepo {
	return &markFailedRepo{row: row, nextRetry: map[uuid.UUID]time.Time{}}
}

func (s *markFailedRepo) Create(context.Context, *domain.SendLog) (*domain.SendLog, error) {
	return s.row, nil
}

func (s *markFailedRepo) GetByID(context.Context, uuid.UUID) (*domain.SendLog, error) {
	cp := *s.row
	return &cp, nil
}

func (s *markFailedRepo) UpdateStatus(_ context.Context, _ uuid.UUID, status domain.SendLogStatus, errMsg *string) error {
	s.updates = append(s.updates, status)
	s.row.Status = status
	if status == domain.SendFailed {
		s.row.AttemptCount++
		if s.row.FirstFailedAt == nil {
			now := time.Now().UTC()
			s.row.FirstFailedAt = &now
		}
		if errMsg != nil {
			s.row.ErrorMessage = errMsg
		}
	}
	return nil
}

func (s *markFailedRepo) UpdateNextRetry(_ context.Context, id uuid.UUID, at time.Time) error {
	s.nextRetry[id] = at
	s.row.NextRetryAt = &at
	return nil
}

func (s *markFailedRepo) ClearNextRetry(_ context.Context, id uuid.UUID) error {
	delete(s.nextRetry, id)
	s.row.NextRetryAt = nil
	return nil
}

func (s *markFailedRepo) MarkPrinted(context.Context, uuid.UUID, uuid.UUID) error { return nil }

func (s *markFailedRepo) List(context.Context, repository.SendLogFilters, domain.PageRequest) ([]domain.SendLog, int, error) {
	return nil, 0, nil
}

func (s *markFailedRepo) GetRetryable(_ context.Context, now time.Time) ([]domain.SendLog, error) {
	if s.row.Status == domain.SendFailed && s.row.NextRetryAt != nil && !s.row.NextRetryAt.After(now) {
		cp := *s.row
		return []domain.SendLog{cp}, nil
	}
	return nil, nil
}

func TestMarkFailed_QueuedToFailed_SchedulerRetryCycle(t *testing.T) {
	id := uuid.New()
	row := &domain.SendLog{
		ID:      id,
		Channel: domain.ChannelSMS,
		Status:  domain.SendQueued,
	}
	repo := newMarkFailedRepo(row)
	svc := NewMessagingService(&stubTemplateRepo{}, repo, &stubNotificationRepo{}, nil)

	if err := svc.MarkFailed(context.Background(), id, "sms gateway down"); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	if row.Status != domain.SendFailed {
		t.Fatalf("expected FAILED, got %v", row.Status)
	}
	if row.FirstFailedAt == nil {
		t.Fatal("expected first_failed_at to be stamped")
	}
	if row.ErrorMessage == nil || *row.ErrorMessage != "sms gateway down" {
		t.Fatalf("expected error_message to be persisted, got %v", row.ErrorMessage)
	}
	if _, ok := repo.nextRetry[id]; !ok {
		t.Fatal("expected next_retry_at to be set so scheduler can pick it up")
	}

	// Force the scheduled retry time to be in the past so RetryPending runs.
	earlier := time.Now().UTC().Add(-time.Minute)
	row.NextRetryAt = &earlier
	repo.nextRetry[id] = earlier

	retried, err := svc.RetryPending(context.Background(), 3)
	if err != nil {
		t.Fatalf("RetryPending: %v", err)
	}
	if retried != 1 {
		t.Fatalf("expected 1 retried, got %d", retried)
	}
	// After a successful retry cycle the row should be back to QUEUED so an
	// operator can pick it up again.
	if row.Status != domain.SendQueued {
		t.Fatalf("expected QUEUED after retry cycle, got %v", row.Status)
	}
}

func TestMarkFailed_RejectsSentOrPrinted(t *testing.T) {
	id := uuid.New()
	row := &domain.SendLog{ID: id, Channel: domain.ChannelInApp, Status: domain.SendSent}
	repo := newMarkFailedRepo(row)
	svc := NewMessagingService(&stubTemplateRepo{}, repo, &stubNotificationRepo{}, nil)
	if err := svc.MarkFailed(context.Background(), id, "boom"); err == nil {
		t.Fatal("expected error when marking a SENT row as failed")
	}
}
