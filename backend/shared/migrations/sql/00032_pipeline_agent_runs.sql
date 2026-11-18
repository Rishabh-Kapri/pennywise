-- +goose Up
-- Pipeline steps that call a model (message check/extract/verify/predict,
-- statement reading) record an agent run, one per job: retries of the job add
-- to the same run. The job holds the run's id, so a job's tokens and cost are
-- one join away.
ALTER TYPE agent_key ADD VALUE IF NOT EXISTS 'pipeline';

ALTER TABLE jobs
    ADD COLUMN agent_run_id UUID REFERENCES agent_runs(id) ON DELETE SET NULL;
CREATE INDEX idx_jobs_agent_run ON jobs (agent_run_id) WHERE agent_run_id IS NOT NULL;

-- +goose Down
DROP INDEX idx_jobs_agent_run;
ALTER TABLE jobs DROP COLUMN agent_run_id;
-- Postgres can't drop an enum value. An unused 'pipeline' label is harmless.
