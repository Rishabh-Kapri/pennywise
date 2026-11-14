package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/google/uuid"
)

func TestCounterpartyAliasesAndSources(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	aliases := NewCounterpartyAliasRepository(pool)
	sources := NewTransactionSourceRepository(pool)

	userID, budgetID, payeeID, otherPayee, categoryID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO auth_users (id) VALUES ($1)`, userID)
	exec(`INSERT INTO budgets (id, user_id, name) VALUES ($1, $2, 'B')`, budgetID, userID)
	exec(`INSERT INTO payees (id, budget_id, name) VALUES ($1, $3, 'Swiggy'), ($2, $3, 'Other')`, payeeID, otherPayee, budgetID)
	exec(`INSERT INTO categories (id, budget_id, name) VALUES ($1, $2, 'Food')`, categoryID, budgetID)

	if m, err := aliases.FindMatch(ctx, budgetID, "UPI", "SWIGGY"); err != nil || m != nil {
		t.Fatalf("unknown key: %+v, %v", m, err)
	}

	// No category yet: known, but not usable for a transaction.
	if err := aliases.Learn(ctx, model.CounterpartyAliasLearn{
		BudgetID: budgetID, Channel: "UPI", Merchant: "SWIGGY", PayeeID: payeeID,
	}); err != nil {
		t.Fatal(err)
	}
	if m, err := aliases.FindMatch(ctx, budgetID, "UPI", "SWIGGY"); err != nil || m != nil {
		t.Fatalf("alias without a category: %+v, %v", m, err)
	}

	// Learning again fills the category, keeps the payee and counts the hit.
	if err := aliases.Learn(ctx, model.CounterpartyAliasLearn{
		BudgetID: budgetID, Channel: "UPI", Merchant: "SWIGGY", PayeeID: otherPayee, CategoryID: &categoryID,
	}); err != nil {
		t.Fatal(err)
	}
	m, err := aliases.FindMatch(ctx, budgetID, "UPI", "SWIGGY")
	if err != nil || m == nil || m.PayeeName != "Swiggy" || m.CategoryName != "Food" {
		t.Fatalf("match = %+v, %v", m, err)
	}
	var hits int
	if err := pool.QueryRow(ctx, `SELECT hit_count FROM counterparty_aliases WHERE budget_id = $1`, budgetID).Scan(&hits); err != nil || hits != 2 {
		t.Fatalf("hits = %d, %v", hits, err)
	}

	// A deleted category makes it a miss.
	exec(`UPDATE categories SET deleted = TRUE WHERE id = $1`, categoryID)
	if m, err := aliases.FindMatch(ctx, budgetID, "UPI", "SWIGGY"); err != nil || m != nil {
		t.Fatalf("deleted category: %+v, %v", m, err)
	}

	// Sources: blank fields become NULL, and a repeat changes nothing.
	accountID, txnID := uuid.New(), uuid.New()
	exec(`INSERT INTO accounts (id, budget_id, name, type) VALUES ($1, $2, 'A', 'checking')`, accountID, budgetID)
	exec(`INSERT INTO transactions (id, budget_id, account_id, amount, date) VALUES ($1, $2, $3, -10, '2026-11-08')`, txnID, budgetID, accountID)
	source := model.MessageSource{Kind: "EMAIL", Text: "first", Channel: "UPI", Merchant: "SWIGGY", Amount: "10.50"}
	if added, err := sources.RecordFromPipeline(ctx, budgetID, txnID, source); err != nil || !added {
		t.Fatalf("first record: %v, %v", added, err)
	}
	source.Text = "second"
	if added, err := sources.RecordFromPipeline(ctx, budgetID, txnID, source); err != nil || added {
		t.Fatalf("second record: %v, %v", added, err)
	}
	key, err := sources.LookupKey(ctx, budgetID, txnID)
	if err != nil || key == nil || key.Channel != "UPI" || key.Merchant != "SWIGGY" {
		t.Fatalf("key = %+v, %v", key, err)
	}
	if none, err := sources.LookupKey(ctx, budgetID, uuid.New()); err != nil || none != nil {
		t.Fatalf("no source: %+v, %v", none, err)
	}

	// Listing returns the source with its parsed fields; blanks stay nil.
	listed, err := sources.ListByTransaction(ctx, budgetID, txnID)
	if err != nil || len(listed) != 1 {
		t.Fatalf("listed = %+v, %v", listed, err)
	}
	if l := listed[0]; l.Kind != "EMAIL" || l.Origin != "PIPELINE" || l.RawText != "first" ||
		l.Channel == nil || *l.Channel != "UPI" || l.Amount == nil || *l.Amount != 10.5 ||
		l.Ref != nil || l.OccurredAt != nil || l.OccurredOn != nil {
		t.Fatalf("listed source = %+v", l)
	}
	// Another budget sees nothing, and a transaction made by hand gets an empty list.
	if other, err := sources.ListByTransaction(ctx, uuid.New(), txnID); err != nil || len(other) != 0 {
		t.Fatalf("other budget: %+v, %v", other, err)
	}
	// The time of day and date a source stated come back.
	at := time.Date(2026, 11, 8, 14, 35, 0, 0, time.FixedZone("IST", 5*3600+1800))
	on := time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC)
	smsTxn := uuid.New()
	exec(`INSERT INTO transactions (id, budget_id, account_id, amount, date) VALUES ($1, $2, $3, -10, '2026-11-08')`, smsTxn, budgetID, accountID)
	if _, err := sources.RecordFromPipeline(ctx, budgetID, smsTxn, model.MessageSource{
		Kind: "SMS", Text: "sms", OccurredOn: &on, OccurredAt: &at,
	}); err != nil {
		t.Fatal(err)
	}
	if got, err := sources.ListByTransaction(ctx, budgetID, smsTxn); err != nil || len(got) != 1 ||
		got[0].OccurredOn == nil || *got[0].OccurredOn != "2026-11-08" ||
		got[0].OccurredAt == nil || !got[0].OccurredAt.Equal(at) {
		t.Fatalf("timed source = %+v, %v", got, err)
	}

	// The user's word replaces the payee and category.
	exec(`UPDATE categories SET deleted = FALSE WHERE id = $1`, categoryID)
	if err := aliases.Confirm(ctx, model.CounterpartyAliasConfirm{
		BudgetID: budgetID, Channel: "UPI", Merchant: "SWIGGY", PayeeID: otherPayee, CategoryID: &categoryID,
	}); err != nil {
		t.Fatal(err)
	}
	if m, err := aliases.FindMatch(ctx, budgetID, "UPI", "SWIGGY"); err != nil || m == nil || m.PayeeName != "Other" {
		t.Fatalf("after confirm: %+v, %v", m, err)
	}
	// Learning afterwards can't undo it.
	if err := aliases.Learn(ctx, model.CounterpartyAliasLearn{
		BudgetID: budgetID, Channel: "UPI", Merchant: "SWIGGY", PayeeID: payeeID, CategoryID: &categoryID,
	}); err != nil {
		t.Fatal(err)
	}
	if m, _ := aliases.FindMatch(ctx, budgetID, "UPI", "SWIGGY"); m == nil || m.PayeeName != "Other" {
		t.Fatalf("learn overwrote a confirmed alias: %+v", m)
	}

	var text, origin string
	var ref *string
	if err := pool.QueryRow(ctx,
		`SELECT raw_text, origin, ref FROM transaction_sources WHERE transaction_id = $1`, txnID,
	).Scan(&text, &origin, &ref); err != nil {
		t.Fatal(err)
	}
	if text != "first" || origin != "PIPELINE" || ref != nil {
		t.Fatalf("source = %q, %q, %v", text, origin, ref)
	}
}
