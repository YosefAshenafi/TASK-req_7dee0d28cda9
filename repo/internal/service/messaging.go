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
	// IN_APP: creates a Notification + SENT send_log.
	// SMS/EMAIL: creates a QUEUED send_log (delivered by external system or print queue).
	Dispatch(ctx context.Context, templateID uuid.UUID, recipientID uuid.UUID, contextData map[string]any) (*domain.SendLog, error)
	// MarkPrinted marks an SMS/EMAIL send_log as PRINTED (handoff queue).
	MarkPrinted(ctx context.Context, id uuid.UUID) error
	// RetryPending re-queues QUEUED in-app send_logs (up to maxAttempts).
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

func (s *messagingService) Dispatch(ctx context.Context, templateID uuid.UUID, recipientID uuid.UUID, contextData map[string]any) (*domain.SendLog, error) {
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
		// Create in-app notification immediately.
		body := tmpl.BodyTemplate
		notification := &domain.Notification{
			UserID:  recipientID,
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
		retryAt := time.Now().UTC().Add(5 * time.Minute)
		log.NextRetryAt = &retryAt
	}

	return s.sendLogRepo.Create(ctx, log)
}

func (s *messagingService) MarkPrinted(ctx context.Context, id uuid.UUID) error {
	actorID, _ := UserIDFromContext(ctx)
	return s.sendLogRepo.MarkPrinted(ctx, id, actorID)
}

func (s *messagingService) RetryPending(ctx context.Context, maxAttempts int) (int, error) {
	now := time.Now().UTC()
	logs, err := s.sendLogRepo.GetRetryable(ctx, now)
	if err != nil {
		return 0, err
	}

	retried := 0
	for _, l := range logs {
		if l.AttemptCount >= maxAttempts {
			if err := s.sendLogRepo.UpdateStatus(ctx, l.ID, domain.SendFailed, nil); err != nil {
				return retried, err
			}
			continue
		}

		// For IN_APP, attempt delivery.
		if l.Channel == domain.ChannelInApp && l.TemplateID != nil {
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

		// Schedule next retry with backoff.
		next := now.Add(time.Duration(l.AttemptCount+1) * 5 * time.Minute)
		_ = s.sendLogRepo.UpdateStatus(ctx, l.ID, domain.SendQueued, nil)
		_ = s.sendLogRepo.UpdateNextRetry(ctx, l.ID, next)
		retried++
	}
	return retried, nil
}
