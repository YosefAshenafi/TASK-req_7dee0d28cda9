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

type SystemSettingRepository interface {
	Get(ctx context.Context, key string) (*domain.SystemSetting, error)
	Set(ctx context.Context, key string, value []byte, updatedBy *uuid.UUID) error
	GetAll(ctx context.Context) ([]domain.SystemSetting, error)
}

type pgSystemSettingRepo struct{ pool *pgxpool.Pool }

func NewSystemSettingRepository(pool *pgxpool.Pool) SystemSettingRepository {
	return &pgSystemSettingRepo{pool: pool}
}

func (r *pgSystemSettingRepo) Get(ctx context.Context, key string) (*domain.SystemSetting, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, key, value, updated_by, updated_at FROM system_settings WHERE key=$1`, key)
	if err != nil {
		return nil, err
	}
	s, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[domain.SystemSetting])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("setting %q: %w", key, domain.ErrNotFound)
	}
	return &s, err
}

func (r *pgSystemSettingRepo) Set(ctx context.Context, key string, value []byte, updatedBy *uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO system_settings (id, key, value, updated_by, updated_at)
		 VALUES (gen_random_uuid(), $1, $2, $3, $4)
		 ON CONFLICT (key) DO UPDATE SET value=$2, updated_by=$3, updated_at=$4`,
		key, value, updatedBy, time.Now().UTC())
	return err
}

func (r *pgSystemSettingRepo) GetAll(ctx context.Context) ([]domain.SystemSetting, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, key, value, updated_by, updated_at FROM system_settings ORDER BY key`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[domain.SystemSetting])
}
