package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TxManager provides an atomic unit-of-work abstraction.
type TxManager interface {
	WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error
}

type txManager struct {
	pool *pgxpool.Pool
}

// NewTxManager creates a TxManager backed by the given pool.
func NewTxManager(pool *pgxpool.Pool) TxManager {
	return &txManager{pool: pool}
}

// WithTx begins a transaction, calls fn, then commits. Rolls back on any error or panic.
func (m *txManager) WithTx(ctx context.Context, fn func(tx pgx.Tx) error) (retErr error) {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p) // re-panic after rollback
		}
		if retErr != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	if err := fn(tx); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// DBTX is satisfied by both *pgxpool.Pool and pgx.Tx, allowing repos to
// work inside or outside transactions without duplication.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
