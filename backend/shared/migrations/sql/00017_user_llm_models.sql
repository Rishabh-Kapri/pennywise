-- +goose Up
-- Splits a user's AI setup into keys and models. user_llm_providers keeps one
-- key per provider; user_llm_models lists the models used with each key. One
-- key can serve several models, of different kinds: an OpenRouter key can back
-- a chat model and a decision model.
--
-- Kinds: 'chat' is a conversational LLM (extraction, prediction, the
-- assistant). 'decision' is a small classification model called through the
-- decisions API at a user-given endpoint. 'embedding' turns text into the
-- vectors semantic search compares. Each kind has at most one default per user.
CREATE TYPE llm_model_kind AS ENUM ('chat', 'decision', 'embedding');

CREATE TABLE user_llm_models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Same as the key's user; stored for the one-default-per-kind index.
    user_id UUID NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    provider_id UUID NOT NULL REFERENCES user_llm_providers(id) ON DELETE CASCADE,
    kind llm_model_kind NOT NULL,
    slug TEXT NOT NULL,
    -- The decisions API to call, entered by the user. Decision models only.
    endpoint TEXT,
    is_default BOOLEAN NOT NULL DEFAULT false,
    -- Lets a user switch one model off without deleting it.
    enabled BOOLEAN NOT NULL DEFAULT true,
    -- Orders the models of a kind, after the default. Lower runs first.
    priority INTEGER NOT NULL DEFAULT 0,
    -- Set by the test call; a model is only saved once a test passed.
    last_verified_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((kind = 'decision') = (endpoint IS NOT NULL)),
    UNIQUE NULLS NOT DISTINCT (provider_id, kind, slug, endpoint)
);
CREATE UNIQUE INDEX idx_user_llm_models_default ON user_llm_models (user_id, kind) WHERE is_default;
CREATE INDEX idx_user_llm_models_provider ON user_llm_models (provider_id);

-- Every saved provider's model becomes a chat model. Each user's first one in
-- the old fallback order (enabled first, then priority) is their default, and
-- the rest keep that order.
INSERT INTO user_llm_models
    (user_id, provider_id, kind, slug, is_default, priority, last_verified_at, last_error, created_at)
SELECT
    user_id, id, 'chat', model,
    row_number() OVER (PARTITION BY user_id ORDER BY NOT enabled, priority, created_at) = 1,
    row_number() OVER (PARTITION BY user_id ORDER BY NOT enabled, priority, created_at) - 1,
    last_verified_at, last_error, created_at
FROM user_llm_providers;

-- Models and their test results moved to user_llm_models; the default
-- replaced priority.
ALTER TABLE user_llm_providers
    DROP COLUMN model,
    DROP COLUMN priority,
    DROP COLUMN last_verified_at,
    DROP COLUMN last_error;

-- +goose Down
ALTER TABLE user_llm_providers
    ADD COLUMN model TEXT,
    ADD COLUMN priority INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN last_verified_at TIMESTAMPTZ,
    ADD COLUMN last_error TEXT;

-- Each key gets back one model: its default chat model, else its oldest one.
UPDATE user_llm_providers p
SET model = m.slug, last_verified_at = m.last_verified_at, last_error = m.last_error
FROM (
    SELECT DISTINCT ON (provider_id) provider_id, slug, last_verified_at, last_error
    FROM user_llm_models
    WHERE kind = 'chat'
    ORDER BY provider_id, is_default DESC, created_at
) m
WHERE m.provider_id = p.id;
-- A key that only served decision models has no chat model to restore.
DELETE FROM user_llm_providers WHERE model IS NULL;
ALTER TABLE user_llm_providers ALTER COLUMN model SET NOT NULL;

DROP TABLE user_llm_models;
DROP TYPE llm_model_kind;
