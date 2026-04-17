package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fulfillops/fulfillops/internal/domain"
)

type SendLogFilters struct {
	RecipientID *uuid.UUID
	Channel     domain.SendLogChannel
	Status      domain.SendLogStatus
	DateFrom    *time.Time
	DateTo      *time.Time
}

type SendLogRepository interface {
	Create(ctx context.Context, log *domain.SendLog) (*domain.SendLog, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.SendLog, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.SendLogStatus, errMsg *string) error
	UpdateNextRetry(ctx context.Context, id uuid.UUID, nextRetryAt time.Time) error
	ClearNextRetry(ctx context.Context, id uuid.UUID) error
	MarkPrinted(ctx context.Context, id uuid.UUID, printedBy uuid.UUID) error
	List(ctx context.Context, filters SendLogFilters, page domain.PageRequest) ([]domain.SendLog, int, error)
	GetRetryable(ctx context.Context, now time.Time) ([]domain.SendLog, error)
}

type pgSendLogRepo struct{ pool *pgxpool.Pool }

func NewSendLogRepository(pool *pgxpool.Pool) SendLogRepository {
	return &pgSendLogRepo{pool: pool}
}

const sendLogCols = `id, template_id, recipient_id, recipient_type, dispatch_id, channel, status,
	attempt_count, next_retry_at, first_failed_at, printed_by, printed_at, context, error_message, created_at, updated_at`

func (r *pgSendLogRepo) Create(ctx context.Context, l *domain.SendLog) (*domain.SendLog, error) {
	l.ID = uuid.New()
	now := time.Now().UTC()
	l.CreatedAt, l.UpdatedAt = now, now
	if l.Context == nil {
		l.Context = []byte(`{}`)
	}
	if l.RecipientType == "" {
		l.RecipientType = domain.RecipientCustomer
	}

	_, err := r.pool.Exec(ctx,
		`INSERT INTO send_logs (id, template_id, recipient_id, recipient_type, dispatch_id,
		                        channel, status, attempt_count, next_retry_at, first_failed_at, context, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		l.ID, l.TemplateID, l.RecipientID, string(l.RecipientType), l.DispatchID,
		string(l.Channel), string(l.Status),
		l.AttemptCount, l.NextRetryAt, l.FirstFailedAt, l.Context, l.CreatedAt, l.UpdatedAt)
	return l, err
}

func (r *pgSendLogRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.SendLog, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+sendLogCols+` FROM send_logs WHERE id=$1`, id)
	if err != nil {
		return nil, err
	}
	l, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[domain.SendLog])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.NewNotFoundError("send log")
	}
	return &l, err
}

func (r *pgSendLogRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.SendLogStatus, errMsg *string) error {
	// Only increment attempt_count on FAILED transitions (not on re-queue).
	// Set first_failed_at once on the first FAILED transition. Casting $1 to
	// varchar keeps the parameter type unambiguous under both assignment and
	// comparison uses in the same statement.
	_, err := r.pool.Exec(ctx,
		`UPDATE send_logs
		 SET status=$1::varchar,
		     error_message=$2,
		     attempt_count = CASE WHEN $1::varchar='FAILED' THEN attempt_count+1 ELSE attempt_count END,
		     first_failed_at = CASE WHEN $1::varchar='FAILED' AND first_failed_at IS NULL THEN NOW() ELSE first_failed_at END,
		     updated_at=NOW()
		 WHERE id=$3`,
		string(status), errMsg, id)
	return err
}

func (r *pgSendLogRepo) UpdateNextRetry(ctx context.Context, id uuid.UUID, nextRetryAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE send_logs SET next_retry_at=$1, updated_at=NOW() WHERE id=$2`,
		nextRetryAt, id)
	return err
}

func (r *pgSendLogRepo) ClearNextRetry(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE send_logs SET next_retry_at=NULL, updated_at=NOW() WHERE id=$1`, id)
	return err
}

func (r *pgSendLogRepo) MarkPrinted(ctx context.Context, id uuid.UUID, printedBy uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE send_logs SET status='PRINTED', printed_by=$1, printed_at=NOW(), updated_at=NOW()
		 WHERE id=$2 AND status='QUEUED'`,
		printedBy, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.NewNotFoundError("send log")
	}
	return nil
}

func (r *pgSendLogRepo) List(ctx context.Context, f SendLogFilters, page domain.PageRequest) ([]domain.SendLog, int, error) {
	page.Normalize()
	args := []any{}
	where := `WHERE 1=1`
	i := 1

	if f.RecipientID != nil {
		where += fmt.Sprintf(` AND recipient_id=$%d`, i)
		args = append(args, *f.RecipientID)
		i++
	}
	if f.Channel != "" {
		where += fmt.Sprintf(` AND channel=$%d`, i)
		args = append(args, string(f.Channel))
		i++
	}
	if f.Status != "" {
		where += fmt.Sprintf(` AND status=$%d`, i)
		args = append(args, string(f.Status))
		i++
	}
	if f.DateFrom != nil {
		where += fmt.Sprintf(` AND created_at >= $%d`, i)
		args = append(args, *f.DateFrom)
		i++
	}
	if f.DateTo != nil {
		where += fmt.Sprintf(` AND created_at <= $%d`, i)
		args = append(args, *f.DateTo)
		i++
	}

	countArgs := make([]any, len(args))
	copy(countArgs, args)
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM send_logs `+where, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, page.PageSize, page.Offset())
	rows, err := r.pool.Query(ctx,
		`SELECT `+sendLogCols+` FROM send_logs `+where+
			fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, i, i+1),
		args...)
	if err != nil {
		return nil, 0, err
	}
	logs, err := pgx.CollectRows(rows, pgx.RowToStructByName[domain.SendLog])
	return logs, total, err
}

func (r *pgSendLogRepo) GetRetryable(ctx context.Context, now time.Time) ([]domain.SendLog, error) {
	// Only FAILED rows consume retry slots. QUEUED rows remain in the handoff
	// queue until an operator picks them up or explicitly fails them.
	rows, err := r.pool.Query(ctx,
		`SELECT `+sendLogCols+`
		 FROM send_logs
		 WHERE status='FAILED' AND next_retry_at IS NOT NULL AND next_retry_at <= $1`,
		now)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[domain.SendLog])
}
