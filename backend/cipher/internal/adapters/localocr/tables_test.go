package localocr

import (
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
