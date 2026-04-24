package model

import (
	"time"

	"github.com/google/uuid"
)

type TransactionEmbedding struct {
	ID             uuid.UUID `json:"id"`
	BudgetID       uuid.UUID `json:"budgetId"`
	EmbeddingText  string    `json:"embeddingText"`
	PayeeID        uuid.UUID `json:"payeeId"`
	CategoryID     uuid.UUID `json:"categoryId"`
	Amount         float64   `json:"amount"`
	Source         string    `json:"source"` // AUTO_LEARNED | MANUAL
	VectorDistance *float64  `json:"similarity,omitempty"`
	AmountPenalty  *float64  `json:"amountPenalty,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}
