package model

import (
	"time"

	"github.com/google/uuid"
)

type TransactionEmbedding struct {
	ID            uuid.UUID `json:"id"`
	BudgetID      uuid.UUID `json:"budgetId"`
	EmbeddingText string    `json:"embeddingText"`
	Payee         string    `json:"payee"`
	Category      string    `json:"category"`
	Account       string    `json:"account"`
	Amount        float64   `json:"amount"`
	TransactionID *uuid.UUID `json:"transactionId,omitempty"`
	Source        string    `json:"source"` // prediction | user_confirmed | user_corrected
	Similarity    *float64  `json:"similarity,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}
