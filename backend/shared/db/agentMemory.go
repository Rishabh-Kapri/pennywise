package db

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AgentMemoryRepository interface {
	BaseRepositoryInterface
	GetWorkingMemory(ctx context.Context, tx pgx.Tx, budgetID uuid.UUID) (*model.AgentWorkingMemory, error)
}

type agentMemoryRepo struct {
	BaseRepository
}

func NewAgentMemoryRepository(pool *pgxpool.Pool) AgentMemoryRepository {
	return &agentMemoryRepo{BaseRepository: NewBaseRepository(pool)}
}

func (r *agentMemoryRepo) GetWorkingMemory(
	ctx context.Context,
	tx pgx.Tx,
	budgetID uuid.UUID,
) (*model.AgentWorkingMemory, error) {
	var memory model.AgentWorkingMemory

	err := r.Executor(tx).QueryRow(
		ctx,
		`
		SELECT id, budget_id, document, created_at, updated_at, deleted_at
		FROM working_memory
		WHERE budget_id = $1 AND deleted_at IS NULL
		`, budgetID).Scan(
		&memory.ID,
		&memory.BudgetID,
		&memory.Document,
		&memory.CreatedAt,
		&memory.UpdatedAt,
		&memory.DeletedAt,
	)
	if err != nil {
		return nil, err
	}

	return &memory, nil
}
