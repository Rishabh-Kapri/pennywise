-- +goose Up
-- A deleted transaction's cipher prediction is deleted with it from now on.
-- This catches up the ones deleted before.
UPDATE cipher_predictions cp
SET deleted = TRUE, updated_at = NOW()
FROM transactions t
WHERE t.id = cp.transaction_id AND t.deleted = TRUE AND cp.deleted = FALSE;

-- +goose Down
-- Which predictions were live before can't be told apart, so they stay deleted.
SELECT 1;
