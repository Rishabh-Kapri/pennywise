package client

import (
	"context"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
)

type MLPClient struct {
	client *transport.Client
}

type PredictRequest struct {
	EmailText string  `json:"email_text"`
	Amount    float64 `json:"amount"`
	Type      string  `json:"type"`
	Account   string  `json:"account,omitempty"`
	Payee     string  `json:"payee,omitempty"`
}

type PredictResponse struct {
	Label      string  `json:"label"`
	Confidence float64 `json:"confidence"`
}

func NewMLPClient(transport *transport.Client) *MLPClient {
	return &MLPClient{
		client: transport,
	}
}

// PredictAll calls the MLP predict endpoint sequentially for account, payee, category
// (matching the current go-gmail flow) and returns all three results.
func (c *MLPClient) PredictAll(ctx context.Context, emailText string, amount float64) (account, payee, category *PredictResponse, err error) {
	account, err = c.predict(ctx, PredictRequest{
		EmailText: emailText,
		Amount:    amount,
		Type:      "account",
	})
	if err != nil {
		return nil, nil, nil, errs.Wrap(errs.CodeInternalError, "predict account", err)
	}

	payee, err = c.predict(ctx, PredictRequest{
		EmailText: emailText,
		Amount:    amount,
		Type:      "payee",
		Account:   account.Label,
	})
	if err != nil {
		return account, nil, nil, errs.Wrap(errs.CodeInternalError, "predict payee", err)
	}

	category, err = c.predict(ctx, PredictRequest{
		EmailText: emailText,
		Amount:    amount,
		Type:      "category",
		Account:   account.Label,
		Payee:     payee.Label,
	})
	if err != nil {
		return account, payee, nil, errs.Wrap(errs.CodeInternalError, "predict category", err)
	}

	return account, payee, category, nil
}

func (c *MLPClient) predict(ctx context.Context, req PredictRequest) (*PredictResponse, error) {
	var headers map[string][]string
	res, err := transport.Post[PredictResponse](ctx, c.client, "/predict", headers, req)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "error in mlp predict", err)
	}

	return &res, nil
}
