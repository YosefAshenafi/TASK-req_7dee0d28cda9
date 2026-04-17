package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
)

// MessagingService dispatches messages via various channels.
type MessagingService interface {
	// Dispatch sends a message to a customer recipient.
	// An IN_APP send_log is ALWAYS created (mandatory delivery record).
	// extraChannels specifies additional SMS/EMAIL outputs queued for handoff.
	// If the template's own channel is SMS or EMAIL it is also queued automatically.
	Dispatch(ctx context.Context, templateID uuid.UUID, recipientID uuid.UUID, extraChannels []domain.SendLogChannel, contextData map[string]any) (*domain.SendLog, error)
	// MarkPrinted marks an SMS/EMAIL send_log as PRINTED (handoff queue).
	MarkPrinted(ctx context.Context, id uuid.UUID) error
	// RetryPending re-queues failed send_logs (up to maxAttempts).
	RetryPending(ctx context.Context, maxAttempts int) (int, error)
}

type messagingService struct {
	templateRepo     repository.MessageTemplateRepository
	sendLogRepo      repository.SendLogRepository
	notificationRepo repository.NotificationRepository
	auditSvc         AuditService
}

// NewMessagingService creates a MessagingService.
func NewMessagingService(
	templateRepo repository.MessageTemplateRepository,
	sendLogRepo repository.SendLogRepository,
	notificationRepo repository.NotificationRepository,
	auditSvc AuditService,
) MessagingService {
	return &messagingService{
		templateRepo:     templateRepo,
		sendLogRepo:      sendLogRepo,
		notificationRepo: notificationRepo,
		auditSvc:         auditSvc,
	}
}

// Dispatch always creates a mandatory IN_APP send_log for the customer, then
// queues any additional SMS/EMAIL channels requested or implied by the template.
func (s *messagingService) Dispatch(ctx context.Context, templateID uuid.UUID, recipientID uuid.UUID, extraChannels []domain.SendLogChannel, contextData map[string]any) (*domain.SendLog, error) {
	tmpl, err := s.templateRepo.GetByID(ctx, templateID)
	if err != nil {
		return nil, err
	}

	ctxJSON, err := json.Marshal(contextData)
	if err != nil {
		ctxJSON = []byte(`{}`)
	}

	dispatchID := uuid.New()

	// Mandatory in-app delivery record for the customer recipient.
	inApp := &domain.SendLog{
		TemplateID:    &templateID,
		RecipientID:   recipientID,
		RecipientType: domain.RecipientCustomer,
		DispatchID:    &dispatchID,
		Channel:       domain.ChannelInApp,
		Status:        domain.SendSent,
		Context:       ctxJSON,
	}
	primary, err := s.sendLogRepo.Create(ctx, inApp)
	if err != nil {
		return nil, err
	}

	// Build the set of additional channels to queue.
	additional := deduplicateChannels(tmpl.Channel, extraChannels)
	for _, ch := range additional {
		if ch == domain.ChannelInApp {
			continue // already created above
		}
		retryAt := time.Now().UTC().Add(10 * time.Minute)
		queued := &domain.SendLog{
			TemplateID:    &templateID,
			RecipientID:   recipientID,
			RecipientType: domain.RecipientCustomer,
			DispatchID:    &dispatchID,
			Channel:       ch,
			Status:        domain.SendQueued,
			NextRetryAt:   &retryAt,
			Context:       ctxJSON,
		}
		if _, err := s.sendLogRepo.Create(ctx, queued); err != nil {
			return nil, err
		}
	}

	if s.auditSvc != nil {
		_ = s.auditSvc.Log(ctx, "send_logs", primary.ID, "DISPATCH", nil, map[string]any{
			"template_id":   templateID,
			"recipient_id":  recipientID,
			"dispatch_id":   dispatchID,
			"channels_sent": append([]domain.SendLogChannel{domain.ChannelInApp}, additional...),
		})
	}

	return primary, nil
}

// deduplicateChannels collects the template's channel plus any extra channels
// requested by the caller, deduplicating (IN_APP is handled separately).
func deduplicateChannels(templateChannel domain.SendLogChannel, extra []domain.SendLogChannel) []domain.SendLogChannel {
	seen := map[domain.SendLogChannel]bool{}
	var out []domain.SendLogChannel
	for _, ch := range append([]domain.SendLogChannel{templateChannel}, extra...) {
		if ch == "" || seen[ch] {
			continue
		}
		seen[ch] = true
		out = append(out, ch)
	}
	return out
}

func (s *messagingService) MarkPrinted(ctx context.Context, id uuid.UUID) error {
	actorID, _ := UserIDFromContext(ctx)
	if err := s.sendLogRepo.MarkPrinted(ctx, id, actorID); err != nil {
		return err
	}
	if s.auditSvc != nil {
		_ = s.auditSvc.Log(ctx, "send_logs", id, "PRINTED", nil, map[string]string{"status": "PRINTED"})
	}
	return nil
}

// RetryPending re-processes FAILED send_log rows whose next_retry_at has
// elapsed. Only FAILED rows consume a retry slot; QUEUED rows are handoff
// state and are not touched here.
//
// Policy: up to maxAttempts (3) failure attempts within a 30-minute window
// starting from first_failed_at. Retry spacing: T+10, T+20 relative to the
// first failure (T+0 is the initial failure that set first_failed_at).
// If the third retry also fails (handled by the caller marking FAILED again),
// the next retry cycle will see attempt_count == maxAttempts and clear the
// next_retry_at to stop further processing.
func (s *messagingService) RetryPending(ctx context.Context, maxAttempts int) (int, error) {
	now := time.Now().UTC()
	logs, err := s.sendLogRepo.GetRetryable(ctx, now)
	if err != nil {
		return 0, err
	}

	retried := 0
	for _, l := range logs {
		// Enforce 30-minute wall-clock window from the first failure.
		if l.FirstFailedAt != nil && now.After(l.FirstFailedAt.Add(30*time.Minute)) {
			_ = s.sendLogRepo.ClearNextRetry(ctx, l.ID)
			continue
		}

		// Terminal: too many failure attempts.
		if l.AttemptCount >= maxAttempts {
			_ = s.sendLogRepo.ClearNextRetry(ctx, l.ID)
			continue
		}

		// Re-queue for operator handoff. Schedule next retry relative to
		// first_failed_at so spacing is T+10, T+20 regardless of wall-clock drift.
		var nextRetry time.Time
		if l.FirstFailedAt != nil {
			nextRetry = l.FirstFailedAt.Add(time.Duration(l.AttemptCount+1) * 10 * time.Minute)
		} else {
			nextRetry = now.Add(10 * time.Minute)
		}
		_ = s.sendLogRepo.UpdateStatus(ctx, l.ID, domain.SendQueued, nil)
		_ = s.sendLogRepo.UpdateNextRetry(ctx, l.ID, nextRetry)
		retried++
	}
	return retried, nil
}
