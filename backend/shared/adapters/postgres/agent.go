package postgres

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type agentRepo struct {
	BaseRepository
}

func NewAgentRepository(pool *pgxpool.Pool) repository.AgentRepository {
	return &agentRepo{BaseRepository: NewBaseRepository(pool)}
}

func (r *agentRepo) GetAllConversations(
	ctx context.Context,
	userID uuid.UUID,
	budgetID uuid.UUID,
) ([]model.AgentConversation, error) {
	var conversations []model.AgentConversation
	rows, err := r.Executor(ctx).Query(
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
	if len(conversations) == 0 {
		return conversations, nil
	}

	ids := make([]uuid.UUID, len(conversations))
	for i, c := range conversations {
		ids[i] = c.ID
	}
	usage, err := r.usageGroupedBy(ctx, `ar.conversation_id`,
		`ar.budget_id = $1 AND ar.conversation_id = ANY($2)`, budgetID, ids)
	if err != nil {
		return nil, err
	}
	for i := range conversations {
		if u, ok := usage[conversations[i].ID]; ok {
			conversations[i].Usage = &u.ModelUsage
		}
	}
	return conversations, nil
}

// ConversationUsage implements [repository.AgentRepository].
func (r *agentRepo) ConversationUsage(
	ctx context.Context,
	budgetID, conversationID uuid.UUID,
) (model.ConversationUsage, error) {
	out := model.ConversationUsage{Runs: map[uuid.UUID]model.ModelUsage{}}
	runs, err := r.usageGroupedBy(ctx, `ar.id`,
		`ar.budget_id = $1 AND ar.conversation_id = $2`, budgetID, conversationID)
	if err != nil {
		return out, err
	}
	total := model.ModelUsage{}
	var lastAt time.Time
	add := func(sum **int64, v *int64) {
		if v == nil {
			return
		}
		if *sum == nil {
			*sum = new(int64)
		}
		**sum += *v
	}
	for id, u := range runs {
		out.Runs[id] = u.ModelUsage
		total.Calls += u.Calls
		add(&total.InputTokens, u.InputTokens)
		add(&total.OutputTokens, u.OutputTokens)
		add(&total.CostMicros, u.CostMicros)
		if u.lastAt.After(lastAt) {
			lastAt, total.Model = u.lastAt, u.Model
		}
	}
	out.Total = total
	return out, nil
}

type groupedUsage struct {
	model.ModelUsage
	lastAt time.Time
}

// usageGroupedBy totals llm_usage rows of agent runs (alias ar) matching where,
// grouped by key (a UUID column).
func (r *agentRepo) usageGroupedBy(
	ctx context.Context,
	key, where string,
	args ...any,
) (map[uuid.UUID]groupedUsage, error) {
	rows, err := r.Executor(ctx).Query(ctx, `
		SELECT `+key+`, count(*), sum(u.input_tokens), sum(u.output_tokens), sum(u.cost_micros),
			(array_agg(u.model ORDER BY u.created_at DESC, u.id DESC))[1], max(u.created_at)
		FROM llm_usage u
		JOIN agent_runs ar ON ar.id = u.agent_run_id
		WHERE `+where+`
		GROUP BY 1`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[uuid.UUID]groupedUsage{}
	for rows.Next() {
		var (
			id  uuid.UUID
			g   groupedUsage
			mdl *string
		)
		if err := rows.Scan(&id, &g.Calls, &g.InputTokens, &g.OutputTokens, &g.CostMicros, &mdl, &g.lastAt); err != nil {
			return nil, err
		}
		if mdl != nil {
			g.Model = *mdl
		}
		out[id] = g
	}
	return out, rows.Err()
}

func (r *agentRepo) CreateConversation(
	ctx context.Context,
	params repository.CreateAgentConversationParams,
) (*model.AgentConversation, error) {
	metadata, err := marshalAgentMetadata(params.Metadata)
	if err != nil {
		return nil, err
	}

	var rawMetadata []byte
	var conversation model.AgentConversation
	err = r.Executor(ctx).QueryRow(
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
	conversationID uuid.UUID,
) ([]model.AgentRun, error) {
	rows, err := r.Executor(ctx).Query(
		ctx,
		`SELECT `+agentRunColumns+`
		FROM agent_runs
		WHERE conversation_id = $1
		AND deleted IS NULL`,
		conversationID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []model.AgentRun
	for rows.Next() {
		run, err := scanAgentRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, *run)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(runs) == 0 {
		return runs, nil
	}

	// Earlier runs are replayed to the model with their tool calls.
	runIDs := make([]uuid.UUID, len(runs))
	for i := range runs {
		runIDs[i] = runs[i].ID
	}
	invocations, err := r.listToolInvocations(ctx, runIDs)
	if err != nil {
		return nil, err
	}
	for i := range runs {
		runs[i].ToolInvocations = invocations[runs[i].ID]
	}
	return runs, nil
}

func (r *agentRepo) listToolInvocations(
	ctx context.Context,
	runIDs []uuid.UUID,
) (map[uuid.UUID][]model.ToolInvocation, error) {
	rows, err := r.Executor(ctx).Query(
		ctx,
		`SELECT id, run_id, sequence, turn, tool_call_id, tool_name, input, output,
			is_error, input_bytes, output_bytes, truncated, duration_ms, created_at
		FROM tool_invocations
		WHERE run_id = ANY($1)
		ORDER BY run_id, sequence`,
		runIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byRun := make(map[uuid.UUID][]model.ToolInvocation)
	for rows.Next() {
		var inv model.ToolInvocation
		var toolCallID *string
		var input, output []byte
		if err := rows.Scan(
			&inv.ID, &inv.RunID, &inv.Sequence, &inv.Turn, &toolCallID, &inv.ToolName, &input, &output,
			&inv.IsError, &inv.InputBytes, &inv.OutputBytes, &inv.Truncated, &inv.DurationMs, &inv.CreatedAt,
		); err != nil {
			return nil, err
		}
		if toolCallID != nil {
			inv.ToolCallID = *toolCallID
		}
		if len(input) > 0 {
			inv.Input = json.RawMessage(input)
		}
		if len(output) > 0 {
			inv.Output = json.RawMessage(output)
		}
		byRun[inv.RunID] = append(byRun[inv.RunID], inv)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return byRun, nil
}

func (r *agentRepo) GetConversationForUpdate(
	ctx context.Context,
	id uuid.UUID,
	userID uuid.UUID,
	budgetID uuid.UUID,
	agentKey string,
) (*model.AgentConversation, error) {
	var rawMetadata []byte
	var conversation model.AgentConversation
	err := r.Executor(ctx).QueryRow(
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
	err := r.Executor(ctx).QueryRow(
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

func (r *agentRepo) CreateRun(ctx context.Context, params repository.CreateAgentRunParams) (*model.AgentRun, error) {
	metadata, err := marshalAgentMetadata(params.Metadata)
	if err != nil {
		return nil, err
	}

	row := r.Executor(ctx).QueryRow(
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
		RETURNING `+agentRunColumns,
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
	)
	return scanAgentRun(row)
}

func (r *agentRepo) StartJobRun(ctx context.Context, params repository.StartJobRunParams) (*model.AgentRun, error) {
	metadata, err := marshalAgentMetadata(params.Metadata)
	if err != nil {
		return nil, err
	}
	var run *model.AgentRun
	err = r.InTx(ctx, func(ctx context.Context) error {
		exec := r.Executor(ctx)
		// Lock the job so two claims of it (a reaped attempt still finishing)
		// can't each make a run.
		var existing *uuid.UUID
		if err := exec.QueryRow(ctx,
			`SELECT agent_run_id FROM jobs WHERE id = $1 FOR UPDATE`, params.JobID,
		).Scan(&existing); err != nil {
			if stderrors.Is(err, pgx.ErrNoRows) {
				return notFoundError("Job")
			}
			return err
		}
		if existing != nil {
			// A retry: the run goes back to running and keeps what earlier
			// attempts spent.
			row := exec.QueryRow(ctx,
				`UPDATE agent_runs
				SET status = 'RUNNING'::agent_run_status, completed_at = NULL, error = NULL,
					started_at = COALESCE(started_at, now()), updated_at = now()
				WHERE id = $1 AND budget_id = $2
				RETURNING `+agentRunColumns,
				*existing, params.BudgetID)
			found, err := scanAgentRun(row)
			if err == nil {
				run = found
				return nil
			}
			if !stderrors.Is(err, pgx.ErrNoRows) {
				return err
			}
			// The run was deleted: start a new one.
		}
		row := exec.QueryRow(ctx,
			`INSERT INTO agent_runs (agent_key, user_id, budget_id, status, metadata, started_at)
			SELECT $1::agent_key, b.user_id, b.id, 'RUNNING'::agent_run_status, $3::jsonb, now()
			FROM budgets b WHERE b.id = $2
			RETURNING `+agentRunColumns,
			params.AgentKey, params.BudgetID, metadata)
		created, err := scanAgentRun(row)
		if err != nil {
			if stderrors.Is(err, pgx.ErrNoRows) {
				return notFoundError("Budget")
			}
			return err
		}
		if _, err := exec.Exec(ctx,
			`UPDATE jobs SET agent_run_id = $2, updated_at = now() WHERE id = $1`,
			params.JobID, created.ID); err != nil {
			return err
		}
		run = created
		return nil
	})
	if err != nil {
		return nil, err
	}
	return run, nil
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
	err := r.Executor(ctx).QueryRow(
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
	params repository.CreateConversationMessageParams,
) (*model.ConversationMessage, error) {
	metadata, err := marshalAgentMetadata(params.Metadata)
	if err != nil {
		return nil, err
	}

	var rawMetadata []byte
	var rawContent []byte
	var message model.ConversationMessage
	err = r.Executor(ctx).QueryRow(
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

	if _, err := r.Executor(ctx).Exec(
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

	rows, err := r.Executor(ctx).Query(ctx, query, args...)
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
	_, err = r.Executor(ctx).Exec(ctx, sql, args...)
	return err
}

func (r *agentRepo) DeleteConversation(
	ctx context.Context,
	conversationID uuid.UUID,
	userID uuid.UUID,
	budgetID uuid.UUID,
) error {
	result, err := r.Executor(ctx).Exec(
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

	if _, err := r.Executor(ctx).Exec(
		ctx,
		`UPDATE conversation_messages
		SET deleted = now(), updated_at = now()
		WHERE conversation_id = $1
			AND deleted IS NULL`,
		conversationID,
	); err != nil {
		return err
	}

	// Tool input and output hold the user's data and are only kept to replay
	// the conversation, so they go with it rather than being soft-deleted.
	if _, err := r.Executor(ctx).Exec(
		ctx,
		`DELETE FROM tool_invocations
		WHERE run_id IN (SELECT id FROM agent_runs WHERE conversation_id = $1)`,
		conversationID,
	); err != nil {
		return err
	}

	_, err = r.Executor(ctx).Exec(
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
	_, err = r.Executor(ctx).Exec(ctx, query, messageID, json.RawMessage(dataJSON))
	if err != nil {
		return err
	}
	return nil
}

func (r *agentRepo) RecordRunResult(ctx context.Context, result repository.RecordAgentRunResult) error {
	return r.InTx(ctx, func(ctx context.Context) error {
		exec := r.Executor(ctx)
		if _, err := exec.Exec(ctx, `DELETE FROM tool_invocations WHERE run_id = $1`, result.RunID); err != nil {
			return err
		}
		for i, inv := range result.ToolInvocations {
			sequence := inv.Sequence
			if sequence <= 0 {
				sequence = i + 1
			}
			turn := inv.Turn
			if turn <= 0 {
				turn = 1
			}
			input := inv.Input
			if len(input) == 0 {
				input = json.RawMessage(`{}`)
			}
			var output any
			if len(inv.Output) > 0 {
				output = string(inv.Output)
			}
			var toolCallID *string
			if inv.ToolCallID != "" {
				toolCallID = &inv.ToolCallID
			}
			if _, err := exec.Exec(
				ctx,
				`INSERT INTO tool_invocations (
					run_id, budget_id, sequence, turn, tool_call_id, tool_name, input, output,
					is_error, input_bytes, output_bytes, truncated, duration_ms
				) VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8::jsonb, $9, $10, $11, $12, $13)`,
				result.RunID, result.BudgetID, sequence, turn, toolCallID, inv.ToolName, string(input), output,
				inv.IsError, inv.InputBytes, inv.OutputBytes, inv.Truncated, inv.DurationMs,
			); err != nil {
				return err
			}
		}

		var stopReason, promptVersion *string
		if result.StopReason != "" {
			stopReason = &result.StopReason
		}
		if result.PromptVersion != "" {
			promptVersion = &result.PromptVersion
		}
		// llm_usage is the source of truth; these columns copy its totals for
		// the run's own turns. A sum is NULL when no call reported the value.
		tag, err := exec.Exec(
			ctx,
			`UPDATE agent_runs ar
			SET stop_reason = $3,
				prompt_version = $4,
				input_tokens = u.input_tokens,
				output_tokens = u.output_tokens,
				cache_read_tokens = u.cache_read_tokens,
				cache_write_tokens = u.cache_write_tokens,
				cost_micros = u.cost_micros,
				resolved_provider = u.provider,
				resolved_model = u.model,
				turn_count = u.turns,
				tool_call_count = (SELECT count(*) FROM tool_invocations ti WHERE ti.run_id = ar.id),
				updated_at = now()
			FROM (
				SELECT
					count(*) AS turns,
					sum(input_tokens) AS input_tokens,
					sum(output_tokens) AS output_tokens,
					sum(cache_read_tokens) AS cache_read_tokens,
					sum(cache_write_tokens) AS cache_write_tokens,
					sum(cost_micros) AS cost_micros,
					(array_agg(provider ORDER BY created_at DESC, id DESC))[1] AS provider,
					(array_agg(model ORDER BY created_at DESC, id DESC))[1] AS model
				FROM llm_usage
				WHERE agent_run_id = $1
					-- A chat run's titles and embeddings aren't its turns; a
					-- pipeline run's every call is its work.
					AND (purpose = 'agent_turn'
						OR (SELECT agent_key FROM agent_runs WHERE id = $1) <> 'chat'::agent_key)
			) u
			WHERE ar.id = $1 AND ar.budget_id = $2`,
			result.RunID, result.BudgetID, stopReason, promptVersion,
		)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return notFoundError("Agent run")
		}
		return nil
	})
}

func (r *agentRepo) UpdateEntityMetadata(
	ctx context.Context,
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
	_, err = r.Executor(ctx).Exec(ctx, query, id, json.RawMessage(dataJSON))
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
	run, err := scanAgentRun(r.Executor(ctx).QueryRow(
		ctx,
		`SELECT `+agentRunColumns+`
		FROM agent_runs
		WHERE id = $1
			AND user_id = $2
			AND budget_id = $3
			AND deleted IS NULL`,
		id,
		userID,
		budgetID,
	))
	if stderrors.Is(err, pgx.ErrNoRows) {
		return nil, errs.New(errs.CodeAgentRunNotFound, "agent run not found")
	}
	return run, err
}

// agentRunColumns is what every agent run read selects, in scanAgentRun's order.
const agentRunColumns = `id, agent_key, user_id, budget_id, conversation_id, status,
	model_provider, model_name, temperature, max_tokens, error,
	metadata, started_at, completed_at, created_at, updated_at,
	stop_reason, input_tokens, output_tokens, cache_read_tokens, cache_write_tokens,
	cost_micros, prompt_version, resolved_provider, resolved_model, turn_count, tool_call_count`

func scanAgentRun(row pgx.Row) (*model.AgentRun, error) {
	var run model.AgentRun
	var rawMetadata []byte
	var userID, budgetID uuid.UUID
	if err := row.Scan(
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
		&run.StopReason,
		&run.InputTokens,
		&run.OutputTokens,
		&run.CacheReadTokens,
		&run.CacheWriteTokens,
		&run.CostMicros,
		&run.PromptVersion,
		&run.ResolvedProvider,
		&run.ResolvedModel,
		&run.TurnCount,
		&run.ToolCallCount,
	); err != nil {
		return nil, err
	}
	run.UserID = &userID
	run.BudgetID = &budgetID
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
