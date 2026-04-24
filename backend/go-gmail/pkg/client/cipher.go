package client

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/google/uuid"
)

type CipherClient struct {
	client *transport.Client
}

func NewCipherClient(client *transport.Client) *CipherClient {
	return &CipherClient{client: client}
}

type PredictRequest struct {
	EmailText string  `json:"emailText"`
	Amount    float64 `json:"amount"`
	Account   string  `json:"account"`
}

type PredictResponse struct {
	PayeeID    uuid.UUID `json:"payeeId"`
	CategoryID uuid.UUID `json:"categoryId"`
	Payee      string    `json:"payee"`
	Category   string    `json:"category"`
	Amount     float64   `json:"amount"`
	Confidence string    `json:"confidence"`
	Source     string    `json:"source"` // pgvector | mlp | fallback
	Reasoning  string    `json:"reasoning,omitempty"`
}

func (c *CipherClient) Predict(ctx context.Context, req PredictRequest) (res *PredictResponse, err error) {
	headers := utils.GetHeaders(ctx)
	logger.Logger(ctx).Info("predict request", "headers", headers)

	resp, err := transport.Post[PredictResponse](ctx, c.client, "/api/predict", headers, req)
	return &resp, err
}
