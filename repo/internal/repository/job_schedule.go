package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fulfillops/fulfillops/internal/domain"
)

type JobScheduleRepository interface {
	List(ctx context.Context) ([]domain.JobSchedule, error)
	GetByKey(ctx context.Context, jobKey string) (*domain.JobSchedule, error)
	Update(ctx context.Context, s *domain.JobSchedule) (*domain.JobSchedule, error)
}

type pgJobScheduleRepo struct{ pool *pgxpool.Pool }

func NewJobScheduleRepository(pool *pgxpool.Pool) JobScheduleRepository {
	return &pgJobScheduleRepo{pool: pool}
}

const jobScheduleCols = `id, job_key, interval_seconds, daily_hour, daily_minute,
	enabled, updated_by, updated_at, version`

func (r *pgJobScheduleRepo) List(ctx context.Context) ([]domain.JobSchedule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+jobScheduleCols+` FROM job_schedules ORDER BY job_key`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[domain.JobSchedule])
}

func (r *pgJobScheduleRepo) GetByKey(ctx context.Context, jobKey string) (*domain.JobSchedule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+jobScheduleCols+` FROM job_schedules WHERE job_key=$1`, jobKey)
	if err != nil {
		return nil, err
	}
	s, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[domain.JobSchedule])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.NewNotFoundError("job schedule")
	}
	return &s, err
}

func (r *pgJobScheduleRepo) Update(ctx context.Context, s *domain.JobSchedule) (*domain.JobSchedule, error) {
	s.UpdatedAt = time.Now().UTC()
	tag, err := r.pool.Exec(ctx,
		`UPDATE job_schedules
		 SET interval_seconds=$1, daily_hour=$2, daily_minute=$3, enabled=$4,
		     updated_by=$5, updated_at=$6, version=version+1
		 WHERE id=$7 AND version=$8`,
		s.IntervalSeconds, s.DailyHour, s.DailyMinute, s.Enabled,
		s.UpdatedBy, s.UpdatedAt, s.ID, s.Version)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, domain.NewConflictError()
	}
	return r.GetByKey(ctx, s.JobKey)
}
