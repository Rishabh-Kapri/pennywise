package service

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/go-pdf/fpdf"
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

func TestMergeExtractedSections(t *testing.T) {
	chunk1 := []model.StatementSection{
		{
			AccountLabel: "HDFC Savings", AccountNumber: "XXXX1234",
			PeriodStart: sp("2026-08-01"), OpeningBalance: f(1000),
			Rows: []model.StatementRow{row("", "2026-08-02", "A", 10, model.StatementDebit)},
		},
		{
			AccountLabel: "HDFC FD", AccountNumber: "XX9876",
			Rows: []model.StatementRow{row("", "2026-08-03", "B", 20, model.StatementCredit)},
		},
	}
	chunk2 := []model.StatementSection{
		// continuation page with no header: belongs to the last section (FD)
		{Rows: []model.StatementRow{row("", "2026-08-04", "C", 30, model.StatementCredit)}},
		// same savings account, number printed differently
		{
			AccountNumber: "50100001234", PeriodEnd: sp("2026-08-31"), ClosingBalance: f(990),
			Rows: []model.StatementRow{row("", "2026-08-05", "D", 40, model.StatementDebit)},
		},
	}

	merged := mergeExtractedSections([][]model.StatementSection{chunk1, chunk2})
	require.Len(t, merged, 2)
	assert.Len(t, merged[0].Rows, 2)
	assert.Equal(t, "2026-08-31", *merged[0].PeriodEnd)
	assert.Equal(t, 990.0, *merged[0].ClosingBalance)
	assert.Equal(t, 1000.0, *merged[0].OpeningBalance)
	assert.Len(t, merged[1].Rows, 2)
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

func TestChunkStatementPages(t *testing.T) {
	pages := []string{"page one   \n\n\n\nline", "", strings.Repeat("x", 50), "short"}
	chunks := chunkStatementPages(pages, 90)
	require.Len(t, chunks, 2)
	assert.Equal(t, 1, chunks[0].FirstPage)
	assert.Equal(t, 1, chunks[0].LastPage)
	assert.Contains(t, chunks[0].Text, "=== Page 1 ===\npage one\n\nline")
	assert.Equal(t, 3, chunks[1].FirstPage)
	assert.Equal(t, 4, chunks[1].LastPage)
}

func TestSplitPDFPages(t *testing.T) {
	pages, err := splitPDFPages("one\ftwo\f")
	require.NoError(t, err)
	assert.Equal(t, []string{"one", "two"}, pages)

	_, err = splitPDFPages(" \f \f")
	assert.ErrorIs(t, err, ErrStatementNoText)
}

// TestExtractPDFPages runs the real pdftotext over generated PDFs, plain and
// password protected. Skipped where poppler is not installed.
func TestExtractPDFPages(t *testing.T) {
	if _, err := exec.LookPath(pdftotextBinary); err != nil {
		t.Skip("pdftotext not installed")
	}

	build := func(password string) []byte {
		pdf := fpdf.New("P", "mm", "A4", "")
		if password != "" {
			pdf.SetProtection(fpdf.CnProtectPrint, password, "owner-"+password)
		}
		pdf.SetFont("Helvetica", "", 10)
		pdf.AddPage()
		pdf.Cell(40, 8, "02/08/2026")
		pdf.Cell(80, 8, "SWIGGY BANGALORE")
		pdf.Cell(30, 8, "450.00 Dr")
		pdf.AddPage()
		pdf.Cell(40, 8, "05/08/2026")
		pdf.Cell(80, 8, "SALARY ACME")
		var buf bytes.Buffer
		require.NoError(t, pdf.Output(&buf))
		return buf.Bytes()
	}

	ctx := context.Background()
	pages, err := extractPDFPages(ctx, build(""), "")
	require.NoError(t, err)
	require.Len(t, pages, 2)
	assert.Regexp(t, `02/08/2026\s+SWIGGY BANGALORE\s+450.00 Dr`, pages[0])

	locked := build("s3cret")
	_, err = extractPDFPages(ctx, locked, "")
	assert.ErrorIs(t, err, ErrStatementPasswordRequired)
	_, err = extractPDFPages(ctx, locked, "wrong")
	assert.ErrorIs(t, err, ErrStatementPasswordRequired)
	pages, err = extractPDFPages(ctx, locked, "s3cret")
	require.NoError(t, err)
	assert.Contains(t, pages[1], "SALARY ACME")
}
