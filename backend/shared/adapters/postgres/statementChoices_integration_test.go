package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/google/uuid"
)

func TestPastStatementChoices(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	sources := NewTransactionSourceRepository(pool)

	userID, budgetID, accountID := uuid.New(), uuid.New(), uuid.New()
	swiggy, cashbacks := uuid.New(), uuid.New()
	older, newer, deleted := uuid.New(), uuid.New(), uuid.New()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO auth_users (id) VALUES ($1)`, userID)
	exec(`INSERT INTO budgets (id, user_id, name) VALUES ($1, $2, 'B')`, budgetID, userID)
	exec(`INSERT INTO accounts (id, budget_id, name, type) VALUES ($1, $2, 'Card', 'creditCard')`, accountID, budgetID)
	exec(`INSERT INTO payees (id, budget_id, name) VALUES ($1, $2, 'Swiggy')`, swiggy, budgetID)
	exec(`INSERT INTO categories (id, budget_id, name) VALUES ($1, $2, 'Cashbacks')`, cashbacks, budgetID)
	exec(`INSERT INTO transactions (id, budget_id, account_id, amount, date, payee_id, category_id) VALUES
		($1, $4, $5, 30, '2026-09-29', $6, NULL),
		($2, $4, $5, 36.7, '2026-10-29', $6, $7),
		($3, $4, $5, 40, '2026-11-01', $6, NULL)`, older, newer, deleted, budgetID, accountID, swiggy, cashbacks)
	exec(`UPDATE transactions SET deleted = TRUE WHERE id = $1`, deleted)
	for _, id := range []uuid.UUID{older, newer, deleted} {
		exec(`INSERT INTO transaction_sources (budget_id, transaction_id, kind, raw_text, origin)
			VALUES ($1, $2, 'STATEMENT_LINE', '10% Swiggy CashBack', 'PIPELINE')`, budgetID, id)
	}

	key := model.StatementTextKey("10% Swiggy CashBack")
	got, err := sources.PastStatementChoices(ctx, budgetID, []string{key, "unknownline"})
	if err != nil {
		t.Fatal(err)
	}
	c, ok := got[key]
	if !ok || c.PayeeName != "Swiggy" || c.CategoryID == nil || *c.CategoryID != cashbacks {
		t.Fatalf("choice = %+v (found %v): want the newest live transaction's Swiggy / Cashbacks", c, ok)
	}
	if _, ok := got["unknownline"]; ok {
		t.Fatal("a line never seen has no choice")
	}
}

func TestReplaceStatementLinesKeepsTime(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	sources := NewTransactionSourceRepository(pool)

	userID, budgetID, accountID, importID, txnID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO auth_users (id) VALUES ($1)`, userID)
	exec(`INSERT INTO budgets (id, user_id, name) VALUES ($1, $2, 'B')`, budgetID, userID)
	exec(`INSERT INTO accounts (id, budget_id, name, type) VALUES ($1, $2, 'Card', 'creditCard')`, accountID, budgetID)
	exec(`INSERT INTO statement_imports (id, budget_id, file_name, file_hash) VALUES ($1, $2, 's.pdf', 'h')`, importID, budgetID)
	exec(`INSERT INTO transactions (id, budget_id, account_id, amount, date) VALUES ($1, $2, $3, -100, '2026-08-06')`,
		txnID, budgetID, accountID)

	at := time.Date(2026, 8, 6, 15, 19, 0, 0, time.UTC)
	err := sources.ReplaceStatementLines(ctx, budgetID, importID, []model.StatementLineSource{{
		TransactionID: txnID, RawText: "ACME", Direction: "DEBIT", Amount: 100, OccurredOn: "2026-08-06", OccurredAt: &at,
	}})
	if err != nil {
		t.Fatal(err)
	}
	// The transactions list shows a transaction's time from its first source
	// that has one.
	var got *time.Time
	if err := pool.QueryRow(ctx, `SELECT occurred_at FROM transaction_sources
		WHERE transaction_id = $1 AND kind = 'STATEMENT_LINE'`, txnID).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got == nil || !got.Equal(at) {
		t.Fatalf("occurred_at = %v, want %v from the statement line", got, at)
	}
}
