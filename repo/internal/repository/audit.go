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

type AuditFilters struct {
	TableName   string
	RecordID    *uuid.UUID
	Operation   string
	PerformedBy *uuid.UUID
	DateFrom    *time.Time
	DateTo      *time.Time
}

type AuditRepository interface {
	Create(ctx context.Context, log *domain.AuditLog) error
	List(ctx context.Context, filters AuditFilters, page domain.PageRequest) ([]domain.AuditLog, int, error)
}

type pgAuditRepo struct{ pool *pgxpool.Pool }

func NewAuditRepository(pool *pgxpool.Pool) AuditRepository {
	return &pgAuditRepo{pool: pool}
}

func (r *pgAuditRepo) Create(ctx context.Context, l *domain.AuditLog) error {
	l.ID = uuid.New()
	l.CreatedAt = time.Now().UTC()

	_, err := r.pool.Exec(ctx,
		`INSERT INTO audit_logs
		   (id, table_name, record_id, operation, performed_by, before_state, after_state, ip_address, request_id, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		l.ID, l.TableName, l.RecordID, l.Operation, l.PerformedBy,
		l.BeforeState, l.AfterState, l.IPAddress, l.RequestID, l.CreatedAt)
	return err
}

func (r *pgAuditRepo) List(ctx context.Context, f AuditFilters, page domain.PageRequest) ([]domain.AuditLog, int, error) {
	page.Normalize()
	args := []any{}
	where := `WHERE 1=1`
	i := 1

	if f.TableName != "" {
		where += fmt.Sprintf(` AND table_name=$%d`, i)
		args = append(args, f.TableName)
		i++
	}
	if f.RecordID != nil {
		where += fmt.Sprintf(` AND record_id=$%d`, i)
		args = append(args, *f.RecordID)
		i++
	}
	if f.Operation != "" {
		where += fmt.Sprintf(` AND operation=$%d`, i)
		args = append(args, f.Operation)
		i++
	}
	if f.PerformedBy != nil {
		where += fmt.Sprintf(` AND performed_by=$%d`, i)
		args = append(args, *f.PerformedBy)
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
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs `+where, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, page.PageSize, page.Offset())
	rows, err := r.pool.Query(ctx,
		`SELECT id, table_name, record_id, operation, performed_by, before_state, after_state, ip_address, request_id, created_at
		 FROM audit_logs `+where+
			fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, i, i+1),
		args...)
	if err != nil {
		return nil, 0, err
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[domain.AuditLog])
	return items, total, err
}
