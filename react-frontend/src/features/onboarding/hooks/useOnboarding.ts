import { useCallback } from 'react';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { budgetOnboardingUpdated } from '@/features/budget';
import type { RootState } from '@/app/store';
import { updateOnboarding } from '../api';
import { ONBOARDING_STEPS, type BudgetOnboarding, type OnboardingStep, type UpdateOnboardingPayload } from '../types';

/**
 * The selected budget's onboarding state. allBudgets is preferred because it
 * is refreshed by fetchAllBudgets while selectedBudget can be a stale copy.
 * A budget from an API without onboarding counts as done.
 */
export const selectOnboarding = (state: RootState): BudgetOnboarding | null => {
  const selected = state.budgets.selectedBudget;
  if (!selected) return null;
  const fresh = state.budgets.allBudgets.find((budget) => budget.id === selected.id);
  return fresh?.onboarding ?? selected.onboarding ?? { step: 'done' };
};

/** The step to show: an unstarted wizard opens on welcome. */
export function currentStep(onboarding: BudgetOnboarding | null): OnboardingStep {
  return onboarding?.step || 'welcome';
}

export function nextStep(step: OnboardingStep): OnboardingStep {
  const index = ONBOARDING_STEPS.indexOf(step);
  return ONBOARDING_STEPS[Math.min(index + 1, ONBOARDING_STEPS.length - 1)];
}

export function previousStep(step: OnboardingStep): OnboardingStep {
  const index = ONBOARDING_STEPS.indexOf(step);
  return ONBOARDING_STEPS[Math.max(index - 1, 0)];
}

export function useOnboarding() {
  const dispatch = useAppDispatch();
  const onboarding = useAppSelector(selectOnboarding);

  const update = useCallback(
    async (payload: UpdateOnboardingPayload) => {
      const saved = await updateOnboarding(payload);
      dispatch(budgetOnboardingUpdated(saved));
      return saved;
    },
    [dispatch],
  );

  return { onboarding, step: currentStep(onboarding), update };
}
