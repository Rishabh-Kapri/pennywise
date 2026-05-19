package memory

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/db"

	"github.com/google/uuid"
)

type Memory interface {
	GetWorkingMemory(ctx context.Context, budgetID uuid.UUID) string
}

type memory struct {
	agentMemoryRepo db.AgentMemoryRepository
}

func NewMemoryService(agentMemoryRepo db.AgentMemoryRepository) Memory {
	return &memory{agentMemoryRepo: agentMemoryRepo}
}

func (m *memory) GetWorkingMemory(ctx context.Context, budgetID uuid.UUID) string {
	workingMemory, err := m.agentMemoryRepo.GetWorkingMemory(ctx, nil, budgetID)
	if err != nil {
		return ""
	}

	return string(workingMemory.Document)
}
