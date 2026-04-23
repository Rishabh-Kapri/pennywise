-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TYPE learning_source AS ENUM ('AUTO_LEARNED', 'MANUAL');
-- Since embeddings are budget scoped, we use specific payee_id and category_id
CREATE TABLE IF NOT EXISTS transaction_embeddings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    budget_id UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    embedding_text TEXT NOT NULL,
    embedding vector(1024) NOT NULL,
    payee_id UUID NOT NULL REFERENCES payees(id) ON DELETE CASCADE,
    category_id UUID NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    amount DECIMAL(12, 2) NOT NULL,
    source learning_source NOT NULL DEFAULT 'MANUAL',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (budget_id, embedding_text)
);

-- Postgres must filter down to the specific user's budget BEFORE doing vector math.
CREATE INDEX idx_embeddings_budget ON transaction_embeddings(budget_id);

-- The Vector Index (HNSW)
-- Hierarchical Navigable Small World (HNSW) is the state-of-the-art index for pgvector.
-- Using 'vector_cosine_ops' optimizes the index for Cosine Distance, which is standard for LLM embeddings.
CREATE INDEX idx_embeddings_vector ON transaction_embeddings 
USING hnsw (embedding vector_cosine_ops);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS transaction_embeddings;
DROP TYPE IF EXISTS learning_source;
-- +goose StatementEnd
