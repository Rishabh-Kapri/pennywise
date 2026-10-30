package repository

import "context"

// BulkInserter loads many rows into one table in a single round trip. Inside
// DBTxn.Do it takes part in the transaction like any repository call.
type BulkInserter interface {
	BulkInsert(ctx context.Context, table string, columns []string, rows [][]any) (int64, error)
}
