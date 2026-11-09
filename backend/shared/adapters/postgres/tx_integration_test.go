package postgres

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These cover the transaction runner (tx.go): the transaction rides in ctx,
// repositories called with that ctx join it, nested Do calls join the outer
// transaction, and every exit path either commits or rolls back.

func countTxns(t *testing.T, repo repository.TransactionRepository, budgetID uuid.UUID) int {
	t.Helper()
	all, err := repo.GetAll(context.Background(), budgetID, nil)
	require.NoError(t, err)
	return len(all)
}

func TestDBTxn_CommitPersistsEveryWrite(t *testing.T) {
	pool, repo, s := setup(t)

	err := NewDBTxn(pool).Do(context.Background(), func(ctx context.Context) error {
		for i := 0; i < 3; i++ {
			if _, err := repo.Create(ctx, newTxn(s, "2026-10-12", -1)); err != nil {
				return err
			}
		}
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, 3, countTxns(t, repo, s.budgetID))
}

func TestDBTxn_ErrorRollsBackEveryWrite(t *testing.T) {
	pool, repo, s := setup(t)
	boom := errors.New("boom")

	err := NewDBTxn(pool).Do(context.Background(), func(ctx context.Context) error {
		if _, err := repo.Create(ctx, newTxn(s, "2026-10-12", -1)); err != nil {
			return err
		}
		if _, err := repo.Create(ctx, newTxn(s, "2026-10-13", -2)); err != nil {
			return err
		}
		return boom
	})

	require.ErrorIs(t, err, boom, "Do returns fn's error unchanged")
	assert.Zero(t, countTxns(t, repo, s.budgetID))
}

func TestDBTxn_FailedStatementRollsBackEarlierWrites(t *testing.T) {
	pool, repo, s := setup(t)

	err := NewDBTxn(pool).Do(context.Background(), func(ctx context.Context) error {
		if _, err := repo.Create(ctx, newTxn(s, "2026-10-12", -1)); err != nil {
			return err
		}
		bad := newTxn(s, "2026-10-12", -1)
		bad.AccountID = ptrUUID(uuid.New()) // foreign key violation
		_, err := repo.Create(ctx, bad)
		return err
	})

	require.Error(t, err)
	assert.Zero(t, countTxns(t, repo, s.budgetID))
}

func TestDBTxn_PanicRollsBackAndPropagates(t *testing.T) {
	pool, repo, s := setup(t)

	assert.PanicsWithValue(t, "kaboom", func() {
		_ = NewDBTxn(pool).Do(context.Background(), func(ctx context.Context) error {
			if _, err := repo.Create(ctx, newTxn(s, "2026-10-12", -1)); err != nil {
				return err
			}
			panic("kaboom")
		})
	})
	assert.Zero(t, countTxns(t, repo, s.budgetID))
	assert.Eventually(t, func() bool { return pool.Stat().AcquiredConns() == 0 }, 2*time.Second, 10*time.Millisecond,
		"the panicking transaction's connection was released, not leaked with an open transaction")

	// the connection went back to the pool in a usable state
	assert.NoError(t, NewDBTxn(pool).Do(context.Background(), func(ctx context.Context) error {
		_, err := repo.Create(ctx, newTxn(s, "2026-10-12", -1))
		return err
	}))
}

func TestDBTxn_UncommittedWritesAreInvisibleOutsideAndVisibleInside(t *testing.T) {
	pool, repo, s := setup(t)
	outside := context.Background()

	err := NewDBTxn(pool).Do(outside, func(ctx context.Context) error {
		created, err := repo.Create(ctx, newTxn(s, "2026-10-12", -1))
		if err != nil {
			return err
		}
		// same transaction: read-your-writes
		if _, err := repo.GetById(ctx, s.budgetID, created[0].ID); err != nil {
			return err
		}
		// the outer ctx carries no transaction: another connection, nothing visible yet
		_, err = repo.GetById(outside, s.budgetID, created[0].ID)
		if !errors.Is(err, repository.ErrNotFound) {
			return errors.Join(errors.New("uncommitted row leaked outside its transaction"), err)
		}
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, 1, countTxns(t, repo, s.budgetID))
}

func TestDBTxn_NestedDoJoinsTheOuterTransaction(t *testing.T) {
	pool, repo, s := setup(t)
	runner := NewDBTxn(pool)
	var outerXID, innerXID int64

	err := runner.Do(context.Background(), func(ctx context.Context) error {
		if err := txFromContext(ctx).QueryRow(ctx, "SELECT txid_current()").Scan(&outerXID); err != nil {
			return err
		}
		return runner.Do(ctx, func(ctx context.Context) error {
			_, err := repo.Create(ctx, newTxn(s, "2026-10-12", -1))
			if err != nil {
				return err
			}
			return txFromContext(ctx).QueryRow(ctx, "SELECT txid_current()").Scan(&innerXID)
		})
	})

	require.NoError(t, err)
	assert.Equal(t, outerXID, innerXID, "a nested Do must not open a second transaction")
}

func TestDBTxn_NestedFailureRollsBackTheWholeUnit(t *testing.T) {
	pool, repo, s := setup(t)
	runner := NewDBTxn(pool)
	boom := errors.New("inner failed")

	err := runner.Do(context.Background(), func(ctx context.Context) error {
		if _, err := repo.Create(ctx, newTxn(s, "2026-10-12", -1)); err != nil {
			return err
		}
		return runner.Do(ctx, func(ctx context.Context) error {
			if _, err := repo.Create(ctx, newTxn(s, "2026-10-12", -2)); err != nil {
				return err
			}
			return boom
		})
	})

	require.ErrorIs(t, err, boom)
	assert.Zero(t, countTxns(t, repo, s.budgetID), "the outer write is rolled back with the inner one")
}

func TestDBTxn_SwallowedInnerErrorStillCommits(t *testing.T) {
	// Joining means an inner Do cannot roll back on its own: if the caller
	// swallows the inner error, the outer transaction commits what both wrote.
	pool, repo, s := setup(t)
	runner := NewDBTxn(pool)

	err := runner.Do(context.Background(), func(ctx context.Context) error {
		_ = runner.Do(ctx, func(ctx context.Context) error {
			if _, err := repo.Create(ctx, newTxn(s, "2026-10-12", -1)); err != nil {
				return err
			}
			return errors.New("ignored by the caller")
		})
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, 1, countTxns(t, repo, s.budgetID))
}

func TestDBTxn_BeginFailsOnCancelledContext(t *testing.T) {
	pool := testPool(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false

	err := NewDBTxn(pool).Do(ctx, func(context.Context) error { called = true; return nil })

	assert.ErrorIs(t, err, context.Canceled)
	assert.False(t, called)
}

func TestDBTxn_ConcurrentUnitsAreIndependent(t *testing.T) {
	pool, repo, s := setup(t)
	runner := NewDBTxn(pool)
	var wg sync.WaitGroup
	errs := make([]error, 8)

	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = runner.Do(context.Background(), func(ctx context.Context) error {
				if _, err := repo.Create(ctx, newTxn(s, "2026-10-12", float64(-i))); err != nil {
					return err
				}
				if i%2 == 1 {
					return errors.New("odd units roll back")
				}
				return nil
			})
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if i%2 == 1 {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
	}
	assert.Equal(t, 4, countTxns(t, repo, s.budgetID), "only the even units committed")
}

func TestDBTxn_WithoutDoEveryCallAutocommits(t *testing.T) {
	_, repo, s := setup(t)
	ctx := context.Background()

	mustCreate(t, repo, newTxn(s, "2026-10-12", -1))
	bad := newTxn(s, "2026-10-12", -1)
	bad.AccountID = ptrUUID(uuid.New())
	_, err := repo.Create(ctx, bad)

	require.Error(t, err)
	assert.Equal(t, 1, countTxns(t, repo, s.budgetID), "the earlier write was already committed")
}

func TestInTx_MergeIsAtomicAndJoinsTheCaller(t *testing.T) {
	pool, repo, s := setup(t)
	ctx := context.Background()
	payees := NewPayeesRepository(pool)
	txn := mustCreate(t, repo, newTxn(s, "2026-10-12", -1))

	t.Run("standalone merge commits", func(t *testing.T) {
		res, err := payees.Merge(ctx, s.budgetID, s.payee2ID, []uuid.UUID{s.payeeID})
		require.NoError(t, err)
		assert.Equal(t, int64(1), res.Transactions)
		got, err := repo.GetById(ctx, s.budgetID, txn.ID)
		require.NoError(t, err)
		assert.Equal(t, s.payee2ID, *got.PayeeID)
	})

	t.Run("merge inside a failed unit rolls back", func(t *testing.T) {
		s2 := newSeed(t, pool)
		moved := mustCreate(t, repo, newTxn(s2, "2026-10-12", -1))
		err := NewDBTxn(pool).Do(ctx, func(ctx context.Context) error {
			if _, err := payees.Merge(ctx, s2.budgetID, s2.payee2ID, []uuid.UUID{s2.payeeID}); err != nil {
				return err
			}
			return errors.New("caller fails after the merge")
		})
		require.Error(t, err)
		got, err := repo.GetById(ctx, s2.budgetID, moved.ID)
		require.NoError(t, err)
		assert.Equal(t, s2.payeeID, *got.PayeeID, "the merge joined the caller's transaction and was undone with it")
	})
}

func TestBulkInsert_JoinsTheTransaction(t *testing.T) {
	pool, repo, s := setup(t)
	bulk := NewBulkInserter(pool)
	columns := []string{"budget_id", "date", "account_id", "amount"}
	rows := [][]any{
		{s.budgetID, "2026-10-12", s.accountID, -1.0},
		{s.budgetID, "2026-10-13", s.accountID, -2.0},
	}

	err := NewDBTxn(pool).Do(context.Background(), func(ctx context.Context) error {
		n, err := bulk.BulkInsert(ctx, "transactions", columns, rows)
		if err != nil {
			return err
		}
		if n != 2 {
			return errors.New("expected 2 rows copied")
		}
		return errors.New("roll back the copy")
	})
	require.Error(t, err)
	assert.Zero(t, countTxns(t, repo, s.budgetID))

	n, err := bulk.BulkInsert(context.Background(), "transactions", columns, rows)
	require.NoError(t, err)
	assert.Equal(t, int64(2), n)
	all, err := repo.GetAll(context.Background(), s.budgetID, nil)
	require.NoError(t, err)
	require.Len(t, all, 2)
	assert.Equal(t, model.TransactionStatusManual, all[0].Status, "the column default applies to copied rows")
}

func TestExecutor_MapsNoRowsToErrNotFound(t *testing.T) {
	pool := testPool(t)
	base := NewBaseRepository(pool)
	ctx := context.Background()

	var n int
	err := base.Executor(ctx).QueryRow(ctx, "SELECT 1 WHERE false").Scan(&n)
	assert.ErrorIs(t, err, repository.ErrNotFound)
	assert.ErrorIs(t, err, pgx.ErrNoRows)

	err = base.Executor(ctx).QueryRow(ctx, "SELECT 1/0").Scan(&n)
	require.Error(t, err)
	assert.NotErrorIs(t, err, repository.ErrNotFound, "other errors are not dressed up as not-found")

	require.NoError(t, base.Executor(ctx).QueryRow(ctx, "SELECT 7").Scan(&n))
	assert.Equal(t, 7, n)
}
