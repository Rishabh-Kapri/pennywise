package postgres

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strPtr(s string) *string { return &s }

func newTxn(s seed, date string, amount float64) model.Transaction {
	return model.Transaction{
		BudgetID:   s.budgetID,
		Date:       model.Date(date),
		AccountID:  &s.accountID,
		PayeeID:    &s.payeeID,
		CategoryID: &s.categoryID,
		Amount:     amount,
		Status:     model.TransactionStatusManual,
	}
}

func mustCreate(t *testing.T, repo repository.TransactionRepository, txn model.Transaction) model.Transaction {
	t.Helper()
	created, err := repo.Create(context.Background(), txn)
	require.NoError(t, err)
	require.Len(t, created, 1)
	return created[0]
}

func ids(txns []model.Transaction) []uuid.UUID {
	out := make([]uuid.UUID, len(txns))
	for i, txn := range txns {
		out[i] = txn.ID
	}
	return out
}

func setup(t *testing.T) (*pgxpool.Pool, repository.TransactionRepository, seed) {
	t.Helper()
	pool := testPool(t)
	return pool, NewTransactionRepository(pool), newSeed(t, pool)
}

// ─── Create / GetById ────────────────────────────────────────────────────────

func TestTransactionRepo_CreateAndGetByIdRoundTripsEveryField(t *testing.T) {
	_, repo, s := setup(t)
	ctx := context.Background()
	tag1, tag2 := uuid.New(), uuid.New()
	lat, lng := 12.9716, 77.5946
	src := model.LocationSourceAuto
	in := newTxn(s, "2026-10-12", -249.5)
	in.Note = "flat white"
	in.Status = model.TransactionStatusUnapproved
	in.DedupeHash = strPtr("hash-roundtrip")
	in.RawBankText = strPtr("Rs 249.50 debited at BLUE TOKAI")
	in.Summary = strPtr("Coffee")
	in.TagIDs = []uuid.UUID{tag1, tag2}
	in.LocationLat, in.LocationLng, in.LocationName, in.LocationSource = &lat, &lng, strPtr("Indiranagar"), &src

	created := mustCreate(t, repo, in)
	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, -249.5, created.Amount)
	assert.Equal(t, model.TransactionStatusUnapproved, created.Status)
	assert.Equal(t, "Coffee", *created.Summary)

	got, err := repo.GetById(ctx, s.budgetID, created.ID)
	require.NoError(t, err)
	assert.Equal(t, s.budgetID, got.BudgetID)
	assert.Equal(t, model.Date("2026-10-12"), got.Date)
	assert.Equal(t, -249.5, got.Amount)
	assert.Equal(t, "flat white", got.Note)
	assert.Equal(t, model.TransactionStatusUnapproved, got.Status)
	assert.Equal(t, in.RawBankText, got.RawBankText)
	assert.Equal(t, in.Summary, got.Summary)
	assert.ElementsMatch(t, []uuid.UUID{tag1, tag2}, got.TagIDs)
	assert.Equal(t, &lat, got.LocationLat)
	assert.Equal(t, &lng, got.LocationLng)
	assert.Equal(t, "Indiranagar", *got.LocationName)
	assert.Equal(t, model.LocationSourceAuto, *got.LocationSource)
	assert.Equal(t, "HDFC Savings", *got.AccountName)
	assert.Equal(t, "Blue Tokai", *got.PayeeName)
	assert.Equal(t, "Eating Out", *got.CategoryName)
	assert.False(t, got.CreatedAt.IsZero())
}

func TestTransactionRepo_AmountKeepsTwoDecimals(t *testing.T) {
	_, repo, s := setup(t)
	created := mustCreate(t, repo, newTxn(s, "2026-10-12", 1024.505))
	got, err := repo.GetById(context.Background(), s.budgetID, created.ID)
	require.NoError(t, err)
	assert.Equal(t, 1024.51, got.Amount, "amount is NUMERIC(12,2): values round to paise")
}

func TestTransactionRepo_CreateWithoutCategoryOrPayee(t *testing.T) {
	_, repo, s := setup(t)
	in := newTxn(s, "2026-10-12", 5000)
	in.CategoryID, in.PayeeID = nil, nil
	created := mustCreate(t, repo, in)

	got, err := repo.GetById(context.Background(), s.budgetID, created.ID)
	require.NoError(t, err)
	assert.Nil(t, got.CategoryID)
	assert.Nil(t, got.CategoryName)
	assert.Nil(t, got.PayeeName)
}

// note is nullable; rows written outside Create (COPY, imports, legacy data)
// can hold NULL, and one such row must not break reading the budget.
func TestTransactionRepo_NullNoteReadsAsEmpty(t *testing.T) {
	pool, repo, s := setup(t)
	ctx := context.Background()
	id := uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO transactions (id, budget_id, date, account_id, amount, note)
		VALUES ($1, $2, '2026-10-12', $3, -1, NULL)`, id, s.budgetID, s.accountID)
	require.NoError(t, err)

	got, err := repo.GetById(ctx, s.budgetID, id)
	require.NoError(t, err)
	assert.Equal(t, "", got.Note)

	all, err := repo.GetAll(ctx, s.budgetID, nil)
	require.NoError(t, err)
	assert.Len(t, all, 1)

	page, err := repo.GetAllNormalized(ctx, s.budgetID, &model.TransactionFilter{})
	require.NoError(t, err)
	assert.Len(t, page.Data, 1)
}

// sortOrder reaches ORDER BY; it must be reduced to ASC/DESC, never spliced.
func TestTransactionRepo_GetAllNormalizedSanitisesSortOrder(t *testing.T) {
	_, repo, s := setup(t)
	ctx := context.Background()
	older := mustCreate(t, repo, newTxn(s, "2026-10-10", -1))
	newer := mustCreate(t, repo, newTxn(s, "2026-10-12", -1))

	for _, order := range []string{"", "desc", "DESC; DROP TABLE transactions; --", "sideways"} {
		page, err := repo.GetAllNormalized(ctx, s.budgetID, &model.TransactionFilter{SortOrder: order})
		require.NoError(t, err, "sortOrder %q", order)
		assert.Equal(t, []uuid.UUID{newer.ID, older.ID}, ids(page.Data), "sortOrder %q sorts newest first", order)
	}
	page, err := repo.GetAllNormalized(ctx, s.budgetID, &model.TransactionFilter{SortOrder: " asc "})
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{older.ID, newer.ID}, ids(page.Data))
}

func TestTransactionRepo_CreateRejectsForeignKeysAndBadStatus(t *testing.T) {
	_, repo, s := setup(t)
	ctx := context.Background()

	missingAccount := newTxn(s, "2026-10-12", -1)
	missingAccount.AccountID = ptrUUID(uuid.New())
	_, err := repo.Create(ctx, missingAccount)
	assert.Error(t, err, "account_id must reference an account")

	badStatus := newTxn(s, "2026-10-12", -1)
	badStatus.Status = "PENDING"
	_, err = repo.Create(ctx, badStatus)
	assert.Error(t, err, "status is an enum")
}

func ptrUUID(id uuid.UUID) *uuid.UUID { return &id }

func TestTransactionRepo_GetByIdNotFoundCases(t *testing.T) {
	pool, repo, s := setup(t)
	ctx := context.Background()
	other := newSeed(t, pool)
	created := mustCreate(t, repo, newTxn(s, "2026-10-12", -10))

	cases := map[string]struct{ budget, id uuid.UUID }{
		"unknown id":          {s.budgetID, uuid.New()},
		"another budget's id": {other.budgetID, created.ID},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := repo.GetById(ctx, c.budget, c.id)
			assert.Nil(t, got)
			assert.ErrorIs(t, err, repository.ErrNotFound)
			assert.ErrorIs(t, err, pgx.ErrNoRows, "the pgx sentinel still matches during migration")
		})
	}

	t.Run("soft-deleted", func(t *testing.T) {
		require.NoError(t, repo.DeleteById(ctx, s.budgetID, created.ID))
		_, err := repo.GetById(ctx, s.budgetID, created.ID)
		assert.ErrorIs(t, err, repository.ErrNotFound)
	})
}

// ─── GetAll ──────────────────────────────────────────────────────────────────

func TestTransactionRepo_GetAllScopesFiltersAndOrders(t *testing.T) {
	pool, repo, s := setup(t)
	ctx := context.Background()
	other := newSeed(t, pool)

	oct10 := mustCreate(t, repo, newTxn(s, "2026-10-10", -10))
	oct12 := mustCreate(t, repo, newTxn(s, "2026-10-12", -20))
	cardTxn := newTxn(s, "2026-10-11", -30)
	cardTxn.AccountID = &s.account2ID
	oct11card := mustCreate(t, repo, cardTxn)
	deleted := mustCreate(t, repo, newTxn(s, "2026-10-13", -40))
	require.NoError(t, repo.DeleteById(ctx, s.budgetID, deleted.ID))
	mustCreate(t, repo, newTxn(other, "2026-10-12", -50))

	t.Run("budget scoped, live rows, newest first", func(t *testing.T) {
		all, err := repo.GetAll(ctx, s.budgetID, nil)
		require.NoError(t, err)
		assert.Equal(t, []uuid.UUID{oct12.ID, oct11card.ID, oct10.ID}, ids(all))
	})

	t.Run("account filter", func(t *testing.T) {
		got, err := repo.GetAll(ctx, s.budgetID, &model.TransactionFilter{AccountIDs: []uuid.UUID{s.account2ID}})
		require.NoError(t, err)
		assert.Equal(t, []uuid.UUID{oct11card.ID}, ids(got))
	})

	t.Run("inclusive date range", func(t *testing.T) {
		got, err := repo.GetAll(ctx, s.budgetID, &model.TransactionFilter{
			StartDate: strPtr("2026-10-11"), EndDate: strPtr("2026-10-12"),
		})
		require.NoError(t, err)
		assert.Equal(t, []uuid.UUID{oct12.ID, oct11card.ID}, ids(got))
	})

	t.Run("all filters combined", func(t *testing.T) {
		got, err := repo.GetAll(ctx, s.budgetID, &model.TransactionFilter{
			AccountIDs: []uuid.UUID{s.accountID}, StartDate: strPtr("2026-10-11"), EndDate: strPtr("2026-10-30"),
		})
		require.NoError(t, err)
		assert.Equal(t, []uuid.UUID{oct12.ID}, ids(got))
	})

	t.Run("empty budget", func(t *testing.T) {
		got, err := repo.GetAll(ctx, uuid.New(), nil)
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

// ─── GetAllNormalized ────────────────────────────────────────────────────────

func TestTransactionRepo_GetAllNormalizedWithoutFilter(t *testing.T) {
	_, repo, s := setup(t)
	mustCreate(t, repo, newTxn(s, "2026-10-12", -20))

	page, err := repo.GetAllNormalized(context.Background(), s.budgetID, nil)

	require.NoError(t, err)
	assert.Equal(t, 1, page.Total)
	assert.Len(t, page.Data, 1)
}

func TestTransactionRepo_GetAllNormalizedFiltersAndDerivedColumns(t *testing.T) {
	_, repo, s := setup(t)
	ctx := context.Background()
	tag := uuid.New()

	coffee := newTxn(s, "2026-10-10", -200)
	coffee.Note = "Morning COFFEE run"
	coffee.TagIDs = []uuid.UUID{tag}
	coffee = mustCreate(t, repo, coffee)

	salary := newTxn(s, "2026-10-11", 50000)
	salary.PayeeID, salary.CategoryID = &s.payee2ID, &s.category2
	salary = mustCreate(t, repo, salary)

	card := newTxn(s, "2026-10-12", -300)
	card.AccountID = &s.account2ID
	card.TagIDs = []uuid.UUID{uuid.New()}
	card = mustCreate(t, repo, card)

	query := func(f *model.TransactionFilter) model.PaginatedResponse[model.Transaction] {
		t.Helper()
		if f.SortOrder == "" {
			f.SortOrder = "DESC"
		}
		page, err := repo.GetAllNormalized(ctx, s.budgetID, f)
		require.NoError(t, err)
		return page
	}

	t.Run("inflow and outflow split", func(t *testing.T) {
		page := query(&model.TransactionFilter{})
		require.Equal(t, 3, page.Total)
		byID := map[uuid.UUID]model.Transaction{}
		for _, txn := range page.Data {
			byID[txn.ID] = txn
		}
		assert.Equal(t, [2]float64{0, 200}, [2]float64{byID[coffee.ID].Inflow, byID[coffee.ID].Outflow})
		assert.Equal(t, [2]float64{50000, 0}, [2]float64{byID[salary.ID].Inflow, byID[salary.ID].Outflow})
		assert.Equal(t, "Blue Tokai", *byID[coffee.ID].PayeeName)
		assert.Zero(t, byID[coffee.ID].Balance, "balance is only computed for an account filter")
	})

	filters := map[string]struct {
		filter *model.TransactionFilter
		want   []uuid.UUID
	}{
		"category":                         {&model.TransactionFilter{CategoryIDs: []uuid.UUID{s.category2}}, []uuid.UUID{salary.ID}},
		"payee":                            {&model.TransactionFilter{PayeeIDs: []uuid.UUID{s.payeeID}}, []uuid.UUID{card.ID, coffee.ID}},
		"tags overlap":                     {&model.TransactionFilter{TagIDs: []uuid.UUID{tag, uuid.New()}}, []uuid.UUID{coffee.ID}},
		"note, case-insensitive substring": {&model.TransactionFilter{Note: strPtr("coffee")}, []uuid.UUID{coffee.ID}},
		"date range":                       {&model.TransactionFilter{StartDate: strPtr("2026-10-11"), EndDate: strPtr("2026-10-11")}, []uuid.UUID{salary.ID}},
		"ascending":                        {&model.TransactionFilter{SortOrder: "ASC"}, []uuid.UUID{coffee.ID, salary.ID, card.ID}},
	}
	for name, tc := range filters {
		t.Run(name, func(t *testing.T) {
			page := query(tc.filter)
			assert.Equal(t, tc.want, ids(page.Data))
			assert.Equal(t, len(tc.want), page.Total, "total uses the same filters as the page")
		})
	}

	t.Run("running balance per account", func(t *testing.T) {
		page := query(&model.TransactionFilter{AccountIDs: []uuid.UUID{s.accountID}, SortOrder: "ASC"})
		require.Equal(t, []uuid.UUID{coffee.ID, salary.ID}, ids(page.Data))
		assert.Equal(t, -200.0, page.Data[0].Balance)
		assert.Equal(t, 49800.0, page.Data[1].Balance)
	})

	t.Run("bad cursor", func(t *testing.T) {
		_, err := repo.GetAllNormalized(ctx, s.budgetID, &model.TransactionFilter{CursorString: "%%%not-base64"})
		assert.Error(t, err)
	})
}

// Rows written in one database transaction share now(), so (date, updated_at)
// ties are normal: both legs of a transfer, a bulk import, the demo seed.
// Paging must still visit every row exactly once, in both directions.
func TestTransactionRepo_GetAllNormalizedPagesThroughTiesWithoutGapsOrRepeats(t *testing.T) {
	pool, repo, s := setup(t)
	ctx := context.Background()

	var want []uuid.UUID
	require.NoError(t, NewDBTxn(pool).Do(ctx, func(ctx context.Context) error {
		for i := 0; i < 7; i++ {
			created, err := repo.Create(ctx, newTxn(s, "2026-10-12", float64(-i-1)))
			if err != nil {
				return err
			}
			want = append(want, created[0].ID)
		}
		return nil
	}))
	mustCreate(t, repo, newTxn(s, "2026-10-13", -100))
	mustCreate(t, repo, newTxn(s, "2026-10-11", -100))

	for _, order := range []string{"DESC", "ASC"} {
		t.Run(order, func(t *testing.T) {
			filter := &model.TransactionFilter{SortOrder: order, Limit: 2}
			var forward [][]uuid.UUID
			seen := map[uuid.UUID]int{}
			for page := 0; page < 20; page++ {
				res, err := repo.GetAllNormalized(ctx, s.budgetID, filter)
				require.NoError(t, err)
				assert.Equal(t, 9, res.Total)
				forward = append(forward, ids(res.Data))
				for _, id := range ids(res.Data) {
					seen[id]++
				}
				if res.Pagination.NextCursor == "" {
					break
				}
				filter = &model.TransactionFilter{SortOrder: order, Limit: 2, CursorString: res.Pagination.NextCursor}
			}
			assert.Len(t, seen, 9, "every row is visited")
			for id, n := range seen {
				assert.Equal(t, 1, n, "row %s visited %d times", id, n)
			}
			for _, id := range want {
				assert.Contains(t, seen, id)
			}

			// walk back from the last page using prev cursors
			last := len(forward) - 1
			res, err := repo.GetAllNormalized(ctx, s.budgetID, &model.TransactionFilter{SortOrder: order, Limit: 2})
			require.NoError(t, err)
			for i := 0; i < last; i++ {
				res, err = repo.GetAllNormalized(ctx, s.budgetID, &model.TransactionFilter{
					SortOrder: order, Limit: 2, CursorString: res.Pagination.NextCursor,
				})
				require.NoError(t, err)
			}
			for i := last; i > 0; i-- {
				require.NotEmpty(t, res.Pagination.PrevCursor, "page %d has a previous page", i)
				res, err = repo.GetAllNormalized(ctx, s.budgetID, &model.TransactionFilter{
					SortOrder: order, Limit: 2, CursorString: res.Pagination.PrevCursor,
				})
				require.NoError(t, err)
				assert.Equal(t, forward[i-1], ids(res.Data), "going back lands on page %d", i-1)
			}
		})
	}
}

// ─── CreateDeduped ───────────────────────────────────────────────────────────

func TestTransactionRepo_CreateDeduped(t *testing.T) {
	pool, repo, s := setup(t)
	ctx := context.Background()

	t.Run("first insert creates", func(t *testing.T) {
		in := newTxn(s, "2026-10-12", -99)
		in.DedupeHash = strPtr("hash-a")
		got, created, err := repo.CreateDeduped(ctx, in)
		require.NoError(t, err)
		assert.True(t, created)
		assert.NotEqual(t, uuid.Nil, got.ID)
	})

	t.Run("same hash returns the existing row", func(t *testing.T) {
		in := newTxn(s, "2026-10-12", -99)
		in.DedupeHash = strPtr("hash-b")
		first, created, err := repo.CreateDeduped(ctx, in)
		require.NoError(t, err)
		require.True(t, created)

		in.Amount = -1 // a retry with drifted fields still maps onto the first row
		again, created, err := repo.CreateDeduped(ctx, in)
		require.NoError(t, err)
		assert.False(t, created)
		assert.Equal(t, first.ID, again.ID)
		assert.Equal(t, -99.0, again.Amount)

		all, err := repo.GetAll(ctx, s.budgetID, nil)
		require.NoError(t, err)
		n := 0
		for _, txn := range all {
			if txn.ID == first.ID {
				n++
			}
		}
		assert.Equal(t, 1, n)
	})

	t.Run("no hash always inserts", func(t *testing.T) {
		in := newTxn(s, "2026-10-12", -5)
		a, createdA, err := repo.CreateDeduped(ctx, in)
		require.NoError(t, err)
		in.DedupeHash = strPtr("")
		b, createdB, err := repo.CreateDeduped(ctx, in)
		require.NoError(t, err)
		assert.True(t, createdA && createdB)
		assert.NotEqual(t, a.ID, b.ID)
	})

	t.Run("hash is per budget", func(t *testing.T) {
		other := newSeed(t, pool)
		in := newTxn(s, "2026-10-12", -7)
		in.DedupeHash = strPtr("hash-shared")
		_, created, err := repo.CreateDeduped(ctx, in)
		require.NoError(t, err)
		require.True(t, created)

		foreign := newTxn(other, "2026-10-12", -7)
		foreign.DedupeHash = strPtr("hash-shared")
		_, created, err = repo.CreateDeduped(ctx, foreign)
		require.NoError(t, err)
		assert.True(t, created)
	})

	t.Run("a deleted row frees its hash", func(t *testing.T) {
		in := newTxn(s, "2026-10-12", -8)
		in.DedupeHash = strPtr("hash-deleted")
		first, _, err := repo.CreateDeduped(ctx, in)
		require.NoError(t, err)
		require.NoError(t, repo.DeleteById(ctx, s.budgetID, first.ID))

		second, created, err := repo.CreateDeduped(ctx, in)
		require.NoError(t, err)
		assert.True(t, created)
		assert.NotEqual(t, first.ID, second.ID)
	})

	t.Run("plain Create on a taken hash violates the unique index", func(t *testing.T) {
		in := newTxn(s, "2026-10-12", -9)
		in.DedupeHash = strPtr("hash-unique")
		mustCreate(t, repo, in)
		_, err := repo.Create(ctx, in)
		assert.Error(t, err)
	})

	t.Run("a conflict inside a transaction leaves it usable", func(t *testing.T) {
		in := newTxn(s, "2026-10-12", -11)
		in.DedupeHash = strPtr("hash-in-tx")
		mustCreate(t, repo, in)

		var after model.Transaction
		err := NewDBTxn(pool).Do(ctx, func(ctx context.Context) error {
			if _, created, err := repo.CreateDeduped(ctx, in); err != nil || created {
				return errors.Join(err, errors.New("expected an existing row"))
			}
			created, err := repo.Create(ctx, newTxn(s, "2026-10-12", -12))
			if err != nil {
				return err
			}
			after = created[0]
			return nil
		})
		require.NoError(t, err, "ON CONFLICT DO NOTHING must not abort the transaction")
		_, err = repo.GetById(ctx, s.budgetID, after.ID)
		assert.NoError(t, err)
	})
}

// ─── Update / UpdateLocation / UpdateStatus / DeleteById ────────────────────

func TestTransactionRepo_UpdateRewritesFieldsAndBumpsUpdatedAt(t *testing.T) {
	_, repo, s := setup(t)
	ctx := context.Background()
	created := mustCreate(t, repo, newTxn(s, "2026-10-12", -10))
	before, err := repo.GetById(ctx, s.budgetID, created.ID)
	require.NoError(t, err)
	time.Sleep(5 * time.Millisecond)

	edit := *before
	edit.Date = "2026-10-14"
	edit.Amount = -42.25
	edit.Note = "edited"
	edit.PayeeID, edit.CategoryID, edit.AccountID = &s.payee2ID, &s.category2, &s.account2ID
	edit.Status = model.TransactionStatusApproved
	edit.TagIDs = []uuid.UUID{uuid.New()}
	require.NoError(t, repo.Update(ctx, s.budgetID, created.ID, edit))

	after, err := repo.GetById(ctx, s.budgetID, created.ID)
	require.NoError(t, err)
	assert.Equal(t, model.Date("2026-10-14"), after.Date)
	assert.Equal(t, -42.25, after.Amount)
	assert.Equal(t, "edited", after.Note)
	assert.Equal(t, "Swiggy", *after.PayeeName)
	assert.Equal(t, "Groceries", *after.CategoryName)
	assert.Equal(t, "HDFC Card", *after.AccountName)
	assert.Equal(t, model.TransactionStatusApproved, after.Status)
	assert.Equal(t, edit.TagIDs, after.TagIDs)
	assert.True(t, after.UpdatedAt.After(before.UpdatedAt))
	assert.Equal(t, before.CreatedAt, after.CreatedAt)
}

func TestTransactionRepo_LinksTransferLegs(t *testing.T) {
	_, repo, s := setup(t)
	ctx := context.Background()
	leg := mustCreate(t, repo, newTxn(s, "2026-10-12", -1000))
	counter := newTxn(s, "2026-10-12", 1000)
	counter.AccountID, counter.CategoryID = &s.account2ID, nil
	counter.TransferAccountID, counter.TransferTransactionID = &s.accountID, &leg.ID
	counterpart := mustCreate(t, repo, counter)

	got, err := repo.GetById(ctx, s.budgetID, leg.ID)
	require.NoError(t, err)
	got.TransferAccountID, got.TransferTransactionID = &s.account2ID, &counterpart.ID
	require.NoError(t, repo.Update(ctx, s.budgetID, leg.ID, *got))

	linked, err := repo.GetById(ctx, s.budgetID, leg.ID)
	require.NoError(t, err)
	assert.Equal(t, counterpart.ID, *linked.TransferTransactionID)
	assert.Equal(t, s.account2ID, *linked.TransferAccountID)
}

func TestTransactionRepo_UpdateLocationSetsAndClears(t *testing.T) {
	_, repo, s := setup(t)
	ctx := context.Background()
	created := mustCreate(t, repo, newTxn(s, "2026-10-12", -10))
	lat, lng, name := 12.97, 77.64, "Office"
	manual := model.LocationSourceManual

	require.NoError(t, repo.UpdateLocation(ctx, s.budgetID, created.ID, &lat, &lng, &name, &manual))
	got, err := repo.GetById(ctx, s.budgetID, created.ID)
	require.NoError(t, err)
	assert.Equal(t, lat, *got.LocationLat)
	assert.Equal(t, "Office", *got.LocationName)
	assert.Equal(t, model.LocationSourceManual, *got.LocationSource)
	assert.Equal(t, -10.0, got.Amount, "other fields are untouched")

	require.NoError(t, repo.UpdateLocation(ctx, s.budgetID, created.ID, nil, nil, nil, nil))
	got, err = repo.GetById(ctx, s.budgetID, created.ID)
	require.NoError(t, err)
	assert.Nil(t, got.LocationLat)
	assert.Nil(t, got.LocationLng)
	assert.Nil(t, got.LocationName)
	assert.Nil(t, got.LocationSource)
}

func TestTransactionRepo_UpdateStatus(t *testing.T) {
	_, repo, s := setup(t)
	ctx := context.Background()
	in := newTxn(s, "2026-10-12", -10)
	in.Status = model.TransactionStatusUnapproved
	created := mustCreate(t, repo, in)

	require.NoError(t, repo.UpdateStatus(ctx, s.budgetID, created.ID, model.TransactionStatusApproved))
	got, err := repo.GetById(ctx, s.budgetID, created.ID)
	require.NoError(t, err)
	assert.Equal(t, model.TransactionStatusApproved, got.Status)

	assert.Error(t, repo.UpdateStatus(ctx, s.budgetID, created.ID, "PENDING"), "status is an enum")
}

func TestTransactionRepo_DeleteByIdIsASoftDelete(t *testing.T) {
	pool, repo, s := setup(t)
	ctx := context.Background()
	created := mustCreate(t, repo, newTxn(s, "2026-10-12", -10))

	require.NoError(t, repo.DeleteById(ctx, s.budgetID, created.ID))

	var deleted bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT deleted FROM transactions WHERE id = $1`, created.ID).Scan(&deleted))
	assert.True(t, deleted, "the row is kept and flagged")
	all, err := repo.GetAll(ctx, s.budgetID, nil)
	require.NoError(t, err)
	assert.NotContains(t, ids(all), created.ID)
}

func TestTransactionRepo_WritesReportNotFoundForMissingRows(t *testing.T) {
	pool, repo, s := setup(t)
	ctx := context.Background()
	other := newSeed(t, pool)
	foreign := mustCreate(t, repo, newTxn(other, "2026-10-12", -10))
	lat, lng := 1.0, 2.0

	writes := map[string]func(id uuid.UUID) error{
		"Update": func(id uuid.UUID) error {
			return repo.Update(ctx, s.budgetID, id, newTxn(s, "2026-10-12", -1))
		},
		"UpdateLocation": func(id uuid.UUID) error {
			return repo.UpdateLocation(ctx, s.budgetID, id, &lat, &lng, nil, nil)
		},
		"UpdateStatus": func(id uuid.UUID) error {
			return repo.UpdateStatus(ctx, s.budgetID, id, model.TransactionStatusApproved)
		},
		"DeleteById": func(id uuid.UUID) error {
			return repo.DeleteById(ctx, s.budgetID, id)
		},
	}
	names := make([]string, 0, len(writes))
	for name := range writes {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		write := writes[name]
		t.Run(name+"/unknown id", func(t *testing.T) {
			assert.ErrorIs(t, write(uuid.New()), repository.ErrNotFound)
		})
		t.Run(name+"/another budget's row", func(t *testing.T) {
			assert.ErrorIs(t, write(foreign.ID), repository.ErrNotFound)
		})
	}

	untouched, err := repo.GetById(ctx, other.budgetID, foreign.ID)
	require.NoError(t, err, "a write scoped to one budget never reaches another")
	assert.Equal(t, -10.0, untouched.Amount)
	assert.Nil(t, untouched.LocationLat)
}
