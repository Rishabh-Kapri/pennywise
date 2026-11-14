package localocr

import (
	"testing"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

// All statement text here is made up.

func TestParseHeaderBankText(t *testing.T) {
	text := `**Example Bank**
Customer ID : 11111111
Account Number : 12345678904321
Account Type : SAVINGS - RESIDENTS
Statement From : 01/10/2026 To 31/10/2026
Currency : INR
Opening Balance : 1,00,000.50          Limit : 0.00
Closing Balance : 90,000.00`
	h := parseHeader(headerLines(text))
	if h.kind != sharedModel.StatementKindBank {
		t.Errorf("kind = %q, want bank", h.kind)
	}
	if h.accountNumber != "12345678904321" {
		t.Errorf("account number = %q", h.accountNumber)
	}
	if h.label != "Savings account" {
		t.Errorf("label = %q", h.label)
	}
	if h.periodStart != "2026-10-01" || h.periodEnd != "2026-10-31" {
		t.Errorf("period = %s..%s", h.periodStart, h.periodEnd)
	}
	if h.opening == nil || *h.opening != 100000.50 {
		t.Errorf("opening = %v", h.opening)
	}
	if h.closing == nil || *h.closing != 90000 {
		t.Errorf("closing = %v", h.closing)
	}
	if h.currency != "INR" {
		t.Errorf("currency = %q", h.currency)
	}
}

func TestParseHeaderCardWithSummaryBoxes(t *testing.T) {
	text := `# Example Credit Card Statement
Card Number: XXXX XXXX XXXX 9876
Statement Period: 03 Aug 2026 - 02 Sep 2026
Statement Date: 02/09/2026

| Total Amount Due | Minimum Amount Due | Payment Due Date |
|---|---|---|
| ₹12,345.67 | ₹620.00 | 22/09/2026 |

| Credit Limit | Available Credit Limit | Cash Limit |
| 2,00,000.00 | 1,87,654.33 | 40,000.00 |

| Reward Points | Opening Balance | Earned | Redeemed | Closing Balance |
| | 1,000 | 250 | 100 | 1,150 |`
	h := parseHeader(headerLines(text))
	if h.kind != sharedModel.StatementKindCard {
		t.Errorf("kind = %q, want card", h.kind)
	}
	if h.accountNumber != "XXXX XXXX XXXX 9876" {
		t.Errorf("account number = %q", h.accountNumber)
	}
	if h.periodStart != "2026-08-03" || h.periodEnd != "2026-09-02" {
		t.Errorf("period = %s..%s", h.periodStart, h.periodEnd)
	}
	if h.statementDate != "2026-09-02" {
		t.Errorf("statement date = %q", h.statementDate)
	}
	if h.dueDate != "2026-09-22" {
		t.Errorf("due date = %q", h.dueDate)
	}
	if h.totalDue == nil || *h.totalDue != 12345.67 {
		t.Errorf("total due = %v", h.totalDue)
	}
	if h.closing == nil || *h.closing != 12345.67 {
		t.Errorf("closing should default to total due, got %v", h.closing)
	}
	if h.minimumDue == nil || *h.minimumDue != 620 {
		t.Errorf("minimum due = %v", h.minimumDue)
	}
	if h.creditLimit == nil || *h.creditLimit != 200000 {
		t.Errorf("credit limit = %v (must skip 'available credit limit')", h.creditLimit)
	}
	// The points box's "Opening Balance" must not become the card's.
	if h.opening != nil {
		t.Errorf("opening = %v, want nil", *h.opening)
	}
	r := h.rewards
	if r == nil || r.Opening == nil || *r.Opening != 1000 || r.Earned == nil || *r.Earned != 250 ||
		r.Redeemed == nil || *r.Redeemed != 100 || r.Closing == nil || *r.Closing != 1150 {
		t.Errorf("rewards = %+v", r)
	}
}

func TestParseHeaderHTMLTables(t *testing.T) {
	html := `<table><tr><td>Card No.</td><td>4111 XXXX XXXX 1234</td></tr>
<tr><td>Statement Date</td><td>15 Oct 2026</td></tr></table>`
	h := parseHeader(headerLines(html))
	if h.accountNumber != "4111 XXXX XXXX 1234" || h.kind != sharedModel.StatementKindCard {
		t.Errorf("number/kind = %q/%q", h.accountNumber, h.kind)
	}
	if h.statementDate != "2026-10-15" {
		t.Errorf("statement date = %q", h.statementDate)
	}
}

func TestParseHeaderNothingFound(t *testing.T) {
	h := parseHeader(headerLines("Thank you for banking with us."))
	if h.accountNumber != "" || h.periodStart != "" || h.rewards != nil || h.opening != nil {
		t.Errorf("found something in nothing: %+v", h)
	}
}

func TestCardTableDirectionMarkers(t *testing.T) {
	var p tableParser
	p.header = statementHeader{kind: sharedModel.StatementKindCard, periodEnd: "2026-09-02"}
	p.parsePage(1, `<table>
<tr><th>Date</th><th>Transaction Details</th><th>Reward Points</th><th>Amount (in Rs.)</th><th>Cr/Dr</th></tr>
<tr><td>05 Aug</td><td>EXAMPLE STORE MUMBAI</td><td>20</td><td>1,000.00</td><td></td></tr>
<tr><td>10 Aug</td><td>PAYMENT RECEIVED - THANK YOU</td><td></td><td>5,000.00</td><td>CR</td></tr>
<tr><td>12/08/2026</td><td>REFUND EXAMPLE SHOP</td><td>-4</td><td>-200.00</td><td></td></tr>
<tr><td>28 Dec</td><td>LAST YEAR</td><td></td><td>10.00</td><td></td></tr>
</table>`)
	rows := p.rows
	if len(rows) != 4 {
		t.Fatalf("rows = %d, want 4: %+v", len(rows), rows)
	}
	want := []struct {
		date string
		dir  sharedModel.StatementDirection
	}{
		{"2026-08-05", sharedModel.StatementDebit},
		{"2026-08-10", sharedModel.StatementCredit},
		{"2026-08-12", sharedModel.StatementCredit}, // minus on a card is a credit
		{"2025-12-28", sharedModel.StatementDebit},  // month after the period's: last year
	}
	for i, w := range want {
		if rows[i].Date != w.date || rows[i].Direction != w.dir {
			t.Errorf("row %d = %s %s, want %s %s", i, rows[i].Date, rows[i].Direction, w.date, w.dir)
		}
	}
	if rows[0].RewardPoints == nil || *rows[0].RewardPoints != 20 {
		t.Errorf("row 0 points = %v", rows[0].RewardPoints)
	}
	if rows[2].RewardPoints == nil || *rows[2].RewardPoints != -4 {
		t.Errorf("row 2 points = %v", rows[2].RewardPoints)
	}

	res := p.result()
	if res.Sections[0].Kind != sharedModel.StatementKindCard {
		t.Errorf("section kind = %q", res.Sections[0].Kind)
	}
}

func TestBankMinusIsDebit(t *testing.T) {
	var p tableParser
	p.header = statementHeader{kind: sharedModel.StatementKindBank}
	p.parsePage(1, `<table>
<tr><th>Date</th><th>Narration</th><th>Amount</th></tr>
<tr><td>01/10/2026</td><td>ATM WITHDRAWAL</td><td>-500.00</td></tr>
<tr><td>02/10/2026</td><td>SALARY</td><td>+900.00</td></tr>
</table>`)
	if len(p.rows) != 2 || p.rows[0].Direction != sharedModel.StatementDebit || p.rows[1].Direction != sharedModel.StatementCredit {
		t.Errorf("rows = %+v", p.rows)
	}
}

func TestYearlessDateNeedsPeriod(t *testing.T) {
	var p tableParser // no header: no year to fill in
	p.parsePage(1, `<table><tr><th>Date</th><th>Details</th><th>Amount</th></tr>
<tr><td>05 Aug</td><td>X</td><td>1.00</td></tr></table>`)
	if len(p.rows) != 0 {
		t.Errorf("a date without a year was read without a period: %+v", p.rows)
	}
}

func TestEMITableReadAsPlansNotRows(t *testing.T) {
	var p tableParser
	p.header = statementHeader{kind: sharedModel.StatementKindCard, periodEnd: "2026-09-02"}
	p.parsePage(1, `<table>
<tr><th>Date</th><th>Transaction Details</th><th>Amount (in Rs.)</th></tr>
<tr><td>05/08/2026</td><td>EXAMPLE STORE</td><td>1,000.00</td></tr>
</table>
<table>
<tr><th>Booking Date</th><th>Description</th><th>Loan Amount</th><th>EMI Amount</th><th>Interest Rate</th><th>Tenure</th><th>Remaining Tenure</th><th>Principal Outstanding</th></tr>
<tr><td>01/01/2026</td><td>EXAMPLE PHONE</td><td>45,000.00</td><td>7,800.00</td><td>15% p.a.</td><td>6 months</td><td>2</td><td>15,000.00</td></tr>
</table>`)
	if len(p.rows) != 1 {
		t.Fatalf("rows = %d, want 1 (the EMI table must not add rows): %+v", len(p.rows), p.rows)
	}
	if len(p.plans) != 1 {
		t.Fatalf("plans = %+v", p.plans)
	}
	pl := p.plans[0]
	if pl.Description != "EXAMPLE PHONE" || pl.BookedOn != "2026-01-01" ||
		pl.Principal == nil || *pl.Principal != 45000 || pl.MonthlyAmount == nil || *pl.MonthlyAmount != 7800 ||
		pl.InterestRate == nil || *pl.InterestRate != 15 || pl.TenureMonths == nil || *pl.TenureMonths != 6 ||
		pl.RemainingMonths == nil || *pl.RemainingMonths != 2 || pl.Outstanding == nil || *pl.Outstanding != 15000 {
		t.Errorf("plan = %+v", pl)
	}
	if res := p.result(); len(res.Sections[0].EMIPlans) != 1 {
		t.Error("plans not on the section")
	}
}
