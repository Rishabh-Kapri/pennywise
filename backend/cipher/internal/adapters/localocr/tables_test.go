package localocr

import (
	"reflect"
	"testing"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

func TestParseBankTable(t *testing.T) {
	var p tableParser
	p.parsePage(1, "```html\n<table>"+
		"<tr><th>Date</th><th>Narration</th><th>Chq./Ref.No.</th><th>Value Dt</th><th>Withdrawal Amt.</th><th>Deposit Amt.</th><th>Closing Balance</th></tr>"+
		"<tr><td>01/10/25</td><td>Opening Balance</td><td></td><td></td><td></td><td></td><td>10,000.00</td></tr>"+
		"<tr><td>02/10/25</td><td>UPI-SWIGGY-<br>swiggy@icici</td><td>0000527512</td><td>02/10/25</td><td>450.00</td><td></td><td>9,550.00</td></tr>"+
		"<tr><td></td><td>FOOD ORDER</td><td></td><td></td><td></td><td></td><td></td></tr>"+
		"<tr><td>05/10/25</td><td>NEFT SALARY</td><td>N123</td><td>05/10/25</td><td></td><td>1,00,000.00</td><td>1,09,550.00</td></tr>"+
		"</table>\n```")

	// A continuation page without the header reuses the layout.
	p.parsePage(2, "<table><tr><td>06-10-2025</td><td>ATM WDL</td><td></td><td>06-10-2025</td><td>2,000</td><td></td><td>1,07,550.00</td></tr></table>")

	rows := p.result().Sections[0].Rows
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3: %+v", len(rows), rows)
	}

	want := []struct {
		date, desc, ref string
		amount, balance float64
		dir             sharedModel.StatementDirection
		page            int
	}{
		{"2025-10-02", "UPI-SWIGGY- swiggy@icici FOOD ORDER", "0000527512", 450, 9550, sharedModel.StatementDebit, 1},
		{"2025-10-05", "NEFT SALARY", "N123", 100000, 109550, sharedModel.StatementCredit, 1},
		{"2025-10-06", "ATM WDL", "", 2000, 107550, sharedModel.StatementDebit, 2},
	}
	for i, w := range want {
		r := rows[i]
		if r.Date != w.date || r.Description != w.desc || r.Reference != w.ref ||
			r.Amount != w.amount || r.Direction != w.dir || r.Page != w.page ||
			r.Balance == nil || *r.Balance != w.balance {
			t.Errorf("row %d = %+v (balance %v), want %+v", i, r, r.Balance, w)
		}
	}
}

func TestParseCardTable(t *testing.T) {
	var p tableParser
	p.parsePage(1, "<table>"+
		"<tr><td>Date</td><td>Transaction Description</td><td>Amount (in Rs.)</td></tr>"+
		"<tr><td>12/10/2025 18:04:11</td><td>AMAZON PAY INDIA</td><td>1,299.00</td></tr>"+
		"<tr><td>15/10/2025</td><td>PAYMENT RECEIVED</td><td>5,000.00 Cr</td></tr>"+
		"<tr><td>Total</td><td></td><td>6,299.00</td></tr>"+
		"</table>")

	rows := p.result().Sections[0].Rows
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2: %+v", len(rows), rows)
	}
	if rows[0].Date != "2025-10-12" || rows[0].Amount != 1299 || rows[0].Direction != sharedModel.StatementDebit {
		t.Errorf("purchase = %+v", rows[0])
	}
	if rows[1].Amount != 5000 || rows[1].Direction != sharedModel.StatementCredit {
		t.Errorf("payment = %+v", rows[1])
	}
}

func TestParseSplitHeader(t *testing.T) {
	var p tableParser
	p.parsePage(1, "<table>"+
		`<tr><th>Txn Date</th><th>Particulars</th><th colspan="2">Amount</th><th>Balance</th></tr>`+
		"<tr><th></th><th></th><th>Debit</th><th>Credit</th><th></th></tr>"+
		"<tr><td>03 Oct 2025</td><td>INTEREST</td><td></td><td>12.50</td><td>512.50</td></tr>"+
		"</table>")

	rows := p.result().Sections[0].Rows
	if len(rows) != 1 || rows[0].Direction != sharedModel.StatementCredit || rows[0].Amount != 12.5 || rows[0].Date != "2025-10-03" {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestNoTablesGivesNoSections(t *testing.T) {
	var p tableParser
	p.parsePage(1, "<p>Customer care: 1800 000 000</p>")
	if got := p.result().Sections; len(got) != 0 {
		t.Fatalf("sections = %+v", got)
	}
}

// Made-up statement text.

func TestMarkdownTableRows(t *testing.T) {
	var p tableParser
	p.parsePage(1, `Some heading

| Date | Narration | Withdrawal Amt. | Deposit Amt. | Closing Balance |
|---|---|---|---|---|
| 01/10/2026 | **EXAMPLE SHOP** | 250.00 | | 9,750.00 |
| 02/10/2026 | EXAMPLE SALARY | | 1,000.00 | 10,750.00 |`)
	if len(p.rows) != 2 {
		t.Fatalf("rows = %+v, stats = %+v", p.rows, p.stats)
	}
	if p.rows[0].Description != "EXAMPLE SHOP" || p.rows[0].Direction != sharedModel.StatementDebit ||
		p.rows[1].Direction != sharedModel.StatementCredit || p.rows[1].Amount != 1000 {
		t.Errorf("rows = %+v", p.rows)
	}
	if p.stats.MarkdownTables != 1 || p.stats.HTMLTables != 0 || p.stats.Rows != 2 {
		t.Errorf("stats = %+v", p.stats)
	}
}

func TestParseStatsSayWhyRowsDropped(t *testing.T) {
	var p tableParser // no header: dates without a year can't be read
	p.parsePage(1, `<table>
<tr><td>Account summary</td><td>x</td></tr>
<tr><th>Date</th><th>Details</th><th>Amount</th></tr>
<tr><td>05 Aug</td><td>A</td><td>1.00</td></tr>
<tr><td>32/13/2026</td><td>B</td><td>1.00</td></tr>
<tr><td>01/08/2026</td><td>C</td><td>0.00</td></tr>
<tr><td>01/08/2026</td><td>D</td></tr>
<tr><td>02/08/2026</td><td>E</td><td>5.00</td></tr>
<tr><td></td><td>more of E</td><td></td></tr>
</table>`)
	s := p.stats
	want := parseStats{HTMLTables: 1, TableRows: 8, Headers: 1, BeforeHeader: 1, YearlessDate: 1,
		BadDate: 1, NoAmount: 1, WidthMismatch: 1, Rows: 1, Continuations: 1,
		MismatchWidths: map[string]int{"2/3": 1}, BadDateShapes: map[string]int{"99/99/9999": 1}}
	s.Layouts = nil
	if !reflect.DeepEqual(s, want) {
		t.Errorf("stats = %+v\nwant    %+v", s, want)
	}
	if got := p.stats.Layouts; len(got) != 1 || got[0] != "date,description,amount" {
		t.Errorf("layouts = %v", got)
	}
}

func TestRowsShortOrLongByUnreadColumns(t *testing.T) {
	var p tableParser
	p.header = statementHeader{kind: sharedModel.StatementKindCard, periodEnd: "2026-09-02"}
	p.parsePage(1, `| Date | Transaction Details | Amount (in Rs.) | |
|---|---|---|---|
| 05/08/2026 | EXAMPLE STORE | 1,000.00 |
| 06/08/2026 | EXAMPLE CAFE | 200.00 | |
| 07/08/2026 | EXAMPLE FUEL | 300.00 |||
| 08/08/2026 | EXAMPLE BOOKS | 400.00 | | extra |

| Total | 1,900.00 |`)
	if len(p.rows) != 3 {
		t.Fatalf("rows = %d, want 3: %+v", len(p.rows), p.rows)
	}
	if p.stats.WidthMismatch != 2 || p.stats.MismatchWidths["5/4"] != 1 || p.stats.MismatchWidths["2/4"] != 1 {
		t.Errorf("stats = %+v", p.stats)
	}
}

func TestShortRowMissingAReadColumnIsSkipped(t *testing.T) {
	var p tableParser
	p.parsePage(1, `| Date | Details | Debit | Credit |
|---|---|---|---|
| 01/10/2026 | X | 5.00 |`)
	if len(p.rows) != 0 || p.stats.WidthMismatch != 1 {
		t.Errorf("a row missing its credit column was guessed: %+v %+v", p.rows, p.stats)
	}
}

// A busy first page can come back as plain lines, not a table.
func TestPlainTextRows(t *testing.T) {
	var p tableParser
	p.header = statementHeader{kind: sharedModel.StatementKindCard, periodEnd: "2026-09-13"}
	p.parsePage(1, `IMPORTANT INFORMATION
1. SOME NOTICE DATED 01/06/2026 ABOUT CHARGES 100.00
Purchase Indicator
RESTAURANT
86%

Domestic Transactions

DATE & TIME TRANSACTION DESCRIPTION AMOUNT PI
CARD HOLDER [ID: 12345]
15/08/2026 | 09:45 CC PAYMENT EXAMPLE (Ref# AB1234567890) + ₹ 2,000.00
16/08/2026 | 14:20 EXAMPLECAFE CITY ₹ 437.00
19/08/2026 | 00:00 10% Example CashBack + ₹ 43.70
20/08/2026 EXAMPLE SHOP ₹ 1,200.50 ●`)
	if len(p.rows) != 4 {
		t.Fatalf("rows = %d, want 4: %+v", len(p.rows), p.rows)
	}
	want := []struct {
		date, desc string
		amount     float64
		dir        sharedModel.StatementDirection
	}{
		{"2026-08-15", "CC PAYMENT EXAMPLE (Ref# AB1234567890)", 2000, sharedModel.StatementCredit},
		{"2026-08-16", "EXAMPLECAFE CITY", 437, sharedModel.StatementDebit},
		{"2026-08-19", "10% Example CashBack", 43.70, sharedModel.StatementCredit},
		{"2026-08-20", "EXAMPLE SHOP", 1200.50, sharedModel.StatementDebit},
	}
	for i, w := range want {
		r := p.rows[i]
		if r.Date != w.date || r.Description != w.desc || r.Amount != w.amount || r.Direction != w.dir {
			t.Errorf("row %d = %s %q %.2f %s, want %s %q %.2f %s", i, r.Date, r.Description, r.Amount, r.Direction,
				w.date, w.desc, w.amount, w.dir)
		}
	}
	if p.stats.TextRows != 4 {
		t.Errorf("stats = %+v", p.stats)
	}
}

func TestTimeSplitIntoDescriptionIsDropped(t *testing.T) {
	var p tableParser
	p.parsePage(1, `| DATE & TIME | TRANSACTION DESCRIPTION | AMOUNT | PI |
| :--- | :--- | :--- | :--- |
| 02/09/2026 | 16:46 EXAMPLE GROCERY | ₹184.00 | ● |`)
	if len(p.rows) != 1 || p.rows[0].Description != "EXAMPLE GROCERY" || p.rows[0].Date != "2026-09-02" {
		t.Errorf("rows = %+v", p.rows)
	}
}

func TestDatesGluedToTime(t *testing.T) {
	for cell, want := range map[string]string{
		"15/08/202609:45":     "2026-08-15",
		"15/08/2026|09:45":    "2026-08-15",
		"15-08-2026,09:45:10": "2026-08-15",
		"05/08/26 10:00":      "2026-08-05",
		"15 Aug 2026,09:45":   "2026-08-15",
		"15Aug202609:45":      "2026-08-15",
		"15 Sept 2026":        "2026-09-15",
		"Ref 123":             "",
		"32/13/2026":          "",
	} {
		got, _ := parseDate(cell)
		if got != want {
			t.Errorf("parseDate(%q) = %q, want %q", cell, got, want)
		}
	}
	if got := dateShape("15/08/2026 Ab"); got != "99/99/9999 aa" {
		t.Errorf("shape = %q", got)
	}
}
