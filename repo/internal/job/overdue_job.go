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
//
// Exception creation is routed through ExceptionService.CreateSystem so every
// auto-opened row has opened_by = system actor and an audit_logs entry — the
// compliance requirement that nothing lands in fulfillment_exceptions without
// a known actor.
type OverdueJob struct {
	fulfillRepo   repository.FulfillmentRepository
	exceptionRepo repository.ExceptionRepository
	exceptionSvc  service.ExceptionService
	slaSvc        service.SLAService
}

func NewOverdueJob(
	fulfillRepo repository.FulfillmentRepository,
	exceptionRepo repository.ExceptionRepository,
	exceptionSvc service.ExceptionService,
	slaSvc service.SLAService,
) *OverdueJob {
	return &OverdueJob{
		fulfillRepo:   fulfillRepo,
		exceptionRepo: exceptionRepo,
		exceptionSvc:  exceptionSvc,
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

		if _, err := j.exceptionSvc.CreateSystem(ctx, f.ID, exType, "auto-opened by overdue-check"); err != nil {
			log.Printf("overdue_job: creating exception for %s: %v", f.ID, err)
			continue
		}
		created++
	}
	return created, nil
}
