package providers

import (
	"context"
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/decisions"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/httpclient"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
)

// openRouterClient calls OpenRouter's alpha Decisions API
// (POST https://openrouter.ai/api/alpha/decisions). The API is marked alpha,
// so its wire shape lives only in this file: if it changes, nothing else does.
type openRouterClient struct {
	tc *transport.Client
}

// Wire types. The provider-neutral ones are in shared/model (decisions.go).

type openRouterDecisionReq struct {
	Model     string                        `json:"model"`
	State     any                           `json:"state"`
	Questions map[string]openRouterQuestion `json:"questions"`
}

type openRouterQuestion struct {
	Type         sharedModel.IntentType `json:"type"`
	Instructions string                 `json:"instructions,omitempty"`
	Criteria     map[string]*string     `json:"criteria,omitempty"`
}

type openRouterDecisionRes struct {
	ID       string                        `json:"id"`
	Model    string                        `json:"model"`
	Provider string                        `json:"provider"`
	Answers  map[string]sharedModel.Answer `json:"answers"`
	Usage    *openRouterDecisionUsage      `json:"usage"`
}

type openRouterDecisionUsage struct {
	InputTokens        int      `json:"input_tokens"`
	OutputTokens       int      `json:"output_tokens"`
	Cost               *float64 `json:"cost"` // OpenRouter credits, not USD
	IsBYOK             bool     `json:"is_byok"`
	InputTokensDetails struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"input_tokens_details"`
}

// NewOpenRouterClient calls the decisions endpoint the user entered (the full
// URL, e.g. https://openrouter.ai/api/alpha/decisions) with their key.
// TypeSafe's https://api.typesafe.ai/v1/systemone (Jev) takes the same request
// and answers without id, provider or cost.
// httpClient must be the SSRF-safe client for user URLs (transport.NewUserURLClient):
// the endpoint is user input, so there is deliberately no default.
func NewOpenRouterClient(endpoint, apiKey string, httpClient *http.Client) (decisions.Decisions, error) {
	switch {
	case endpoint == "":
		return nil, errs.New(errs.CodeDecisionsNotConfigured, "no decisions endpoint")
	case apiKey == "":
		return nil, errs.New(errs.CodeDecisionsNotConfigured, "no decisions api key")
	case httpClient == nil:
		return nil, errs.New(errs.CodeInternalError, "decisions client needs the safe http client for user URLs")
	}

	tc := transport.NewClient(
		"openrouter-decisions",
		httpclient.NewHttpTransportWithClient(endpoint, httpClient),
		// Internal headers (service tokens, budget IDs) must never reach a third party.
		transport.WithPropagateInternalHeaders(false),
		transport.WithDefaultHeaders(map[string][]string{
			"Content-Type":  {"application/json"},
			"Authorization": {"Bearer " + apiKey},
		}),
	)
	return &openRouterClient{tc: tc}, nil
}

// Ask implements [decisions.Decisions].
func (c *openRouterClient) Ask(
	ctx context.Context,
	req sharedModel.DecisionRequest,
) (*sharedModel.DecisionResponse, error) {
	if req.Model == "" {
		return nil, errs.New(errs.CodeDecisionsNotConfigured, "decisions request has no model")
	}
	if len(req.Questions) == 0 {
		return nil, errs.New(errs.CodeInvalidArgument, "decisions request has no questions")
	}

	res, err := transport.Post[openRouterDecisionRes](ctx, c.tc, "", nil, toOpenRouterDecisionReq(req))
	if err != nil {
		// The transport error carries the HTTP status (errs.StatusOf) and the
		// body, e.g. 402 "Insufficient credits".
		return nil, errs.Wrap(errs.CodeDecisionsRequestFailed, "openrouter decisions request", err,
			errs.WithDetail("model", req.Model))
	}
	return fromOpenRouterDecisionRes(res), nil
}

func toOpenRouterDecisionReq(req sharedModel.DecisionRequest) openRouterDecisionReq {
	questions := make(map[string]openRouterQuestion, len(req.Questions))
	for key, q := range req.Questions {
		questions[key] = openRouterQuestion{
			Type:         q.Type,
			Instructions: q.Instructions,
			Criteria:     q.Criteria,
		}
	}
	return openRouterDecisionReq{
		Model:     req.Model,
		State:     req.State,
		Questions: questions,
	}
}

func fromOpenRouterDecisionRes(res openRouterDecisionRes) *sharedModel.DecisionResponse {
	out := &sharedModel.DecisionResponse{
		Model:   res.Model,
		Answers: res.Answers,
	}
	if res.ID != "" {
		out.ID = &res.ID
	}
	if res.Provider != "" {
		out.Provider = &res.Provider
	}
	if u := res.Usage; u != nil {
		out.Usage = sharedModel.Usage{
			Available:       true,
			InputTokens:     u.InputTokens,
			OutputTokens:    u.OutputTokens,
			TotalTokens:     u.InputTokens + u.OutputTokens,
			CacheReadTokens: u.InputTokensDetails.CachedTokens,
			Cost:            u.Cost,
		}
	}
	return out
}
