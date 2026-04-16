package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fulfillops/fulfillops/internal/domain"
)

type JobRunFilters struct {
	JobName     string
	Status      domain.JobStatus
	StartedFrom *time.Time
	StartedTo   *time.Time
}

type JobRunRepository interface {
	Create(ctx context.Context, run *domain.JobRunHistory) (*domain.JobRunHistory, error)
	Finish(ctx context.Context, id uuid.UUID, status domain.JobStatus, recordsProcessed int, errorStack *string) error
	List(ctx context.Context, filters JobRunFilters, page domain.PageRequest) ([]domain.JobRunHistory, int, error)
}

type pgJobRunRepo struct{ pool *pgxpool.Pool }

func NewJobRunRepository(pool *pgxpool.Pool) JobRunRepository {
	return &pgJobRunRepo{pool: pool}
}

func (r *pgJobRunRepo) Create(ctx context.Context, run *domain.JobRunHistory) (*domain.JobRunHistory, error) {
	run.ID = uuid.New()
	run.StartedAt = time.Now().UTC()
	run.Status = domain.JobRunning
	run.CreatedAt = run.StartedAt

	_, err := r.pool.Exec(ctx,
		`INSERT INTO job_run_history (id, job_name, status, started_at, records_processed, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		run.ID, run.JobName, string(run.Status), run.StartedAt, run.RecordsProcessed, run.CreatedAt)
	return run, err
}

func (r *pgJobRunRepo) Finish(ctx context.Context, id uuid.UUID, status domain.JobStatus, records int, errStack *string) error {
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx,
		`UPDATE job_run_history SET status=$1, finished_at=$2, records_processed=$3, error_stack=$4 WHERE id=$5`,
		string(status), now, records, errStack, id)
	return err
}

func (r *pgJobRunRepo) List(ctx context.Context, f JobRunFilters, page domain.PageRequest) ([]domain.JobRunHistory, int, error) {
	page.Normalize()
	args := []any{}
	where := `WHERE 1=1`
	i := 1

	if f.JobName != "" {
		where += fmt.Sprintf(` AND job_name=$%d`, i)
		args = append(args, f.JobName)
		i++
	}
	if f.Status != "" {
		where += fmt.Sprintf(` AND status=$%d`, i)
		args = append(args, string(f.Status))
		i++
	}
	if f.StartedFrom != nil {
		where += fmt.Sprintf(` AND started_at >= $%d`, i)
		args = append(args, *f.StartedFrom)
		i++
	}
	if f.StartedTo != nil {
		where += fmt.Sprintf(` AND started_at <= $%d`, i)
		args = append(args, *f.StartedTo)
		i++
	}

	countArgs := make([]any, len(args))
	copy(countArgs, args)
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM job_run_history `+where, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, page.PageSize, page.Offset())
	rows, err := r.pool.Query(ctx,
		`SELECT id, job_name, status, started_at, finished_at, records_processed, error_stack, created_at
		 FROM job_run_history `+where+
			fmt.Sprintf(` ORDER BY started_at DESC LIMIT $%d OFFSET $%d`, i, i+1),
		args...)
	if err != nil {
		return nil, 0, err
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[domain.JobRunHistory])
	return items, total, err
}
