package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CommittableTx defines the minimal interface for committing and rolling back transactions.
type CommittableTx interface {
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// RunTx executes fn inside a transaction abstraction, rolling back on error or panic, and committing on success.
func RunTx(ctx context.Context, tx CommittableTx, fn func(ctx context.Context) error) (err error) {
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		} else if err != nil {
			_ = tx.Rollback(ctx)
		} else {
			commitErr := tx.Commit(ctx)
			if commitErr != nil {
				err = fmt.Errorf("failed to commit transaction: %w", commitErr)
			}
		}
	}()

	err = fn(ctx)
	return err
}

// WithTx begins a transaction on pool and executes fn within it using RunTx.
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	return RunTx(ctx, tx, func(ctx context.Context) error {
		return fn(tx)
	})
}
