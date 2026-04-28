package service

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
)

type TransactionEmbeddingRequest struct {
	RawBankText string  `json:"rawBankText"`
	Amount      float64 `json:"amount"`
}

type TransactionEmbeddingResponse struct {
	MatchString   string `json:"matchString"`
	EmbeddingText string `json:"embeddingText"`
	Embedding     string `json:"embedding"`
}

type CipherClient interface {
	GenerateTransactionEmbedding(ctx context.Context, req TransactionEmbeddingRequest) (*TransactionEmbeddingResponse, error)
}

type cipherClient struct {
	client *transport.Client
}

func NewCipherClient(client *transport.Client) CipherClient {
	return &cipherClient{client: client}
}

func (c *cipherClient) GenerateTransactionEmbedding(ctx context.Context, req TransactionEmbeddingRequest) (*TransactionEmbeddingResponse, error) {
	return transport.Post[*TransactionEmbeddingResponse](ctx, c.client, "/api/embeddings/transaction", nil, req)
}
