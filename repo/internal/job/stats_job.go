package job

import (
	"context"
	"log"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
)

// StatsJob refreshes nightly dashboard aggregations. Currently it logs a
// summary of fulfillment counts by status; extend to populate a materialized
// view or cache as needed.
type StatsJob struct {
	fulfillRepo repository.FulfillmentRepository
	tierRepo    repository.TierRepository
}

func NewStatsJob(fulfillRepo repository.FulfillmentRepository, tierRepo repository.TierRepository) *StatsJob {
	return &StatsJob{fulfillRepo: fulfillRepo, tierRepo: tierRepo}
}

func (j *StatsJob) Run(ctx context.Context) (int, error) {
	statuses := []domain.FulfillmentStatus{
		domain.StatusDraft,
		domain.StatusReadyToShip,
		domain.StatusShipped,
		domain.StatusDelivered,
		domain.StatusVoucherIssued,
		domain.StatusCompleted,
		domain.StatusOnHold,
		domain.StatusCanceled,
	}

	total := 0
	for _, s := range statuses {
		_, count, err := j.fulfillRepo.List(ctx, repository.FulfillmentFilters{Status: s}, domain.PageRequest{Page: 1, PageSize: 1})
		if err != nil {
			continue
		}
		if count > 0 {
			log.Printf("stats_job: status=%s count=%d", s, count)
		}
		total += count
	}

	tiers, _ := j.tierRepo.List(ctx, "", false)
	alertCount := 0
	for _, t := range tiers {
		if t.InventoryCount <= t.AlertThreshold {
			alertCount++
		}
	}
	if alertCount > 0 {
		log.Printf("stats_job: tiers_below_threshold=%d", alertCount)
	}

	return total, nil
}
