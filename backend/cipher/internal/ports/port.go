// This port defines the contract for an email extract flow
package ports

import (
	"context"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

type Source string

const (
	EmailSource Source = "email"
	SMSSource   Source = "sms"
	TextSource  Source = "text" // used for api calls
)

// Defines the raw input data for this the prediction input flow
type Message struct {
	// The input that we want to extract the information from
	Body   string `json:"body"`
	Source Source `json:"source"`
	Sender string `json:"sender"` // api call, bank email address etc
}

type (
	TransactionType string
	MessageType     string
)

const (
	TransactionMessageType MessageType = "transaction"
	StatementMessageType   MessageType = "statement"
)

type TxnVerdict struct {
	MessageType   MessageType     `json:"messageType"` //
	IsTransaction bool            `json:"isTransaction"`
	Type          TransactionType `json:"type"`
	Confidence    *float64        `json:"confidence,omitempty"` // between 0 - 1
	// Usage is what the model call behind this verdict cost. Nil for adapters
	// that make no paid call (regex, noop).
	Usage *Usage `json:"usage,omitempty"`
}

// Usage of one model call, as the provider reported it.
type Usage struct {
	Provider         string   `json:"provider,omitempty"`
	Model            string   `json:"model"`
	InputTokens      int      `json:"inputTokens"`
	OutputTokens     int      `json:"outputTokens"`
	CacheReadTokens  *int     `json:"cacheReadTokens"`
	CacheWriteTokens *int     `json:"cacheWriteTokens"`
	Cost             *float64 `json:"cost,omitempty"` // nil when the provider doesn't report one
}

// Defines the port that checks whether the current input provided is a transaction or not
type CheckTransaction interface {
	Check(ctx context.Context, msg Message) (TxnVerdict, error)
}

const (
	DebitTransactionType  TransactionType = "debit"
	CreditTransactionType TransactionType = "credit"
)

type ExtractedEntities struct {
	Account   string    `json:"account"`
	Merchant  string    `json:"merchant"`
	Currency  string    `json:"currency"`
	Amount    float64   `json:"amount"`
	Timestamp time.Time `json:"timestamp"`
	// Channel, VPA and Ref come from the verify step. Blank means unknown.
	Channel string `json:"channel,omitempty"`
	VPA     string `json:"vpa,omitempty"`
	Ref     string `json:"ref,omitempty"`
	Usage   *Usage `json:"usage"`
}

type Extract interface {
	Extract(ctx context.Context, msg Message) (ExtractedEntities, error)
}

type VerifyResult struct {
	Valid      bool    `json:"valid"`
	Confidence float64 `json:"confidence"`
	// What verify could add. Blank means it couldn't tell. Account is only set
	// when extract left it blank.
	Channel string `json:"channel,omitempty"`
	VPA     string `json:"vpa,omitempty"`
	Ref     string `json:"ref,omitempty"`
	Account string `json:"account,omitempty"`
	Model   string `json:"model,omitempty"` // the model that answered
}

type Verifier interface {
	Verify(ctx context.Context, msg Message, entities ExtractedEntities) (VerifyResult, error)
}

type PredictionResult struct {
	PayeeName    string  `json:"payeeName"`
	CategoryName string  `json:"categoryName"`
	Confidence   float64 `json:"confidence"` // in the category
	// PayeeConfidence is set when the model scored the payee on its own.
	PayeeConfidence *float64 `json:"payeeConfidence,omitempty"`
	Reasoning       string   `json:"reasoning,omitempty"`
	Model           string   `json:"model,omitempty"`
}

// MemoryLookup finds a payee and category from what the budget already knows:
// payee rules first, then similar past transactions. found is false on a miss,
// which sends the message to the Predictor; a miss is not an error.
type MemoryLookup interface {
	Lookup(
		ctx context.Context,
		msg Message,
		entities ExtractedEntities,
	) (result PredictionResult, found bool, err error)
}

// Predictor picks a payee and category when memory has no answer. An empty
// CategoryName means it couldn't decide, which ends the message without a
// transaction; errors are for calls that failed and are worth retrying.
type Predictor interface {
	Predict(ctx context.Context, msg Message, entities ExtractedEntities) (PredictionResult, error)
}

// StatementImport reads a bank or card statement PDF.
type StatementImport interface {
	// Extract returns the statement's sections and rows. A locked PDF is
	// opened with the first of passwords that works; the response says which
	// (PasswordIndex), or -1 when it needed none.
	Extract(ctx context.Context, pdf []byte, passwords []string) (model.StatementExtractResponse, error)
}
