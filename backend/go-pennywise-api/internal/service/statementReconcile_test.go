package service

import (
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func f(v float64) *float64 { return &v }
func sp(v string) *string  { return &v }

func row(id, date, desc string, amount float64, dir model.StatementDirection) model.StatementRow {
	return model.StatementRow{ID: id, Date: date, Description: desc, Amount: amount, Direction: dir}
}

func ledger(date string, amount float64, payee string) model.StatementLedgerTxn {
	return model.StatementLedgerTxn{ID: uuid.New(), Date: date, Amount: amount, PayeeName: payee}
}

func TestReconcileStatementRows(t *testing.T) {
	t.Run("exact match within window, missing and ledger-only", func(t *testing.T) {
		rows := []model.StatementRow{
			row("0-0", "2026-08-03", "SWIGGY BANGALORE", 450, model.StatementDebit),
			row("0-1", "2026-08-05", "SALARY ACME", 90000, model.StatementCredit),
			row("0-2", "2026-08-10", "ZEPTO MARKETPLACE", 96, model.StatementDebit),
		}
		swiggy := ledger("2026-08-01", -450, "Swiggy")
		salary := ledger("2026-08-05", 90000, "Acme")
		cash := ledger("2026-08-12", -2000, "Cash")
		outside := ledger("2026-08-30", -10, "Late") // past period end: not ledger-only

		matched, missing, ledgerOnly := reconcileStatementRows(
			rows, []model.StatementLedgerTxn{swiggy, salary, cash, outside},
			"2026-08-01", "2026-08-20", map[uuid.UUID]bool{},
		)

		require.Len(t, matched, 2)
		assert.Equal(t, swiggy.ID, matched[0].Transaction.ID)
		assert.Equal(t, 2, matched[0].DateDiffDays)
		assert.Equal(t, model.StatementMatchExact, matched[0].Kind)
		assert.Equal(t, salary.ID, matched[1].Transaction.ID)
		require.Len(t, missing, 1)
		assert.Equal(t, "0-2", missing[0].ID)
		require.Len(t, ledgerOnly, 1)
		assert.Equal(t, cash.ID, ledgerOnly[0].ID)
	})

	t.Run("direction must agree", func(t *testing.T) {
		rows := []model.StatementRow{row("0-0", "2026-08-03", "REFUND AMAZON", 500, model.StatementCredit)}
		purchase := ledger("2026-08-03", -500, "Amazon")
		matched, missing, _ := reconcileStatementRows(rows, []model.StatementLedgerTxn{purchase}, "2026-08-01", "2026-08-31", map[uuid.UUID]bool{})
		assert.Empty(t, matched)
		assert.Len(t, missing, 1)
	})

	t.Run("identical amounts pair one to one by closest date", func(t *testing.T) {
		rows := []model.StatementRow{
			row("0-0", "2026-08-13", "RAZ*Makemytrip", 343, model.StatementDebit),
			row("0-1", "2026-08-28", "RAZ*Makemytrip", 343, model.StatementDebit),
		}
		late := ledger("2026-08-27", -343, "MakeMyTrip")
		early := ledger("2026-08-12", -343, "MakeMyTrip")
		matched, missing, _ := reconcileStatementRows(rows, []model.StatementLedgerTxn{late, early}, "2026-08-01", "2026-08-31", map[uuid.UUID]bool{})
		require.Len(t, matched, 2)
		assert.Empty(t, missing)
		assert.Equal(t, early.ID, matched[0].Transaction.ID)
		assert.Equal(t, late.ID, matched[1].Transaction.ID)
	})

	t.Run("near match needs similar text", func(t *testing.T) {
		rows := []model.StatementRow{
			row("0-0", "2026-08-08", "ANTHROPIC* CLAUDE SUBANTHROPIC.", 2250.73, model.StatementDebit),
			row("0-1", "2026-08-09", "RAILWAYRAILWAY.CO", 576.99, model.StatementDebit),
		}
		claude := ledger("2026-08-08", -2210.00, "Anthropic")
		unrelated := ledger("2026-08-09", -570.00, "Grocer")
		matched, missing, ledgerOnly := reconcileStatementRows(rows, []model.StatementLedgerTxn{claude, unrelated}, "2026-08-01", "2026-08-31", map[uuid.UUID]bool{})
		require.Len(t, matched, 1)
		assert.Equal(t, model.StatementMatchAmountDiffers, matched[0].Kind)
		assert.Equal(t, claude.ID, matched[0].Transaction.ID)
		assert.Len(t, missing, 1)
		assert.Equal(t, []model.StatementLedgerTxn{unrelated}, ledgerOnly)
	})

	t.Run("ledger rows claimed by another section are skipped", func(t *testing.T) {
		txn := ledger("2026-08-03", -100, "Shop")
		used := map[uuid.UUID]bool{txn.ID: true}
		rows := []model.StatementRow{row("1-0", "2026-08-03", "SHOP", 100, model.StatementDebit)}
		matched, missing, ledgerOnly := reconcileStatementRows(rows, []model.StatementLedgerTxn{txn}, "2026-08-01", "2026-08-31", used)
		assert.Empty(t, matched)
		assert.Len(t, missing, 1)
		assert.Empty(t, ledgerOnly)
	})
}

func TestStatementTextSimilarity(t *testing.T) {
	r := model.StatementRow{Description: "MC DONALDSPUNE"}
	assert.Equal(t, 1.0, statementTextSimilarity(r, model.StatementLedgerTxn{PayeeName: "McDonalds"}))
	assert.Equal(t, 0.0, statementTextSimilarity(r, model.StatementLedgerTxn{PayeeName: "Zepto"}))
}

func TestCleanStatementSections(t *testing.T) {
	sections := []model.StatementSection{{
		AccountNumber: "1234",
		PeriodStart:   sp("01/08/2026"),
		Rows: []model.StatementRow{
			row("", "2026-08-02", "OK", 10.004, model.StatementDebit),
			row("", "02/08/2026", "BAD DATE", 10, model.StatementDebit),
			row("", "2026-08-02", "ZERO", 0, model.StatementDebit),
			row("", "2026-08-02", "NO DIRECTION", 5, ""),
			{Date: "2026-08-03", Payee: "Payee Only", Amount: 7, Direction: model.StatementCredit},
		},
	}, {
		AccountNumber: "9999", // nothing usable: dropped entirely
		Rows:          []model.StatementRow{row("", "x", "BAD", 1, model.StatementDebit)},
	}}

	cleaned, dropped := cleanStatementSections(sections)
	require.Len(t, cleaned, 1)
	assert.Equal(t, 4, dropped)
	assert.Nil(t, cleaned[0].PeriodStart)
	require.Len(t, cleaned[0].Rows, 2)
	assert.Equal(t, 10.0, cleaned[0].Rows[0].Amount)
	assert.Equal(t, "0-0", cleaned[0].Rows[0].ID)
	assert.Equal(t, "Payee Only", cleaned[0].Rows[1].Description)
	assert.Equal(t, "0-1", cleaned[0].Rows[1].ID)
}

func TestRepairDirectionsFromBalance(t *testing.T) {
	t.Run("bank convention flips the contradicting row", func(t *testing.T) {
		rows := []model.StatementRow{
			{Date: "2026-08-01", Amount: 100, Direction: model.StatementCredit, Balance: f(1100)},
			{Date: "2026-08-02", Amount: 50, Direction: model.StatementDebit, Balance: f(1050)},
			{Date: "2026-08-03", Amount: 25, Direction: model.StatementDebit, Balance: f(1025)},
			{Date: "2026-08-04", Amount: 200, Direction: model.StatementDebit, Balance: f(1225)}, // misread
			{Date: "2026-08-05", Amount: 25, Direction: model.StatementDebit, Balance: f(1200)},
		}
		repairDirectionsFromBalance(rows)
		assert.Equal(t, model.StatementCredit, rows[3].Direction)
		assert.True(t, rows[3].DirectionCorrected)
		assert.False(t, rows[1].DirectionCorrected)
	})

	t.Run("card convention (owed balance) is respected", func(t *testing.T) {
		rows := []model.StatementRow{
			{Date: "2026-08-01", Amount: 100, Direction: model.StatementDebit, Balance: f(100)},
			{Date: "2026-08-02", Amount: 50, Direction: model.StatementDebit, Balance: f(150)},
			{Date: "2026-08-03", Amount: 20, Direction: model.StatementDebit, Balance: f(170)},
			{Date: "2026-08-04", Amount: 70, Direction: model.StatementCredit, Balance: f(100)},
		}
		repairDirectionsFromBalance(rows)
		for _, r := range rows {
			assert.False(t, r.DirectionCorrected)
		}
	})

	t.Run("too few checkable rows changes nothing", func(t *testing.T) {
		rows := []model.StatementRow{
			{Amount: 100, Direction: model.StatementCredit, Balance: f(100)},
			{Amount: 50, Direction: model.StatementCredit, Balance: f(50)},
		}
		repairDirectionsFromBalance(rows)
		assert.Equal(t, model.StatementCredit, rows[1].Direction)
	})
}

func TestStatementBalanceCheck(t *testing.T) {
	bank := model.StatementSection{
		OpeningBalance: f(1000), ClosingBalance: f(1050),
		Rows: []model.StatementRow{
			{Amount: 100, Direction: model.StatementCredit},
			{Amount: 50, Direction: model.StatementDebit},
		},
	}
	check := statementBalanceCheck(bank)
	require.NotNil(t, check)
	assert.True(t, check.OK)

	card := bank
	card.ClosingBalance = f(950) // owed: purchases raise, payments lower
	assert.True(t, statementBalanceCheck(card).OK)

	short := bank
	short.ClosingBalance = f(1000)
	check = statementBalanceCheck(short)
	assert.False(t, check.OK)
	assert.Equal(t, -50.0, check.Difference) // reported - computed

	assert.Nil(t, statementBalanceCheck(model.StatementSection{}))
}

func TestStatementRowHashOccurrence(t *testing.T) {
	acc := uuid.New()
	r := row("0-0", "2026-08-02", "COFFEE", 3, model.StatementDebit)
	key := statementRowKey(acc, r)
	assert.NotEqual(t, statementRowHash(key, 0), statementRowHash(key, 1))
	assert.Equal(t, statementRowHash(key, 0), statementRowHash(statementRowKey(acc, r), 0))
}

// TestExtractPDFPages runs the real pdftotext over generated PDFs, plain and
// password protected. Skipped where poppler is not installed.

func TestReconcileMatchesByReference(t *testing.T) {
	// Ten days apart and the text shares nothing, but both sides state the
	// same reference: the same payment.
	r := row("0-0", "2026-10-12", "UPI-SOMEONE-x@ybl-YESB0YBLUPI-555566667777", 250, model.StatementDebit)
	r.Reference = "555566667777"
	other := ledger("2026-10-02", -250, "Unrelated Name")
	same := ledger("2026-10-02", -250, "Also Unrelated")
	same.Refs = []string{"555566667777"}

	matched, missing, _ := reconcileStatementRows(
		[]model.StatementRow{r}, []model.StatementLedgerTxn{other, same}, "2026-10-01", "2026-10-31", map[uuid.UUID]bool{},
	)
	require.Len(t, matched, 1)
	assert.Empty(t, missing)
	assert.Equal(t, same.ID, matched[0].Transaction.ID)
	assert.Equal(t, model.StatementMatchExact, matched[0].Kind)
}

func TestCleanStatementSectionsReadsNarration(t *testing.T) {
	r := row("", "2026-10-02",
		"UPI-Example Shop-shop.1@okaxis-UTIB0000001-123412341234-Payment Value Dt 09/10/2026 Ref 123412341234",
		99, model.StatementDebit)
	out, dropped := cleanStatementSections([]model.StatementSection{{Rows: []model.StatementRow{r}}})
	require.Zero(t, dropped)
	got := out[0].Rows[0]
	assert.Equal(t, "UPI-Example Shop-shop.1@okaxis-UTIB0000001-123412341234-Payment", got.Description)
	assert.Contains(t, got.RawDescription, "Value Dt 09/10/2026")
	assert.Equal(t, "UPI", got.Channel)
	assert.Equal(t, "EXAMPLE SHOP", got.Merchant)
	assert.Equal(t, "shop.1@okaxis", got.VPA)
	assert.Equal(t, "123412341234", got.Reference)
	assert.Equal(t, "2026-10-09", got.ValueDate)
	assert.True(t, got.ValueDateMismatch, "value date a week after the row's date")
	assert.Equal(t, "Example Shop", got.Payee)
}
