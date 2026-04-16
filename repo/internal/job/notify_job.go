package job

import (
	"context"
	"fmt"

	"github.com/fulfillops/fulfillops/internal/service"
)

// NotifyJob retries QUEUED in-app send_logs up to maxAttempts times.
type NotifyJob struct {
	messagingSvc service.MessagingService
	maxAttempts  int
}

func NewNotifyJob(messagingSvc service.MessagingService, maxAttempts int) *NotifyJob {
	return &NotifyJob{messagingSvc: messagingSvc, maxAttempts: maxAttempts}
}

func (j *NotifyJob) Run(ctx context.Context) (int, error) {
	n, err := j.messagingSvc.RetryPending(ctx, j.maxAttempts)
	if err != nil {
		return 0, fmt.Errorf("retrying pending notifications: %w", err)
	}
	return n, nil
}
