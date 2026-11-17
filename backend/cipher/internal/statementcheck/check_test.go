package statementcheck

import (
	"testing"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

func f(v float64) *float64 { return &v }

func row(amount float64, dir sharedModel.StatementDirection, balance *float64) sharedModel.StatementRow {
	return sharedModel.StatementRow{Amount: amount, Direction: dir, Balance: balance}
}

func res(sections ...sharedModel.StatementSection) sharedModel.StatementExtractResponse {
	return sharedModel.StatementExtractResponse{Sections: sections}
}

const (
	debit  = sharedModel.StatementDebit
	credit = sharedModel.StatementCredit
)

func TestBankRowsThatAddUp(t *testing.T) {
	r := Check(res(sharedModel.StatementSection{
		OpeningBalance: f(1000), ClosingBalance: f(1300),
		Rows: []sharedModel.StatementRow{row(200, debit, f(800)), row(500, credit, f(1300))},
	}))
	if !r.OK() || r.GoodRows != 2 {
		t.Fatalf("want OK with 2 good rows, got %+v", r)
	}
}

func TestCardTotalDueStandsInForClosing(t *testing.T) {
	// Owed 1000, spent 300, paid 1000: 300 due.
	r := Check(res(sharedModel.StatementSection{
		Kind: sharedModel.StatementKindCard, OpeningBalance: f(1000), TotalDue: f(300),
		Rows: []sharedModel.StatementRow{row(300, debit, nil), row(1000, credit, nil)},
	}))
	if !r.OK() {
		t.Fatalf("want OK, got %+v", r)
	}
}

func TestRowsThatDontAddUp(t *testing.T) {
	r := Check(res(sharedModel.StatementSection{
		OpeningBalance: f(1000), ClosingBalance: f(1300),
		Rows: []sharedModel.StatementRow{row(200, debit, nil)}, // a missed credit
	}))
	if r.OK() || r.Mismatched != 1 {
		t.Fatalf("want a mismatch, got %+v", r)
	}
}

func TestRunningBalanceAloneVerifies(t *testing.T) {
	r := Check(res(sharedModel.StatementSection{
		Rows: []sharedModel.StatementRow{row(0, credit, f(500)), row(100, debit, f(400)), row(50, debit, f(350))},
	}))
	if !r.OK() {
		t.Fatalf("want OK, got %+v", r)
	}

	r = Check(res(sharedModel.StatementSection{
		Rows: []sharedModel.StatementRow{row(0, credit, f(500)), row(100, debit, f(400)), row(50, credit, f(350))},
	}))
	if r.OK() || r.BalanceBreaks != 1 {
		t.Fatalf("want one break, got %+v", r)
	}
}

func TestNothingToCheckAgainstIsNotOK(t *testing.T) {
	r := Check(res(sharedModel.StatementSection{Rows: []sharedModel.StatementRow{row(100, debit, nil)}}))
	if r.OK() || r.Verified != 0 {
		t.Fatalf("want not OK, got %+v", r)
	}
	if Check(res()).OK() {
		t.Fatal("no rows: want not OK")
	}
}

func TestBetter(t *testing.T) {
	ok := Result{Rows: 2, Sections: 1, Verified: 1, GoodRows: 2}
	unchecked := Result{Rows: 5, Sections: 1}
	mismatch := Result{Rows: 5, Sections: 1, Mismatched: 1}
	empty := Result{}
	if !Better(ok, unchecked) || Better(unchecked, ok) {
		t.Fatal("OK must beat unchecked")
	}
	if !Better(unchecked, mismatch) {
		t.Fatal("fewer problems must win")
	}
	if !Better(mismatch, empty) || Better(empty, mismatch) {
		t.Fatal("rows must beat no rows")
	}
}
