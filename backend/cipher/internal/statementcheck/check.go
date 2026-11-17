// Package statementcheck decides whether a statement read checks out: do the
// rows add up to the balances and totals the statement printed? It doesn't
// care which reader produced the rows, so the statement pipeline runs it
// after its extract step to decide whether to run the fallback, and to keep
// the better of the two reads.
package statementcheck

import (
	"fmt"
	"math"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

// tolerance absorbs rounding in printed balances.
const tolerance = 0.5

// Result is what the check found. Reasons say what failed, by section number
// and never by amount: they're logged, and amounts are the user's.
type Result struct {
	Rows int
	// Sections with rows; Verified of them had printed balances or totals to
	// check against and matched them.
	Sections int
	Verified int
	// GoodRows are the rows in verified sections.
	GoodRows int
	// Mismatched sections printed balances the rows don't add up to.
	// BalanceBreaks are rows whose running balance doesn't follow from the
	// row before.
	Mismatched    int
	BalanceBreaks int
	Reasons       []string
}

// OK: there are rows, and every section with rows was checked against what
// the statement printed and matched. A section that printed nothing to check
// against isn't OK: the rows can't be trusted yet.
func (r Result) OK() bool {
	return r.Rows > 0 && r.Verified == r.Sections && r.Mismatched == 0 && r.BalanceBreaks == 0
}

// Check checks every section of res.
func Check(res sharedModel.StatementExtractResponse) Result {
	var r Result
	for i, s := range res.Sections {
		if len(s.Rows) == 0 {
			continue
		}
		r.Sections++
		r.Rows += len(s.Rows)
		name := fmt.Sprintf("section %d", i+1)

		totals, totalsOK := checkTotals(s)
		breaks, balances := checkRunningBalance(s)
		r.BalanceBreaks += breaks
		if breaks > 0 {
			r.Reasons = append(r.Reasons, fmt.Sprintf("%s: %d rows break the running balance", name, breaks))
		}
		switch {
		case totals && !totalsOK:
			r.Mismatched++
			r.Reasons = append(r.Reasons, name+": rows don't add up to the opening and closing balance")
		case !totals && !balances:
			r.Reasons = append(r.Reasons, name+": nothing printed to check the rows against")
		case breaks == 0:
			r.Verified++
			r.GoodRows += len(s.Rows)
		}
	}
	if r.Rows == 0 {
		r.Reasons = append(r.Reasons, "no rows")
	}
	return r
}

// Better reports whether a checks out better than b: OK beats not OK, some
// rows beat none, then more rows in verified sections, then fewer problems,
// then more rows.
func Better(a, b Result) bool {
	if a.OK() != b.OK() {
		return a.OK()
	}
	if (a.Rows > 0) != (b.Rows > 0) {
		return a.Rows > 0
	}
	if a.GoodRows != b.GoodRows {
		return a.GoodRows > b.GoodRows
	}
	if pa, pb := a.Mismatched+a.BalanceBreaks, b.Mismatched+b.BalanceBreaks; pa != pb {
		return pa < pb
	}
	return a.Rows > b.Rows
}

// checkTotals compares the rows with the printed opening and closing balance
// (a card's total due when it printed no closing balance). checked is false
// when the section printed neither pair.
//
// A bank balance grows with credits; a card's (what's owed) grows with
// debits. When the reader didn't say which kind it is, either one matching is
// enough.
func checkTotals(s sharedModel.StatementSection) (checked, ok bool) {
	closing := s.ClosingBalance
	if closing == nil && s.Kind != sharedModel.StatementKindBank {
		closing = s.TotalDue
	}
	if s.OpeningBalance == nil || closing == nil {
		return false, false
	}
	net := 0.0
	for _, row := range s.Rows {
		net += row.SignedAmount()
	}
	bank := near(*s.OpeningBalance+net, *closing)
	card := near(*s.OpeningBalance-net, *closing)
	switch s.Kind {
	case sharedModel.StatementKindBank:
		return true, bank
	case sharedModel.StatementKindCard:
		return true, card
	default:
		return true, bank || card
	}
}

// checkRunningBalance follows the balance printed on each row from the one
// before it (or from the opening balance). checked is false when fewer than
// two balances were printed.
func checkRunningBalance(s sharedModel.StatementSection) (breaks int, checked bool) {
	sign := 0.0 // +1 bank, -1 card, 0 not known yet
	switch s.Kind {
	case sharedModel.StatementKindBank:
		sign = 1
	case sharedModel.StatementKindCard:
		sign = -1
	}

	prev := s.OpeningBalance
	pairs := 0
	for _, row := range s.Rows {
		if row.Balance == nil {
			prev = nil // can't follow across a row with no balance
			continue
		}
		if prev != nil {
			pairs++
			bank := near(*prev+row.SignedAmount(), *row.Balance)
			card := near(*prev-row.SignedAmount(), *row.Balance)
			if sign == 0 && bank != card {
				// The first pair that tells them apart decides the kind.
				if bank {
					sign = 1
				} else {
					sign = -1
				}
			}
			if !((sign >= 0 && bank) || (sign <= 0 && card)) {
				breaks++
			}
		}
		prev = row.Balance
	}
	return breaks, pairs > 0
}

func near(a, b float64) bool {
	return math.Abs(a-b) <= tolerance
}
