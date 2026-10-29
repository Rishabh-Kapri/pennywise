// This port defines the contract for an email extract flow
package ports

import (
	"context"
	"time"
)

type Source string

const (
	EmailSource Source = "email"
	SMSSource   Source = "sms"
	TextSource  Source = "text" // used for api calls
)

// Defines the raw input data for this port
type Message struct {
	// The input that we want to extract the information from
	Body   string `json:"body"`
	Source Source `json:"source"`
	Sender string `json:"sender"` // api call, bank email address etc
}

type TransactionType string

type TxnVerdict struct {
	IsTransaction bool    `json:"IsTransaction"`
	Confidence    float64 `json:"confidence"` // between 0 - 1
}

// Defines the port that checks whether the current input provided is a transaction or not
type CheckTransaction interface {
	IsTransaction(ctx context.Context, msg Message) (TxnVerdict, error)
}

const (
	DebitTransactionType  TransactionType = "debit"
	CreditTransactionType TransactionType = "credit"
)

type ExtractEntities struct {
	Account  string          `json:"account"`
	Merchant string          `json:"merchant"`
	Amount   float64         `json:"amount"`
	Type     TransactionType `json:"type"`
	Date     time.Time       `json:"date"`
}

type Extract interface {
	Extract(ctx context.Context, msg Message) (ExtractEntities, error)
}

type VerifyResult struct {
	Valid      bool    `json:"valid"`
	Confidence float64 `json:"confidence"`
}

type Verifier interface {
	Verify(ctx context.Context, msg Message, entities ExtractEntities) (VerifyResult, error)
}

type PredictionResult struct {
	PayeeName    string  `json:"payeeName"`
	CategoryName string  `json:"categoryName"`
	Confidence   float64 `json:"confidence"`
}

type Predictor interface {
	Predict(ctx context.Context, msg Message, entities ExtractEntities) (PredictionResult, error)
}
