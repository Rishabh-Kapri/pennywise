package repository

import "context"

// DBTxn runs a unit of work in one database transaction.
//
// Do commits when fn returns nil and rolls back otherwise. Repositories called
// with the ctx passed to fn take part in that transaction, so no driver type
// appears in any repository signature. Calling Do again inside fn joins the
// outer transaction instead of opening a second one.
type DBTxn interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}
