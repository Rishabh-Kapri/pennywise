-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS transaction_embeddings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    budget_id UUID NOT NULL,
    embedding_text TEXT NOT NULL,
    embedding vector(1024) NOT NULL,
    payee TEXT NOT NULL,
    category TEXT NOT NULL,
    account TEXT NOT NULL,
    amount FLOAT NOT NULL,
    transaction_id UUID,
    source VARCHAR(20) NOT NULL DEFAULT 'prediction',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_txn_embed_txn_id_unique
    ON transaction_embeddings(transaction_id)
    WHERE transaction_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_txn_embed_cosine
    ON transaction_embeddings
    USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 20);

CREATE INDEX IF NOT EXISTS idx_txn_embed_budget
    ON transaction_embeddings(budget_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS transaction_embeddings;
-- +goose StatementEnd
