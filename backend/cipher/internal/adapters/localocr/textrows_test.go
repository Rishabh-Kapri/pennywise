package localocr

import (
	"testing"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

// Made-up statement text.

func TestTextPassReadsRowsWithoutHeading(t *testing.T) {
	page := `Example Bank Credit Card
Payment Due Date 03 Oct, 2026
05/08/2026 EXAMPLE STORE CITY ₹ 1,000.00
06/08/2026 10:15 PAYMENT RECEIVED 500.00 Cr
Total Amount Due 4,458.00`

	strict := tableParser{header: statementHeader{kind: sharedModel.StatementKindCard}}
	strict.parsePage(1, page)
	if len(strict.rows) != 0 {
		t.Errorf("first pass read rows without a heading: %+v", strict.rows)
	}

	loose := tableParser{header: statementHeader{kind: sharedModel.StatementKindCard}, textWithoutHeading: true}
	loose.parsePage(1, page)
	if len(loose.rows) != 2 {
		t.Fatalf("rows = %+v", loose.rows)
	}
	if loose.rows[0].Amount != 1000 || loose.rows[0].Direction != sharedModel.StatementDebit ||
		loose.rows[1].Amount != 500 || loose.rows[1].Direction != sharedModel.StatementCredit ||
		loose.rows[1].Description != "PAYMENT RECEIVED" {
		t.Errorf("rows = %+v", loose.rows)
	}
}

// A bank statement read as text: amount, then the balance after it.
func TestTextRowsWithBalance(t *testing.T) {
	p := tableParser{header: statementHeader{kind: sharedModel.StatementKindBank}, textWithoutHeading: true}
	p.parsePage(1, `Date Opening Balance
04-Oct-2026 SMS CHRG FOR:2026-07-01to2026-09-30 1.18 2650.09CR.
28-Oct-2026 UPI/DR/111111111111/Example/UTIB/shop@bank/Ma 2.00 2648.09CR.
28-Oct-2026 UPI/CR/222222222222/Example P/UTIB/shop@bank/ 2.00 2650.09CR.
30-Oct-2026 EXAMPLE TRANSFER 3,000.00 349.91DR.
Closing Balance 349.91DR.
Total relationship value of customer with bank as on 31-Oct-2026`)
	want := []struct {
		desc    string
		amount  float64
		dir     sharedModel.StatementDirection
		balance float64
	}{
		{"SMS CHRG FOR:2026-07-01to2026-09-30", 1.18, sharedModel.StatementDebit, 2650.09}, // no opening: narration
		{"UPI/DR/111111111111/Example/UTIB/shop@bank/Ma", 2, sharedModel.StatementDebit, 2648.09},
		{"UPI/CR/222222222222/Example P/UTIB/shop@bank/", 2, sharedModel.StatementCredit, 2650.09},
		{"EXAMPLE TRANSFER", 3000, sharedModel.StatementDebit, -349.91}, // overdrawn
	}
	if len(p.rows) != len(want) {
		t.Fatalf("rows = %d, want %d: %+v", len(p.rows), len(want), p.rows)
	}
	for i, w := range want {
		r := p.rows[i]
		if r.Description != w.desc || r.Amount != w.amount || r.Direction != w.dir || r.Balance == nil || *r.Balance != w.balance {
			t.Errorf("row %d = %q %.2f %s %v, want %q %.2f %s %.2f", i, r.Description, r.Amount, r.Direction, r.Balance,
				w.desc, w.amount, w.dir, w.balance)
		}
	}
	if p.stats.TextDirectionGuessed != 1 {
		t.Errorf("guessed = %d, want 1 (only the first row)", p.stats.TextDirectionGuessed)
	}
}

func TestTextRowBalanceUsesOpening(t *testing.T) {
	opening := 100.0
	p := tableParser{header: statementHeader{kind: sharedModel.StatementKindBank, opening: &opening}, textWithoutHeading: true}
	p.parsePage(1, `01/10/2026 SALARY EXAMPLE 50.00 150.00`)
	if len(p.rows) != 1 || p.rows[0].Direction != sharedModel.StatementCredit || p.stats.TextDirectionGuessed != 0 {
		t.Errorf("rows = %+v stats = %+v", p.rows, p.stats)
	}
}
