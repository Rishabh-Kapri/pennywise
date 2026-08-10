package model

import (
	"testing"

	"github.com/google/uuid"
)

func TestRecurringFrequencyNext(t *testing.T) {
	tests := []struct {
		name      string
		date      Date
		frequency RecurringFrequency
		interval  int
		want      Date
	}{
		{"daily", "2026-07-13", RecurringFrequencyDaily, 1, "2026-07-14"},
		{"every 3 days", "2026-07-13", RecurringFrequencyDaily, 3, "2026-07-16"},
		{"weekly", "2026-07-13", RecurringFrequencyWeekly, 1, "2026-07-20"},
		{"biweekly", "2026-07-13", RecurringFrequencyWeekly, 2, "2026-07-27"},
		{"monthly", "2026-07-13", RecurringFrequencyMonthly, 1, "2026-08-13"},
		{"monthly across year end", "2026-12-15", RecurringFrequencyMonthly, 1, "2027-01-15"},
		{"quarterly", "2026-01-31", RecurringFrequencyMonthly, 3, "2026-04-30"},
		{"yearly", "2026-07-13", RecurringFrequencyYearly, 1, "2027-07-13"},
		// day-of-month clamping: Jan 31 must land on Feb 28, not Mar 3
		{"month end clamps to shorter month", "2026-01-31", RecurringFrequencyMonthly, 1, "2026-02-28"},
		{"month end clamps in leap year", "2028-01-31", RecurringFrequencyMonthly, 1, "2028-02-29"},
		{"leap day yearly falls back", "2028-02-29", RecurringFrequencyYearly, 1, "2029-02-28"},
		{"zero interval treated as one", "2026-07-13", RecurringFrequencyDaily, 0, "2026-07-14"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.frequency.Next(tt.date, tt.interval)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Next(%q, %d) = %q, want %q", tt.date, tt.interval, got, tt.want)
			}
		})
	}
}

func TestRecurringFrequencyNextInvalid(t *testing.T) {
	if _, err := RecurringFrequencyDaily.Next("not-a-date", 1); err == nil {
		t.Error("expected error for malformed date")
	}
	if _, err := RecurringFrequency("HOURLY").Next("2026-07-13", 1); err == nil {
		t.Error("expected error for unsupported frequency")
	}
}

func TestRecurringFrequencyValid(t *testing.T) {
	for _, f := range []RecurringFrequency{
		RecurringFrequencyDaily,
		RecurringFrequencyWeekly,
		RecurringFrequencyMonthly,
		RecurringFrequencyYearly,
	} {
		if err := f.Valid(); err != nil {
			t.Errorf("%s should be valid: %v", f, err)
		}
	}
	if err := RecurringFrequency("MINUTELY").Valid(); err == nil {
		t.Error("expected MINUTELY to be invalid")
	}
}

func TestRecurringTransactionValidate(t *testing.T) {
	valid := func() RecurringTransaction {
		payee := uuid.New()
		account := uuid.New()
		return RecurringTransaction{
			Name:          "Rent",
			AccountID:     account,
			PayeeID:       &payee,
			Amount:        -18000,
			Frequency:     RecurringFrequencyMonthly,
			IntervalCount: 1,
			NextDate:      "2026-08-01",
		}
	}

	if err := validRule(valid()); err != nil {
		t.Fatalf("expected valid rule, got %v", err)
	}

	t.Run("missing name", func(t *testing.T) {
		r := valid()
		r.Name = ""
		if err := validRule(r); err == nil {
			t.Error("expected error")
		}
	})
	t.Run("missing payee", func(t *testing.T) {
		r := valid()
		r.PayeeID = nil
		if err := validRule(r); err == nil {
			t.Error("expected error")
		}
	})
	t.Run("zero amount", func(t *testing.T) {
		r := valid()
		r.Amount = 0
		if err := validRule(r); err == nil {
			t.Error("expected error")
		}
	})
	t.Run("end before next", func(t *testing.T) {
		r := valid()
		end := Date("2026-07-01")
		r.EndDate = &end
		if err := validRule(r); err == nil {
			t.Error("expected error")
		}
	})
	t.Run("end after next is fine", func(t *testing.T) {
		r := valid()
		end := Date("2027-01-01")
		r.EndDate = &end
		if err := validRule(r); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func validRule(r RecurringTransaction) error { return r.Validate() }
