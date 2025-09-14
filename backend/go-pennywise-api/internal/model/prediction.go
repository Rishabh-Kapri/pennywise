package model

import (
	"fmt"
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
    UpdatedAt: %v
}`,
		p.ID, p.BudgetID, p.TransactionID, p.EmailText, p.Amount,
		ptrToString(p.Account), ptrToFloat64String(p.AccountPrediction),
		ptrToString(p.Payee), ptrToFloat64String(p.PayeePrediction),
		ptrToString(p.Category), ptrToFloat64String(p.CategoryPrediction),
		ptrToBoolString(p.HasUserCorrected),
		ptrToString(p.UserCorrectedPayee), ptrToString(p.UserCorrectedAccount),
		ptrToString(p.UserCorrectedCategory), p.CreatedAt, p.UpdatedAt)
}
