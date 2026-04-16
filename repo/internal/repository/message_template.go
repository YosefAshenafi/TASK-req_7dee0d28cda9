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

type MessageTemplateRepository interface {
	List(ctx context.Context, category domain.TemplateCategory, channel domain.SendLogChannel, includeDeleted bool) ([]domain.MessageTemplate, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.MessageTemplate, error)
	Create(ctx context.Context, t *domain.MessageTemplate) (*domain.MessageTemplate, error)
	Update(ctx context.Context, t *domain.MessageTemplate) (*domain.MessageTemplate, error)
	SoftDelete(ctx context.Context, id uuid.UUID, deletedBy uuid.UUID) error
	Restore(ctx context.Context, id uuid.UUID) error
}

type pgMessageTemplateRepo struct{ pool *pgxpool.Pool }

func NewMessageTemplateRepository(pool *pgxpool.Pool) MessageTemplateRepository {
	return &pgMessageTemplateRepo{pool: pool}
}

func (r *pgMessageTemplateRepo) List(ctx context.Context, cat domain.TemplateCategory, ch domain.SendLogChannel, includeDeleted bool) ([]domain.MessageTemplate, error) {
	args := []any{}
	where := `WHERE 1=1`
	i := 1
	if cat != "" {
		where += fmt.Sprintf(` AND category=$%d`, i)
		args = append(args, string(cat))
		i++
	}
	if ch != "" {
		where += fmt.Sprintf(` AND channel=$%d`, i)
		args = append(args, string(ch))
		i++
	}
	if !includeDeleted {
		where += ` AND deleted_at IS NULL`
	}
	_ = i

	rows, err := r.pool.Query(ctx,
		`SELECT id, name, category, channel, body_template, version, created_at, updated_at, deleted_at, deleted_by
		 FROM message_templates `+where+` ORDER BY name`, args...)
	if err != nil {
		return nil, fmt.Errorf("listing templates: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[domain.MessageTemplate])
}

func (r *pgMessageTemplateRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.MessageTemplate, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, category, channel, body_template, version, created_at, updated_at, deleted_at, deleted_by
		 FROM message_templates WHERE id=$1`, id)
	if err != nil {
		return nil, err
	}
	t, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[domain.MessageTemplate])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.NewNotFoundError("message template")
	}
	return &t, err
}

func (r *pgMessageTemplateRepo) Create(ctx context.Context, t *domain.MessageTemplate) (*domain.MessageTemplate, error) {
	t.ID = uuid.New()
	now := time.Now().UTC()
	t.CreatedAt, t.UpdatedAt, t.Version = now, now, 1

	_, err := r.pool.Exec(ctx,
		`INSERT INTO message_templates (id, name, category, channel, body_template, version, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		t.ID, t.Name, string(t.Category), string(t.Channel), t.BodyTemplate, t.Version, t.CreatedAt, t.UpdatedAt)
	return t, err
}

func (r *pgMessageTemplateRepo) Update(ctx context.Context, t *domain.MessageTemplate) (*domain.MessageTemplate, error) {
	t.UpdatedAt = time.Now().UTC()
	tag, err := r.pool.Exec(ctx,
		`UPDATE message_templates SET name=$1, category=$2, channel=$3, body_template=$4, updated_at=$5, version=version+1
		 WHERE id=$6 AND version=$7 AND deleted_at IS NULL`,
		t.Name, string(t.Category), string(t.Channel), t.BodyTemplate, t.UpdatedAt, t.ID, t.Version)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, domain.NewConflictError()
	}
	return r.GetByID(ctx, t.ID)
}

func (r *pgMessageTemplateRepo) SoftDelete(ctx context.Context, id uuid.UUID, deletedBy uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE message_templates SET deleted_at=NOW(), deleted_by=$1 WHERE id=$2 AND deleted_at IS NULL`, deletedBy, id)
	if tag.RowsAffected() == 0 && err == nil {
		return domain.NewNotFoundError("message template")
	}
	return err
}

func (r *pgMessageTemplateRepo) Restore(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE message_templates SET deleted_at=NULL, deleted_by=NULL, updated_at=NOW()
		 WHERE id=$1 AND deleted_at IS NOT NULL AND deleted_at > NOW() - INTERVAL '30 days'`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrSoftDeleteExpired
	}
	return nil
}
