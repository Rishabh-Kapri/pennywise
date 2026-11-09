package decisionsAdapter

import (
	"context"
	"net/http"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/httpclient"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
)

type Client struct {
	TC    *transport.Client
	tc    *transport.Client
	model string
}

func NewClient(tc *transport.Client, model string) *Client {
	return &Client{TC: tc, tc: tc, model: model}
}

func (c *Client) ask(ctx context.Context, query string, questions map[string]intent) (*decisionResponse, error) {
	queryBody := decisionQuery{
		State: state{
			Query: query,
		},
		Model:     c.model,
		Questions: questions,
	}

	res, err := transport.Post[decisionResponse](ctx, c.tc, "", nil, queryBody)
	if err != nil {
		return nil, errs.Wrap(errs.CodeDecisionsRequestFailed, "ask decisions model "+c.model, err,
			errs.WithDetail("model", c.model))
	}
	logger.Logger(ctx).Info("ask:", "res", res)
	return &res, nil
}

// NewUserClient calls a decision model at a user-given endpoint with the
// user's own key. httpClient should refuse private addresses (see
// transport.NewUserURLClient); the user picks the URL.
func NewUserClient(endpoint, apiKey, model string, httpClient *http.Client) *Client {
	tc := transport.NewClient("decisions-user", httpclient.NewHttpTransportWithClient(endpoint, httpClient),
		transport.WithPropagateInternalHeaders(false),
		transport.WithDefaultHeaders(map[string][]string{"Authorization": {"Bearer " + apiKey}}),
	)
	return NewClient(tc, model)
}

// Probe asks one trivial question to check the endpoint serves the model, and
// returns the model name the endpoint reported.
func (c *Client) Probe(ctx context.Context) (string, error) {
	res, err := c.ask(ctx, "Your card was debited Rs. 100 at a cafe.", map[string]intent{
		"ok": {
			Type:         noulIntentType,
			Instructions: "Does this text mention money?",
			Criteria: map[string]*string{
				"true":  new("It mentions an amount of money."),
				"false": new("It doesn't mention money."),
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
