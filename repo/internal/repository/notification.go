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

type NotificationRepository interface {
	Create(ctx context.Context, n *domain.Notification) (*domain.Notification, error)
	// CreateTx inserts a notification inside a caller-owned transaction so the
	// write participates in the same atomic bundle as its triggering action.
	CreateTx(ctx context.Context, tx pgx.Tx, n *domain.Notification) error
	ListByUserID(ctx context.Context, userID uuid.UUID, isRead *bool, page domain.PageRequest) ([]domain.Notification, int, error)
	MarkRead(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
}

type pgNotificationRepo struct{ pool *pgxpool.Pool }

func NewNotificationRepository(pool *pgxpool.Pool) NotificationRepository {
	return &pgNotificationRepo{pool: pool}
}

func (r *pgNotificationRepo) Create(ctx context.Context, n *domain.Notification) (*domain.Notification, error) {
	n.ID = uuid.New()
	n.CreatedAt = time.Now().UTC()
	if n.Context == nil {
		n.Context = []byte(`{}`)
	}

	_, err := r.pool.Exec(ctx,
		`INSERT INTO notifications (id, user_id, title, body, is_read, context, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		n.ID, n.UserID, n.Title, n.Body, n.IsRead, n.Context, n.CreatedAt)
	return n, err
}

func (r *pgNotificationRepo) CreateTx(ctx context.Context, tx pgx.Tx, n *domain.Notification) error {
	n.ID = uuid.New()
	n.CreatedAt = time.Now().UTC()
	if n.Context == nil {
		n.Context = []byte(`{}`)
	}
	_, err := tx.Exec(ctx,
		`INSERT INTO notifications (id, user_id, title, body, is_read, context, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		n.ID, n.UserID, n.Title, n.Body, n.IsRead, n.Context, n.CreatedAt)
	return err
}

func (r *pgNotificationRepo) ListByUserID(ctx context.Context, userID uuid.UUID, isRead *bool, page domain.PageRequest) ([]domain.Notification, int, error) {
	page.Normalize()

	where := `WHERE user_id=$1`
	args := []any{userID}
	if isRead != nil {
		where += ` AND is_read=$2`
		args = append(args, *isRead)
	}

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM notifications `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting notifications: %w", err)
	}

	args = append(args, page.PageSize, page.Offset())
	n := len(args)
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, title, body, is_read, context, created_at
		 FROM notifications `+where+
			fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, n-1, n),
		args...)
	if err != nil {
		return nil, 0, err
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[domain.Notification])
	return items, total, err
}

func (r *pgNotificationRepo) MarkRead(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE notifications SET is_read=TRUE WHERE id=$1 AND user_id=$2`, id, userID)
	return err
}
