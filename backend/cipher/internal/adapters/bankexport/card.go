package bankexport

import (
	"strconv"
	"strings"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

// summary reads a line outside the transactions: HDFC's card export prints
// "key~|~value" lines, then small tables whose values sit on the line after
// their header.
func (p *parser) summary(rec []string) {
	s := &p.s
	key := strings.ToLower(rec[0])
	value := field(rec, 1)

	switch p.next {
	case "account":
		// Opening Bal - Payment / Credit + Purchases / Debits + Finance Charges = Total Dues
		p.next = ""
		var n []float64
		for _, f := range rec {
			if v, ok := amount(f); ok {
				n = append(n, v)
			}
		}
		if len(n) == 5 {
			// Computed, not the printed total: that is rounded to the rupee.
			closing := round(n[0] - n[1] + n[2] + n[3])
			s.OpeningBalance, s.ClosingBalance = &n[0], &closing
		}
		return
	case "points":
		p.next = ""
		r := sharedModel.StatementRewards{}
		for i, dst := range []**int{&r.Opening, &r.Earned, &r.Redeemed, &r.Expired, &r.Closing} {
			if v, ok := points(field(rec, i)); ok {
				*dst = &v
			}
		}
		s.Rewards = &r
		return
	case "loans":
		if plan, ok := emiPlan(rec); ok {
			s.EMIPlans = append(s.EMIPlans, plan)
			return
		}
		p.next = ""
	}

	switch {
	case strings.HasPrefix(key, "card no"):
		_, number, _ := strings.Cut(rec[0], ":")
		s.AccountNumber = strings.TrimSpace(number)
	case key == "payment due date":
		s.DueDate = date(value)
	case key == "statement date":
		s.StatementDate = date(value)
	case key == "total amount due":
		s.TotalDue = money(value)
	case key == "minimum amount due":
		s.MinimumDue = money(value)
	case key == "credit limit":
		s.CreditLimit = money(value)
	case key == "opening bal" && containsAny(strings.ToLower(strings.Join(rec, " ")), "total dues"):
		p.next = "account"
	case key == "opening balance" && containsAny(strings.ToLower(strings.Join(rec, " ")), "reward points earned"):
		p.next = "points"
	case key == "loan type":
		p.next = "loans"
	}
}

// emiPlan reads a Loan Summary row: type, number, booked on, amount,
// tenure, rate, principal left, interest left, months left.
func emiPlan(rec []string) (sharedModel.StatementEMIPlan, bool) {
	booked, ok := parseDate(field(rec, 2))
	if !ok || len(rec) < 9 {
		return sharedModel.StatementEMIPlan{}, false
	}
	plan := sharedModel.StatementEMIPlan{
		Description:     strings.TrimSpace(rec[0] + " " + rec[1]),
		BookedOn:        booked,
		Principal:       money(rec[3]),
		TenureMonths:    months(rec[4]),
		Outstanding:     money(rec[6]),
		RemainingMonths: months(rec[8]),
	}
	if v, err := strconv.ParseFloat(strings.TrimSuffix(rec[5], "%"), 64); err == nil {
		plan.InterestRate = &v
	}
	return plan, true
}

func money(s string) *float64 {
	if v, ok := amount(s); ok {
		return &v
	}
	return nil
}

func date(s string) *string {
	if d, ok := parseDate(s); ok {
		return &d
	}
	return nil
}

// months reads "9 Months", "1 Month".
func months(s string) *int {
	if n, err := strconv.Atoi(strings.Fields(s + " x")[0]); err == nil {
		return &n
	}
	return nil
}
