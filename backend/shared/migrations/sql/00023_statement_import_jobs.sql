-- +goose Up
-- A statement is now read by a statement.extract job in cipher instead of in
-- the API's process. job_id links the import to that job, so the import's
-- progress and failure can be read from it. Older imports have none.
-- No foreign key: the jobs table may live in another database (JOB_STORE_URL).
ALTER TABLE statement_imports ADD COLUMN job_id UUID;

-- +goose Down
ALTER TABLE statement_imports DROP COLUMN job_id;
