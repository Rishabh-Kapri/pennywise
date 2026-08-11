-- +goose Up
-- +goose StatementBegin
-- marks a cipher prediction as triaged in the review queue; NULL = still pending
ALTER TABLE cipher_predictions ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_cipher_predictions_pending
    ON cipher_predictions (budget_id, created_at DESC) WHERE deleted = FALSE AND reviewed_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_cipher_predictions_pending;
ALTER TABLE cipher_predictions DROP COLUMN IF EXISTS reviewed_at;
-- +goose StatementEnd
