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
	// Dispatch sends a message via the template's configured channel.
	// userID is the system user for IN_APP notifications (FK to users.id).
	// recipientID is the logical recipient for SMS/EMAIL send_log tracking (no FK).
	Dispatch(ctx context.Context, templateID uuid.UUID, userID uuid.UUID, recipientID uuid.UUID, contextData map[string]any) (*domain.SendLog, error)
	// MarkPrinted marks an SMS/EMAIL send_log as PRINTED (handoff queue).
	MarkPrinted(ctx context.Context, id uuid.UUID) error
	// RetryPending re-queues failed send_logs (up to maxAttempts).
	RetryPending(ctx context.Context, maxAttempts int) (int, error)
}

type messagingService struct {
	templateRepo     repository.MessageTemplateRepository
	sendLogRepo      repository.SendLogRepository
	notificationRepo repository.NotificationRepository
}

// NewMessagingService creates a MessagingService.
func NewMessagingService(
	templateRepo repository.MessageTemplateRepository,
	sendLogRepo repository.SendLogRepository,
	notificationRepo repository.NotificationRepository,
) MessagingService {
	return &messagingService{
		templateRepo:     templateRepo,
		sendLogRepo:      sendLogRepo,
		notificationRepo: notificationRepo,
	}
}

func (s *messagingService) Dispatch(ctx context.Context, templateID uuid.UUID, userID uuid.UUID, recipientID uuid.UUID, contextData map[string]any) (*domain.SendLog, error) {
	tmpl, err := s.templateRepo.GetByID(ctx, templateID)
	if err != nil {
		return nil, err
	}

	ctxJSON, err := json.Marshal(contextData)
	if err != nil {
		ctxJSON = []byte(`{}`)
	}

	log := &domain.SendLog{
		TemplateID:  &templateID,
		RecipientID: recipientID,
		Channel:     tmpl.Channel,
		Status:      domain.SendQueued,
		Context:     ctxJSON,
	}

	switch tmpl.Channel {
	case domain.ChannelInApp:
		// Create in-app notification using userID (FK to users.id).
		body := tmpl.BodyTemplate
		notification := &domain.Notification{
			UserID:  userID,
			Title:   tmpl.Name,
			Body:    &body,
			Context: ctxJSON,
		}
		if _, err := s.notificationRepo.Create(ctx, notification); err != nil {
			return nil, err
		}
		log.Status = domain.SendSent

	case domain.ChannelSMS, domain.ChannelEmail:
		// Queued for handoff / external delivery.
		retryAt := time.Now().UTC().Add(10 * time.Minute)
		log.NextRetryAt = &retryAt
	}

	return s.sendLogRepo.Create(ctx, log)
}

func (s *messagingService) MarkPrinted(ctx context.Context, id uuid.UUID) error {
	actorID, _ := UserIDFromContext(ctx)
	return s.sendLogRepo.MarkPrinted(ctx, id, actorID)
}

// RetryPending processes send_log rows in QUEUED or FAILED state whose
// next_retry_at has elapsed. Rows that have reached maxAttempts are
// permanently marked FAILED (terminal). All others are re-queued with a
// new retry window, implementing the "retry up to N times over 30 minutes"
// requirement.
func (s *messagingService) RetryPending(ctx context.Context, maxAttempts int) (int, error) {
	now := time.Now().UTC()
	logs, err := s.sendLogRepo.GetRetryable(ctx, now)
	if err != nil {
		return 0, err
	}

	retried := 0
	for _, l := range logs {
		// AttemptCount is incremented by UpdateStatus, so compare with maxAttempts-1
		// to allow the final attempt before marking terminal FAILED.
		if l.AttemptCount >= maxAttempts {
			// Terminal failure — permanently mark FAILED and do not count as retried.
			errMsg := "max retry attempts reached"
			if err := s.sendLogRepo.UpdateStatus(ctx, l.ID, domain.SendFailed, &errMsg); err != nil {
				return retried, err
			}
			continue
		}

		switch l.Channel {
		case domain.ChannelInApp:
			if l.TemplateID != nil {
				tmpl, err := s.templateRepo.GetByID(ctx, *l.TemplateID)
				if err == nil {
					retryBody := tmpl.BodyTemplate
					notification := &domain.Notification{
						UserID:  l.RecipientID,
						Title:   tmpl.Name,
						Body:    &retryBody,
						Context: l.Context,
					}
					if _, err := s.notificationRepo.Create(ctx, notification); err == nil {
						_ = s.sendLogRepo.UpdateStatus(ctx, l.ID, domain.SendSent, nil)
						retried++
						continue
					}
				}
			}
			// Delivery failed — re-queue with 10-min back-off.
			next := now.Add(10 * time.Minute)
			_ = s.sendLogRepo.UpdateStatus(ctx, l.ID, domain.SendQueued, nil)
			_ = s.sendLogRepo.UpdateNextRetry(ctx, l.ID, next)
			retried++

		case domain.ChannelSMS, domain.ChannelEmail:
			// Re-queue for operator handoff with 10-min back-off.
			next := now.Add(10 * time.Minute)
			_ = s.sendLogRepo.UpdateStatus(ctx, l.ID, domain.SendQueued, nil)
			_ = s.sendLogRepo.UpdateNextRetry(ctx, l.ID, next)
			retried++
		}
	}
	return retried, nil
}
