-- +goose Up
-- Uploads stored their PDF at statements/<budget>/<import>.pdf but never wrote
-- that key to file_path, so the app thought no PDF was kept. Fill it in for
-- imports made since PDFs started being kept (the commit that added 00024).
-- An import whose file turns out to be missing just answers "no PDF" when
-- opened.
UPDATE statement_imports
SET file_path = 'statements/' || budget_id::text || '/' || id::text || '.pdf'
WHERE file_path = ''
  AND deleted = FALSE
  AND created_at >= TIMESTAMPTZ '2026-11-14 11:23:41+05:30';

-- +goose Down
SELECT 1;
