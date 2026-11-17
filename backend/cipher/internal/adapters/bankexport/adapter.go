// Package bankexport reads a statement file exported from the bank, not a
// PDF: CSV, or HDFC's "~|~"-separated card export. Nothing is guessed:
// columns are found by their header names, and the direction comes from
// which column an amount is in, its sign, or a Dr/Cr column.
package bankexport

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/ports"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

type adapter struct{}

func New() ports.StatementImport { return adapter{} }

// ExtractStatement implements [ports.StatementImport]. passwords are for
// PDFs; an export has none.
func (adapter) ExtractStatement(
	_ context.Context,
	data []byte,
	_ []string,
) (sharedModel.StatementExtractResponse, error) {
	res := sharedModel.StatementExtractResponse{Sections: []sharedModel.StatementSection{}, PasswordIndex: -1}
	if bytes.HasPrefix(bytes.TrimLeft(data, "\x00\t\r\n "), []byte("%PDF-")) {
		return res, errs.New(errs.CodeStatementExtractFailed,
			"Bank export reads CSV files, and this is a PDF. Pick a PDF reader for Read statement, or turn off From email if you only upload CSVs")
	}
	recs, err := records(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")))
	if err != nil {
		return res, errs.Wrap(errs.CodeStatementExtractFailed, "the file isn't a readable CSV", err)
	}

	var p parser
	for i, rec := range recs {
		p.add(i+1, rec)
	}
	if p.cols == nil {
		return res, errs.New(errs.CodeStatementExtractFailed,
			"no header row with date, narration and amount columns found in the CSV")
	}
	if len(p.s.Rows) > 0 {
		res.Sections = append(res.Sections, p.section())
	}
	return res, nil
}

// records splits the file into trimmed fields: on "~|~" for HDFC's card
// export, else as CSV.
func records(data []byte) ([][]string, error) {
	var recs [][]string
	if bytes.Contains(data, []byte("~|~")) {
		for line := range strings.SplitSeq(string(data), "\n") {
			recs = append(recs, strings.Split(line, "~|~"))
		}
	} else {
		r := csv.NewReader(bytes.NewReader(data))
		r.FieldsPerRecord = -1
		r.LazyQuotes = true
		for {
			rec, err := r.Read()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return nil, err
			}
			recs = append(recs, rec)
		}
	}
	for _, rec := range recs {
		for i := range rec {
			rec[i] = strings.TrimSpace(rec[i])
		}
	}
	return recs, nil
}

// parser reads the file top to bottom. Lines before the transactions' header
// row and after its last row are the bank's summary (card.go).
type parser struct {
	s     sharedModel.StatementSection
	cols  *columns
	ended bool   // the transactions table is over
	next  string // the card summary table whose values come next: account, points or loans
}

func (p *parser) add(line int, rec []string) {
	switch {
	case p.cols == nil:
		if p.cols = header(rec); p.cols == nil {
			p.summary(rec)
		}
	case !p.ended:
		if row, ok := p.cols.row(rec); ok {
			row.Line = line
			p.s.Rows = append(p.s.Rows, row)
		} else if len(p.s.Rows) > 0 && !blank(rec) {
			// The first line that isn't a transaction ends the table: an EMI
			// table after it has dates and amounts too.
			p.ended = true
			p.summary(rec)
		}
	default:
		p.summary(rec)
	}
}

func (p *parser) section() sharedModel.StatementSection {
	s := p.s
	first, last := s.Rows[0], s.Rows[len(s.Rows)-1]
	s.PeriodStart, s.PeriodEnd = &first.Date, &last.Date
	if s.TotalDue != nil || s.AccountNumber != "" {
		s.Kind = sharedModel.StatementKindCard
		return s
	}
	s.Kind = sharedModel.StatementKindBank
	if first.Balance != nil && last.Balance != nil {
		opening := round(*first.Balance - first.SignedAmount())
		s.OpeningBalance, s.ClosingBalance = &opening, last.Balance
	}
	return s
}

// columns are a header row's column indexes; -1 when it has none.
type columns struct {
	date, narration, ref, debit, credit, amount, drcr, balance, rewards int
}

// header finds the columns in rec, or nil when it isn't the header row: that
// needs a date, a narration, and either debit and credit or an amount.
func header(rec []string) *columns {
	c := columns{-1, -1, -1, -1, -1, -1, -1, -1, -1}
	for i, h := range rec {
		h = strings.ToLower(h)
		set := func(col *int) {
			if *col < 0 {
				*col = i
			}
		}
		switch {
		case strings.Contains(h, "value"): // HDFC's "Value Dat"
		case strings.Contains(h, "date"):
			set(&c.date)
		case containsAny(h, "narration", "description", "particulars", "details", "remarks"):
			set(&c.narration)
		case containsAny(h, "debit /credit", "debit/credit", "dr/cr", "cr/dr"):
			set(&c.drcr)
		case containsAny(h, "debit", "withdrawal"):
			set(&c.debit)
		case containsAny(h, "credit", "deposit"):
			set(&c.credit)
		case strings.Contains(h, "balance"):
			set(&c.balance)
		case containsAny(h, "reward", "points"):
			set(&c.rewards)
		case containsAny(h, "ref", "chq", "cheque"):
			set(&c.ref)
		case h == "amt" || strings.Contains(h, "amount"):
			set(&c.amount)
		}
	}
	split := c.debit >= 0 && c.credit >= 0
	if c.date < 0 || c.narration < 0 || (!split && c.amount < 0) {
		return nil
	}
	if split {
		c.amount = -1
	}
	return &c
}

// row reads one transaction; false for a line that isn't one.
func (c columns) row(rec []string) (sharedModel.StatementRow, bool) {
	date, ok := parseDate(field(rec, c.date))
	if !ok {
		return sharedModel.StatementRow{}, false
	}
	row := sharedModel.StatementRow{
		Date:        date,
		Time:        exportTime(field(rec, c.date)),
		Description: strings.Join(strings.Fields(field(rec, c.narration)), " "),
		Reference:   field(rec, c.ref),
		Direction:   sharedModel.StatementDebit,
	}
	switch {
	case c.amount < 0: // debit and credit columns; the unused one is empty or 0.00
		debit, _ := amount(field(rec, c.debit))
		credit, _ := amount(field(rec, c.credit))
		row.Amount = math.Abs(debit)
		if credit != 0 {
			row.Amount, row.Direction = math.Abs(credit), sharedModel.StatementCredit
		}
	case c.drcr >= 0: // a positive amount and a Dr/Cr column
		v, _ := amount(field(rec, c.amount))
		row.Amount = math.Abs(v)
		if strings.HasPrefix(strings.ToLower(field(rec, c.drcr)), "c") {
			row.Direction = sharedModel.StatementCredit
		}
	default: // a signed amount
		v, _ := amount(field(rec, c.amount))
		row.Amount = math.Abs(v)
		if v > 0 {
			row.Direction = sharedModel.StatementCredit
		}
	}
	if row.Amount == 0 {
		return sharedModel.StatementRow{}, false
	}
	if b, ok := amount(field(rec, c.balance)); ok {
		row.Balance = &b
	}
	if n, ok := points(field(rec, c.rewards)); ok && n != 0 {
		row.RewardPoints = &n
	}
	return row, true
}

func field(rec []string, i int) string {
	if i < 0 || i >= len(rec) {
		return ""
	}
	return rec[i]
}

func blank(rec []string) bool { return strings.Join(rec, "") == "" }

// amount reads "78,000.00", "-727.00", "3,54,143.00", "407047.77 Cr".
func amount(s string) (float64, bool) {
	s = strings.NewReplacer(",", "", "₹", "", " ", "").Replace(strings.ToUpper(s))
	s = strings.TrimSuffix(strings.TrimSuffix(strings.TrimPrefix(s, "INR"), "CR"), "DR")
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	return round(v), err == nil
}

// points reads "+ 25", "- 10", "11,122".
func points(s string) (int, bool) {
	n, err := strconv.Atoi(strings.NewReplacer(" ", "", ",", "", "+", "").Replace(s))
	return n, err == nil
}

// dateLayouts are day-first, as Indian banks print them.
var dateLayouts = []string{
	"02/01/06", "02/01/2006", "2/1/06", "2/1/2006",
	"02-01-06", "02-01-2006", "02-Jan-06", "02-Jan-2006",
	"02 Jan 06", "02 Jan 2006", "02 Jan, 2006", "2006-01-02",
}

// parseDate reads a date, ignoring a time after it ("03/09/2026 16:45:49"),
// which the row reads on its own (ParseStatementTime).
func parseDate(s string) (string, bool) {
	for _, v := range []string{s, strings.SplitN(s, " ", 2)[0]} {
		for _, layout := range dateLayouts {
			if t, err := time.Parse(layout, v); err == nil {
				return t.Format("2006-01-02"), true
			}
		}
	}
	return "", false
}

// exportTime is the row's time. HDFC's export stamps lines without one (fees,
// GST, payments) "00:00:00", so a midnight to the second counts as none.
func exportTime(s string) string {
	if strings.Contains(s, "00:00:00") {
		return ""
	}
	return sharedModel.ParseStatementTime(s)
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func round(v float64) float64 { return math.Round(v*100) / 100 }
