-- +goose Up
-- A budget's changes to the system pipelines (shared/pipeline/pipelines/*.json).
-- Only what the budget changed is stored, so steps it never touched keep
-- following the system default as it changes. NULL = the step's default
-- adapter, or the owner's default model of the kind the adapter needs.
-- pipeline, step and adapter are IDs from those files, not enums: a file can
-- retire one, and readers then fall back to the default.
CREATE TABLE budget_pipeline_steps (
    budget_id UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    pipeline TEXT NOT NULL,
    step TEXT NOT NULL,
    adapter TEXT,
    -- Deleting the model sends the step back to the default model.
    model_id UUID REFERENCES user_llm_models(id) ON DELETE SET NULL,
    -- Whether the step tries the owner's other models of the kind when its
    -- model fails. NULL = the step's default in the pipeline file.
    fallback BOOLEAN,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (budget_id, pipeline, step)
);

-- +goose Down
DROP TABLE budget_pipeline_steps;
