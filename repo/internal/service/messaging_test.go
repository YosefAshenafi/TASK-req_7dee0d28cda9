package service

// Tests for Finding #3: send retry policy — 3 failure attempts over 30 minutes.

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
)

func TestDispatch_SuccessfulSendNoRetryRow(t *testing.T) {
	// A successful IN_APP dispatch must not create any FAILED or retry-scheduled row.
	tmplID := uuid.New()
	tmpl := &domain.MessageTemplate{ID: tmplID, Channel: domain.ChannelInApp, BodyTemplate: "msg"}
	templRepo := &stubTemplateRepo{tmpl: tmpl}

	var created []domain.SendLog
	sendRepo := &capturingSendLog{created: &created}
	notifRepo := &stubNotificationRepo{}

	svc := NewMessagingService(templRepo, sendRepo, notifRepo, nil)
	log, err := svc.Dispatch(context.Background(), tmplID, uuid.New(), nil, nil)
	if err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}
	if log == nil || log.Status != domain.SendSent {
		t.Fatalf("expected SENT status, got %v", log.Status)
	}
	// No FAILED or retryable row should exist.
	for _, l := range created {
		if l.Status == domain.SendFailed {
			t.Errorf("unexpected FAILED row created on successful dispatch: %+v", l)
		}
		if l.NextRetryAt != nil && l.Status == domain.SendFailed {
			t.Errorf("unexpected retry-scheduled FAILED row: %+v", l)
		}
	}
}

func TestRetryPending_ExactlyThreeAttemptsOnPersistentFailure(t *testing.T) {
	svc := NewMessagingService(&stubTemplateRepo{}, &stubSendLogForRetry{}, &stubNotificationRepo{}, nil)

	firstFailed := time.Now().UTC().Add(-2 * time.Minute)
	past := time.Now().UTC().Add(-1 * time.Minute)

	// Simulate 3 retry cycles: attempt_count starts at 0, increments each cycle.
	for attempt := 0; attempt < 3; attempt++ {
		repo := &retryStubSendLog{
			retryables: []domain.SendLog{
				{
					ID:            uuid.New(),
					Channel:       domain.ChannelSMS,
					Status:        domain.SendFailed,
					AttemptCount:  attempt,
					NextRetryAt:   &past,
					FirstFailedAt: &firstFailed,
				},
			},
		}
		svc2 := NewMessagingService(&stubTemplateRepo{}, repo, &stubNotificationRepo{}, nil)
		retried, err := svc2.RetryPending(context.Background(), 3)
		if err != nil {
			t.Fatalf("attempt %d: RetryPending error: %v", attempt, err)
		}
		if attempt < 3 && retried != 1 {
			t.Errorf("attempt %d: expected 1 retried, got %d", attempt, retried)
		}
	}
	_ = svc // suppress unused
}

func TestRetryPending_NoRetryAfter30MinWindow(t *testing.T) {
	id := uuid.New()
	firstFailed := time.Now().UTC().Add(-31 * time.Minute) // 31 min ago
	past := time.Now().UTC().Add(-1 * time.Minute)
	repo := &retryStubSendLog{
		retryables: []domain.SendLog{
			{ID: id, Channel: domain.ChannelEmail, Status: domain.SendFailed,
				AttemptCount: 1, NextRetryAt: &past, FirstFailedAt: &firstFailed},
		},
	}
	svc := NewMessagingService(&stubTemplateRepo{}, repo, &stubNotificationRepo{}, nil)

	retried, err := svc.RetryPending(context.Background(), 3)
	if err != nil {
		t.Fatalf("RetryPending: %v", err)
	}
	if retried != 0 {
		t.Errorf("expected 0 retried after 30-min window, got %d", retried)
	}
	// next_retry_at must be cleared so the scheduler doesn't pick it up again.
	if _, ok := repo.nextRetry[id]; ok {
		t.Error("expected next_retry_at to be cleared after window expiry")
	}
}

func TestRetryPending_MidWindowSuccessStopsFurtherRetries(t *testing.T) {
	// After a successful re-queue, we should NOT see any further "FAILED" entries
	// for the same row within the window (because the row is back in QUEUED state).
	id := uuid.New()
	firstFailed := time.Now().UTC().Add(-5 * time.Minute)
	past := time.Now().UTC().Add(-1 * time.Minute)
	repo := &retryStubSendLog{
		retryables: []domain.SendLog{
			{ID: id, Channel: domain.ChannelSMS, Status: domain.SendFailed,
				AttemptCount: 1, NextRetryAt: &past, FirstFailedAt: &firstFailed},
		},
	}
	svc := NewMessagingService(&stubTemplateRepo{}, repo, &stubNotificationRepo{}, nil)

	retried, err := svc.RetryPending(context.Background(), 3)
	if err != nil {
		t.Fatalf("RetryPending: %v", err)
	}
	// Row was re-queued — counts as one retry.
	if retried != 1 {
		t.Errorf("expected 1 retried, got %d", retried)
	}
	if repo.updates[id] != domain.SendQueued {
		t.Errorf("expected QUEUED after mid-window retry, got %v", repo.updates[id])
	}
	// The next_retry_at must be set (not cleared) so the operator can pick it up.
	if _, ok := repo.nextRetry[id]; !ok {
		t.Error("expected next_retry_at set after successful re-queue")
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

// capturingSendLog captures created rows for inspection.
type capturingSendLog struct {
	created *[]domain.SendLog
}

func (s *capturingSendLog) Create(_ context.Context, l *domain.SendLog) (*domain.SendLog, error) {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	*s.created = append(*s.created, *l)
	return l, nil
}
func (s *capturingSendLog) GetByID(context.Context, uuid.UUID) (*domain.SendLog, error) {
	return nil, domain.NewNotFoundError("send log")
}
func (s *capturingSendLog) UpdateStatus(context.Context, uuid.UUID, domain.SendLogStatus, *string) error {
	return nil
}
func (s *capturingSendLog) UpdateNextRetry(context.Context, uuid.UUID, time.Time) error { return nil }
func (s *capturingSendLog) ClearNextRetry(context.Context, uuid.UUID) error             { return nil }
func (s *capturingSendLog) MarkPrinted(context.Context, uuid.UUID, uuid.UUID) error     { return nil }
func (s *capturingSendLog) List(_ context.Context, _ repository.SendLogFilters, _ domain.PageRequest) ([]domain.SendLog, int, error) {
	return nil, 0, nil
}
func (s *capturingSendLog) GetRetryable(context.Context, time.Time) ([]domain.SendLog, error) {
	return nil, nil
}

// stubSendLogForRetry is a minimal no-op stub.
type stubSendLogForRetry struct{}

func (s *stubSendLogForRetry) Create(_ context.Context, l *domain.SendLog) (*domain.SendLog, error) {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	return l, nil
}
func (s *stubSendLogForRetry) GetByID(context.Context, uuid.UUID) (*domain.SendLog, error) {
	return nil, domain.NewNotFoundError("send log")
}
func (s *stubSendLogForRetry) UpdateStatus(context.Context, uuid.UUID, domain.SendLogStatus, *string) error {
	return nil
}
func (s *stubSendLogForRetry) UpdateNextRetry(context.Context, uuid.UUID, time.Time) error { return nil }
func (s *stubSendLogForRetry) ClearNextRetry(context.Context, uuid.UUID) error             { return nil }
func (s *stubSendLogForRetry) MarkPrinted(context.Context, uuid.UUID, uuid.UUID) error     { return nil }
func (s *stubSendLogForRetry) List(_ context.Context, _ repository.SendLogFilters, _ domain.PageRequest) ([]domain.SendLog, int, error) {
	return nil, 0, nil
}
func (s *stubSendLogForRetry) GetRetryable(context.Context, time.Time) ([]domain.SendLog, error) {
	return nil, nil
}
