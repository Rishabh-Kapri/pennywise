package localocr

import (
	"math"
	"regexp"
	"strconv"
	"strings"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

// A card statement lists its running EMIs in a table of their own: what was
// bought, the amount financed, the monthly instalment, the tenure, months
// left and principal outstanding. Its booking date and amount columns would
// otherwise read as a transaction, so it's recognised by its header first.

type emiColumn int

const (
	emiNone emiColumn = iota
	emiDescription
	emiBooked
	emiPrincipal
	emiMonthly
	emiRate
	emiTenure
	emiRemaining
	emiOutstanding
)

type emiLayout []emiColumn

func (l emiLayout) has(c emiColumn) bool {
	for _, x := range l {
		if x == c {
			return true
		}
	}
	return false
}

// emiHeaderLayout reads a row as an EMI table's header: it names a tenure (or
// months left) and an amount column.
func emiHeaderLayout(cells []string) (emiLayout, bool) {
	l := make(emiLayout, len(cells))
	for i, c := range cells {
		l[i] = emiHeaderColumn(c)
	}
	ok := (l.has(emiTenure) || l.has(emiRemaining)) &&
		(l.has(emiMonthly) || l.has(emiPrincipal) || l.has(emiOutstanding))
	return l, ok
}

// emiHeaderColumn names an EMI table header cell, most specific first:
// "Outstanding Principal" is what's left, "Principal Amount" what was lent.
func emiHeaderColumn(cell string) emiColumn {
	h := strings.ToLower(cell)
	has := func(words ...string) bool {
		for _, w := range words {
			if strings.Contains(h, w) {
				return true
			}
		}
		return false
	}
	switch {
	case h == "":
		return emiNone
	case has("remaining", "balance tenure", "pending", "left", "residual"):
		return emiRemaining
	case has("outstanding", "balance principal", "principal balance", "balance amount"):
		return emiOutstanding
	case has("tenure", "term", "no. of months", "number of months"):
		return emiTenure
	case has("rate", "roi", "%"):
		return emiRate
	case has("emi amount", "monthly", "instalment amount", "installment amount") || h == "emi":
		return emiMonthly
	case has("principal", "loan amount", "original", "financed", "purchase amount", "amount"):
		return emiPrincipal
	case has("date", "booked", "booking"):
		return emiBooked
	case has("description", "details", "merchant", "particulars", "loan", "plan", "product", "transaction"):
		return emiDescription
	}
	return emiNone
}

var monthsRe = regexp.MustCompile(`\d+`)

// plan reads one EMI table row. It is one when it states an amount.
func (l emiLayout) plan(cells []string) (sharedModel.StatementEMIPlan, bool) {
	var p sharedModel.StatementEMIPlan
	amount := func(cell string) *float64 {
		if v, m := parseAmount(cell); v > 0 || (m == markNone && strings.TrimSpace(cell) == "0") {
			return &v
		}
		return nil
	}
	months := func(cell string) *int {
		if m := monthsRe.FindString(cell); m != "" {
			if n, err := strconv.Atoi(m); err == nil {
				return &n
			}
		}
		return nil
	}
	for i, c := range l {
		cell := cells[i]
		switch c {
		case emiDescription:
			p.Description = strings.TrimSpace(p.Description + " " + cell)
		case emiBooked:
			p.BookedOn, _ = parseDate(cell)
		case emiPrincipal:
			p.Principal = amount(cell)
		case emiMonthly:
			p.MonthlyAmount = amount(cell)
		case emiRate:
			s := strings.NewReplacer("%", "", "p.a.", "", "pa", "", " ", "").Replace(strings.ToLower(cell))
			if v, err := strconv.ParseFloat(s, 64); err == nil && v > 0 && v < 100 {
				v = math.Round(v*100) / 100
				p.InterestRate = &v
			}
		case emiTenure:
			p.TenureMonths = months(cell)
		case emiRemaining:
			p.RemainingMonths = months(cell)
		case emiOutstanding:
			p.Outstanding = amount(cell)
		}
	}
	ok := p.Principal != nil || p.MonthlyAmount != nil || p.Outstanding != nil
	return p, ok
}
