package job

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/fulfillops/fulfillops/internal/repository"
	"github.com/fulfillops/fulfillops/internal/service"
)

// ExportCleanupJob removes expired report export files from disk and deletes
// their database records via the ExportService so file removal and audit
// logging are consistent with the manual delete path.
type ExportCleanupJob struct {
	reportRepo repository.ReportExportRepository
	exportSvc  service.ExportService
}

func NewExportCleanupJob(reportRepo repository.ReportExportRepository, exportSvc service.ExportService) *ExportCleanupJob {
	return &ExportCleanupJob{reportRepo: reportRepo, exportSvc: exportSvc}
}

func (j *ExportCleanupJob) Run(ctx context.Context) (int, error) {
	expired, err := j.reportRepo.GetExpired(ctx, time.Now().UTC())
	if err != nil {
		return 0, fmt.Errorf("fetching expired exports: %w", err)
	}

	removed := 0
	for _, e := range expired {
		if j.exportSvc != nil {
			if err := j.exportSvc.Delete(ctx, e.ID, uuid.Nil); err != nil {
				log.Printf("export_cleanup: deleting export %s: %v", e.ID, err)
				continue
			}
		} else {
			if err := j.reportRepo.Delete(ctx, e.ID); err != nil {
				log.Printf("export_cleanup: deleting export record %s: %v", e.ID, err)
				continue
			}
		}
		removed++
	}
	return removed, nil
}
