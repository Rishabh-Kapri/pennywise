package model

import (
	"time"

	"github.com/google/uuid"
)

// PipelineStepOverride is a budget's change to one step of a system pipeline
// (shared/pipeline). A nil field follows the default: the step's adapter and
// fallback setting, or the owner's default model of the kind the adapter needs.
type PipelineStepOverride struct {
	BudgetID  uuid.UUID  `json:"-"`
	Pipeline  string     `json:"pipeline"`
	Step      string     `json:"step"`
	Adapter   *string    `json:"adapter"`
	ModelID   *uuid.UUID `json:"modelId"`
	Fallback  *bool      `json:"fallback"`
	UpdatedAt time.Time  `json:"updatedAt"`
}
