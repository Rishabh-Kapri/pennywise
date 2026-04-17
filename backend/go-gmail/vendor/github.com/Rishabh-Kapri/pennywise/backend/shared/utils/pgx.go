package utils

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Helper method to execute a function within a transaction. It will commit if the function returns nil error, otherwise it will rollback.
// This is useful to avoid repeating the same transaction handling code in multiple places. Just pass the function that contains the logic that needs to be executed within the transaction.
// This also ensures that the transaction is properly rolled back in case of any error, preventing potential data inconsistencies.
//
// Example usage:
//
//	err := WithTx(ctx, pool, func(tx pgx.Tx) error {
//	    // Your transactional code here, using the provided tx
//	    // For example:
//	    err := repo.Create(ctx, tx, data)
//	    if err != nil {
//	        return err
//	    }
//	    return nil
//	})
//
//	if err != nil {
//	    // Handle error
//	}
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Pass the transaction to the function and execute it
	// This function is passed from the service layer and contains the actual logic that needs to be executed within the transaction
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
