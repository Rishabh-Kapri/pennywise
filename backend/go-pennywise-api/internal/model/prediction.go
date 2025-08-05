package model

import (
	"time"

	"github.com/google/uuid"
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
}
