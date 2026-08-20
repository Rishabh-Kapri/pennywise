package model

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PredictionSource mirrors the prediction_source DB enum.
type PredictionSource string

const (
	PredictionSourceLLM           PredictionSource = "LLM"
	PredictionSourceManual        PredictionSource = "MANUAL"
	PredictionSourceRule          PredictionSource = "RULE"
	PredictionSourceVector        PredictionSource = "VECTOR"
	PredictionSourceUncategorized PredictionSource = "UNCATEGORIZED"
)

type Prediction struct {
	ID                    uuid.UUID `json:"id"`
	BudgetID              uuid.UUID `json:"budgetId"`
	TransactionID         uuid.UUID `json:"transactionId"`
	EmailText             string    `json:"emailText"`
	Amount                float64   `json:"amount"`
	Account               *string   `json:"account,omitempty"`
	AccountPrediction     *float64  `json:"accountPrediction,omitempty"`
	Payee                 *string   `json:"payee,omitempty"`
	PayeePrediction       *float64  `json:"payeePrediction,omitempty"`
	Category              *string   `json:"category,omitempty"`
	CategoryPrediction    *float64  `json:"categoryPrediction,omitempty"`
	HasUserCorrected      *bool     `json:"hasUserCorrected,omitempty"`
	UserCorrectedPayee    *string   `json:"userCorrectedPayee,omitempty"`
	UserCorrectedAccount  *string   `json:"userCorrectedAccount,omitempty"`
	UserCorrectedCategory *string   `json:"userCorrectedCategory,omitempty"`
	CreatedAt             time.Time `json:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt"`
	Deleted               bool      `json:"deleted"`
}

// CipherPredictionRecord maps to the cipher_predictions table.
type CipherPredictionRecord struct {
	ID                  uuid.UUID        `json:"id"`
	BudgetID            uuid.UUID        `json:"budgetId"`
	TransactionID       uuid.UUID        `json:"transactionId"`
	EmailText           *string          `json:"emailText,omitempty"`
	LLMReasoning        *string          `json:"llmReasoning,omitempty"`
	Metadata            json.RawMessage  `json:"metadata,omitempty"`
	Amount              *float64         `json:"amount,omitempty"`
	ExtractedAccount    *string          `json:"extractedAccount,omitempty"`
	ExtractedPayee      *string          `json:"extractedPayee,omitempty"`
	PredictedPayeeID    *uuid.UUID       `json:"predictedPayeeId,omitempty"`
	PredictedCategoryID *uuid.UUID       `json:"predictedCategoryId,omitempty"`
	AccountConfidence   *float64         `json:"accountConfidence,omitempty"`
	PayeeConfidence     *float64         `json:"payeeConfidence,omitempty"`
	CategoryConfidence  *float64         `json:"categoryConfidence,omitempty"`
	Source              PredictionSource `json:"source"`
	HasUserCorrected    bool             `json:"hasUserCorrected"`
	ActualPayeeID       *uuid.UUID       `json:"actualPayeeId,omitempty"`
	ActualCategoryID    *uuid.UUID       `json:"actualCategoryId,omitempty"`
	ReviewedAt          *time.Time       `json:"reviewedAt,omitempty"`
	// LearnedAt is when this prediction last taught the pipeline (payee rule +
	// embedding); nil means it never has. LearnError carries why the last
	// attempt failed, and is cleared on success.
	LearnedAt  *time.Time `json:"learnedAt,omitempty"`
	LearnError *string    `json:"learnError,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
	Deleted    bool       `json:"deleted"`
}

// LearningCandidate is a transaction whose cipher prediction has not taught the
// pipeline yet, carrying the fields the learning step needs so the backfill can
// run without a second lookup per row.
type LearningCandidate struct {
	TransactionID uuid.UUID  `json:"transactionId"`
	PayeeID       *uuid.UUID `json:"payeeId,omitempty"`
	CategoryID    *uuid.UUID `json:"categoryId,omitempty"`
	Amount        float64    `json:"amount"`
	RawBankText   string     `json:"rawBankText"`
	LearnError    *string    `json:"learnError,omitempty"`
}

// LearningStats summarises how much of a budget's prediction history has been
// learned from. Pending counts only rows a backfill can actually process, of
// which Failed is the subset whose last attempt errored; Skipped counts rows
// that can never be learned from (no payee, category or raw bank text). Total =
// Learned + Pending + Skipped.
type LearningStats struct {
	Total   int `json:"total"`
	Learned int `json:"learned"`
	Pending int `json:"pending"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
	// Running reports whether a backfill is working through the pending set
	// right now, so a poller can tell "nothing left" from "still going".
	Running bool `json:"running"`
}

// LearningBackfillRequest asks the API to (re)run learning over past
// predictions. Limit is clamped by the service; RetryFailed includes rows whose
// previous attempt recorded an error.
type LearningBackfillRequest struct {
	Limit       int  `json:"limit"`
	RetryFailed bool `json:"retryFailed"`
}

// LearningBackfillResult acknowledges a backfill request. The work runs in the
// background -- each item costs two LLM round-trips, far past any sane request
// timeout -- so this reports what was queued, not what was done. Started is
// false with Running true when a run was already in flight, and false with
// Queued zero when there was nothing left to learn. Follow progress by polling
// the learning status endpoint.
type LearningBackfillResult struct {
	Started   bool `json:"started"`
	Running   bool `json:"running"`
	Queued    int  `json:"queued"`
	Remaining int  `json:"remaining"`
}

// PredictionReviewItem is a cipher prediction joined with the transaction it
// classified, so the review queue can show predicted vs. current values
// without a second round trip per row.
type PredictionReviewItem struct {
	CipherPredictionRecord

	TransactionDate     Date       `json:"transactionDate"`
	TransactionAmount   float64    `json:"transactionAmount"`
	TransactionNote     string     `json:"transactionNote"`
	AccountName         *string    `json:"accountName,omitempty"`
	CurrentPayeeID      *uuid.UUID `json:"currentPayeeId,omitempty"`
	CurrentPayeeName    *string    `json:"currentPayeeName,omitempty"`
	CurrentCategoryID   *uuid.UUID `json:"currentCategoryId,omitempty"`
	CurrentCategoryName *string    `json:"currentCategoryName,omitempty"`

	PredictedPayeeName    *string `json:"predictedPayeeName,omitempty"`
	PredictedCategoryName *string `json:"predictedCategoryName,omitempty"`
}

type PredictionReviewAction string

const (
	// PredictionReviewAccept confirms the prediction matches reality.
	PredictionReviewAccept PredictionReviewAction = "accept"
	// PredictionReviewCorrect rewrites the transaction's payee/category and
	// records the correction as training signal.
	PredictionReviewCorrect PredictionReviewAction = "correct"
)

type PredictionReviewRequest struct {
	Action     PredictionReviewAction `json:"action"`
	PayeeID    *uuid.UUID             `json:"payeeId,omitempty"`
	CategoryID *uuid.UUID             `json:"categoryId,omitempty"`
}

type TransactionPredictionDetails struct {
	Prediction       *Prediction             `json:"prediction,omitempty"`
	CipherPrediction *CipherPredictionRecord `json:"cipherPrediction,omitempty"`
}

func (p *Prediction) String() string {
	return fmt.Sprintf(`Prediction{
    ID: %v,
    BudgetID: %v,
    TransactionID: %v,
    EmailText: %q,
    Amount: %.2f,
    Account: %s,
    AccountPrediction: %s,
    Payee: %s,
    PayeePrediction: %s,
    Category: %s,
    CategoryPrediction: %s,
    HasUserCorrected: %s,
    UserCorrectedPayee: %s,
    UserCorrectedAccount: %s,
    UserCorrectedCategory: %s,
    CreatedAt: %v,
    UpdatedAt: %v,
		Deleted: %v
}`,
		p.ID, p.BudgetID, p.TransactionID, p.EmailText, p.Amount,
		ptrToString(p.Account), ptrToFloat64String(p.AccountPrediction),
		ptrToString(p.Payee), ptrToFloat64String(p.PayeePrediction),
		ptrToString(p.Category), ptrToFloat64String(p.CategoryPrediction),
		ptrToBoolString(p.HasUserCorrected),
		ptrToString(p.UserCorrectedPayee), ptrToString(p.UserCorrectedAccount),
		ptrToString(p.UserCorrectedCategory), p.CreatedAt, p.UpdatedAt, p.Deleted)
}
