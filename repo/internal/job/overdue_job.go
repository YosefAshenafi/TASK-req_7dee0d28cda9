package job

import (
	"context"
	"fmt"
	"log"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
	"github.com/fulfillops/fulfillops/internal/service"
)

// OverdueJob scans fulfillments that should have been actioned by now and
// opens OVERDUE exceptions if none already exist for that fulfillment+type.
type OverdueJob struct {
	fulfillRepo  repository.FulfillmentRepository
	exceptionRepo repository.ExceptionRepository
	slaSvc       service.SLAService
}

func NewOverdueJob(
	fulfillRepo repository.FulfillmentRepository,
	exceptionRepo repository.ExceptionRepository,
	slaSvc service.SLAService,
) *OverdueJob {
	return &OverdueJob{
		fulfillRepo:   fulfillRepo,
		exceptionRepo: exceptionRepo,
		slaSvc:        slaSvc,
	}
}

func (j *OverdueJob) Run(ctx context.Context) (int, error) {
	candidates, err := j.fulfillRepo.ListOverdue(ctx)
	if err != nil {
		return 0, fmt.Errorf("listing overdue candidates: %w", err)
	}

	created := 0
	for _, f := range candidates {
		if f.ReadyAt == nil {
			continue
		}

		deadline, err := j.slaSvc.CalculateDeadline(ctx, f.Type, *f.ReadyAt)
		if err != nil {
			log.Printf("overdue_job: SLA calc for %s: %v", f.ID, err)
			continue
		}
		if !j.slaSvc.IsOverdue(deadline) {
			continue
		}

		// Determine exception type
		exType := domain.ExceptionOverdueShipment
		if f.Type == domain.TypeVoucher {
			exType = domain.ExceptionOverdueVoucher
		}

		// Deduplicate — skip if an open exception of this type already exists
		exists, err := j.exceptionRepo.ExistsOpenForFulfillment(ctx, f.ID, exType)
		if err != nil {
			log.Printf("overdue_job: checking exception existence for %s: %v", f.ID, err)
			continue
		}
		if exists {
			continue
		}

		ex := &domain.FulfillmentException{
			FulfillmentID: f.ID,
			Type:          exType,
			Status:        domain.ExceptionOpen,
		}
		if _, err := j.exceptionRepo.Create(ctx, ex); err != nil {
			log.Printf("overdue_job: creating exception for %s: %v", f.ID, err)
			continue
		}
		created++
	}
	return created, nil
}
