package job

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
)

func TestSchedulerRegisterStartStop(t *testing.T) {
	repo := &fakeJobRunRepo{}
	scheduler := NewScheduler(repo)

	ticks := make(chan struct{}, 4)
	scheduler.Register("fast", 10*time.Millisecond, func(ctx context.Context) (int, error) {
		select {
		case ticks <- struct{}{}:
		default:
		}
		return 1, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	scheduler.Start(ctx)

	// Expect at least one tick within a reasonable window.
	select {
	case <-ticks:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("fast job did not tick after Start")
	}

	scheduler.Stop()

	if len(repo.created) == 0 || len(repo.finished) == 0 {
		t.Fatalf("expected scheduler to record runs, got created=%d finished=%d",
			len(repo.created), len(repo.finished))
	}
}

// ── Job edge-case coverage ───────────────────────────────────────────────────

type erroringJobRunRepo struct{}

func (erroringJobRunRepo) Create(ctx context.Context, _ *domain.JobRunHistory) (*domain.JobRunHistory, error) {
	return nil, errNoCreate
}
func (erroringJobRunRepo) Finish(ctx context.Context, _ uuid.UUID, _ domain.JobStatus, _ int, _ *string) error {
	return nil
}
func (erroringJobRunRepo) List(context.Context, repository.JobRunFilters, domain.PageRequest) ([]domain.JobRunHistory, int, error) {
	return nil, 0, nil
}

type finishErrJobRunRepo struct {
	finishErr error
	created   []*domain.JobRunHistory
}

func (f *finishErrJobRunRepo) Create(_ context.Context, run *domain.JobRunHistory) (*domain.JobRunHistory, error) {
	run.ID = uuid.New()
	f.created = append(f.created, run)
	return run, nil
}
func (f *finishErrJobRunRepo) Finish(context.Context, uuid.UUID, domain.JobStatus, int, *string) error {
	return f.finishErr
}
func (f *finishErrJobRunRepo) List(context.Context, repository.JobRunFilters, domain.PageRequest) ([]domain.JobRunHistory, int, error) {
	return nil, 0, nil
}

var errNoCreate = errors.New("cannot create run history")

func TestSchedulerRunOnce_CreateFailAndFinishFail(t *testing.T) {
	// Create returns error → runOnce still executes the job fn but cannot finish.
	s := NewScheduler(erroringJobRunRepo{})
	done := make(chan struct{}, 1)
	s.Register("errcreate", time.Hour, func(context.Context) (int, error) {
		done <- struct{}{}
		return 0, errors.New("job failed")
	})
	if err := s.RunOnce(context.Background(), "errcreate"); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("job did not run")
	}

	// Create succeeds but Finish errors → scheduler logs and carries on.
	repo := &finishErrJobRunRepo{finishErr: errors.New("finish broke")}
	s2 := NewScheduler(repo)
	s2.Register("errfinish", time.Hour, func(context.Context) (int, error) { return 1, nil })
	if err := s2.RunOnce(context.Background(), "errfinish"); err != nil {
		t.Fatalf("RunOnce(errfinish): %v", err)
	}
	// Give the goroutine a chance to run.
	time.Sleep(50 * time.Millisecond)
	if len(repo.created) != 1 {
		t.Fatalf("expected 1 create, got %d", len(repo.created))
	}
}

// ── Overdue / Notify / Report / ExportCleanup / Backup error branches ────────

func TestOverdueJob_SkipNilReadyAndCreateError(t *testing.T) {
	// Use ReadyAt as a per-fulfillment signal: nil = skip; epoch = create error.
	readyAt := time.Now().UTC().Add(-time.Hour)
	errReady := time.Unix(0, 0).UTC()
	nilReady := domain.Fulfillment{ID: uuid.New(), Type: domain.TypePhysical}
	createErrFF := domain.Fulfillment{ID: uuid.New(), Type: domain.TypePhysical, ReadyAt: &errReady}
	ok := domain.Fulfillment{ID: uuid.New(), Type: domain.TypePhysical, ReadyAt: &readyAt}

	fulfill := &branchFulfillRepo{overdue: []domain.Fulfillment{nilReady, createErrFF, ok}}
	ex := &branchExceptionRepo{createErrFor: map[uuid.UUID]bool{createErrFF.ID: true}}
	sla := &branchSLA{}

	created, err := NewOverdueJob(fulfill, ex, sla).Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if created != 1 {
		t.Fatalf("expected 1 created (nil skip + err skip + ok), got %d", created)
	}

	// ListOverdue error propagates.
	failing := &branchFulfillRepo{listErr: errors.New("list fail")}
	if _, err := NewOverdueJob(failing, ex, sla).Run(context.Background()); err == nil {
		t.Fatal("expected list error")
	}
}

type branchSLA struct{}

func (b *branchSLA) CalculateDeadline(_ context.Context, _ domain.FulfillmentType, readyAt time.Time) (time.Time, error) {
	return readyAt.Add(time.Hour), nil
}
func (b *branchSLA) IsOverdue(time.Time) bool { return true }

// branchFulfillRepo simulates ListOverdue and per-item behaviour.
type branchFulfillRepo struct {
	overdue []domain.Fulfillment
	listErr error
}

func (b *branchFulfillRepo) List(context.Context, repository.FulfillmentFilters, domain.PageRequest) ([]domain.Fulfillment, int, error) {
	return nil, 0, nil
}
func (b *branchFulfillRepo) GetByID(context.Context, uuid.UUID) (*domain.Fulfillment, error) {
	return nil, domain.ErrNotFound
}
func (b *branchFulfillRepo) GetByIDForUpdate(context.Context, pgx.Tx, uuid.UUID) (*domain.Fulfillment, error) {
	return nil, domain.ErrNotFound
}
func (b *branchFulfillRepo) Create(context.Context, pgx.Tx, *domain.Fulfillment) (*domain.Fulfillment, error) {
	return nil, nil
}
func (b *branchFulfillRepo) Update(context.Context, pgx.Tx, *domain.Fulfillment) (*domain.Fulfillment, error) {
	return nil, nil
}
func (b *branchFulfillRepo) CountByCustomerAndTier(context.Context, pgx.Tx, uuid.UUID, uuid.UUID, time.Time) (int, error) {
	return 0, nil
}
func (b *branchFulfillRepo) SoftDelete(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (b *branchFulfillRepo) Restore(context.Context, uuid.UUID) error               { return nil }
func (b *branchFulfillRepo) ListOverdue(context.Context) ([]domain.Fulfillment, error) {
	return b.overdue, b.listErr
}

type branchExceptionRepo struct {
	existsErrFor map[uuid.UUID]bool
	createErrFor map[uuid.UUID]bool
	createdIDs   []uuid.UUID
}

func (b *branchExceptionRepo) List(context.Context, repository.ExceptionFilters) ([]domain.FulfillmentException, error) {
	return nil, nil
}
func (b *branchExceptionRepo) GetByID(context.Context, uuid.UUID) (*domain.FulfillmentException, error) {
	return nil, domain.ErrNotFound
}
func (b *branchExceptionRepo) Create(_ context.Context, e *domain.FulfillmentException) (*domain.FulfillmentException, error) {
	if b.createErrFor[e.FulfillmentID] {
		return nil, errors.New("create error")
	}
	b.createdIDs = append(b.createdIDs, e.FulfillmentID)
	return e, nil
}
func (b *branchExceptionRepo) UpdateStatus(context.Context, uuid.UUID, domain.ExceptionStatus, *string, *uuid.UUID) error {
	return nil
}
func (b *branchExceptionRepo) ExistsOpenForFulfillment(_ context.Context, fulfillmentID uuid.UUID, _ domain.ExceptionType) (bool, error) {
	if b.existsErrFor[fulfillmentID] {
		return false, errors.New("exists error")
	}
	return false, nil
}

func TestBackupJob_Branches(t *testing.T) {
	// nil service → 0 returned.
	j := NewBackupJob(nil)
	if n, err := j.Run(context.Background()); err != nil || n != 0 {
		t.Fatalf("nil backup = (%d, %v)", n, err)
	}
	// Service error → 0 + error.
	svc := &fakeBackupService{err: errors.New("backup fail")}
	j2 := NewBackupJob(svc)
	if _, err := j2.Run(context.Background()); err == nil {
		t.Fatal("expected backup error propagation")
	}
}

func TestNotifyJob_Error(t *testing.T) {
	svc := &fakeMessagingService{err: errors.New("retry broke")}
	if _, err := NewNotifyJob(svc, 3).Run(context.Background()); err == nil {
		t.Fatal("expected retry error")
	}
}

type branchReportRepo struct {
	createErr     error
	created       int
	expired       []domain.ReportExport
	getExpiredErr error
	deleteErr     error
}

func (b *branchReportRepo) Create(_ context.Context, e *domain.ReportExport) (*domain.ReportExport, error) {
	if b.createErr != nil {
		return nil, b.createErr
	}
	e.ID = uuid.New()
	b.created++
	return e, nil
}
func (b *branchReportRepo) GetByID(context.Context, uuid.UUID) (*domain.ReportExport, error) {
	return nil, domain.ErrNotFound
}
func (b *branchReportRepo) List(context.Context, domain.PageRequest) ([]domain.ReportExport, int, error) {
	return nil, 0, nil
}
func (b *branchReportRepo) UpdateStatus(context.Context, uuid.UUID, domain.ExportStatus, *string, *int64, *string, *time.Time) error {
	return nil
}
func (b *branchReportRepo) GetExpired(context.Context, time.Time) ([]domain.ReportExport, error) {
	return b.expired, b.getExpiredErr
}
func (b *branchReportRepo) Delete(context.Context, uuid.UUID) error {
	return b.deleteErr
}

func TestScheduledReportJob_NilDeps(t *testing.T) {
	j := NewScheduledReportJob(nil, nil)
	if n, err := j.Run(context.Background()); err != nil || n != 0 {
		t.Fatalf("nil deps = (%d, %v)", n, err)
	}
}

func TestScheduledReportJob_CreateError(t *testing.T) {
	// Create returns an error → Run returns (0, err).
	repo := &branchReportRepo{createErr: errors.New("create failed")}
	exp := &fakeExportService{}
	n, err := NewScheduledReportJob(repo, exp).Run(context.Background())
	if err == nil {
		t.Fatalf("expected create error, got n=%d", n)
	}
}

func TestScheduledReportJob_ExportError(t *testing.T) {
	// Create succeeds, GenerateExport fails → (0, err).
	repo := &branchReportRepo{}
	exp := &fakeExportService{err: errors.New("export failed")}
	if _, err := NewScheduledReportJob(repo, exp).Run(context.Background()); err == nil {
		t.Fatal("expected export error")
	}
}

func TestExportCleanupJob_MissingFilesAndRepoErrors(t *testing.T) {
	// File path points to a nonexistent path — os.Remove errors silently (ENOENT)
	// and Delete succeeds → cleanup proceeds.
	repo := &branchReportRepo{
		expired: []domain.ReportExport{{ID: uuid.New(), FilePath: strPtr("/nonexistent/path.csv")}},
	}
	if _, err := NewExportCleanupJob(repo).Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Delete error is logged but NOT propagated — we simply don't count the record
	// as removed. Run returns (0, nil).
	repo2 := &branchReportRepo{
		expired:   []domain.ReportExport{{ID: uuid.New()}},
		deleteErr: errors.New("del fail"),
	}
	n, err := NewExportCleanupJob(repo2).Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 removed when Delete errors, got %d", n)
	}

	// GetExpired error propagates.
	repo3 := &branchReportRepo{getExpiredErr: errors.New("fetch fail")}
	if _, err := NewExportCleanupJob(repo3).Run(context.Background()); err == nil {
		t.Fatal("expected GetExpired error propagation")
	}
}

func strPtr(s string) *string { return &s }

func TestSchedulerRegisterDailyRunsOnStartStopImmediately(t *testing.T) {
	repo := &fakeJobRunRepo{}
	scheduler := NewScheduler(repo)

	// Daily job scheduled one minute in the future — won't fire in the test
	// window; we just need to exercise runDailyLoop + Stop cancel path.
	scheduler.RegisterDaily("nightly", 23, 59, func(ctx context.Context) (int, error) {
		return 0, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	scheduler.Start(ctx)

	time.Sleep(30 * time.Millisecond)
	scheduler.Stop()
}
