-- +goose Up
-- AI providers a user brings for cipher (email parsing, statement extraction,
-- agent chat). Nothing falls back to the server's own keys: a user without an
-- enabled row gets no LLM features.
CREATE TYPE llm_provider AS ENUM ('openai', 'anthropic', 'openrouter', 'ollama');

CREATE TABLE user_llm_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    provider llm_provider NOT NULL,
    -- AES-256-GCM (nonce || ciphertext) under LLM_CREDENTIALS_KEY; see
    -- shared/utils/secretbox.go. NULL for a keyless self-hosted ollama.
    api_key_encrypted BYTEA,
    -- Last 4 characters, so the UI can show which key is saved.
    api_key_hint TEXT,
    -- Only for ollama: the user's own server.
    base_url TEXT,
    model TEXT NOT NULL,
    -- Fallback order: the lowest enabled priority is tried first.
    priority INTEGER NOT NULL DEFAULT 0,
    enabled BOOLEAN NOT NULL DEFAULT true,
    last_verified_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, provider)
);
-- Deliberately absent from 00009_agent_readonly_rls: the agent must never read
-- credentials, even encrypted ones.

-- +goose Down
DROP TABLE user_llm_providers;
DROP TYPE llm_provider;
