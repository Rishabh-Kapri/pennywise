package model

import "time"

type Transaction struct {
	ID                    string    `json:"id"`
	BudgetID              string    `json:"budgetId"`
	Date                  string    `json:"date"`
	PayeeID               string    `json:"payeeId"`
	CategoryID            string    `json:"categoryId"`
	AccountID             string    `json:"accountId"`
	Note                  string    `json:"note"`
	Amount                float64   `json:"amount"`
	Source                string    `json:"source"`
	TransferAccountID     string    `json:"transferAccountId"`
	TransferTransactionID string    `json:"transferTransactionId"`
	Deleted               bool      `json:"deleted"`
	CreatedAt             time.Time `json:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt"`
}
