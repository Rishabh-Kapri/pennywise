-- +goose Up
-- Databases built by the pre-squash history have the column as
-- "allowed_referers"; 00002 creates it as "allowed_referrers". Rename the old
-- spelling where it exists so every database matches the code.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'api_keys'
          AND column_name = 'allowed_referers'
    ) THEN
        ALTER TABLE api_keys RENAME COLUMN allowed_referers TO allowed_referrers;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- Nothing to undo: the old spelling was a typo, and fresh databases never had it.
SELECT 1;
