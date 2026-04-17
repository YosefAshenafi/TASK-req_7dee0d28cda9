package job

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
	"github.com/fulfillops/fulfillops/internal/service"
)

type fakeJobRunRepo struct {
	created  []*domain.JobRunHistory
	finished []struct {
		id      uuid.UUID
		status  domain.JobStatus
		records int
		errMsg  *string
	}
}

func (f *fakeJobRunRepo) Create(_ context.Context, run *domain.JobRunHistory) (*domain.JobRunHistory, error) {
	run.ID = uuid.New()
	f.created = append(f.created, run)
	return run, nil
}

func (f *fakeJobRunRepo) Finish(_ context.Context, id uuid.UUID, status domain.JobStatus, records int, errMsg *string) error {
	f.finished = append(f.finished, struct {
		id      uuid.UUID
		status  domain.JobStatus
		records int
		errMsg  *string
	}{id: id, status: status, records: records, errMsg: errMsg})
	return nil
}

func (f *fakeJobRunRepo) List(_ context.Context, _ repository.JobRunFilters, _ domain.PageRequest) ([]domain.JobRunHistory, int, error) {
	items := make([]domain.JobRunHistory, 0, len(f.created))
	for _, run := range f.created {
		items = append(items, *run)
	}
	return items, len(items), nil
}

type fakeBackupService struct {
	runs int
	err  error
}

func (f *fakeBackupService) RunBackup(_ context.Context) (*service.BackupEntry, error) {
	f.runs++
	return nil, f.err
}

func (f *fakeBackupService) ListBackups(context.Context) ([]service.BackupEntry, error) {
	return nil, nil
}

func (f *fakeBackupService) RestoreFromBackup(context.Context, string, bool) error {
	return nil
}

type fakeReportRepo struct {
	created []*domain.ReportExport
	expired []domain.ReportExport
	delErr  error
}

func (f *fakeReportRepo) Create(_ context.Context, e *domain.ReportExport) (*domain.ReportExport, error) {
	e.ID = uuid.New()
	f.created = append(f.created, e)
	return e, nil
}

func (f *fakeReportRepo) GetByID(context.Context, uuid.UUID) (*domain.ReportExport, error) {
	return nil, domain.ErrNotFound
}

func (f *fakeReportRepo) List(context.Context, repository.ReportExportFilters, domain.PageRequest) ([]domain.ReportExport, int, error) {
	return nil, 0, nil
}

func (f *fakeReportRepo) UpdateStatus(context.Context, uuid.UUID, domain.ExportStatus, *string, *int64, *string, *time.Time) error {
	return nil
}

func (f *fakeReportRepo) GetExpired(context.Context, time.Time) ([]domain.ReportExport, error) {
	return f.expired, nil
}

func (f *fakeReportRepo) Delete(_ context.Context, _ uuid.UUID) error {
	return f.delErr
}

type fakeExportService struct {
	ids []uuid.UUID
	err error
}

func (f *fakeExportService) GenerateExport(_ context.Context, id uuid.UUID) error {
	f.ids = append(f.ids, id)
	return f.err
}

func (f *fakeExportService) VerifyChecksum(context.Context, uuid.UUID) (bool, error) {
	return true, nil
}

func (f *fakeExportService) Delete(_ context.Context, id uuid.UUID, _ uuid.UUID) error {
	return f.err
}

type fakeMessagingService struct {
	count int
	err   error
}

func (f *fakeMessagingService) Dispatch(context.Context, uuid.UUID, uuid.UUID, []domain.SendLogChannel, map[string]any) (*domain.SendLog, error) {
	return nil, nil
}

func (f *fakeMessagingService) MarkPrinted(context.Context, uuid.UUID) error {
	return nil
}

func (f *fakeMessagingService) MarkFailed(context.Context, uuid.UUID, string) error {
	return nil
}

func (f *fakeMessagingService) RetryPending(context.Context, int) (int, error) {
	return f.count, f.err
}

type fakeSLAService struct {
	deadline time.Time
	overdue  bool
}

func (f *fakeSLAService) CalculateDeadline(context.Context, domain.FulfillmentType, time.Time) (time.Time, error) {
	return f.deadline, nil
}

func (f *fakeSLAService) IsOverdue(time.Time) bool {
	return f.overdue
}

type fakeFulfillmentRepo struct {
	overdue []domain.Fulfillment
	lists   map[domain.FulfillmentStatus]int
}

func (f *fakeFulfillmentRepo) List(_ context.Context, filters repository.FulfillmentFilters, _ domain.PageRequest) ([]domain.Fulfillment, int, error) {
	return nil, f.lists[filters.Status], nil
}

func (f *fakeFulfillmentRepo) GetByID(context.Context, uuid.UUID) (*domain.Fulfillment, error) {
	return nil, domain.ErrNotFound
}

func (f *fakeFulfillmentRepo) GetByIDForUpdate(context.Context, pgx.Tx, uuid.UUID) (*domain.Fulfillment, error) {
	return nil, domain.ErrNotFound
}

func (f *fakeFulfillmentRepo) Create(context.Context, pgx.Tx, *domain.Fulfillment) (*domain.Fulfillment, error) {
	return nil, nil
}

func (f *fakeFulfillmentRepo) Update(context.Context, pgx.Tx, *domain.Fulfillment) (*domain.Fulfillment, error) {
	return nil, nil
}

func (f *fakeFulfillmentRepo) CountByCustomerAndTier(context.Context, pgx.Tx, uuid.UUID, uuid.UUID, time.Time) (int, error) {
	return 0, nil
}

func (f *fakeFulfillmentRepo) SoftDelete(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

func (f *fakeFulfillmentRepo) Restore(context.Context, uuid.UUID) error {
	return nil
}

func (f *fakeFulfillmentRepo) BumpVersion(context.Context, pgx.Tx, uuid.UUID, int) error {
	return nil
}

func (f *fakeFulfillmentRepo) ListOverdue(context.Context) ([]domain.Fulfillment, error) {
	return f.overdue, nil
}

type fakeExceptionRepo struct {
	exists map[uuid.UUID]bool
	created []*domain.FulfillmentException
}

func (f *fakeExceptionRepo) List(context.Context, repository.ExceptionFilters) ([]domain.FulfillmentException, error) {
	return nil, nil
}

func (f *fakeExceptionRepo) GetByID(context.Context, uuid.UUID) (*domain.FulfillmentException, error) {
	return nil, domain.ErrNotFound
}

func (f *fakeExceptionRepo) Create(_ context.Context, e *domain.FulfillmentException) (*domain.FulfillmentException, error) {
	f.created = append(f.created, e)
	return e, nil
}

func (f *fakeExceptionRepo) UpdateStatus(context.Context, uuid.UUID, domain.ExceptionStatus, *string, *uuid.UUID) error {
	return nil
}

func (f *fakeExceptionRepo) ExistsOpenForFulfillment(_ context.Context, fulfillmentID uuid.UUID, _ domain.ExceptionType) (bool, error) {
	return f.exists[fulfillmentID], nil
}

// fakeExceptionSvc delegates CreateSystem to a repo.Create call while
// stamping opened_by = SystemActorID, matching the real service behavior.
type fakeExceptionSvc struct {
	repo repository.ExceptionRepository
}

func (s *fakeExceptionSvc) Create(ctx context.Context, fulfillmentID uuid.UUID, exType domain.ExceptionType, note string) (*domain.FulfillmentException, error) {
	return nil, errors.New("not implemented in test")
}

func (s *fakeExceptionSvc) CreateSystem(ctx context.Context, fulfillmentID uuid.UUID, exType domain.ExceptionType, note string) (*domain.FulfillmentException, error) {
	sys := domain.SystemActorID
	ex := &domain.FulfillmentException{
		FulfillmentID: fulfillmentID,
		Type:          exType,
		Status:        domain.ExceptionOpen,
		OpenedBy:      &sys,
	}
	return s.repo.Create(ctx, ex)
}

func (s *fakeExceptionSvc) UpdateStatus(context.Context, uuid.UUID, domain.ExceptionStatus, string) (*domain.FulfillmentException, error) {
	return nil, errors.New("not implemented in test")
}

func (s *fakeExceptionSvc) AddEvent(context.Context, uuid.UUID, string, string) (*domain.ExceptionEvent, error) {
	return nil, errors.New("not implemented in test")
}

func (s *fakeExceptionSvc) GetByID(context.Context, uuid.UUID) (*domain.FulfillmentException, error) {
	return nil, domain.ErrNotFound
}

func (s *fakeExceptionSvc) List(context.Context, repository.ExceptionFilters) ([]domain.FulfillmentException, error) {
	return nil, nil
}

type fakeTierRepo struct {
	tiers []domain.RewardTier
}

func (f *fakeTierRepo) List(context.Context, string, bool) ([]domain.RewardTier, error) {
	return f.tiers, nil
}

func (f *fakeTierRepo) GetByID(context.Context, uuid.UUID) (*domain.RewardTier, error) {
	return nil, domain.ErrNotFound
}

func (f *fakeTierRepo) GetByIDForUpdate(context.Context, pgx.Tx, uuid.UUID) (*domain.RewardTier, error) {
	return nil, domain.ErrNotFound
}

func (f *fakeTierRepo) Create(context.Context, *domain.RewardTier) (*domain.RewardTier, error) {
	return nil, nil
}

func (f *fakeTierRepo) Update(context.Context, *domain.RewardTier) (*domain.RewardTier, error) {
	return nil, nil
}

func (f *fakeTierRepo) SoftDelete(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

func (f *fakeTierRepo) Restore(context.Context, uuid.UUID) error {
	return nil
}

func (f *fakeTierRepo) DecrementInventory(context.Context, pgx.Tx, uuid.UUID, int) error {
	return nil
}

func (f *fakeTierRepo) IncrementInventory(context.Context, pgx.Tx, uuid.UUID, int) error {
	return nil
}

func TestSchedulerRunOnceAndStatus(t *testing.T) {
	repo := &fakeJobRunRepo{}
	scheduler := NewScheduler(repo)

	done := make(chan struct{}, 1)
	scheduler.Register("demo", time.Hour, func(context.Context) (int, error) {
		done <- struct{}{}
		return 7, nil
	})

	if err := scheduler.RunOnce(context.Background(), "demo"); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("scheduled job did not run")
	}

	// Wait for the runOnce goroutine to record Finish(), which happens after fn returns.
	deadline := time.Now().Add(2 * time.Second)
	for len(repo.finished) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	runs, err := scheduler.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if len(repo.finished) != 1 || repo.finished[0].status != domain.JobCompleted || repo.finished[0].records != 7 {
		t.Fatalf("unexpected finish record: %#v", repo.finished)
	}

	if err := scheduler.RunOnce(context.Background(), "missing"); err == nil {
		t.Fatal("expected not found error for missing job")
	}
}

func TestSchedulerRunOnceFailureRecordsFailedStatus(t *testing.T) {
	repo := &fakeJobRunRepo{}
	scheduler := NewScheduler(repo)
	scheduler.Register("broken", time.Hour, func(context.Context) (int, error) {
		return 0, errors.New("boom")
	})

	if err := scheduler.RunOnce(context.Background(), "broken"); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	if len(repo.finished) != 1 || repo.finished[0].status != domain.JobFailed {
		t.Fatalf("unexpected finish record: %#v", repo.finished)
	}
}

func TestBackupAndNotifyJobs(t *testing.T) {
	backupSvc := &fakeBackupService{}
	backupJob := NewBackupJob(backupSvc)
	if count, err := backupJob.Run(context.Background()); err != nil || count != 1 || backupSvc.runs != 1 {
		t.Fatalf("backup job = (%d, %v), runs=%d", count, err, backupSvc.runs)
	}

	notifySvc := &fakeMessagingService{count: 3}
	notifyJob := NewNotifyJob(notifySvc, 5)
	if count, err := notifyJob.Run(context.Background()); err != nil || count != 3 {
		t.Fatalf("notify job = (%d, %v)", count, err)
	}
}

func TestScheduledReportAndExportCleanupJobs(t *testing.T) {
	reportRepo := &fakeReportRepo{}
	exportSvc := &fakeExportService{}
	reportJob := NewScheduledReportJob(reportRepo, exportSvc)

	count, err := reportJob.Run(context.Background())
	if err != nil {
		t.Fatalf("ScheduledReportJob.Run() error = %v", err)
	}
	if count != 2 || len(reportRepo.created) != 2 || len(exportSvc.ids) != 2 {
		t.Fatalf("unexpected scheduled report results: count=%d created=%d exports=%d", count, len(reportRepo.created), len(exportSvc.ids))
	}

	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "expired.csv")
	if err := os.WriteFile(filePath, []byte("old"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	reportRepo.expired = []domain.ReportExport{{ID: uuid.New(), FilePath: &filePath}}

	cleanupJob := NewExportCleanupJob(reportRepo, exportSvc)
	removed, err := cleanupJob.Run(context.Background())
	if err != nil {
		t.Fatalf("ExportCleanupJob.Run() error = %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
}

func TestOverdueAndStatsJobs(t *testing.T) {
	now := time.Now().UTC().Add(-6 * time.Hour)
	physID := uuid.New()
	voucherID := uuid.New()
	fulfillRepo := &fakeFulfillmentRepo{
		overdue: []domain.Fulfillment{
			{ID: physID, Type: domain.TypePhysical, ReadyAt: &now},
			{ID: voucherID, Type: domain.TypeVoucher, ReadyAt: &now},
		},
		lists: map[domain.FulfillmentStatus]int{
			domain.StatusDraft:        2,
			domain.StatusCompleted:    1,
			domain.StatusVoucherIssued: 1,
		},
	}
	exRepo := &fakeExceptionRepo{exists: map[uuid.UUID]bool{voucherID: true}}
	slaSvc := &fakeSLAService{deadline: now.Add(-time.Hour), overdue: true}

	overdueJob := NewOverdueJob(fulfillRepo, exRepo, &fakeExceptionSvc{repo: exRepo}, slaSvc)
	created, err := overdueJob.Run(context.Background())
	if err != nil {
		t.Fatalf("OverdueJob.Run() error = %v", err)
	}
	if created != 1 || len(exRepo.created) != 1 || exRepo.created[0].Type != domain.ExceptionOverdueShipment {
		t.Fatalf("unexpected overdue results: created=%d exceptions=%#v", created, exRepo.created)
	}
	if exRepo.created[0].OpenedBy == nil || *exRepo.created[0].OpenedBy != domain.SystemActorID {
		t.Fatalf("expected OpenedBy=SystemActorID, got %v", exRepo.created[0].OpenedBy)
	}

	tierRepo := &fakeTierRepo{tiers: []domain.RewardTier{
		{Name: "safe", InventoryCount: 5, AlertThreshold: 1},
		{Name: "low", InventoryCount: 1, AlertThreshold: 1},
	}}
	statsJob := NewStatsJob(fulfillRepo, tierRepo)
	total, err := statsJob.Run(context.Background())
	if err != nil {
		t.Fatalf("StatsJob.Run() error = %v", err)
	}
	if total != 4 {
		t.Fatalf("total = %d, want 4", total)
	}
}
