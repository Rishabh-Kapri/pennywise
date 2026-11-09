package service

import (
	"errors"
	"testing"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

func TestApplyOnboardingUpdate(t *testing.T) {
	now := time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)
	step := func(s string) *string { return &s }

	t.Run("rejects an unknown step", func(t *testing.T) {
		_, err := applyOnboardingUpdate(model.BudgetOnboarding{}, model.UpdateBudgetOnboardingRequest{Step: step("nope")}, now)
		if !errors.Is(err, ErrInvalidOnboardingStep) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("rejects skipping done", func(t *testing.T) {
		_, err := applyOnboardingUpdate(model.BudgetOnboarding{}, model.UpdateBudgetOnboardingRequest{SkippedSteps: []string{"done"}}, now)
		if !errors.Is(err, ErrInvalidOnboardingStep) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("accumulates skipped steps without duplicates", func(t *testing.T) {
		got, err := applyOnboardingUpdate(
			model.BudgetOnboarding{SkippedSteps: []string{"bills"}},
			model.UpdateBudgetOnboardingRequest{Step: step("email"), SkippedSteps: []string{"bills", "ai"}},
			now,
		)
		if err != nil {
			t.Fatal(err)
		}
		if got.Step != "email" || len(got.SkippedSteps) != 2 || got.CompletedAt != nil {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("done stamps completedAt once", func(t *testing.T) {
		earlier := now.Add(-time.Hour)
		got, err := applyOnboardingUpdate(
			model.BudgetOnboarding{CompletedAt: &earlier},
			model.UpdateBudgetOnboardingRequest{Step: step("done")},
			now,
		)
		if err != nil {
			t.Fatal(err)
		}
		if got.CompletedAt == nil || !got.CompletedAt.Equal(earlier) {
			t.Fatalf("completedAt = %v", got.CompletedAt)
		}

		got, _ = applyOnboardingUpdate(model.BudgetOnboarding{}, model.UpdateBudgetOnboardingRequest{Step: step("done")}, now)
		if got.CompletedAt == nil || !got.CompletedAt.Equal(now) {
			t.Fatalf("completedAt = %v", got.CompletedAt)
		}
	})
}

func TestNormalizeProviderURL(t *testing.T) {
	for raw, ok := range map[string]bool{
		"https://ollama.example.com/": true,
		"http://10.0.0.5:11434":       true, // cipher's dialer decides about private hosts
		"ftp://example.com":           false,
		"https://user:pw@example.com": false,
		"https://example.com/?x=1":    false,
		"":                            false,
		"not a url":                   false,
	} {
		_, err := normalizeProviderURL(raw)
		if (err == nil) != ok {
			t.Errorf("normalizeProviderURL(%q) err = %v, want ok=%v", raw, err, ok)
		}
	}
}

func TestKeyHint(t *testing.T) {
	if got := keyHint("sk-abcdefgh1234"); got != "1234" {
		t.Errorf("keyHint = %q", got)
	}
	if got := keyHint("short"); got != "" {
		t.Errorf("short key leaked hint %q", got)
	}
}

func TestApplyOnboardingUpdateStampsEmailsImportedOnce(t *testing.T) {
	yes := true
	first := time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)

	current, err := applyOnboardingUpdate(model.BudgetOnboarding{}, model.UpdateBudgetOnboardingRequest{EmailsImported: &yes}, first)
	if err != nil {
		t.Fatal(err)
	}
	again, err := applyOnboardingUpdate(current, model.UpdateBudgetOnboardingRequest{EmailsImported: &yes}, first.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if again.EmailsImportedAt == nil || !again.EmailsImportedAt.Equal(first) {
		t.Fatalf("stamped %v", again.EmailsImportedAt)
	}
}
