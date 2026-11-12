package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type agentMemoryRepo struct {
	BaseRepository
}

func NewAgentMemoryRepository(pool *pgxpool.Pool) repository.AgentMemoryRepository {
	return &agentMemoryRepo{BaseRepository: NewBaseRepository(pool)}
}

func (r *agentMemoryRepo) GetWorkingMemory(
	ctx context.Context,
	budgetID uuid.UUID,
) (*model.AgentWorkingMemory, error) {
	var memory model.AgentWorkingMemory

	err := r.Executor(ctx).QueryRow(
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
	budgetID uuid.UUID,
	userID uuid.UUID,
	conversationID uuid.UUID,
) (int, error) {
	var sequence int

	err := r.Executor(ctx).QueryRow(
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
	budgetID uuid.UUID,
	userID uuid.UUID,
	conversationID uuid.UUID,
) ([]model.AgentObservationalMemory, error) {
	var oms []model.AgentObservationalMemory

	rows, err := r.Executor(ctx).Query(
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
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return oms, err
}

func (r *agentMemoryRepo) CreateObservationalMemory(
	ctx context.Context,
	data model.AgentObservationalMemory,
) (*model.AgentObservationalMemory, error) {
	om := data

	obsJSON, err := json.Marshal(data.Observations)
	if err != nil {
		return nil, err
	}

	err = r.Executor(ctx).QueryRow(
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

func (r *agentMemoryRepo) PatchWorkingMemory(
	ctx context.Context,
	budgetID uuid.UUID,
	op repository.WorkingMemoryOp,
	path []string,
	value json.RawMessage,
) error {
	var query string
	switch op {
	case repository.WorkingMemoryAdd:
		query = `
		  INSERT INTO working_memory(budget_id, document)
		  VALUES ($3, jsonb_set('{}', $1::text[], $2::jsonb, true))
		  ON CONFLICT (budget_id) DO UPDATE
		  SET document = jsonb_set(
		    working_memory.document,
		    $1::text[],
			  (COALESCE(working_memory.document #> $1::text[], '[]'::jsonb) || $2::jsonb),
			  true
			),
			updated_at = now()
			`
	case repository.WorkingMemoryReplace:
		query = `
		  INSERT INTO working_memory(budget_id, document)
      VALUES ($3, jsonb_set('{}', $1::text[], $2::jsonb, true))
		  ON CONFLICT (budget_id) DO UPDATE
		  SET document = jsonb_set(
		    working_memory.document,
		    $1::text[],
			  $2::jsonb,
			  true
			),
			updated_at = now()
			`
	case repository.WorkingMemoryRemove:
		query = `
		  INSERT INTO working_memory(budget_id, document)
      VALUES ($3, '{}'::jsonb)
		  ON CONFLICT (budget_id) DO UPDATE
			SET document = jsonb_set(
				working_memory.document,
				$1::text[],
				COALESCE((
					SELECT jsonb_agg(elem)
          FROM jsonb_array_elements(working_memory.document #> $1::text[]) elem
					WHERE elem != $2::jsonb
				), '[]'::jsonb),
				true
			),
			updated_at = now()
		`
	default:
		return repository.ErrUnknownWorkingMemoryOp
	}
	_, err := r.Executor(ctx).Exec(ctx, query, path, string(value), budgetID)
	return err
}
