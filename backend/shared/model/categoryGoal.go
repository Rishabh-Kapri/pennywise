package model

import (
	"errors"
	"fmt"
	"time"
)

// GoalType mirrors YNAB's target types (its API's goal_type values).
type GoalType string

const (
	// GoalTypeNeed is "needed for spending": an amount due on a cadence
	// (weekly, monthly, yearly, every N months, or once by a date).
	GoalTypeNeed GoalType = "NEED"
	// GoalTypeTargetBalance is "savings balance": keep the available balance at
	// or above Target, with no deadline.
	GoalTypeTargetBalance GoalType = "TB"
	// GoalTypeTargetBalanceByDate is "savings balance by date": reach Target by
	// TargetMonth, spread evenly over the months left.
	GoalTypeTargetBalanceByDate GoalType = "TBD"
	// GoalTypeMonthlyFunding is "monthly savings builder": assign Target every
	// month regardless of the balance.
	GoalTypeMonthlyFunding GoalType = "MF"
	// GoalTypeDebt is "monthly debt payment": assign Target every month toward
	// a debt.
	GoalTypeDebt GoalType = "DEBT"
)

// GoalCadence is how often a NEED target repeats.
type GoalCadence string

const (
	GoalCadenceNone    GoalCadence = "none" // once, by TargetMonth
	GoalCadenceWeekly  GoalCadence = "weekly"
	GoalCadenceMonthly GoalCadence = "monthly"
	GoalCadenceYearly  GoalCadence = "yearly"
)

const maxGoalCadenceFrequency = 120

// maxGoalTarget keeps targets inside the NUMERIC(12,2) range amounts use.
const maxGoalTarget = 9_999_999_999.99

// CategoryGoal is a category's target, one category_goals row. Only the
// definition is stored; how much a month still needs is derived from that
// month's budgeted/activity/balance.
type CategoryGoal struct {
	Type   GoalType `json:"type"`
	Target float64  `json:"target"`

	// NEED only.
	Cadence GoalCadence `json:"cadence,omitempty"`
	// CadenceFrequency is "every N months/years" for monthly/yearly cadences.
	CadenceFrequency int `json:"cadenceFrequency,omitempty"`
	// Day is the weekday (0 = Sunday) for weekly targets, otherwise the day of
	// the month the amount is due. Nil means the last day of the month.
	Day *int `json:"day,omitempty"`
	// NeedsWholeAmount is YNAB's "next month I want to": true refills the
	// category up to Target (leftover counts toward it), false sets aside
	// another Target on top of whatever is left.
	NeedsWholeAmount bool `json:"needsWholeAmount"`

	// TargetMonth (YYYY-MM) is the deadline of a TBD target, and the first due
	// month of a once/yearly/every-N-months NEED target, which then rolls
	// forward by the cadence.
	TargetMonth string `json:"targetMonth,omitempty"`

	// CreationMonth (YYYY-MM) is the first month the target applies to; months
	// before it show no target.
	CreationMonth string `json:"creationMonth"`
	// SnoozedMonth (YYYY-MM) skips the target for that one month.
	SnoozedMonth string `json:"snoozedMonth,omitempty"`
}

var ErrInvalidGoal = errors.New("invalid goal")

func invalidGoal(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidGoal, fmt.Sprintf(format, args...))
}

func validGoalMonth(month string) bool {
	_, err := time.Parse("2006-01", month)
	return err == nil
}

// Normalize validates the goal and clears the fields its type does not use,
// so a stored goal never carries a stale cadence from a previous type. now
// supplies the creation month when the caller did not send one.
func (g *CategoryGoal) Normalize(now time.Time) error {
	if g.Target <= 0 || g.Target > maxGoalTarget {
		return invalidGoal("target must be greater than zero")
	}
	if g.CreationMonth == "" {
		g.CreationMonth = now.Format("2006-01")
	}
	if !validGoalMonth(g.CreationMonth) {
		return invalidGoal("creationMonth must be YYYY-MM")
	}
	if g.SnoozedMonth != "" && !validGoalMonth(g.SnoozedMonth) {
		return invalidGoal("snoozedMonth must be YYYY-MM")
	}

	switch g.Type {
	case GoalTypeTargetBalance, GoalTypeMonthlyFunding, GoalTypeDebt:
		g.clearSchedule()
		g.TargetMonth = ""
	case GoalTypeTargetBalanceByDate:
		g.clearSchedule()
		if !validGoalMonth(g.TargetMonth) {
			return invalidGoal("targetMonth (YYYY-MM) is required for a savings balance by date")
		}
	case GoalTypeNeed:
		return g.normalizeNeed()
	default:
		return invalidGoal("unknown type %q", g.Type)
	}
	return nil
}

func (g *CategoryGoal) clearSchedule() {
	g.Cadence = ""
	g.CadenceFrequency = 0
	g.Day = nil
	g.NeedsWholeAmount = false
}

func (g *CategoryGoal) normalizeNeed() error {
	if g.Cadence == "" {
		g.Cadence = GoalCadenceMonthly
	}
	if g.CadenceFrequency == 0 {
		g.CadenceFrequency = 1
	}
	if g.CadenceFrequency < 1 || g.CadenceFrequency > maxGoalCadenceFrequency {
		return invalidGoal("cadenceFrequency must be between 1 and %d", maxGoalCadenceFrequency)
	}

	switch g.Cadence {
	case GoalCadenceWeekly:
		g.CadenceFrequency = 1
		g.TargetMonth = ""
		if g.Day == nil || *g.Day < 0 || *g.Day > 6 {
			return invalidGoal("a weekly target needs a weekday (0 = Sunday … 6 = Saturday)")
		}
		return nil
	case GoalCadenceMonthly:
		// Every month needs no anchor; every N months repeats from TargetMonth.
		if g.CadenceFrequency == 1 {
			g.TargetMonth = ""
		} else if !validGoalMonth(g.TargetMonth) {
			return invalidGoal("targetMonth (YYYY-MM) is required for a target every %d months", g.CadenceFrequency)
		}
	case GoalCadenceYearly:
		if !validGoalMonth(g.TargetMonth) {
			return invalidGoal("targetMonth (YYYY-MM) is required for a yearly target")
		}
	case GoalCadenceNone:
		g.CadenceFrequency = 1
		if !validGoalMonth(g.TargetMonth) {
			return invalidGoal("targetMonth (YYYY-MM) is required for a target by date")
		}
	default:
		return invalidGoal("unknown cadence %q", g.Cadence)
	}
	if g.Day != nil && (*g.Day < 1 || *g.Day > 31) {
		return invalidGoal("day must be between 1 and 31")
	}
	return nil
}
