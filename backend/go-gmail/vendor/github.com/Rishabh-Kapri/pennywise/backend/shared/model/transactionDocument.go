package model

import (
	"time"

	"github.com/google/uuid"
)

// TransactionDocument is a receipt/document attached to a transaction.
// The file body lives on disk under the API's uploads directory; StoragePath
// is relative to that root and never exposed to clients.
type TransactionDocument struct {
	ID            uuid.UUID `json:"id"`
	BudgetID      uuid.UUID `json:"budgetId"`
	TransactionID uuid.UUID `json:"transactionId"`
	FileName      string    `json:"fileName"`
	MimeType      string    `json:"mimeType"`
	SizeBytes     int64     `json:"sizeBytes"`
	StoragePath   string    `json:"-"`
	Deleted       bool      `json:"deleted"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}
