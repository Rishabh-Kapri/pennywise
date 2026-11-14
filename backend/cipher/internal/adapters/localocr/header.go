package localocr

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// A statement's first page says whose statement it is: the masked account or
// card number, the period, balances and, on a card, what's due and the reward
// points. The OCR model reads page 1 twice for it ("Text Recognition:" for the
// prose, "Table Recognition:" for the summary boxes) and statementHeader is
// what the fixed rules below find in both. Nothing here guesses: a field the
// page didn't state stays empty.

type statementHeader struct {
	kind          string // sharedModel.StatementKindCard or StatementKindBank, or ""
	label         string
	accountNumber string
	currency      string

	periodStart, periodEnd, statementDate, dueDate string // YYYY-MM-DD

	opening, closing, totalDue, minimumDue, creditLimit *float64
	rewards                                             *sharedModel.StatementRewards
}

// headerLines turns OCR output (plain lines, markdown tables or HTML tables)
// into "label : value" lines the rules can read. A summary box is often a row
// of labels over a row of values; those are zipped into one line per pair.
func headerLines(text string) []string {
	var lines []string
	addTable := func(rows [][]string) {
		pointsTable := false
		for _, cells := range rows {
			if strings.Contains(strings.ToLower(strings.Join(cells, " ")), "point") {
				pointsTable = true
			}
		}
		prefix := ""
		if pointsTable {
			prefix = "points "
		}
		for i, cells := range rows {
			lines = append(lines, prefix+strings.Join(cells, " : "))
			if i+1 < len(rows) && len(rows[i+1]) == len(cells) && allLabels(cells) && anyAmount(rows[i+1]) {
				for j, label := range cells {
					if label != "" && rows[i+1][j] != "" {
						lines = append(lines, prefix+label+" : "+rows[i+1][j])
					}
				}
			}
		}
	}

	if strings.Contains(strings.ToLower(text), "<table") {
		if doc, err := html.Parse(strings.NewReader(text)); err == nil {
			for _, table := range findAll(doc, atom.Table) {
				addTable(tableCells(table))
			}
			// Text outside the tables.
			var walk func(*html.Node)
			walk = func(n *html.Node) {
				if n.Type == html.ElementNode && n.DataAtom == atom.Table {
					return
				}
				if n.Type == html.TextNode {
					for _, l := range strings.Split(n.Data, "\n") {
						if l = cleanLine(l); l != "" {
							lines = append(lines, l)
						}
					}
				}
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					walk(c)
				}
			}
			walk(doc)
		}
		return lines
	}

	var table [][]string
	flush := func() {
		if len(table) > 0 {
			addTable(table)
			table = nil
		}
	}
	for _, raw := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(raw)
		if strings.HasPrefix(trimmed, "|") {
			cells := strings.Split(strings.Trim(trimmed, "|"), "|")
			if isMarkdownRule(cells) {
				continue
			}
			for i := range cells {
				cells[i] = cleanLine(cells[i])
			}
			table = append(table, cells)
			continue
		}
		flush()
		if l := cleanLine(raw); l != "" {
			lines = append(lines, l)
		}
	}
	flush()
	return lines
}

func cleanLine(s string) string {
	s = strings.NewReplacer("**", "", "__", "", "`", "").Replace(s)
	s = strings.TrimLeft(strings.TrimSpace(s), "#>-* ")
	return strings.Join(strings.Fields(s), " ")
}

func isMarkdownRule(cells []string) bool {
	for _, c := range cells {
		if strings.Trim(strings.TrimSpace(c), ":-") != "" {
			return false
		}
	}
	return true
}

// allLabels: no cell is a number or a date, so the row can head a box.
func allLabels(cells []string) bool {
	for _, c := range cells {
		if valueCellRe.MatchString(c) || dateCellRe.MatchString(c) {
			return false
		}
	}
	return true
}

func anyAmount(cells []string) bool {
	for _, c := range cells {
		if valueCellRe.MatchString(c) {
			return true
		}
	}
	return false
}

const datePattern = `(\d{1,2}[/.\-]\d{1,2}[/.\-]\d{2,4}|\d{1,2}[\s\-/]?[A-Za-z]{3,9}[\s\-/,]*\d{2,4}|[A-Za-z]{3,9}\s+\d{1,2},?\s+\d{4}|\d{4}-\d{2}-\d{2})`

var (
	// valueCellRe is a cell holding only a number: "₹12,345.67", "1,150", "620.00 Cr".
	valueCellRe = regexp.MustCompile(`(?i)^\s*-?\s*(?:rs\.?|inr|₹|\$)?\s*-?\d[\d,]*(?:\.\d{1,2})?\s*(?:cr|dr)?\s*$`)
	dateCellRe  = regexp.MustCompile(`^\s*` + datePattern + `\s*$`)
	// valueRe is the first number after a label: "₹ 9,51,406.80 Cr", "1234".
	valueRe = regexp.MustCompile(`(?i)^[\s:=\-–]*(?:rs\.?|inr|₹|\$)?\s*(-?\d[\d,]*(?:\.\d{1,2})?)\s*(cr|dr)?\b`)

	periodRe = regexp.MustCompile(`(?i)(?:period|statement\s+from|from)\s*(?:[:\-]|of)?\s*` + datePattern +
		`\s*(?:to|till|until|-|–|—)\s*` + datePattern)
	statementDateRe = regexp.MustCompile(`(?i)statement\s+(?:generation\s+)?date\s*[:\-]?\s*` + datePattern)
	dueDateRe       = regexp.MustCompile(`(?i)(?:payment\s+)?due\s+(?:date|by)\s*[:\-]?\s*` + datePattern)

	// accountLabelRe finds a labelled number: "Account Number : 5010...",
	// "Card No: XXXX XXXX XXXX 1234". The number may be masked.
	accountLabelRe = regexp.MustCompile(`(?i)\b(card|account|a/c)\s*(?:no\.?|number|num\.?|#)?\s*[:\-]?\s*([0-9Xx*•#][0-9Xx*•#\s\-]{2,30}\d{3,4})\b`)
	// maskedCardRe finds a card number printed on its own.
	maskedCardRe = regexp.MustCompile(`\b(\d{4}[\s\-]?[Xx*•]{2,4}[\s\-]?[Xx*•]{4}[\s\-]?\d{4}|(?:[Xx*•]{4}[\s\-]?){2,3}\d{4}|\d{6}[Xx*•]{6}\d{4})\b`)
)

// parseHeader reads the first page's lines.
func parseHeader(lines []string) statementHeader {
	var h statementHeader
	all := strings.ToLower(strings.Join(lines, "\n"))

	for _, l := range lines {
		if h.periodStart == "" {
			if m := periodRe.FindStringSubmatch(l); m != nil {
				start, okStart := parseDate(m[1])
				end, okEnd := parseDate(m[2])
				if okStart && okEnd && start <= end {
					h.periodStart, h.periodEnd = start, end
				}
			}
		}
		if h.statementDate == "" {
			if m := statementDateRe.FindStringSubmatch(l); m != nil {
				h.statementDate, _ = parseDate(m[1])
			}
		}
		if h.dueDate == "" {
			if m := dueDateRe.FindStringSubmatch(l); m != nil {
				h.dueDate, _ = parseDate(m[1])
			}
		}
	}

	h.accountNumber, h.kind = findAccountNumber(lines)
	if h.kind == "" {
		h.kind = guessKind(all)
	}
	switch {
	case h.kind == sharedModel.StatementKindCard:
		h.label = "Credit card"
	case strings.Contains(all, "savings"):
		h.label = "Savings account"
	case strings.Contains(all, "current account"):
		h.label = "Current account"
	case h.kind == sharedModel.StatementKindBank:
		h.label = "Bank account"
	}
	if strings.Contains(all, "inr") || strings.Contains(all, "₹") || strings.Contains(all, "rs.") {
		h.currency = "INR"
	}

	for _, l := range lines {
		lower := strings.ToLower(l)
		if strings.Contains(lower, "point") {
			h.readRewards(lower)
			continue
		}
		setIfNil(&h.opening, labelledAmount(lower, []string{"opening balance", "previous balance", "previous statement balance", "last statement balance"}, nil))
		setIfNil(&h.closing, labelledAmount(lower, []string{"closing balance"}, nil))
		setIfNil(&h.totalDue, labelledAmount(lower, []string{"total amount due", "total dues", "total due", "total outstanding", "total payment due"}, nil))
		setIfNil(&h.minimumDue, labelledAmount(lower, []string{"minimum amount due", "minimum due", "minimum payment due", "min. amount due", "min amount due"}, nil))
		setIfNil(&h.creditLimit, labelledAmount(lower, []string{"credit limit", "total credit limit"}, []string{"available", "cash"}))
	}
	// A card's closing balance is what it says is owed.
	if h.kind == sharedModel.StatementKindCard && h.closing == nil {
		h.closing = h.totalDue
	}
	return h
}

// findAccountNumber prefers a labelled card number, then a printed masked
// card number, then a labelled account number. The kind follows from which.
func findAccountNumber(lines []string) (number, kind string) {
	var account string
	for _, l := range lines {
		for _, m := range accountLabelRe.FindAllStringSubmatch(l, -1) {
			n := strings.TrimSpace(m[2])
			if digitsIn(n) < 4 {
				continue
			}
			if strings.EqualFold(m[1], "card") {
				return n, sharedModel.StatementKindCard
			}
			if account == "" {
				account = n
			}
		}
	}
	for _, l := range lines {
		if m := maskedCardRe.FindString(l); m != "" {
			return m, sharedModel.StatementKindCard
		}
	}
	if account != "" {
		return account, ""
	}
	return "", ""
}

func guessKind(all string) string {
	for _, w := range []string{"credit card", "minimum amount due", "total amount due", "payment due date", "credit limit"} {
		if strings.Contains(all, w) {
			return sharedModel.StatementKindCard
		}
	}
	for _, w := range []string{"savings", "current account", "withdrawal", "deposit", "ifsc"} {
		if strings.Contains(all, w) {
			return sharedModel.StatementKindBank
		}
	}
	return ""
}

func (h *statementHeader) readRewards(lower string) {
	if h.rewards == nil {
		h.rewards = &sharedModel.StatementRewards{}
	}
	r := h.rewards
	setIntIfNil(&r.Opening, labelledAmount(lower, []string{"opening balance", "previous balance", "opening", "previous"}, nil))
	setIntIfNil(&r.Earned, labelledAmount(lower, []string{"earned", "accrued"}, nil))
	setIntIfNil(&r.Redeemed, labelledAmount(lower, []string{"redeemed", "redemption"}, nil))
	setIntIfNil(&r.Expired, labelledAmount(lower, []string{"expired", "lapsed", "expiring"}, nil))
	setIntIfNil(&r.Closing, labelledAmount(lower,
		[]string{"closing balance", "available balance", "closing", "available", "balance"},
		[]string{"opening", "previous"}))
	if *r == (sharedModel.StatementRewards{}) {
		h.rewards = nil
	}
}

// labelledAmount finds one of labels in line and reads the number right after
// it. A label preceded by one of skip ("available credit limit") is ignored.
func labelledAmount(line string, labels, skip []string) *float64 {
	for _, label := range labels {
		from := 0
		for {
			i := strings.Index(line[from:], label)
			if i < 0 {
				break
			}
			i += from
			from = i + len(label)
			if precededBy(line[:i], skip) {
				continue
			}
			rest := line[i+len(label):]
			// Allow a unit or note between label and value: "(in rs.)", "(₹)".
			if j := strings.Index(rest, ")"); j >= 0 && j < 12 && strings.HasPrefix(strings.TrimSpace(rest), "(") {
				rest = rest[j+1:]
			}
			m := valueRe.FindStringSubmatch(rest)
			if m == nil {
				continue
			}
			v, err := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", ""), 64)
			if err != nil {
				continue
			}
			return &v
		}
	}
	return nil
}

func precededBy(before string, words []string) bool {
	before = strings.TrimSpace(before)
	for _, w := range words {
		if strings.HasSuffix(before, w) {
			return true
		}
	}
	return false
}

func setIfNil(dst **float64, v *float64) {
	if *dst == nil && v != nil {
		*dst = v
	}
}

func setIntIfNil(dst **int, v *float64) {
	if *dst == nil && v != nil {
		n := int(math.Round(*v))
		*dst = &n
	}
}

func digitsIn(s string) int {
	n := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			n++
		}
	}
	return n
}

// apply copies the header onto the section built from the tables.
func (h statementHeader) apply(s *sharedModel.StatementSection) {
	s.Kind = h.kind
	s.AccountLabel = h.label
	s.AccountNumber = h.accountNumber
	s.Currency = h.currency
	s.PeriodStart = strPtr(h.periodStart)
	s.PeriodEnd = strPtr(h.periodEnd)
	s.StatementDate = strPtr(h.statementDate)
	s.DueDate = strPtr(h.dueDate)
	s.OpeningBalance = h.opening
	s.ClosingBalance = h.closing
	s.TotalDue = h.totalDue
	s.MinimumDue = h.minimumDue
	s.CreditLimit = h.creditLimit
	s.Rewards = h.rewards
}

// refDate is the date that fills in a year a row didn't print: the end of
// the period, else the statement date. Zero when neither is known.
func (h statementHeader) refDate() time.Time {
	for _, d := range []string{h.periodEnd, h.statementDate} {
		if t, err := time.Parse("2006-01-02", d); err == nil {
			return t
		}
	}
	return time.Time{}
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
