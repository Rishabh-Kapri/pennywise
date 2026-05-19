-- +goose Up
-- +goose StatementBegin

-- Stores embeddings for budget entities (categories, payees, accounts).
-- Used by cipher's VectorResolver to match free-text query terms to entity IDs
-- without exposing entity names to a cloud LLM.
CREATE TYPE entity_type AS ENUM ('category', 'payee', 'account');

CREATE TABLE IF NOT EXISTS entity_embeddings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    budget_id UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    entity_type entity_type NOT NULL,
    entity_id UUID NOT NULL,
    -- The text that was embedded, e.g. "category: Dining Out/Entertainment"
    embedding_text TEXT NOT NULL,
    embedding vector(1024) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (budget_id, entity_type, entity_id)
);

-- Filter to budget before doing vector math.
CREATE INDEX idx_entity_embeddings_budget ON entity_embeddings(budget_id);

-- HNSW index for cosine similarity search.
CREATE INDEX idx_entity_embeddings_vector ON entity_embeddings
USING hnsw (embedding vector_cosine_ops);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS entity_embeddings;
DROP TYPE IF EXISTS entity_type;
-- +goose StatementEnd
