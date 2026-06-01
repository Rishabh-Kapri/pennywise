-- +goose Up
-- +goose StatementBegin
CREATE TYPE agent_run_status AS ENUM ('QUEUED', 'RUNNING', 'COMPLETED', 'FAILED', 'CANCELLED');
CREATE TYPE message_role AS ENUM ('user', 'assistant', 'tool', 'system');
CREATE TYPE agent_key AS ENUM ('chat');

CREATE TABLE IF NOT EXISTS conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    budget_id UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    agent_key agent_key NOT NULL DEFAULT 'chat', -- which agent owns this conversation
    title TEXT,
    -- Conversation-level metadata example:
    -- {"defaultModel":"openai/gpt-4.1-mini","titleSource":"auto","lastSummaryAt":"2026-05-08T10:00:00Z"}
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb, -- conversation-level metadata
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS agent_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_key agent_key NOT NULL DEFAULT 'chat',
    user_id UUID NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    budget_id UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    conversation_id UUID REFERENCES conversations(id) ON DELETE SET NULL,
    status agent_run_status NOT NULL DEFAULT 'QUEUED',
    model_provider TEXT,
    model_name TEXT,
    temperature REAL,
    max_tokens INTEGER,
    error TEXT,
    -- Run-level metadata example: 
    -- {"traceId":"trace-123","toolsEnabled":["get_schema","execute_sql"],"inputTokens":1200,"outputTokens":340,"latencyMs":5400}
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted TIMESTAMPTZ,
    CHECK (temperature IS NULL OR temperature >= 0),
    CHECK (max_tokens IS NULL OR max_tokens > 0)
);

CREATE TABLE IF NOT EXISTS conversation_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    run_id UUID REFERENCES agent_runs(id) ON DELETE SET NULL,
    sequence INTEGER NOT NULL,
    role message_role NOT NULL,
    -- Content example:
    -- {"id": "1", "role": "user", "parts": [{"type": "text", "content": "hi!"}], "created_at": "2026-05-14 23:19:41.797 +0530"}
    -- {"id": "2", "role": "assistant", "parts": [{"type": "tool_call", "id": "tool_id_xyz", "name": "get_today", "args": {}, "result": {}], "created_at": "2026-05-14 23:19:41.797 +0530"}
    content JSONB,
    -- Message-level metadata example:
    -- {"stream":true}
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted TIMESTAMPTZ,
    CHECK (sequence > 0),
    UNIQUE (conversation_id, sequence)
);

CREATE INDEX IF NOT EXISTS idx_conversations_budget_updated
    ON conversations(budget_id, updated_at DESC)
    WHERE deleted IS NULL;

CREATE INDEX IF NOT EXISTS idx_agent_runs_budget_status_created
    ON agent_runs(budget_id, status, created_at DESC)
    WHERE deleted IS NULL;

CREATE INDEX IF NOT EXISTS idx_agent_runs_conversation_created
    ON agent_runs(conversation_id, created_at DESC)
    WHERE conversation_id IS NOT NULL AND deleted IS NULL;

CREATE INDEX IF NOT EXISTS idx_conversation_messages_conversation_sequence
    ON conversation_messages(conversation_id, sequence)
    WHERE deleted IS NULL;

CREATE INDEX IF NOT EXISTS idx_conversation_messages_run
    ON conversation_messages(run_id)
    WHERE run_id IS NOT NULL AND deleted IS NULL;

COMMENT ON COLUMN conversations.metadata IS
'Conversation-level metadata example: {"defaultModel":"openai/gpt-4.1-mini","titleSource":"auto","lastSummaryAt":"2026-05-08T10:00:00Z"}';

COMMENT ON COLUMN agent_runs.metadata IS
'Run-level metadata example: {"traceId":"trace-123","toolsEnabled":["get_schema","execute_sql"],"inputTokens":1200,"outputTokens":340,"latencyMs":5400}';

COMMENT ON COLUMN conversation_messages.metadata IS
'Message-level metadata example: {"streamed":true,"final":true,"toolName":"execute_sql","toolCallId":"call_123","durationMs":84}';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS conversation_messages;
DROP TABLE IF EXISTS agent_runs;
DROP TABLE IF EXISTS conversations;
DROP TYPE IF EXISTS agent_key;
DROP TYPE IF EXISTS message_role;
DROP TYPE IF EXISTS agent_run_status;
-- +goose StatementEnd
