package job

import (
	"context"
	"fmt"
	"time"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
	"github.com/fulfillops/fulfillops/internal/service"
)

// ScheduledReportJob generates a daily fulfillments snapshot and a daily
// audit-log export so compliance/ops teams have an archive without needing
// to click "Generate Export" in the UI.
type ScheduledReportJob struct {
	reportRepo repository.ReportExportRepository
	exportSvc  service.ExportService
}

func NewScheduledReportJob(reportRepo repository.ReportExportRepository, exportSvc service.ExportService) *ScheduledReportJob {
	return &ScheduledReportJob{reportRepo: reportRepo, exportSvc: exportSvc}
}

func (j *ScheduledReportJob) Run(ctx context.Context) (int, error) {
	if j.reportRepo == nil || j.exportSvc == nil {
		return 0, nil
	}

	// Yesterday's window.
	end := time.Now().UTC().Truncate(24 * time.Hour)
	start := end.Add(-24 * time.Hour)
	filtersJSON := []byte(fmt.Sprintf(`{"date_from":%q,"date_to":%q}`,
		start.Format(time.RFC3339), end.Format(time.RFC3339)))

	reports := []string{"fulfillments", "audit"}
	generated := 0
	for _, t := range reports {
		r := &domain.ReportExport{
			ReportType: t,
			Filters:    filtersJSON,
			Status:     domain.ExportQueued,
		}
		created, err := j.reportRepo.Create(ctx, r)
		if err != nil {
			return generated, fmt.Errorf("creating scheduled %s export: %w", t, err)
		}
		if err := j.exportSvc.GenerateExport(ctx, created.ID); err != nil {
			return generated, fmt.Errorf("generating scheduled %s export: %w", t, err)
		}
		generated++
	}
	return generated, nil
}
