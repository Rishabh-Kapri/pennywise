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

const upiEmail = "Dear Customer,Greetings from HDFC Bank!Rs.450.00 is debited from your account ending 4567 to VPA shop@oksbi BLUE TOKAI on 14-07-25."

type fakeAliases struct {
	repository.CounterpartyAliasRepository
	match             *model.CounterpartyAliasMatch
	err               error
	channel, merchant string
	calls             int
}

func (f *fakeAliases) FindMatch(
	_ context.Context, _ uuid.UUID, channel, merchant string,
) (*model.CounterpartyAliasMatch, error) {
	f.calls++
	f.channel, f.merchant = channel, merchant
	return f.match, f.err
}

func budgetContext() context.Context {
	return utils.WithBudgetID(context.Background(), uuid.New())
}

func TestLookupHit(t *testing.T) {
	aliases := &fakeAliases{match: &model.CounterpartyAliasMatch{PayeeName: "Blue Tokai", CategoryName: "Coffee"}}
	got, found, err := NewLookup(aliases).Lookup(budgetContext(), ports.Message{Body: upiEmail}, ports.ExtractedEntities{})
	if err != nil || !found {
		t.Fatalf("found = %v, err = %v (channel %q, merchant %q)", found, err, aliases.channel, aliases.merchant)
	}
	if got.PayeeName != "Blue Tokai" || got.CategoryName != "Coffee" || got.Confidence != 1 {
		t.Fatalf("got %+v", got)
	}
	if aliases.channel == "" || aliases.merchant == "" {
		t.Fatalf("key = %q | %q", aliases.channel, aliases.merchant)
	}
}

func TestLookupMissesWithoutAnAlias(t *testing.T) {
	_, found, err := NewLookup(&fakeAliases{}).Lookup(budgetContext(), ports.Message{Body: upiEmail}, ports.ExtractedEntities{})
	if err != nil || found {
		t.Fatalf("found = %v, err = %v", found, err)
	}
}

func TestLookupSkipsTextWithNoKey(t *testing.T) {
	aliases := &fakeAliases{}
	_, found, err := NewLookup(aliases).Lookup(
		budgetContext(), ports.Message{Body: "Dear Customer, your statement is ready to view online today."}, ports.ExtractedEntities{},
	)
	if err != nil || found || aliases.calls != 0 {
		t.Fatalf("found = %v, err = %v, calls = %d", found, err, aliases.calls)
	}
}

func TestLookupFailureIsAnError(t *testing.T) {
	aliases := &fakeAliases{err: errors.New("db down")}
	_, _, err := NewLookup(aliases).Lookup(budgetContext(), ports.Message{Body: upiEmail}, ports.ExtractedEntities{})
	if !errs.HasCode(err, errs.CodeMessageLookupFailed) {
		t.Fatalf("err = %v", err)
	}
}
