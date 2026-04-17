package job

import (
	"context"

	"github.com/fulfillops/fulfillops/internal/service"
)

// BackupJob runs pg_dump on a schedule so compliance retention is enforced
// regardless of whether an operator clicks "Run Backup Now" in the UI.
type BackupJob struct {
	backupSvc service.BackupService
}

func NewBackupJob(backupSvc service.BackupService) *BackupJob {
	return &BackupJob{backupSvc: backupSvc}
}

func (j *BackupJob) Run(ctx context.Context) (int, error) {
	if j.backupSvc == nil {
		return 0, nil
	}
	if _, err := j.backupSvc.RunBackup(ctx); err != nil {
		return 0, err
	}
	return 1, nil
}
