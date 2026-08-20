-- +goose Up
-- +goose StatementBegin
-- Records whether the pipeline has learned from this prediction (payee rule +
-- AUTO_LEARNED embedding). NULL = never learned. Learning runs best-effort in a
-- goroutine and every failure used to be log-only, so there was no way to tell
-- a prediction that taught the pipeline from one whose learning step died.
ALTER TABLE cipher_predictions ADD COLUMN IF NOT EXISTS learned_at TIMESTAMPTZ;

-- Why the last learning attempt failed; cleared on success. Kept alongside a
-- NULL learned_at so a backfill can retry only the rows that actually broke.
ALTER TABLE cipher_predictions ADD COLUMN IF NOT EXISTS learn_error TEXT;

-- Drives the backfill's "what still needs learning" scan.
CREATE INDEX IF NOT EXISTS idx_cipher_predictions_unlearned
    ON cipher_predictions (budget_id, created_at DESC)
    WHERE deleted = FALSE AND learned_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_cipher_predictions_unlearned;
ALTER TABLE cipher_predictions DROP COLUMN IF EXISTS learn_error;
ALTER TABLE cipher_predictions DROP COLUMN IF EXISTS learned_at;
-- +goose StatementEnd
