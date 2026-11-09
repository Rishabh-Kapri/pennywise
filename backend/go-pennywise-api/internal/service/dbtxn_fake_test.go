package service

import (
	"context"
	"errors"
)

// fakeDBTxn runs fn directly: unit tests have no database, so there is no
// transaction to open or join.
type fakeDBTxn struct{}

func (fakeDBTxn) Do(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

// failingDBTxn fails before running fn, like a database that refuses BEGIN.
type failingDBTxn struct{}

func (failingDBTxn) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	return errors.New("begin failed")
}
