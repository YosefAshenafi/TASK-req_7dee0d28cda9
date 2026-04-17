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

// ReportExportFilters scope a report export listing. IncludeSensitiveOnly=false
// plus sensitiveVisible=false lets the repository exclude sensitive rows from
// both the rows returned and the total count so pagination totals match what
// the caller actually sees.
type ReportExportFilters struct {
	// SensitiveVisible == true: sensitive rows are included (admin view).
	// false: sensitive rows are filtered out of both the page and the total.
	SensitiveVisible bool
}

type ReportExportRepository interface {
	Create(ctx context.Context, e *domain.ReportExport) (*domain.ReportExport, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.ReportExport, error)
	List(ctx context.Context, filters ReportExportFilters, page domain.PageRequest) ([]domain.ReportExport, int, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.ExportStatus, filePath *string, fileSize *int64, checksum *string, expiresAt *time.Time) error
	GetExpired(ctx context.Context, now time.Time) ([]domain.ReportExport, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type pgReportExportRepo struct{ pool *pgxpool.Pool }

func NewReportExportRepository(pool *pgxpool.Pool) ReportExportRepository {
	return &pgReportExportRepo{pool: pool}
}

const exportCols = `id, report_type, filters, file_path, file_size_bytes, checksum_sha256,
	include_sensitive, status, expires_at, generated_by, created_at, updated_at`

func (r *pgReportExportRepo) Create(ctx context.Context, e *domain.ReportExport) (*domain.ReportExport, error) {
	e.ID = uuid.New()
	now := time.Now().UTC()
	e.CreatedAt, e.UpdatedAt = now, now
	if e.Filters == nil {
		e.Filters = []byte(`{}`)
	}

	_, err := r.pool.Exec(ctx,
		`INSERT INTO report_exports (id, report_type, filters, include_sensitive, status, generated_by, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		e.ID, e.ReportType, e.Filters, e.IncludeSensitive, string(e.Status), e.GeneratedBy, e.CreatedAt, e.UpdatedAt)
	return e, err
}

func (r *pgReportExportRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.ReportExport, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+exportCols+` FROM report_exports WHERE id=$1`, id)
	if err != nil {
		return nil, err
	}
	e, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[domain.ReportExport])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.NewNotFoundError("report export")
	}
	return &e, err
}

func (r *pgReportExportRepo) List(ctx context.Context, f ReportExportFilters, page domain.PageRequest) ([]domain.ReportExport, int, error) {
	page.Normalize()
	where := ""
	if !f.SensitiveVisible {
		where = ` WHERE include_sensitive = FALSE`
	}
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM report_exports`+where).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+exportCols+` FROM report_exports`+where+` ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		page.PageSize, page.Offset())
	if err != nil {
		return nil, 0, err
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[domain.ReportExport])
	return items, total, err
}

func (r *pgReportExportRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.ExportStatus, filePath *string, fileSize *int64, checksum *string, expiresAt *time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE report_exports SET status=$1, file_path=$2, file_size_bytes=$3,
		  checksum_sha256=$4, expires_at=$5, updated_at=NOW() WHERE id=$6`,
		string(status), filePath, fileSize, checksum, expiresAt, id)
	return err
}

func (r *pgReportExportRepo) GetExpired(ctx context.Context, now time.Time) ([]domain.ReportExport, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+exportCols+` FROM report_exports WHERE expires_at < $1 AND status='COMPLETED'`, now)
	if err != nil {
		return nil, fmt.Errorf("getting expired exports: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[domain.ReportExport])
}

func (r *pgReportExportRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM report_exports WHERE id=$1`, id)
	return err
}
