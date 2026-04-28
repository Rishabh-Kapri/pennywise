package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	PennywiseTaskQueue                   = "pennywise-tasks"
	GmailActivitiesTaskQueue             = "gmail-activities"
	CipherActivitiesTaskQueue            = "cipher-activities"
	PennywiseActivitiesTaskQueue         = "pennywise-activities"
	EmailToTransactionWorkflowName       = "EmailToTransactionWorkflow"
	ParsedEmailToTransactionWorkflowName = "ParsedEmailToTransactionWorkflow"

	// RetryPredictSignal is sent to a waiting workflow to trigger a manual retry
	// of the Predict step (e.g. after Ollama comes back online).
	RetryPredictSignal = "retry-predict"

	// RetryPredictWaitTimeout is how long the workflow parks waiting for a
	// RetryPredictSignal before permanently failing.
	RetryPredictWaitTimeout = 24 * time.Hour

	// PredictRetryInterval is the fixed delay between automatic Predict activity
	// retry attempts. Set this to the expected Ollama recovery window.
	PredictRetryInterval = 10 * time.Minute
)

// EmailWorflowInput is the input to the EmailToTransactionWorkflow,
// dispatched by go-gmail on receiving a Gmail Pub/Sub notification.
type EmailWorflowInput struct {
	Email     string `json:"email"`
	HistoryId uint64 `json:"historyId"`
}

// ParsedEmail is a single parsed transaction email.
type ParsedEmail struct {
	MessageId       string  `json:"messageId"`
	EmailText       string  `json:"emailText"`
	Amount          float64 `json:"amount"`
	Date            string  `json:"date"`
	TransactionType string  `json:"transactionType"`
	Account         string  `json:"account"`
	Payee           string  `json:"payee"`
	Category        string  `json:"category"`
}

// ParsedEmailsInput is the result of FetchAndParseEmails (go-gmail)
// and the input to Predict (cipher).
type ParsedEmailsInput struct {
	ParsedEmails []ParsedEmail `json:"parsedEmails"`
	BudgetID     uuid.UUID     `json:"budgetId"`
}

// CipherPredictionResult is the result of the Predict activity (cipher).
type CipherPredictionResult struct {
	OriginalRawText string           `json:"rawText"`
	AccountID       uuid.UUID        `json:"accountId"`
	Account         string           `json:"account,omitempty"`
	PayeeID         uuid.UUID        `json:"payeeId"`
	CategoryID      uuid.UUID        `json:"categoryId"`
	Payee           string           `json:"payee,omitempty"`
	Category        string           `json:"category,omitempty"`
	Date            string           `json:"date"`
	Amount          float64          `json:"amount"`
	Confidence      string           `json:"confidence"`
	Source          PredictionSource `json:"source"` // pgvector | rule | llm
	Reasoning       string           `json:"reasoning,omitempty"`
	Metadata        map[string]any   `json:"metadata,omitempty"`
}

type PredictionResultInput struct {
	Predictions []CipherPredictionResult `json:"predictions"`
	BudgetID    uuid.UUID                `json:"budgetId"`
}

// CreateCipherPredictionInput is passed to the CreateCipherPrediction activity.
// It pairs each created transaction with the cipher prediction that generated it.
type CreateCipherPredictionInput struct {
	Transactions []Transaction            `json:"transactions"`
	Predictions  []CipherPredictionResult `json:"predictions"`
	BudgetID     uuid.UUID                `json:"budgetId"`
}
