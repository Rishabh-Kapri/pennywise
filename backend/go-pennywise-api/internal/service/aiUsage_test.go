package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/google/uuid"
)

type fakeUsageRepo struct {
	budgetID uuid.UUID
	since    time.Time
	tz       string
}

func (f *fakeUsageRepo) Record(context.Context, model.LLMUsage) error { return nil }

func (f *fakeUsageRepo) Summary(_ context.Context, budgetID uuid.UUID, since time.Time, tz string) (model.LLMUsageSummary, error) {
	f.budgetID, f.since, f.tz = budgetID, since, tz
	return model.LLMUsageSummary{Since: since}, nil
}

func TestAIUsageSummaryStartsAtLocalMidnight(t *testing.T) {
	repo := &fakeUsageRepo{}
	svc := &aiUsageService{repo: repo, now: func() time.Time {
		return time.Date(2026, 11, 18, 20, 0, 0, 0, time.UTC) // 01:30 on the 19th in Kolkata
	}}
	budgetID := uuid.New()
	ctx := utils.WithBudgetID(context.Background(), budgetID)

	if _, err := svc.Summary(ctx, 7, "Asia/Kolkata"); err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("Asia/Kolkata")
	want := time.Date(2026, 11, 13, 0, 0, 0, 0, loc)
	if !repo.since.Equal(want) || repo.tz != "Asia/Kolkata" || repo.budgetID != budgetID {
		t.Fatalf("since %v tz %q budget %v, want %v Asia/Kolkata %v", repo.since, repo.tz, repo.budgetID, want, budgetID)
	}
}

func TestAIUsageSummaryRejectsBadWindow(t *testing.T) {
	svc := NewAIUsageService(&fakeUsageRepo{})
	ctx := utils.WithBudgetID(context.Background(), uuid.New())
	for _, c := range []struct {
		days int
		tz   string
	}{{0, "UTC"}, {400, "UTC"}, {7, "Not/AZone"}} {
		if _, err := svc.Summary(ctx, c.days, c.tz); !errors.Is(err, ErrInvalidUsageWindow) {
			t.Errorf("days %d tz %q: err = %v", c.days, c.tz, err)
		}
	}
}
