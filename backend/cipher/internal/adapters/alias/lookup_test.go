package aliasAdapter

import (
	"context"
	"errors"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/ports"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/google/uuid"
)

var upiEntities = ports.ExtractedEntities{Channel: "UPI", Merchant: "Blue Tokai", VPA: "shop@oksbi"}

type fakeAliases struct {
	repository.CounterpartyAliasRepository
	match             *model.CounterpartyAliasMatch
	err               error
	channel, merchant string
	calls             int
	tried             []string
	matchOn           string // only this merchant matches, when set
}

func (f *fakeAliases) FindMatch(
	_ context.Context, _ uuid.UUID, channel, merchant string,
) (*model.CounterpartyAliasMatch, error) {
	f.calls++
	f.channel, f.merchant = channel, merchant
	f.tried = append(f.tried, merchant)
	if f.matchOn != "" && merchant != f.matchOn {
		return nil, f.err
	}
	return f.match, f.err
}

func budgetContext() context.Context {
	return utils.WithBudgetID(context.Background(), uuid.New())
}

func TestLookupHit(t *testing.T) {
	aliases := &fakeAliases{match: &model.CounterpartyAliasMatch{PayeeName: "Blue Tokai", CategoryName: "Coffee", Source: "MANUAL"}}
	got, found, err := NewLookup(aliases).Lookup(budgetContext(), ports.Message{}, upiEntities)
	if err != nil || !found {
		t.Fatalf("found = %v, err = %v (channel %q, merchant %q)", found, err, aliases.channel, aliases.merchant)
	}
	if got.PayeeName != "Blue Tokai" || got.CategoryName != "Coffee" || got.Confidence != 1 {
		t.Fatalf("got %+v", got)
	}
	if aliases.channel != "UPI" || aliases.merchant != "SHOP@OKSBI" {
		t.Fatalf("key = %q | %q", aliases.channel, aliases.merchant)
	}
}

func TestLookupMissesWithoutAnAlias(t *testing.T) {
	_, found, err := NewLookup(&fakeAliases{}).Lookup(budgetContext(), ports.Message{}, upiEntities)
	if err != nil || found {
		t.Fatalf("found = %v, err = %v", found, err)
	}
}

func TestLookupSkipsTextWithNoKey(t *testing.T) {
	aliases := &fakeAliases{}
	_, found, err := NewLookup(aliases).Lookup(
		budgetContext(), ports.Message{}, ports.ExtractedEntities{Merchant: "Blue Tokai"},
	)
	if err != nil || found || aliases.calls != 0 {
		t.Fatalf("found = %v, err = %v, calls = %d", found, err, aliases.calls)
	}
}

func TestLookupFailureIsAnError(t *testing.T) {
	aliases := &fakeAliases{err: errors.New("db down")}
	_, _, err := NewLookup(aliases).Lookup(budgetContext(), ports.Message{}, upiEntities)
	if !errs.HasCode(err, errs.CodeMessageLookupFailed) {
		t.Fatalf("err = %v", err)
	}
}

func TestLookupFallsBackToTheNameForOlderUPIAliases(t *testing.T) {
	aliases := &fakeAliases{matchOn: "BLUE TOKAI", match: &model.CounterpartyAliasMatch{PayeeName: "Blue Tokai", CategoryName: "Coffee"}}
	_, found, err := NewLookup(aliases).Lookup(budgetContext(), ports.Message{}, upiEntities)
	if err != nil || !found || len(aliases.tried) != 2 || aliases.tried[0] != "SHOP@OKSBI" {
		t.Fatalf("found = %v, err = %v, tried %v", found, err, aliases.tried)
	}
}

func TestLookupTriesACardMerchantOnce(t *testing.T) {
	aliases := &fakeAliases{}
	_, found, err := NewLookup(aliases).Lookup(budgetContext(), ports.Message{}, ports.ExtractedEntities{Channel: "CARD", Merchant: "Redbus"})
	if err != nil || found || len(aliases.tried) != 1 || aliases.tried[0] != "REDBUS" {
		t.Fatalf("found = %v, err = %v, tried %v", found, err, aliases.tried)
	}
}

func TestLookupConfidenceFollowsWhoMadeTheAlias(t *testing.T) {
	aliases := &fakeAliases{match: &model.CounterpartyAliasMatch{PayeeName: "Shop", CategoryName: "Food", Source: "AUTO_LEARNED"}}
	got, found, err := NewLookup(aliases).Lookup(budgetContext(), ports.Message{}, upiEntities)
	if err != nil || !found || got.Confidence != learnedAliasConfidence {
		t.Fatalf("got %+v, found %v, err %v", got, found, err)
	}
}

func TestLookupSkipsAnAliasWithoutARealCategory(t *testing.T) {
	aliases := &fakeAliases{match: &model.CounterpartyAliasMatch{PayeeName: "PolicyBazaar", CategoryName: "Uncategorized", Source: "AUTO_LEARNED"}}
	_, found, err := NewLookup(aliases).Lookup(budgetContext(), ports.Message{}, ports.ExtractedEntities{Channel: "CARD", Merchant: "POLICYBAZAAR COM"})
	if err != nil || found {
		t.Fatalf("found %v, err %v", found, err)
	}
}
