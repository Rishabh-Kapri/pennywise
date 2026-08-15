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

// DocumentListItem is a document carrying the transaction context a
// budget-wide library needs. Embedding the document rather than repeating its
// fields keeps one JSON shape for clients: everything TransactionDocument has,
// plus the payee/date/amount a gallery tile shows without a second request per
// document.
type DocumentListItem struct {
	TransactionDocument
	TransactionDate   Date    `json:"transactionDate"`
	TransactionAmount float64 `json:"transactionAmount"`
	PayeeName         *string `json:"payeeName,omitempty"`
	AccountName       *string `json:"accountName,omitempty"`
	CategoryName      *string `json:"categoryName,omitempty"`
}

// DocumentKind groups mime types into the two categories worth filtering on.
// The stored mime type is finer-grained than anyone browsing a receipt library
// cares about -- the question is only ever "photo or PDF".
type DocumentKind string

const (
	DocumentKindAll   DocumentKind = ""
	DocumentKindImage DocumentKind = "image"
	DocumentKindPDF   DocumentKind = "pdf"
)

func (k DocumentKind) Valid() bool {
	return k == DocumentKindAll || k == DocumentKindImage || k == DocumentKindPDF
}

// DocumentFilter narrows a document library query.
//
// Dates filter on the transaction's date, not the upload's: a receipt belongs
// to when the money moved, which is the same rule the stored file name follows.
type DocumentFilter struct {
	// Search matches the file name and the payee name, case-insensitively.
	Search    string
	Kind      DocumentKind
	StartDate string
	EndDate   string
	Limit     int
	Offset    int
}

// DocumentLibraryResponse is an offset page of the document library.
//
// Offset rather than the cursor pagination transactions use: this list is
// browsed by scrolling a grid from the newest end, never jumped into at an
// arbitrary point, and offsets survive the filter changes a cursor would
// invalidate.
type DocumentLibraryResponse struct {
	Data    []DocumentListItem `json:"data"`
	Total   int                `json:"total"`
	HasMore bool               `json:"hasMore"`
}
