package db

import (
	"context"
	"encoding/json"
	stderrors "errors"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CreateAgentConversationParams struct {
	UserID   uuid.UUID
	BudgetID uuid.UUID
	AgentKey string
	Title    *string
	Metadata map[string]any
}

type CreateAgentRunParams struct {
	UserID         uuid.UUID
	BudgetID       uuid.UUID
	AgentKey       string
	ConversationID uuid.UUID
	ModelProvider  *string
	ModelName      *string
	Temperature    *float64
	MaxTokens      *int
	Metadata       map[string]any
}

type CreateConversationMessageParams struct {
	ConversationID uuid.UUID
	RunID          *uuid.UUID
	Role           model.Role
	Content        json.RawMessage
	Metadata       map[string]any
}

type AgentRepository interface {
	BaseRepositoryInterface
	CreateConversation(
		ctx context.Context,
		tx pgx.Tx,
		params CreateAgentConversationParams,
	) (*model.AgentConversation, error)
	GetConversationForUpdate(
		ctx context.Context,
		tx pgx.Tx,
		id uuid.UUID,
		userID uuid.UUID,
		budgetID uuid.UUID,
		agentKey string,
	) (*model.AgentConversation, error)
	GetAllConversations(ctx context.Context, userID uuid.UUID, budgetID uuid.UUID) ([]model.AgentConversation, error)
	GetAllConversationRuns(ctx context.Context, tx pgx.Tx, runID uuid.UUID) ([]model.AgentRun, error)
	GetConversation(
		ctx context.Context,
		id uuid.UUID,
		userID uuid.UUID,
		budgetID uuid.UUID,
	) (*model.AgentConversation, error)
	CreateRun(ctx context.Context, tx pgx.Tx, params CreateAgentRunParams) (*model.AgentRun, error)
	GetRun(ctx context.Context, userID uuid.UUID, budgetID uuid.UUID, id uuid.UUID) (*model.AgentRun, error)
	UpdateRunStatus(
		ctx context.Context,
		id uuid.UUID,
		budgetID uuid.UUID,
		status model.AgentRunStatus,
		errorMessage *string,
	) (*model.AgentRun, error)
	CreateConversationMessage(
		ctx context.Context,
		tx pgx.Tx,
		params CreateConversationMessageParams,
	) (*model.ConversationMessage, error)
	ListConversationMessages(
		ctx context.Context,
		conversationID uuid.UUID,
		runID *uuid.UUID,
	) ([]model.ConversationMessage, error)
	UpdateConversation(ctx context.Context, tx pgx.Tx, conversationID uuid.UUID, data model.AgentConversation) error
	DeleteConversation(
		ctx context.Context,
		tx pgx.Tx,
		conversationID uuid.UUID,
		userID uuid.UUID,
		budgetID uuid.UUID,
	) error
	// currently appends a whole message content object
	// @TODO: support content message part append
	UpdateConversationMessageContent(
		ctx context.Context,
		tx pgx.Tx,
		messageID uuid.UUID,
		data []model.MessagePart,
	) error
	UpdateEntityMetadata(
		ctx context.Context,
		tx pgx.Tx,
		entity string,
		id uuid.UUID,
		data map[string]any,
	) error
}

type agentRepo struct {
	BaseRepository
}

func NewAgentRepository(pool *pgxpool.Pool) AgentRepository {
	return &agentRepo{BaseRepository: NewBaseRepository(pool)}
}

func (r *agentRepo) GetAllConversations(
	ctx context.Context,
	userID uuid.UUID,
	budgetID uuid.UUID,
) ([]model.AgentConversation, error) {
	var conversations []model.AgentConversation
	rows, err := r.Executor(nil).Query(
		ctx, `
			SELECT id, agent_key, user_id, budget_id, title, metadata, created_at, updated_at
			FROM conversations
			WHERE user_id = $1
				AND budget_id = $2
				AND deleted IS NULL
			ORDER BY updated_at DESC
		`, userID, budgetID,
	)
	defer rows.Close()
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var conversation model.AgentConversation
		var rawMetadata []byte
		if err := rows.Scan(
			&conversation.ID,
			&conversation.AgentKey,
			&conversation.UserID,
			&conversation.BudgetID,
			&conversation.Title,
			&rawMetadata,
			&conversation.CreatedAt,
			&conversation.UpdatedAt,
		); err != nil {
			return nil, err
		}
		conversation.Metadata = unmarshalAgentMetadata(rawMetadata)
		conversations = append(conversations, conversation)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return conversations, nil
}

func (r *agentRepo) CreateConversation(
	ctx context.Context,
	tx pgx.Tx,
	params CreateAgentConversationParams,
) (*model.AgentConversation, error) {
	metadata, err := marshalAgentMetadata(params.Metadata)
	if err != nil {
		return nil, err
	}

	var rawMetadata []byte
	var conversation model.AgentConversation
	err = r.Executor(tx).QueryRow(
		ctx,
		`INSERT INTO conversations (
			user_id,
			budget_id,
			agent_key,
			title,
			metadata
		) VALUES ($1, $2, $3::agent_key, $4, $5::jsonb)
		RETURNING id, agent_key, user_id, budget_id, title, metadata, created_at, updated_at`,
		params.UserID,
		params.BudgetID,
		params.AgentKey,
		params.Title,
		metadata,
	).Scan(
		&conversation.ID,
		&conversation.AgentKey,
		&conversation.UserID,
		&conversation.BudgetID,
		&conversation.Title,
		&rawMetadata,
		&conversation.CreatedAt,
		&conversation.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	conversation.Metadata = unmarshalAgentMetadata(rawMetadata)
	return &conversation, nil
}

func (r *agentRepo) GetAllConversationRuns(
	ctx context.Context,
	tx pgx.Tx,
	agentID uuid.UUID,
) ([]model.AgentRun, error) {
	var runs []model.AgentRun

	rows, err := r.Executor(tx).Query(
		ctx,
		`SELECT id, agent_key, user_id, budget_id, conversation_id, status, model_provider, model_name, temperature, max_tokens, error, metadata, started_at, completed_at, created_at, updated_at
		FROM agent_runs
		WHERE conversation_id = $1
		AND deleted IS NULL`,

		agentID,
	)
	defer rows.Close()
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var run model.AgentRun
		if err := rows.Scan(
			&run.ID,
			&run.AgentKey,
			&run.UserID,
			&run.BudgetID,
			&run.ConversationID,
			&run.Status,
			&run.ModelProvider,
			&run.ModelName,
			&run.Temperature,
			&run.MaxTokens,
			&run.Error,
			&run.Metadata,
			&run.StartedAt,
			&run.CompletedAt,
			&run.CreatedAt,
			&run.UpdatedAt,
		); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return runs, nil
}

func (r *agentRepo) GetConversationForUpdate(
	ctx context.Context,
	tx pgx.Tx,
	id uuid.UUID,
	userID uuid.UUID,
	budgetID uuid.UUID,
	agentKey string,
) (*model.AgentConversation, error) {
	var rawMetadata []byte
	var conversation model.AgentConversation
	err := r.Executor(tx).QueryRow(
		ctx,
		`SELECT id, agent_key, user_id, budget_id, title, metadata, created_at, updated_at
		FROM conversations
		WHERE id = $1
			AND user_id = $2
			AND budget_id = $3
			AND agent_key = $4::agent_key
			AND deleted IS NULL
		FOR UPDATE`,
		id,
		userID,
		budgetID,
		agentKey,
	).Scan(
		&conversation.ID,
		&conversation.AgentKey,
		&conversation.UserID,
		&conversation.BudgetID,
		&conversation.Title,
		&rawMetadata,
		&conversation.CreatedAt,
		&conversation.UpdatedAt,
	)
	if stderrors.Is(err, pgx.ErrNoRows) {
		return nil, errs.New(errs.CodeAgentConversationNotFound, "agent conversation not found")
	}
	if err != nil {
		return nil, err
	}
	conversation.Metadata = unmarshalAgentMetadata(rawMetadata)
	return &conversation, nil
}

func (r *agentRepo) GetConversation(
	ctx context.Context,
	id uuid.UUID,
	userID uuid.UUID,
	budgetID uuid.UUID,
) (*model.AgentConversation, error) {
	var rawMetadata []byte
	var conversation model.AgentConversation
	err := r.Executor(nil).QueryRow(
		ctx,
		`SELECT id, agent_key, user_id, budget_id, title, metadata, created_at, updated_at
		FROM conversations
		WHERE id = $1
			AND user_id = $2
			AND budget_id = $3
			AND deleted IS NULL`,
		id,
		userID,
		budgetID,
	).Scan(
		&conversation.ID,
		&conversation.AgentKey,
		&conversation.UserID,
		&conversation.BudgetID,
		&conversation.Title,
		&rawMetadata,
		&conversation.CreatedAt,
		&conversation.UpdatedAt,
	)
	if stderrors.Is(err, pgx.ErrNoRows) {
		return nil, errs.New(errs.CodeAgentConversationNotFound, "agent conversation not found")
	}
	if err != nil {
		return nil, err
	}
	conversation.Metadata = unmarshalAgentMetadata(rawMetadata)
	return &conversation, nil
}

func (r *agentRepo) CreateRun(ctx context.Context, tx pgx.Tx, params CreateAgentRunParams) (*model.AgentRun, error) {
	metadata, err := marshalAgentMetadata(params.Metadata)
	if err != nil {
		return nil, err
	}

	var rawMetadata []byte
	var userID uuid.UUID
	var budgetID uuid.UUID
	var run model.AgentRun
	err = r.Executor(tx).QueryRow(
		ctx,
		`INSERT INTO agent_runs (
			agent_key,
			user_id,
			budget_id,
			conversation_id,
			status,
			model_provider,
			model_name,
			temperature,
			max_tokens,
			metadata
		) VALUES ($1::agent_key, $2, $3, $4, $5::agent_run_status, $6, $7, $8, $9, $10::jsonb)
		RETURNING
			id, agent_key, user_id, budget_id, conversation_id, status,
			model_provider, model_name, temperature, max_tokens, error,
			metadata, started_at, completed_at, created_at, updated_at`,
		params.AgentKey,
		params.UserID,
		params.BudgetID,
		params.ConversationID,
		model.AgentRunStatusQueued,
		params.ModelProvider,
		params.ModelName,
		params.Temperature,
		params.MaxTokens,
		metadata,
	).Scan(
		&run.ID,
		&run.AgentKey,
		&userID,
		&budgetID,
		&run.ConversationID,
		&run.Status,
		&run.ModelProvider,
		&run.ModelName,
		&run.Temperature,
		&run.MaxTokens,
		&run.Error,
		&rawMetadata,
		&run.StartedAt,
		&run.CompletedAt,
		&run.CreatedAt,
		&run.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	run.UserID = &userID
	run.BudgetID = &budgetID
	run.Metadata = unmarshalAgentMetadata(rawMetadata)
	return &run, nil
}

func (r *agentRepo) GetRun(
	ctx context.Context,
	userID uuid.UUID,
	budgetID uuid.UUID,
	id uuid.UUID,
) (*model.AgentRun, error) {
	return r.getRun(ctx, userID, budgetID, id)
}

func (r *agentRepo) UpdateRunStatus(
	ctx context.Context,
	id uuid.UUID,
	budgetID uuid.UUID,
	status model.AgentRunStatus,
	errorMessage *string,
) (*model.AgentRun, error) {
	var errorValue any
	if errorMessage != nil {
		errorValue = *errorMessage
	}

	var userID uuid.UUID
	err := r.Executor(nil).QueryRow(
		ctx,
		`UPDATE agent_runs
		SET status = $3::agent_run_status,
			started_at = CASE
				WHEN $3::agent_run_status = 'RUNNING'::agent_run_status
					THEN COALESCE(started_at, now())
				ELSE started_at
			END,
			completed_at = CASE
				WHEN $3::agent_run_status IN (
					'COMPLETED'::agent_run_status,
					'FAILED'::agent_run_status,
					'CANCELLED'::agent_run_status
				)
					THEN COALESCE(completed_at, now())
				ELSE completed_at
			END,
			error = $4,
			updated_at = now()
		WHERE id = $1 AND budget_id = $2 AND deleted IS NULL
		RETURNING user_id`,
		id,
		budgetID,
		status,
		errorValue,
	).Scan(&userID)
	if stderrors.Is(err, pgx.ErrNoRows) {
		return nil, errs.New(errs.CodeAgentRunNotFound, "agent run not found")
	}
	if err != nil {
		return nil, err
	}

	return r.getRun(ctx, userID, budgetID, id)
}

func (r *agentRepo) CreateConversationMessage(
	ctx context.Context,
	tx pgx.Tx,
	params CreateConversationMessageParams,
) (*model.ConversationMessage, error) {
	metadata, err := marshalAgentMetadata(params.Metadata)
	if err != nil {
		return nil, err
	}

	var rawMetadata []byte
	var rawContent []byte
	var message model.ConversationMessage
	err = r.Executor(tx).QueryRow(
		ctx,
		`WITH next_message AS (
			SELECT COALESCE(MAX(sequence), 0) + 1 AS sequence
			FROM conversation_messages
			WHERE conversation_id = $1
		)
		INSERT INTO conversation_messages (
			conversation_id,
			run_id,
			sequence,
		  role,
			content,
			metadata
		)
		SELECT $1, $2, sequence, $3, $4::jsonb, $5::jsonb
		FROM next_message
		RETURNING id, conversation_id, run_id, sequence, role, content, metadata, created_at, updated_at`,
		params.ConversationID,
		params.RunID,
		params.Role,
		params.Content,
		metadata,
	).Scan(
		&message.ID,
		&message.ConversationID,
		&message.RunID,
		&message.Sequence,
		&message.Role,
		&rawContent,
		&rawMetadata,
		&message.CreatedAt,
		&message.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if _, err := r.Executor(tx).Exec(
		ctx,
		`UPDATE conversations SET updated_at = now() WHERE id = $1`,
		params.ConversationID,
	); err != nil {
		return nil, err
	}

	message.Content = json.RawMessage(rawContent)
	message.Metadata = unmarshalAgentMetadata(rawMetadata)
	return &message, nil
}

func (r *agentRepo) ListConversationMessages(
	ctx context.Context,
	conversationID uuid.UUID,
	runID *uuid.UUID,
) ([]model.ConversationMessage, error) {
	query := `SELECT id, conversation_id, run_id, sequence, role, content, metadata, created_at, updated_at
		FROM conversation_messages
		WHERE conversation_id = $1 AND deleted IS NULL`
	args := []any{conversationID}
	if runID != nil {
		query += ` AND run_id = $2`
		args = append(args, *runID)
	}
	query += ` ORDER BY sequence ASC`

	rows, err := r.Executor(nil).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := []model.ConversationMessage{}
	for rows.Next() {
		var rawMetadata []byte
		var message model.ConversationMessage
		if err := rows.Scan(
			&message.ID,
			&message.ConversationID,
			&message.RunID,
			&message.Sequence,
			&message.Role,
			&message.Content,
			&rawMetadata,
			&message.CreatedAt,
			&message.UpdatedAt,
		); err != nil {
			return nil, err
		}
		message.Metadata = unmarshalAgentMetadata(rawMetadata)
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return messages, nil
}

func (r *agentRepo) UpdateConversation(
	ctx context.Context,
	tx pgx.Tx,
	conversationID uuid.UUID,
	data model.AgentConversation,
) error {
	sql, args, hasUpdates, err := buildUpdateConversationSQL(conversationID, data)
	if err != nil {
		return err
	}
	if !hasUpdates {
		return nil
	}
	_, err = r.Executor(tx).Exec(ctx, sql, args...)
	return err
}

func (r *agentRepo) DeleteConversation(
	ctx context.Context,
	tx pgx.Tx,
	conversationID uuid.UUID,
	userID uuid.UUID,
	budgetID uuid.UUID,
) error {
	result, err := r.Executor(tx).Exec(
		ctx,
		`UPDATE conversations
		SET deleted = now(), updated_at = now()
		WHERE id = $1
			AND user_id = $2
			AND budget_id = $3
			AND deleted IS NULL`,
		conversationID,
		userID,
		budgetID,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errs.New(errs.CodeAgentConversationNotFound, "agent conversation not found")
	}

	if _, err := r.Executor(tx).Exec(
		ctx,
		`UPDATE conversation_messages
		SET deleted = now(), updated_at = now()
		WHERE conversation_id = $1
			AND deleted IS NULL`,
		conversationID,
	); err != nil {
		return err
	}

	_, err = r.Executor(tx).Exec(
		ctx,
		`UPDATE agent_runs
		SET deleted = now(), updated_at = now()
		WHERE conversation_id = $1
			AND deleted IS NULL`,
		conversationID,
	)
	return err
}

func buildUpdateConversationSQL(
	conversationID uuid.UUID,
	data model.AgentConversation,
) (string, []any, bool, error) {
	hasUpdates := false
	query := sq.StatementBuilder.PlaceholderFormat(sq.Dollar).Update("conversations")
	if data.Title != nil {
		query = query.Set("title", *data.Title)
		hasUpdates = true
	}
	if !hasUpdates {
		return "", nil, false, nil
	}
	query = query.Set("updated_at", sq.Expr("now()"))
	query = query.Where(sq.Eq{"id": conversationID})
	query = query.Where(sq.Eq{"deleted": nil})

	sql, args, err := query.ToSql()
	if err != nil {
		return "", nil, false, err
	}
	return sql, args, true, nil
}

func (r *agentRepo) UpdateConversationMessageContent(
	ctx context.Context,
	tx pgx.Tx,
	messageID uuid.UUID,
	data []model.MessagePart,
) error {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return err
	}

	query := `
	  UPDATE conversation_messages
	  SET content = content || $2
	  WHERE id = $1
	`
	_, err = r.Executor(tx).Exec(ctx, query, messageID, json.RawMessage(dataJSON))
	if err != nil {
		return err
	}
	return nil
}

func (r *agentRepo) UpdateEntityMetadata(
	ctx context.Context,
	tx pgx.Tx,
	entity string,
	id uuid.UUID,
	data map[string]any,
) error {
	var table string
	switch entity {
	case "run":
		table = "agent_runs"
	case "conversation":
		table = "conversations"
	case "message":
		table = "conversation_messages"
	default:
		return errs.New(errs.CodeInternalError, "invalid entity type")
	}

	dataJSON, err := json.Marshal(data)
	if err != nil {
		return err
	}

	query := "UPDATE " + table + " SET metadata = COALESCE(metadata, '{}'::jsonb) || $2::jsonb WHERE id = $1"
	_, err = r.Executor(tx).Exec(ctx, query, id, json.RawMessage(dataJSON))
	if err != nil {
		return err
	}
	return nil
}

func (r *agentRepo) getRun(
	ctx context.Context,
	userID uuid.UUID,
	budgetID uuid.UUID,
	id uuid.UUID,
) (*model.AgentRun, error) {
	var rawMetadata []byte
	var runUserID uuid.UUID
	var runBudgetID uuid.UUID
	var run model.AgentRun
	err := r.Executor(nil).QueryRow(
		ctx,
		`SELECT
			id, agent_key, user_id, budget_id, conversation_id, status,
			model_provider, model_name, temperature, max_tokens, error,
			metadata, started_at, completed_at, created_at, updated_at
		FROM agent_runs
		WHERE id = $1
			AND user_id = $2
			AND budget_id = $3
			AND deleted IS NULL`,
		id,
		userID,
		budgetID,
	).Scan(
		&run.ID,
		&run.AgentKey,
		&runUserID,
		&runBudgetID,
		&run.ConversationID,
		&run.Status,
		&run.ModelProvider,
		&run.ModelName,
		&run.Temperature,
		&run.MaxTokens,
		&run.Error,
		&rawMetadata,
		&run.StartedAt,
		&run.CompletedAt,
		&run.CreatedAt,
		&run.UpdatedAt,
	)
	if stderrors.Is(err, pgx.ErrNoRows) {
		return nil, errs.New(errs.CodeAgentRunNotFound, "agent run not found")
	}
	if err != nil {
		return nil, err
	}
	run.UserID = &runUserID
	run.BudgetID = &runBudgetID
	run.Metadata = unmarshalAgentMetadata(rawMetadata)
	return &run, nil
}

func marshalAgentMetadata(metadata map[string]any) (string, error) {
	if metadata == nil {
		return "{}", nil
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func unmarshalAgentMetadata(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}

	var metadata map[string]any
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return map[string]any{}
	}
	if metadata == nil {
		return map[string]any{}
	}
	return metadata
}
