package model

import (
	"time"

	"github.com/google/uuid"
)

type Embedding struct {
	ID              uuid.UUID `json:"id"`
	Content         string    `json:"content"`
	DocType         string    `json:"docType"` // "journal_bullet", "daily_summary", "weekly_summary"
	Embedding       []float64 `json:"embedding"`
	SourceID        string    `json:"sourceId"`      // daily_date or weekly_date
	SequenceIndex   int       `json:"sequenceIndex"` // can be null or bullet index for journal_bullet
	Email           string    `json:"email"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
	SimilarityScore *float64   `json:"similarityScore"`
}
