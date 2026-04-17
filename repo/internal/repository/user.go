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

type UserRepository interface {
	GetByUsername(ctx context.Context, username string) (*domain.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	List(ctx context.Context, role domain.UserRole, isActive *bool) ([]domain.User, error)
	Create(ctx context.Context, u *domain.User) (*domain.User, error)
	Update(ctx context.Context, u *domain.User) (*domain.User, error)
	Deactivate(ctx context.Context, id uuid.UUID) error
	UpdatePassword(ctx context.Context, id uuid.UUID, hash string, clearRotate bool) error
	CountByRole(ctx context.Context, role domain.UserRole) (int, error)
}

type pgUserRepo struct{ pool *pgxpool.Pool }

func NewUserRepository(pool *pgxpool.Pool) UserRepository {
	return &pgUserRepo{pool: pool}
}

const userCols = `id, username, email, password_hash, role, is_active, must_rotate_password, version, created_at, updated_at`

func (r *pgUserRepo) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+userCols+` FROM users WHERE username=$1`, username)
	if err != nil {
		return nil, err
	}
	u, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[domain.User])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.NewNotFoundError("user")
	}
	return &u, err
}

func (r *pgUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+userCols+` FROM users WHERE id=$1`, id)
	if err != nil {
		return nil, err
	}
	u, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[domain.User])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.NewNotFoundError("user")
	}
	return &u, err
}

func (r *pgUserRepo) List(ctx context.Context, role domain.UserRole, isActive *bool) ([]domain.User, error) {
	where := `WHERE 1=1`
	args := []any{}
	i := 1
	if role != "" {
		where += fmt.Sprintf(` AND role=$%d`, i)
		args = append(args, string(role))
		i++
	}
	if isActive != nil {
		where += fmt.Sprintf(` AND is_active=$%d`, i)
		args = append(args, *isActive)
		i++
	}
	_ = i
	rows, err := r.pool.Query(ctx, `SELECT `+userCols+` FROM users `+where+` ORDER BY username`, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[domain.User])
}

func (r *pgUserRepo) Create(ctx context.Context, u *domain.User) (*domain.User, error) {
	u.ID = uuid.New()
	now := time.Now().UTC()
	u.CreatedAt, u.UpdatedAt, u.Version = now, now, 1

	_, err := r.pool.Exec(ctx,
		`INSERT INTO users (id, username, email, password_hash, role, is_active, must_rotate_password, version, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		u.ID, u.Username, u.Email, u.PasswordHash, string(u.Role), u.IsActive, u.MustRotatePassword, u.Version, u.CreatedAt, u.UpdatedAt)
	return u, err
}

func (r *pgUserRepo) UpdatePassword(ctx context.Context, id uuid.UUID, hash string, clearRotate bool) error {
	mustRotate := !clearRotate
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET password_hash=$1, must_rotate_password=$2, updated_at=NOW(), version=version+1 WHERE id=$3`,
		hash, mustRotate, id)
	return err
}

func (r *pgUserRepo) CountByRole(ctx context.Context, role domain.UserRole) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM users WHERE role=$1 AND is_active=TRUE`, string(role)).Scan(&count)
	return count, err
}

func (r *pgUserRepo) Update(ctx context.Context, u *domain.User) (*domain.User, error) {
	u.UpdatedAt = time.Now().UTC()
	tag, err := r.pool.Exec(ctx,
		`UPDATE users SET username=$1, email=$2, role=$3, is_active=$4, updated_at=$5, version=version+1
		 WHERE id=$6 AND version=$7`,
		u.Username, u.Email, string(u.Role), u.IsActive, u.UpdatedAt, u.ID, u.Version)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, domain.NewConflictError()
	}
	return r.GetByID(ctx, u.ID)
}

func (r *pgUserRepo) Deactivate(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET is_active=FALSE, updated_at=NOW() WHERE id=$1`, id)
	return err
}
