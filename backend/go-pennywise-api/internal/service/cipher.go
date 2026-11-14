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

// DecisionMessage is the input cipher's /api/test-decisions classifies
// (cipher's ports.Message).
type DecisionMessage struct {
	Body   string `json:"body"`
	Source string `json:"source"`
	Sender string `json:"sender"`
}

// DecisionUsage mirrors cipher's ports.Usage: one model call, as the provider reported it.
type DecisionUsage struct {
	Provider     string   `json:"provider,omitempty"`
	Model        string   `json:"model"`
	InputTokens  int      `json:"inputTokens"`
	OutputTokens int      `json:"outputTokens"`
	Cost         *float64 `json:"cost,omitempty"`
}

// DecisionVerdict mirrors cipher's ports.TxnVerdict.
type DecisionVerdict struct {
	IsTransaction bool           `json:"isTransaction"`
	Type          string         `json:"type"`
	Confidence    *float64       `json:"confidence,omitempty"`
	Usage         *DecisionUsage `json:"usage,omitempty"`
}

// DecisionsClient is kept apart from CipherClient so the SMS test flow
// doesn't widen the interface every CipherClient fake implements.
type DecisionsClient interface {
	TestDecisions(ctx context.Context, msg DecisionMessage) (*DecisionVerdict, error)
}

func NewDecisionsClient(client *transport.Client) DecisionsClient {
	return &cipherClient{client: client}
}

func (c *cipherClient) TestDecisions(ctx context.Context, msg DecisionMessage) (*DecisionVerdict, error) {
	return transport.Post[*DecisionVerdict](ctx, c.client, "/api/test-decisions", nil, msg)
}
