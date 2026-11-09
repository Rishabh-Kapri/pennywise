package decisionsAdapter

import (
	"context"
	"testing"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/ports"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

type fakeDecisions struct{ res *sharedModel.DecisionResponse }

func (f fakeDecisions) Ask(context.Context, sharedModel.DecisionRequest) (*sharedModel.DecisionResponse, error) {
	return f.res, nil
}

func TestCheckReadsAnswersAndUsage(t *testing.T) {
	noul, debit, transaction, provider := 0.93, "debit", "transaction", "TypeSafe"
	cost := 0.00002
	a := NewDecisionsAdapter(fakeDecisions{&sharedModel.DecisionResponse{
		Model:    "typesafe/jev",
		Provider: &provider,
		Answers: map[string]sharedModel.Answer{
			"isTransaction":   {Type: "noul", Noul: &noul},
			"transactionType": {Type: "choice", Choice: &debit},
			"messageType":     {Type: "choice", Choice: &transaction},
		},
		Usage: sharedModel.Usage{Available: true, InputTokens: 476, OutputTokens: 70, Cost: &cost},
	}}, "typesafe/jev")

	got, err := a.Check(context.Background(), ports.Message{Body: "Rs. 450 debited"})
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsTransaction || got.Type != "debit" || got.MessageType != "transaction" {
		t.Fatalf("verdict = %+v", got)
	}
	if got.Usage == nil || got.Usage.Model != "typesafe/jev" || got.Usage.Provider != "TypeSafe" || got.Usage.InputTokens != 476 {
		t.Fatalf("usage = %+v", got.Usage)
	}
}

// A refused question must come back as an error, so a chain can try the next
// model.
func TestCheckRefusalIsAnError(t *testing.T) {
	noul := 0.93
	a := NewDecisionsAdapter(fakeDecisions{&sharedModel.DecisionResponse{
		Answers: map[string]sharedModel.Answer{
			"isTransaction":   {Type: "noul", Noul: &noul},
			"transactionType": {Type: "refusal"},
		},
	}}, "m")

	if _, err := a.Check(context.Background(), ports.Message{}); !errs.HasCode(err, errs.CodeDecisionsAnswerMissing) {
		t.Fatalf("err = %v", err)
	}
}

func TestVerify(t *testing.T) {
	yes, no := 0.9, 0.2
	details := ports.ExtractedEntities{Amount: -450, Timestamp: time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)}
	answer := func(p *float64) DecisionAdapter {
		return NewDecisionsAdapter(fakeDecisions{&sharedModel.DecisionResponse{
			Answers: map[string]sharedModel.Answer{"detailsMatch": {Type: "noul", Noul: p}},
		}}, "m")
	}

	if got, err := answer(&yes).Verify(context.Background(), ports.Message{}, details); err != nil || !got.Valid {
		t.Fatalf("match: %+v, %v", got, err)
	}
	if got, err := answer(&no).Verify(context.Background(), ports.Message{}, details); err != nil || got.Valid {
		t.Fatalf("mismatch: %+v, %v", got, err)
	}
	if _, err := answer(nil).Verify(context.Background(), ports.Message{}, details); !errs.HasCode(err, errs.CodeDecisionsAnswerMissing) {
		t.Fatalf("no answer: %v", err)
	}

	// Without an amount or date there's nothing to ask a model.
	unused := NewDecisionsAdapter(nil, "m")
	if got, err := unused.Verify(context.Background(), ports.Message{}, ports.ExtractedEntities{}); err != nil || got.Valid {
		t.Fatalf("blank details: %+v, %v", got, err)
	}
}
