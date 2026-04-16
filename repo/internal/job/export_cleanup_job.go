package job

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/fulfillops/fulfillops/internal/repository"
)

// ExportCleanupJob removes expired report export files from disk and deletes
// their database records.
type ExportCleanupJob struct {
	reportRepo repository.ReportExportRepository
}

func NewExportCleanupJob(reportRepo repository.ReportExportRepository) *ExportCleanupJob {
	return &ExportCleanupJob{reportRepo: reportRepo}
}

func (j *ExportCleanupJob) Run(ctx context.Context) (int, error) {
	expired, err := j.reportRepo.GetExpired(ctx, time.Now().UTC())
	if err != nil {
		return 0, fmt.Errorf("fetching expired exports: %w", err)
	}

	removed := 0
	for _, e := range expired {
		// Remove file from disk if it exists
		if e.FilePath != nil && *e.FilePath != "" {
			if removeErr := os.Remove(*e.FilePath); removeErr != nil && !os.IsNotExist(removeErr) {
				log.Printf("export_cleanup: removing file %s: %v", *e.FilePath, removeErr)
			}
		}

		// Delete database record
		if err := j.reportRepo.Delete(ctx, e.ID); err != nil {
			log.Printf("export_cleanup: deleting export record %s: %v", e.ID, err)
			continue
		}
		removed++
	}
	return removed, nil
}
