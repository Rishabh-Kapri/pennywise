package db

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AgentMemoryRepository interface {
	BaseRepositoryInterface
	GetWorkingMemory(ctx context.Context, tx pgx.Tx, budgetID uuid.UUID) (*model.AgentWorkingMemory, error)
	GetLastObservedSequence(
		ctx context.Context,
		tx pgx.Tx,
		budgetID uuid.UUID,
		userID uuid.UUID,
		conversationID uuid.UUID,
	) (int, error)
	GetObservationalMemory(
		ctx context.Context,
		tx pgx.Tx,
		budgetID uuid.UUID,
		userID uuid.UUID,
		conversationID uuid.UUID,
	) ([]model.AgentObservationalMemory, error)
	CreateObservationalMemory(
		ctx context.Context,
		tx pgx.Tx,
		data model.AgentObservationalMemory,
	) (*model.AgentObservationalMemory, error)
	UpdateObservationalMemory(
		ctx context.Context,
		tx pgx.Tx,
		id uuid.UUID,
		data model.AgentObservationalMemory,
	) (*model.AgentObservationalMemory, error)
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

func (r *agentMemoryRepo) GetLastObservedSequence(
	ctx context.Context,
	tx pgx.Tx,
	budgetID uuid.UUID,
	userID uuid.UUID,
	conversationID uuid.UUID,
) (int, error) {
	var sequence int

	err := r.Executor(tx).QueryRow(
		ctx,
		`
		SELECT sequence_end
		FROM observational_memory
		WHERE budget_id = $1 AND user_id = $2 AND conversation_id = $3 AND deleted_at IS NULL
		ORDER BY sequence_end DESC
		LIMIT 1
		`, budgetID, userID, conversationID).Scan(&sequence)

	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}

	if err != nil {
		return 0, err
	}

	return sequence, nil
}

func (r *agentMemoryRepo) GetObservationalMemory(
	ctx context.Context,
	tx pgx.Tx,
	budgetID uuid.UUID,
	userID uuid.UUID,
	conversationID uuid.UUID,
) ([]model.AgentObservationalMemory, error) {
	var oms []model.AgentObservationalMemory

	rows, err := r.Executor(tx).Query(
		ctx, `
		  SELECT id, budget_id, user_id, conversation_id, sequence_start, sequence_end, observations, current_task, suggested_response, created_at, updated_at
		  FROM observational_memory
		  WHERE budget_id = $1 AND user_id = $2 AND conversation_id = $3 AND deleted_at IS NULL
		`, budgetID, userID, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var om model.AgentObservationalMemory
		var obsJSON []byte

		if err := rows.Scan(
			&om.ID,
			&om.BudgetID,
			&om.UserID,
			&om.ConversationID,
			&om.SequenceStart,
			&om.SequenceEnd,
			&obsJSON,
			&om.CurrentTask,
			&om.SuggestedResponse,
			&om.CreatedAt,
			&om.UpdatedAt,
		); err != nil {
			return nil, err
		}

		if err := json.Unmarshal(obsJSON, &om.Observations); err != nil {
			return nil, err
		}
		oms = append(oms, om)
	}

	return oms, err
}

func (r *agentMemoryRepo) CreateObservationalMemory(
	ctx context.Context,
	tx pgx.Tx,
	data model.AgentObservationalMemory,
) (*model.AgentObservationalMemory, error) {
	om := data

	obsJSON, err := json.Marshal(data.Observations)
	if err != nil {
		return nil, err
	}

	err = r.Executor(tx).QueryRow(
		ctx, `
		  INSERT INTO observational_memory (
		    budget_id,
		    user_id,
		    conversation_id,
		    sequence_start,
		    sequence_end,
		    observations,
		    current_task,
		    suggested_response
		  ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		  RETURNING id, created_at, updated_at
		`, data.BudgetID,
		data.UserID,
		data.ConversationID,
		data.SequenceStart,
		data.SequenceEnd,
		obsJSON,
		data.CurrentTask,
		data.SuggestedResponse,
	).Scan(&om.ID, &om.CreatedAt, &om.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &om, nil
}

func (r *agentMemoryRepo) UpdateObservationalMemory(
	ctx context.Context,
	tx pgx.Tx,
	id uuid.UUID,
	data model.AgentObservationalMemory,
) (*model.AgentObservationalMemory, error) {
	return nil, nil
}
