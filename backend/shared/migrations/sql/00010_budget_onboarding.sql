-- +goose Up
-- Onboarding progress, kept on the server so the wizard resumes after a reload
-- or on another device. Shape: model.BudgetOnboarding
-- ({step, completedAt, skippedSteps, checklistDismissed}).
ALTER TABLE budgets ADD COLUMN onboarding JSONB NOT NULL DEFAULT '{}';

-- Budgets that predate onboarding are already set up: don't send their owners
-- through the wizard or show them the getting-started checklist.
UPDATE budgets
SET onboarding = jsonb_build_object('step', 'done', 'completedAt', now(), 'checklistDismissed', true);

-- +goose Down
ALTER TABLE budgets DROP COLUMN onboarding;
