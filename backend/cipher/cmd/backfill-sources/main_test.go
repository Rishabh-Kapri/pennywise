package main

import (
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils/narration"

	"github.com/google/uuid"
)

func TestLearnAliasesMajorityPayeeAndCategory(t *testing.T) {
	budget := uuid.New()
	swiggy, other := uuid.New(), uuid.New()
	food, misc := uuid.New(), uuid.New()

	row := func(merchant string, payee *uuid.UUID, cat *uuid.UUID, date string) parsedTxn {
		return parsedTxn{
			txnRow: txnRow{ID: uuid.New(), BudgetID: budget, PayeeID: payee, CategoryID: cat, Date: date},
			Parsed: narration.Parsed{Kind: narration.KindEmail, Channel: narration.ChannelCard, Merchant: merchant},
		}
	}

	aliases, conflicts := learnAliases([]parsedTxn{
		row("SWIGGY", &swiggy, &food, "2026-10-01"),
		row("SWIGGY", &swiggy, &food, "2026-10-02"),
		row("SWIGGY", &swiggy, &misc, "2026-10-03"),
		row("SWIGGY", &other, &misc, "2026-10-04"),
		row("TODOIST", nil, nil, "2026-10-05"),                              // no payee: nothing to learn
		{txnRow: txnRow{ID: uuid.New(), BudgetID: budget, PayeeID: &other}}, // no key
	})

	if len(aliases) != 1 {
		t.Fatalf("aliases = %d, want 1: %+v", len(aliases), aliases)
	}
	a := aliases[0]
	if a.PayeeID != swiggy || a.CategoryID == nil || *a.CategoryID != food || a.HitCount != 3 {
		t.Errorf("alias = %+v, want payee swiggy, category food, hits 3", a)
	}
	if got := a.LastSeen.Format("2006-01-02"); got != "2026-10-03" {
		t.Errorf("last seen = %s, want 2026-10-03 (latest for the winning payee)", got)
	}
	if len(conflicts) != 1 || conflicts[0].Key != "CARD | SWIGGY" || conflicts[0].Payees[other] != 1 {
		t.Errorf("conflicts = %+v, want CARD | SWIGGY with 1 vote for the other payee", conflicts)
	}
}

func TestFillFromStatementOnlyFillsWhatTheNarrationLacks(t *testing.T) {
	st := statementRow{Amount: "5743", Direction: "credit", Date: "2026-10-01", Currency: "inr", AccountNumber: "50100000001111"}

	p := narration.Parse("INTEREST PAID TILL 30-SEP-2026")
	fillFromStatement(&p, st)
	if p.Amount != "5743" || p.Direction != narration.Credit || p.Currency != "INR" || p.AccountHint != "1111" ||
		p.OccurredOn == nil || p.OccurredOn.Format("2006-01-02") != "2026-10-01" {
		t.Errorf("filled = %+v", p)
	}

	// A narration that states its own direction keeps it.
	p = narration.Parse("NEFT CR-ICIC0099999-CENTRAL DEPOSITORY SERVICES INDIA LIMITED G-SAMPLE US ER-IN22000000000001")
	fillFromStatement(&p, statementRow{Direction: "debit"})
	if p.Direction != narration.Credit {
		t.Errorf("direction = %q, want the narration's CREDIT", p.Direction)
	}
}
