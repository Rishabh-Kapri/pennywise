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
	ExtractedMerchant   *string          `json:"extractedMerchant,omitempty"`
	PredictedPayeeID    *uuid.UUID       `json:"predictedPayeeId,omitempty"`
	PredictedCategoryID *uuid.UUID       `json:"predictedCategoryId,omitempty"`
	AccountConfidence   *float64         `json:"accountConfidence,omitempty"`
	PayeeConfidence     *float64         `json:"payeeConfidence,omitempty"`
	CategoryConfidence  *float64         `json:"categoryConfidence,omitempty"`
	Source              PredictionSource `json:"source"`
	HasUserCorrected    bool             `json:"hasUserCorrected"`
	ActualPayeeID       *uuid.UUID       `json:"actualPayeeId,omitempty"`
	ActualCategoryID    *uuid.UUID       `json:"actualCategoryId,omitempty"`
	CreatedAt           time.Time        `json:"createdAt"`
	UpdatedAt           time.Time        `json:"updatedAt"`
	Deleted             bool             `json:"deleted"`
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
