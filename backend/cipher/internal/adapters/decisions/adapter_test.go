package decisionsAdapter

import (
	"context"
	"slices"
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

type recordingDecisions struct {
	req sharedModel.DecisionRequest
	res *sharedModel.DecisionResponse
}

func (f *recordingDecisions) Ask(_ context.Context, req sharedModel.DecisionRequest) (*sharedModel.DecisionResponse, error) {
	f.req = req
	return f.res, nil
}

const verifySample = "Rs.750.00 is debited from your account ending 1111 towards VPA 9000000001-3@ybl (SAMPLE PERSON) on 07-10-26." +
	"UPI transaction reference no.: 900000000001.If you did not authorize this, call 18002586161Or email cc.disputes@hdfc.bank.in."

func choice(value string, confidence *float64) sharedModel.Answer {
	return sharedModel.Answer{Choice: &value, Confidence: confidence}
}

func TestVerifyFillsInWhatTheModelPicked(t *testing.T) {
	yes, sure := 0.95, 0.95
	client := &recordingDecisions{res: &sharedModel.DecisionResponse{Answers: map[string]sharedModel.Answer{
		"detailsMatch": {Noul: &yes},
		"channel":      choice("UPI", &sure),
		"ref":          choice("900000000001", &sure),
		"vpa":          choice("9000000001-3@ybl", &sure),
		"account":      choice("1111", &sure),
	}}}
	details := ports.ExtractedEntities{Amount: 750, Timestamp: time.Now()}

	got, err := NewDecisionsAdapter(client, "m").Verify(context.Background(), ports.Message{Body: verifySample}, details)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Valid || got.Channel != "UPI" || got.Ref != "900000000001" || got.VPA != "9000000001-3@ybl" || got.Account != "1111" {
		t.Fatalf("result %+v", got)
	}

	for _, key := range []string{"900000000001", "18002586161", "none"} {
		if _, ok := client.req.Questions["ref"].Criteria[key]; !ok {
			t.Errorf("ref options lack %q: %v", key, client.req.Questions["ref"].Criteria)
		}
	}
	if _, ok := client.req.Questions["ref"].Criteria["9000000001"]; ok {
		t.Error("a VPA's digits were offered as a reference")
	}
}

func TestVerifyDropsAnswersItCantTrust(t *testing.T) {
	yes, unsure, sure := 0.95, 0.6, 0.95
	details := ports.ExtractedEntities{Account: "1111", Amount: 750, Timestamp: time.Now()}
	tests := []struct {
		name    string
		answers map[string]sharedModel.Answer
		check   func(ports.VerifyResult) bool
	}{
		{"a ref below the bar", map[string]sharedModel.Answer{"ref": choice("900000000001", &unsure)},
			func(r ports.VerifyResult) bool { return r.Ref == "" }},
		{"a ref with no score", map[string]sharedModel.Answer{"ref": choice("900000000001", nil)},
			func(r ports.VerifyResult) bool { return r.Ref == "" }},
		{"a ref that wasn't offered", map[string]sharedModel.Answer{"ref": choice("123456789", &sure)},
			func(r ports.VerifyResult) bool { return r.Ref == "" }},
		{"none", map[string]sharedModel.Answer{"ref": choice("none", &sure), "channel": choice("none", &sure)},
			func(r ports.VerifyResult) bool { return r.Ref == "" && r.Channel == "" }},
		{"refusals", map[string]sharedModel.Answer{"ref": {Type: "refusal"}, "channel": {Type: "refusal"}},
			func(r ports.VerifyResult) bool { return r.Ref == "" && r.Channel == "" }},
		{"a channel with no score is kept", map[string]sharedModel.Answer{"channel": choice("CARD", nil)},
			func(r ports.VerifyResult) bool { return r.Channel == "CARD" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.answers["detailsMatch"] = sharedModel.Answer{Noul: &yes}
			client := &recordingDecisions{res: &sharedModel.DecisionResponse{Answers: tt.answers}}
			got, err := NewDecisionsAdapter(client, "m").Verify(context.Background(), ports.Message{Body: verifySample}, details)
			if err != nil {
				t.Fatal(err)
			}
			if !got.Valid || !tt.check(got) {
				t.Fatalf("result %+v", got)
			}
		})
	}
}

func TestVerifySkipsQuestionsWithNothingToPick(t *testing.T) {
	yes := 0.95
	client := &recordingDecisions{res: &sharedModel.DecisionResponse{Answers: map[string]sharedModel.Answer{"detailsMatch": {Noul: &yes}}}}
	details := ports.ExtractedEntities{Account: "1111", Amount: 12.71, Timestamp: time.Now()}
	if _, err := NewDecisionsAdapter(client, "m").Verify(context.Background(), ports.Message{Body: "Spent Rs.12 on your card"}, details); err != nil {
		t.Fatal(err)
	}
	for _, question := range []string{"ref", "vpa", "account"} {
		if _, asked := client.req.Questions[question]; asked {
			t.Errorf("asked %s with no candidates", question)
		}
	}
}

func TestCandidates(t *testing.T) {
	if got := vpaCandidates("paid to shop@oksbi. Then x-1@ybl."); !slices.Equal(got, []string{"shop@oksbi", "x-1@ybl"}) {
		t.Errorf("vpas %v", got)
	}
	if got := refCandidates("UTR HDFCN520221 no 123 and 123456 and 123456"); !slices.Equal(got, []string{"HDFCN520221", "123456"}) {
		t.Errorf("refs %v", got)
	}
	if got := accountCandidates("ending 1111. Card XX2222, year 2026, phone 18002586161"); !slices.Equal(got, []string{"1111", "2222", "2026"}) {
		t.Errorf("accounts %v", got)
	}
}

func TestCheckFallsBackToTheAnswerForConfidence(t *testing.T) {
	p := 0.2
	typeChoice, kind := "none", "none"
	client := &recordingDecisions{res: &sharedModel.DecisionResponse{Answers: map[string]sharedModel.Answer{
		"isTransaction":   {Noul: &p},
		"transactionType": {Choice: &typeChoice},
		"messageType":     {Choice: &kind},
	}}}
	got, err := NewDecisionsAdapter(client, "m").Check(context.Background(), ports.Message{Body: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Confidence == nil || *got.Confidence != 0.8 {
		t.Fatalf("confidence %v", got.Confidence)
	}
}
