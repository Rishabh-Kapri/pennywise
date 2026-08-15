package model

import (
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"

	"github.com/google/uuid"
)

type RecurringFrequency string

const (
	RecurringFrequencyDaily   RecurringFrequency = "DAILY"
	RecurringFrequencyWeekly  RecurringFrequency = "WEEKLY"
	RecurringFrequencyMonthly RecurringFrequency = "MONTHLY"
	RecurringFrequencyYearly  RecurringFrequency = "YEARLY"
)

func (f RecurringFrequency) Valid() error {
	switch f {
	case RecurringFrequencyDaily, RecurringFrequencyWeekly, RecurringFrequencyMonthly, RecurringFrequencyYearly:
		return nil
	default:
		return errs.New(errs.CodeInvalidArgument, "frequency must be one of DAILY, WEEKLY, MONTHLY, YEARLY")
	}
}

// Next returns the date that follows d by intervalCount periods of this
// frequency. Monthly and yearly steps clamp to the last day of the target
// month, so a rule anchored on the 31st still fires in February.
func (f RecurringFrequency) Next(d Date, intervalCount int) (Date, error) {
	parsed, err := time.Parse("2006-01-02", string(d))
	if err != nil {
		return "", errs.Wrap(errs.CodeInvalidArgument, "invalid date format", err)
	}
	if intervalCount < 1 {
		intervalCount = 1
	}

	switch f {
	case RecurringFrequencyDaily:
		return Date(parsed.AddDate(0, 0, intervalCount).Format("2006-01-02")), nil
	case RecurringFrequencyWeekly:
		return Date(parsed.AddDate(0, 0, 7*intervalCount).Format("2006-01-02")), nil
	case RecurringFrequencyMonthly:
		return Date(addMonthsClamped(parsed, intervalCount).Format("2006-01-02")), nil
	case RecurringFrequencyYearly:
		return Date(addMonthsClamped(parsed, 12*intervalCount).Format("2006-01-02")), nil
	default:
		return "", errs.New(errs.CodeInvalidArgument, "unsupported frequency")
	}
}

// addMonthsClamped shifts t by n months without Go's day overflow
// (time.AddDate turns Jan 31 + 1 month into Mar 3; this yields Feb 28/29).
func addMonthsClamped(t time.Time, n int) time.Time {
	year, month, day := t.Date()
	target := time.Date(year, month, 1, 0, 0, 0, 0, t.Location()).AddDate(0, n, 0)
	lastDay := time.Date(target.Year(), target.Month()+1, 0, 0, 0, 0, 0, t.Location()).Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(target.Year(), target.Month(), day, 0, 0, 0, 0, t.Location())
}

type RecurringTransaction struct {
	ID            uuid.UUID          `json:"id"`
	BudgetID      uuid.UUID          `json:"budgetId"`
	Name          string             `json:"name"`
	AccountID     uuid.UUID          `json:"accountId"`
	PayeeID       *uuid.UUID         `json:"payeeId,omitempty"`
	CategoryID    *uuid.UUID         `json:"categoryId,omitempty"`
	Amount        float64            `json:"amount"`
	Note          string             `json:"note"`
	Frequency     RecurringFrequency `json:"frequency"`
	IntervalCount int                `json:"intervalCount"`
	NextDate      Date               `json:"nextDate"`
	EndDate       *Date              `json:"endDate,omitempty"`
	LastRunDate   *Date              `json:"lastRunDate,omitempty"`
	Paused        bool               `json:"paused"`
	Deleted       bool               `json:"deleted"`
	CreatedAt     time.Time          `json:"createdAt"`
	UpdatedAt     time.Time          `json:"updatedAt"`

	// resolved for display; not persisted on this table
	AccountName  *string `json:"accountName,omitempty"`
	PayeeName    *string `json:"payeeName,omitempty"`
	CategoryName *string `json:"categoryName,omitempty"`
}

func (r *RecurringTransaction) Validate() error {
	if r.Name == "" {
		return errs.New(errs.CodeInvalidArgument, "name is required")
	}
	if r.AccountID == uuid.Nil {
		return errs.New(errs.CodeInvalidArgument, "accountId is required")
	}
	// transactions require a payee, so a rule that materializes into one must have it
	if r.PayeeID == nil || *r.PayeeID == uuid.Nil {
		return errs.New(errs.CodeInvalidArgument, "payeeId is required")
	}
	if r.Amount == 0 {
		return errs.New(errs.CodeInvalidArgument, "amount must be non-zero")
	}
	if err := r.Frequency.Valid(); err != nil {
		return err
	}
	if r.IntervalCount < 1 {
		return errs.New(errs.CodeInvalidArgument, "intervalCount must be at least 1")
	}
	if err := r.NextDate.Valid(); err != nil {
		return err
	}
	if r.EndDate != nil && *r.EndDate != "" {
		if err := r.EndDate.Valid(); err != nil {
			return err
		}
		if string(*r.EndDate) < string(r.NextDate) {
			return errs.New(errs.CodeInvalidArgument, "endDate must not be before nextDate")
		}
	}
	return nil
}

// RecurringRunResult reports what a materialization pass created.
type RecurringRunResult struct {
	Created int `json:"created"`
	Skipped int `json:"skipped"`
}
