package llmAdapter

import (
	"testing"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	sharedUtils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

func TestStatementPagesMergeIntoTheirSection(t *testing.T) {
	page1 := `{"sections": [{"accountLabel": "Savings", "accountNumber": "XX12", "currency": "inr", "kind": "Card",
		"periodStart": "2026-01-01", "periodEnd": "", "openingBalance": "1,000.00", "closingBalance": null,
		"rows": [{"date": "2026-01-02", "description": "UPI  SWIGGY\n 123", "payee": "Swiggy",
			"amount": "200", "direction": "Debit", "balance": 800, "reference": 4411}]}]}`
	page2 := `{"sections": [{"accountLabel": "Savings", "accountNumber": "XX12", "closingBalance": 900,
		"dueDate": "2026-02-01", "totalDue": "1,234.50", "minimumDue": null,
		"rewards": {"opening": "1,200", "earned": 40, "redeemed": null, "closing": 1240},
		"rows": [{"date": "2026-01-05", "description": "Salary", "payee": "Acme", "amount": 100,
			"direction": "credit", "balance": 900, "reference": "", "rewardPoints": "-12"}]}]}`

	var sections []sharedModel.StatementSection
	for i, body := range []string{page1, page2} {
		parsed, err := sharedUtils.UnmarshalResponse[rawStatementExtraction]([]byte(body))
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range parsed.toSections(i + 1) {
			sections = mergeSection(sections, s)
		}
	}

	if len(sections) != 1 {
		t.Fatalf("got %d sections, want 1", len(sections))
	}
	s := sections[0]
	if len(s.Rows) != 2 || s.Rows[0].Page != 1 || s.Rows[1].Page != 2 {
		t.Fatalf("rows = %+v", s.Rows)
	}
	if *s.OpeningBalance != 1000 || *s.ClosingBalance != 900 || s.PeriodEnd != nil || s.Currency != "INR" {
		t.Fatalf("header = %+v", s)
	}
	if s.Kind != sharedModel.StatementKindCard || s.DueDate == nil || *s.DueDate != "2026-02-01" ||
		s.TotalDue == nil || *s.TotalDue != 1234.5 || s.MinimumDue != nil {
		t.Fatalf("card fields = kind %q due %v total %v min %v", s.Kind, s.DueDate, s.TotalDue, s.MinimumDue)
	}
	if s.Rewards == nil || *s.Rewards.Opening != 1200 || *s.Rewards.Earned != 40 ||
		s.Rewards.Redeemed != nil || *s.Rewards.Closing != 1240 {
		t.Fatalf("rewards = %+v", s.Rewards)
	}
	if s.Rows[0].RewardPoints != nil || s.Rows[1].RewardPoints == nil || *s.Rows[1].RewardPoints != -12 {
		t.Fatalf("row points = %v, %v", s.Rows[0].RewardPoints, s.Rows[1].RewardPoints)
	}
	r := s.Rows[0]
	if r.Description != "UPI SWIGGY 123" || r.Direction != sharedModel.StatementDebit || r.Reference != "4411" {
		t.Fatalf("row = %+v", r)
	}
}
