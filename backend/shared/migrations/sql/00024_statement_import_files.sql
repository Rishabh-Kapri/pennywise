-- +goose Up
-- The uploaded PDF is kept so the user can open their statement later. The body
-- lives in the API's document store (document_blobs or UPLOADS_DIR), like a
-- receipt; file_path is its key there. Empty for imports uploaded before this.
ALTER TABLE statement_imports ADD COLUMN file_path TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE statement_imports DROP COLUMN file_path;
