package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

// WorkingMemoryOp is how PatchWorkingMemory changes the value at a path.
type WorkingMemoryOp string

const (
	WorkingMemoryAdd     WorkingMemoryOp = "add"     // append to the array at path
	WorkingMemoryReplace WorkingMemoryOp = "replace" // overwrite the value at path
	WorkingMemoryRemove  WorkingMemoryOp = "remove"  // drop matching elements from the array at path
)

// ErrUnknownWorkingMemoryOp is returned for an op outside the constants above.
var ErrUnknownWorkingMemoryOp = errors.New("unknown working memory operation")

type AgentMemoryRepository interface {
	// PatchWorkingMemory applies op with value (JSON) at path in the budget's
	// working memory document, creating the document when missing.
	PatchWorkingMemory(ctx context.Context, budgetID uuid.UUID, op WorkingMemoryOp, path []string, value json.RawMessage) error
	GetWorkingMemory(ctx context.Context, budgetID uuid.UUID) (*model.AgentWorkingMemory, error)
	GetLastObservedSequence(
		ctx context.Context,
		budgetID uuid.UUID,
		userID uuid.UUID,
		conversationID uuid.UUID,
	) (int, error)
	GetObservationalMemory(
		ctx context.Context,
		budgetID uuid.UUID,
		userID uuid.UUID,
		conversationID uuid.UUID,
	) ([]model.AgentObservationalMemory, error)
	CreateObservationalMemory(
		ctx context.Context,
		data model.AgentObservationalMemory,
	) (*model.AgentObservationalMemory, error)
	UpdateObservationalMemory(
		ctx context.Context,
		id uuid.UUID,
		data model.AgentObservationalMemory,
	) (*model.AgentObservationalMemory, error)
}
