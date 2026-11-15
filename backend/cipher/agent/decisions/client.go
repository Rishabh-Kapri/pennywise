package decisions

import (
	"context"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

type Decisions interface {
	Ask(ctx context.Context, req sharedModel.DecisionRequest) (*sharedModel.DecisionResponse, error)
}

// Probe asks one trivial question to check the endpoint serves model, and
// returns the model name the endpoint reported.
func Probe(ctx context.Context, d Decisions, model string) (string, error) {
	res, err := d.Ask(ctx, sharedModel.DecisionRequest{
		State: sharedModel.State{Query: "Your card was debited Rs. 100 at a cafe."},
		Model: model,
		Questions: map[string]sharedModel.Intent{
			"ok": {
				Type:         sharedModel.NoulIntentType,
				Instructions: "Does this text mention money?",
				Criteria: map[string]*string{
					"true":  new("It mentions an amount of money."),
					"false": new("It doesn't mention money."),
				},
			},
		},
	})
	if err != nil {
		return "", err
	}
	if ans, ok := res.Answers["ok"]; !ok || ans.Noul == nil {
		return res.Model, errs.New(errs.CodeDecisionsAnswerMissing, "the endpoint answered without a result for the test question")
	}
	return res.Model, nil
}
