package model

import (
	"errors"
	"testing"
	"time"
)

func intPtr(v int) *int { return &v }

func TestCategoryGoalNormalize(t *testing.T) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)

	t.Run("defaults a NEED target to monthly and stamps the creation month", func(t *testing.T) {
		g := CategoryGoal{Type: GoalTypeNeed, Target: 500, NeedsWholeAmount: true}
		if err := g.Normalize(now); err != nil {
			t.Fatal(err)
		}
		if g.Cadence != GoalCadenceMonthly || g.CadenceFrequency != 1 || g.CreationMonth != "2026-10" {
			t.Fatalf("unexpected normalized goal: %+v", g)
		}
	})

	t.Run("clears schedule fields a type does not use", func(t *testing.T) {
		g := CategoryGoal{
			Type: GoalTypeMonthlyFunding, Target: 100, Cadence: GoalCadenceWeekly,
			Day: intPtr(3), NeedsWholeAmount: true, TargetMonth: "2027-01",
		}
		if err := g.Normalize(now); err != nil {
			t.Fatal(err)
		}
		if g.Cadence != "" || g.Day != nil || g.NeedsWholeAmount || g.TargetMonth != "" {
			t.Fatalf("stale schedule kept: %+v", g)
		}
	})

	t.Run("monthly every month drops a stray target month", func(t *testing.T) {
		g := CategoryGoal{Type: GoalTypeNeed, Target: 100, Cadence: GoalCadenceMonthly, TargetMonth: "2027-01"}
		if err := g.Normalize(now); err != nil {
			t.Fatal(err)
		}
		if g.TargetMonth != "" {
			t.Fatalf("target month kept: %+v", g)
		}
	})

	valid := []CategoryGoal{
		{Type: GoalTypeTargetBalance, Target: 1000},
		{Type: GoalTypeTargetBalanceByDate, Target: 1000, TargetMonth: "2027-06"},
		{Type: GoalTypeDebt, Target: 250},
		{Type: GoalTypeNeed, Target: 50, Cadence: GoalCadenceWeekly, Day: intPtr(0)},
		{Type: GoalTypeNeed, Target: 50, Cadence: GoalCadenceMonthly, Day: intPtr(31)},
		{Type: GoalTypeNeed, Target: 50, Cadence: GoalCadenceMonthly, CadenceFrequency: 3, TargetMonth: "2026-12"},
		{Type: GoalTypeNeed, Target: 50, Cadence: GoalCadenceYearly, TargetMonth: "2027-03", Day: intPtr(15)},
		{Type: GoalTypeNeed, Target: 50, Cadence: GoalCadenceNone, TargetMonth: "2027-03"},
	}
	for _, g := range valid {
		if err := g.Normalize(now); err != nil {
			t.Errorf("%+v: unexpected error %v", g, err)
		}
	}

	invalid := map[string]CategoryGoal{
		"zero target":          {Type: GoalTypeMonthlyFunding, Target: 0},
		"unknown type":         {Type: "SPEND", Target: 10},
		"TBD without month":    {Type: GoalTypeTargetBalanceByDate, Target: 10},
		"weekly without day":   {Type: GoalTypeNeed, Target: 10, Cadence: GoalCadenceWeekly},
		"weekday out of range": {Type: GoalTypeNeed, Target: 10, Cadence: GoalCadenceWeekly, Day: intPtr(7)},
		"day of month 0":       {Type: GoalTypeNeed, Target: 10, Cadence: GoalCadenceMonthly, Day: intPtr(0)},
		"every N w/o anchor":   {Type: GoalTypeNeed, Target: 10, Cadence: GoalCadenceMonthly, CadenceFrequency: 2},
		"yearly w/o month":     {Type: GoalTypeNeed, Target: 10, Cadence: GoalCadenceYearly},
		"once w/o month":       {Type: GoalTypeNeed, Target: 10, Cadence: GoalCadenceNone},
		"bad cadence":          {Type: GoalTypeNeed, Target: 10, Cadence: "daily"},
		"bad snooze":           {Type: GoalTypeDebt, Target: 10, SnoozedMonth: "2026-13"},
		"bad creation month":   {Type: GoalTypeDebt, Target: 10, CreationMonth: "Oct 2026"},
	}
	for name, g := range invalid {
		if err := g.Normalize(now); !errors.Is(err, ErrInvalidGoal) {
			t.Errorf("%s: want ErrInvalidGoal, got %v", name, err)
		}
	}
}
