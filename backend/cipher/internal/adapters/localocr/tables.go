package localocr

import (
	"math"
	"strconv"
	"strings"
	"time"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// The OCR model answers each page with HTML tables. tableParser turns those
// tables into statement rows, one page at a time. It remembers the column
// layout of the last header it saw, because a continuation page often repeats
// the table without its header row.

// column is what a table column holds.
type column int

const (
	colNone column = iota
	colDate
	colDescription
	colDebit
	colCredit
	colAmount // one amount column, direction from a Cr/Dr marker
	colBalance
	colReference
	colDirection // a cell of its own saying Cr or Dr
	colPoints    // reward points on a card statement
)

// layout maps a column index to what it holds.
type layout []column

func (l layout) has(c column) bool {
	for _, x := range l {
		if x == c {
			return true
		}
	}
	return false
}

type tableParser struct {
	layout layout // from the last header row; nil before the first one
	rows   []sharedModel.StatementRow
	header statementHeader
	plans  []sharedModel.StatementEMIPlan
}

// rowContext is what reading a row needs from the statement as a whole.
type rowContext struct {
	// negativeIsCredit: a minus sign on a single amount column marks a
	// payment or refund on a card statement, but a withdrawal on a bank one.
	negativeIsCredit bool
	// ref fills in the year of a date printed without one.
	ref time.Time
}

func (p *tableParser) rowContext() rowContext {
	return rowContext{
		negativeIsCredit: p.header.kind != sharedModel.StatementKindBank,
		ref:              p.header.refDate(),
	}
}

// parsePage adds the transaction rows found in one page's OCR output.
func (p *tableParser) parsePage(page int, text string) {
	doc, err := html.Parse(strings.NewReader(text))
	if err != nil {
		return // html.Parse only fails on a reader error
	}
	for _, table := range findAll(doc, atom.Table) {
		// A row only continues a narration within the same table.
		last := -1
		var emi emiLayout // set once this table turns out to list EMIs
		for _, cells := range tableCells(table) {
			if l, ok := emiHeaderLayout(cells); ok {
				emi = l
				continue
			}
			if emi != nil {
				if len(cells) == len(emi) {
					if plan, ok := emi.plan(cells); ok {
						p.plans = append(p.plans, plan)
					}
				}
				continue
			}
			if l, ok := headerLayout(cells); ok {
				p.layout = l
				last = -1
				continue
			}
			if p.layout == nil || len(cells) != len(p.layout) {
				continue // a summary or address table, not transactions
			}
			if p.layout.applySubHeader(cells) {
				continue
			}
			row, ok := p.layout.row(cells, p.rowContext())
			if !ok {
				// A narration that wrapped onto its own row: no date or
				// amount, only more text for the row above.
				if extra := p.layout.continuation(cells); extra != "" && last >= 0 {
					p.rows[last].Description = strings.TrimSpace(p.rows[last].Description + " " + extra)
				}
				continue
			}
			row.Page = page
			p.rows = append(p.rows, row)
			last = len(p.rows) - 1
		}
	}
}

// result returns everything parsed so far as one section, with what the
// first page said about the account.
func (p *tableParser) result() sharedModel.StatementExtractResponse {
	if len(p.rows) == 0 {
		return sharedModel.StatementExtractResponse{Sections: []sharedModel.StatementSection{}}
	}
	section := sharedModel.StatementSection{Rows: p.rows, EMIPlans: p.plans}
	p.header.apply(&section)
	return sharedModel.StatementExtractResponse{Sections: []sharedModel.StatementSection{section}}
}

// headerLayout reads a row as a header. It is one when it names a date column
// and at least one amount column.
func headerLayout(cells []string) (layout, bool) {
	l := make(layout, len(cells))
	for i, c := range cells {
		l[i] = headerColumn(c)
	}
	// "Txn Date" and "Value Date" side by side: keep the first as the date.
	seenDate := false
	for i, c := range l {
		if c == colDate {
			if seenDate {
				l[i] = colNone
			}
			seenDate = true
		}
	}
	ok := seenDate && (l.has(colAmount) || l.has(colDebit) || l.has(colCredit))
	return l, ok
}

// applySubHeader handles a header split over two rows: "Amount" spanning two
// columns, then "Debit | Credit" under it. A row whose filled cells are all
// header words, with a debit or credit among them, refines the layout.
func (l layout) applySubHeader(cells []string) bool {
	found := false
	for _, cell := range cells {
		if cell == "" {
			continue
		}
		switch headerColumn(cell) {
		case colNone:
			return false
		case colDebit, colCredit:
			found = true
		}
	}
	if !found {
		return false
	}
	for i, cell := range cells {
		if c := headerColumn(cell); c != colNone {
			l[i] = c
		}
	}
	return true
}

// headerColumn names a header cell. The checks go from most to least
// specific: "Withdrawal Amt." is a debit, not a plain amount, and
// "Closing Balance" a balance.
func headerColumn(cell string) column {
	h := strings.ToLower(cell)
	has := func(words ...string) bool {
		for _, w := range words {
			if strings.Contains(h, w) {
				return true
			}
		}
		return false
	}
	both := func(a, b string) bool { return strings.Contains(h, a) && strings.Contains(h, b) }
	switch {
	case h == "":
		return colNone
	// "Cr/Dr", "Dr / Cr", "Debit/Credit": a marker column. "Amount (Dr/Cr)"
	// is an amount whose cells carry the marker, handled by parseAmount.
	case (both("cr", "dr") || both("debit", "credit")) && !has("amount", "amt") && len(h) <= 14:
		return colDirection
	case h == "type" || h == "txn type" || h == "transaction type":
		return colDirection
	case has("date", "dt"):
		return colDate
	case has("balance"):
		return colBalance
	case has("debit", "withdrawal", "paid out") || h == "dr" || h == "dr.":
		return colDebit
	case has("credit", "deposit", "paid in") || h == "cr" || h == "cr.":
		return colCredit
	case has("point", "reward"):
		return colPoints
	case has("ref", "chq", "cheque", "check no"):
		return colReference
	case has("amount", "amt"):
		return colAmount
	case has("description", "narration", "particulars", "details", "remarks", "transaction"):
		return colDescription
	}
	return colNone
}

// row reads cells as a transaction. It is one when it has a date and a
// non-zero amount; opening balance and total lines fail that.
func (l layout) row(cells []string, rc rowContext) (sharedModel.StatementRow, bool) {
	var (
		r             sharedModel.StatementRow
		date          string
		debit, credit float64
		amount        float64
		amountMark    mark
		directionMark mark
	)
	for i, c := range l {
		cell := cells[i]
		switch c {
		case colDate:
			date = cell
		case colDescription:
			r.Description = strings.TrimSpace(r.Description + " " + cell)
		case colDebit:
			debit, _ = parseAmount(cell)
		case colCredit:
			credit, _ = parseAmount(cell)
		case colAmount:
			amount, amountMark = parseAmount(cell)
		case colDirection:
			directionMark = parseMark(cell)
		case colPoints:
			if n, ok := parsePoints(cell); ok {
				r.RewardPoints = &n
			}
		case colBalance:
			if v, ok := parseBalance(cell); ok {
				r.Balance = &v
			}
		case colReference:
			r.Reference = cell
		}
	}

	d, ok := parseDate(date)
	if !ok {
		d, ok = parseDateNoYear(date, rc.ref)
	}
	if !ok {
		return r, false
	}
	r.Date = d

	// A Cr/Dr cell of its own outranks a marker inside the amount cell.
	if directionMark != markNone {
		amountMark = directionMark
	}
	switch {
	case debit > 0:
		r.Amount, r.Direction = debit, sharedModel.StatementDebit
	case credit > 0:
		r.Amount, r.Direction = credit, sharedModel.StatementCredit
	case amount > 0:
		r.Amount, r.Direction = amount, sharedModel.StatementDebit
		if amountMark == markCredit || (amountMark == markNegative && rc.negativeIsCredit) {
			r.Direction = sharedModel.StatementCredit
		}
	default:
		return r, false
	}
	return r, true
}

// continuation returns the description text of a row that has nothing else.
func (l layout) continuation(cells []string) string {
	var text []string
	for i, c := range l {
		cell := cells[i]
		if cell == "" {
			continue
		}
		if c != colDescription {
			return ""
		}
		text = append(text, cell)
	}
	return strings.Join(text, " ")
}

// mark is what an amount said about its direction.
type mark int

const (
	markNone mark = iota
	markCredit
	markDebit
	markNegative // "-1,234.00": a credit on a card, a debit on a bank account
)

// parseMark reads a Cr/Dr cell of its own.
func parseMark(cell string) mark {
	switch strings.Trim(strings.ToLower(strings.TrimSpace(cell)), ".") {
	case "cr", "c", "credit", "+":
		return markCredit
	case "dr", "d", "debit", "-":
		return markDebit
	}
	return markNone
}

// parseAmount reads "1,234.50", "₹ 1,234.50 Cr", "Cr 1,234.50", "-1234.5" or
// "(1,234.50)". The mark says how the cell flagged the direction: a card
// statement flags payments and refunds and leaves purchases unmarked or Dr.
func parseAmount(cell string) (v float64, m mark) {
	s := strings.ToLower(strings.TrimSpace(cell))
	s = strings.TrimSuffix(s, ".")
	switch {
	case strings.HasSuffix(s, "cr"), strings.HasPrefix(s, "cr "), strings.HasPrefix(s, "+"):
		m = markCredit
	case strings.HasSuffix(s, "dr"), strings.HasPrefix(s, "dr "):
		m = markDebit
	}
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(s, "cr"), "dr"))
	s = strings.TrimPrefix(strings.TrimPrefix(s, "cr "), "dr ")
	s = strings.NewReplacer(",", "", " ", "", "₹", "", "$", "", "rs.", "", "rs", "", "inr", "", "+", "").Replace(s)
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		s = "-" + strings.Trim(s, "()")
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, markNone
	}
	if f < 0 && m == markNone {
		m = markNegative
	}
	return math.Abs(f), m
}

// parsePoints reads a reward points cell: "120", "-12", "+1,200".
func parsePoints(cell string) (int, bool) {
	s := strings.NewReplacer(",", "", " ", "", "+", "").Replace(strings.TrimSpace(cell))
	if s == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return int(math.Round(f)), true
}

func parseBalance(cell string) (float64, bool) {
	if strings.TrimSpace(cell) == "" {
		return 0, false
	}
	s := strings.NewReplacer(",", "", " ", "", "₹", "").Replace(strings.ToLower(cell))
	s = strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(s, "."), "cr"), "dr")
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return math.Abs(f), true
}

// dateLayouts are the formats Indian statements print, day first.
var dateLayouts = []string{
	"02/01/2006", "02-01-2006", "02.01.2006",
	"02/01/06", "02-01-06", "02.01.06",
	"2/1/2006", "2-1-2006",
	"02 Jan 2006", "02-Jan-2006", "02 Jan 06", "02-Jan-06", "02/Jan/2006",
	"2 Jan 2006", "02 January 2006", "2 January 2006", "Jan 02, 2006", "Jan 2, 2006",
	"January 02, 2006", "January 2, 2006", "02Jan2006", "02 Jan, 2006",
	"2006-01-02",
}

// dateLayoutsNoYear are for statements that print "03 Aug" or "03/08" and
// leave the year to the period at the top.
var dateLayoutsNoYear = []string{"02/01", "02-01", "02 Jan", "02-Jan", "2 Jan", "Jan 02", "02Jan"}

// parseDateNoYear reads a date without a year, taking the year from ref (the
// end of the statement period). A month after ref's is from the year before:
// a period ending 05 Jan 2026 holds "28 Dec" of 2025.
func parseDateNoYear(cell string, ref time.Time) (string, bool) {
	if ref.IsZero() {
		return "", false
	}
	fields := strings.Fields(cell)
	if len(fields) == 0 {
		return "", false
	}
	candidates := []string{fields[0]}
	if len(fields) >= 2 {
		candidates = append([]string{fields[0] + " " + fields[1]}, candidates...)
	}
	for _, c := range candidates {
		for _, l := range dateLayoutsNoYear {
			t, err := time.Parse(l, c)
			if err != nil {
				continue
			}
			year := ref.Year()
			if t.Month() > ref.Month() {
				year--
			}
			d := time.Date(year, t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
			if d.Month() != t.Month() { // 29 Feb in a year without one
				return "", false
			}
			return d.Format("2006-01-02"), true
		}
	}
	return "", false
}

// parseDate returns the date as YYYY-MM-DD. A time after the date
// ("01/10/2025 10:23:11") is ignored. A date without a year is not read.
func parseDate(cell string) (string, bool) {
	fields := strings.Fields(cell)
	if len(fields) == 0 {
		return "", false
	}
	// Try the whole cell, then the first three words ("02 Jan 2006 10:23"),
	// then the first word ("02/01/2006 10:23").
	candidates := []string{strings.Join(fields, " ")}
	if len(fields) > 3 {
		candidates = append(candidates, strings.Join(fields[:3], " "))
	}
	candidates = append(candidates, fields[0])
	for _, c := range candidates {
		for _, l := range dateLayouts {
			if t, err := time.Parse(l, c); err == nil {
				return t.Format("2006-01-02"), true
			}
		}
	}
	return "", false
}

// findAll returns every element of type a under n, in document order.
func findAll(n *html.Node, a atom.Atom) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.DataAtom == a {
			out = append(out, n)
			return // a table inside a table is read as part of its cell
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return out
}

// tableCells returns each row's cell texts. A cell spanning several columns
// is repeated, so every row lines up with the header.
func tableCells(table *html.Node) [][]string {
	var rows [][]string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.DataAtom == atom.Tr {
			var cells []string
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type != html.ElementNode || (c.DataAtom != atom.Td && c.DataAtom != atom.Th) {
					continue
				}
				text := cellText(c)
				span := 1
				for _, a := range c.Attr {
					if a.Key == "colspan" {
						if v, err := strconv.Atoi(a.Val); err == nil && v > 1 && v < 50 {
							span = v
						}
					}
				}
				for range span {
					cells = append(cells, text)
				}
			}
			rows = append(rows, cells)
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(table)
	return rows
}

// cellText is the cell's text with whitespace and line breaks collapsed.
func cellText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		switch {
		case n.Type == html.TextNode:
			b.WriteString(n.Data)
		case n.Type == html.ElementNode && n.DataAtom == atom.Br:
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if n.Type == html.ElementNode && (n.DataAtom == atom.P || n.DataAtom == atom.Div) {
			b.WriteByte(' ')
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}
