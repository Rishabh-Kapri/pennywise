-- +goose Up
-- pgvector: transaction_embeddings and entity_embeddings.
CREATE EXTENSION IF NOT EXISTS vector;

-- Trigram similarity backs the fuzzy category lookup in cipher's LLM fallback:
-- local models routinely return a category name that is semantically right but
-- not byte-identical (dropping the emoji prefix, changing case or spacing), and
-- an exact match there loses the whole prediction.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- +goose Down
DROP EXTENSION IF EXISTS pg_trgm;
DROP EXTENSION IF EXISTS vector;
