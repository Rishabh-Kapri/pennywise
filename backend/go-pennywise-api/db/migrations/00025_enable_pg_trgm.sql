-- +goose Up
-- +goose StatementBegin
-- Trigram similarity backs the fuzzy category lookup in cipher's LLM fallback:
-- local models routinely return a category name that is semantically right but
-- not byte-identical (dropping the emoji prefix, changing case or spacing), and
-- an exact match there loses the whole prediction.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- The lookup is always budget-scoped and matches on categories.name, so a
-- GIN trigram index on the name keeps it cheap as budgets grow.
CREATE INDEX IF NOT EXISTS idx_categories_name_trgm
    ON categories USING gin (name gin_trgm_ops);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_categories_name_trgm;
DROP EXTENSION IF EXISTS pg_trgm;
-- +goose StatementEnd
