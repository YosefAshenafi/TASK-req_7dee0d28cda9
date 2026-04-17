package job

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
)

// JobFunc is a function executed by the scheduler. It returns the number of
// records processed and an optional error.
type JobFunc func(ctx context.Context) (int, error)

type jobEntry struct {
	name     string
	interval time.Duration
	fn       JobFunc
	// daily scheduling
	daily  bool
	hour   int
	minute int
}

// Scheduler runs registered jobs at fixed intervals and records run history.
type Scheduler struct {
	mu         sync.RWMutex
	jobs       []jobEntry
	jobRunRepo repository.JobRunRepository
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	tz         *time.Location // timezone for daily job scheduling (default UTC)
}

// NewScheduler creates a Scheduler backed by the given job run repository.
func NewScheduler(jobRunRepo repository.JobRunRepository) *Scheduler {
	return &Scheduler{jobRunRepo: jobRunRepo, tz: time.UTC}
}

// WithTimezone sets the timezone used for daily job scheduling.
// If loc is nil, UTC is used.
func (s *Scheduler) WithTimezone(loc *time.Location) *Scheduler {
	if loc != nil {
		s.tz = loc
	}
	return s
}

// Register adds a job to be run every interval.
func (s *Scheduler) Register(name string, interval time.Duration, fn JobFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs = append(s.jobs, jobEntry{name: name, interval: interval, fn: fn})
}

// RegisterDaily adds a job to be run once per day at the given wall-clock time
// in the scheduler's configured timezone (default UTC).
func (s *Scheduler) RegisterDaily(name string, hour, minute int, fn JobFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs = append(s.jobs, jobEntry{name: name, fn: fn, daily: true, hour: hour, minute: minute})
}

// Start launches all registered jobs as goroutines.
func (s *Scheduler) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	s.mu.RLock()
	jobs := make([]jobEntry, len(s.jobs))
	copy(jobs, s.jobs)
	s.mu.RUnlock()

	for _, j := range jobs {
		j := j
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.runLoop(ctx, j)
		}()
	}
	log.Printf("scheduler: started %d jobs", len(jobs))
}

// Stop signals all jobs to stop and waits for them to finish.
func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	log.Println("scheduler: stopped")
}

func (s *Scheduler) runLoop(ctx context.Context, j jobEntry) {
	if j.daily {
		s.runDailyLoop(ctx, j)
		return
	}
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runOnce(ctx, j)
		}
	}
}

func (s *Scheduler) runDailyLoop(ctx context.Context, j jobEntry) {
	tz := s.tz
	if tz == nil {
		tz = time.UTC
	}
	for {
		now := time.Now().In(tz)
		next := time.Date(now.Year(), now.Month(), now.Day(), j.hour, j.minute, 0, 0, tz)
		if !next.After(now) {
			// Advance by 24 hours and re-normalise to handle DST transitions.
			next = time.Date(now.Year(), now.Month(), now.Day()+1, j.hour, j.minute, 0, 0, tz)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(next)):
			s.runOnce(ctx, j)
		}
	}
}

func (s *Scheduler) runOnce(ctx context.Context, j jobEntry) {
	run := &domain.JobRunHistory{
		JobName:   j.name,
		Status:    domain.JobRunning,
		StartedAt: time.Now().UTC(),
	}
	created, err := s.jobRunRepo.Create(ctx, run)
	if err != nil {
		log.Printf("scheduler: failed to create job_run_history for %s: %v", j.name, err)
	}

	records, jobErr := j.fn(ctx)

	status := domain.JobCompleted
	var errStack *string
	if jobErr != nil {
		status = domain.JobFailed
		msg := jobErr.Error()
		errStack = &msg
		log.Printf("scheduler: job %s failed: %v", j.name, jobErr)
	} else {
		log.Printf("scheduler: job %s completed, processed %d records", j.name, records)
	}

	if created != nil {
		if finishErr := s.jobRunRepo.Finish(ctx, created.ID, status, records, errStack); finishErr != nil {
			log.Printf("scheduler: failed to finish job_run_history for %s: %v", j.name, finishErr)
		}
	}
}

// RunOnce triggers a job by name immediately (for manual trigger endpoints).
func (s *Scheduler) RunOnce(ctx context.Context, name string) error {
	s.mu.RLock()
	var found *jobEntry
	for i := range s.jobs {
		if s.jobs[i].name == name {
			found = &s.jobs[i]
			break
		}
	}
	s.mu.RUnlock()

	if found == nil {
		return domain.NewNotFoundError("job")
	}
	go s.runOnce(ctx, *found)
	return nil
}

// Status returns a snapshot of recent job run history (last 20 runs per job).
func (s *Scheduler) Status(ctx context.Context) ([]domain.JobRunHistory, error) {
	runs, _, err := s.jobRunRepo.List(ctx, repository.JobRunFilters{}, domain.PageRequest{Page: 1, PageSize: 50})
	return runs, err
}
