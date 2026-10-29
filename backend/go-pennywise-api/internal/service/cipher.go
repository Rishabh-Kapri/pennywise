package service

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
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

type CategoryDescriptionResponse struct {
	Description string `json:"description"`
}

type CipherClient interface {
	GenerateTransactionEmbedding(ctx context.Context, req TransactionEmbeddingRequest) (*TransactionEmbeddingResponse, error)
	SuggestCategoryDescription(ctx context.Context, req model.CategoryDescriptionContext) (*CategoryDescriptionResponse, error)
	ExtractStatement(ctx context.Context, req model.StatementExtractRequest) (*model.StatementExtractResponse, error)
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

func (c *cipherClient) SuggestCategoryDescription(
	ctx context.Context,
	req model.CategoryDescriptionContext,
) (*CategoryDescriptionResponse, error) {
	return transport.Post[*CategoryDescriptionResponse](ctx, c.client, "/api/categories/describe", nil, req)
}

func (c *cipherClient) ExtractStatement(
	ctx context.Context,
	req model.StatementExtractRequest,
) (*model.StatementExtractResponse, error) {
	return transport.Post[*model.StatementExtractResponse](ctx, c.client, "/api/statements/extract", nil, req)
}
