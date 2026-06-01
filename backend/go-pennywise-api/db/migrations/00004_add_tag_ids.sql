-- +goose Up
-- +goose StatementBegin
ALTER TABLE transactions Add COLUMN IF NOT EXISTS tag_ids UUID[] DEFAULT '{}';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE transactions DROP COLUMN IF EXISTS tag_ids;
-- +goose StatementEnd
