import { useEffect, useState, type ComponentType } from 'react';
import { useNavigate } from 'react-router-dom';
import { Check } from '@phosphor-icons/react';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { fetchAllAccounts } from '@/features/accounts/store/accountSlice';
import { fetchInflowAmount } from '@/features/category/store/categorySlice';
import { errorMessage, toast } from '@/utils';
import { nextStep, previousStep, useOnboarding } from '../hooks/useOnboarding';
import { ONBOARDING_STEPS, type OnboardingStep } from '../types';
import { WelcomeStep } from './steps/WelcomeStep';
import { AccountsStep } from './steps/AccountsStep';
import { BillsStep } from './steps/BillsStep';
import { AIStep } from './steps/AIStep';
import { EmailStep } from './steps/EmailStep';
import { AssignStep } from './steps/AssignStep';
import styles from './OnboardingWizard.module.css';

export interface StepProps {
  /** Moves on; `skipped` records the step for the getting-started checklist. */
  onNext: (options?: { skipped?: boolean }) => Promise<void>;
  onBack: () => void;
  /** Jumps to another step (the email step links back to AI). */
  goTo: (step: OnboardingStep) => void;
  busy: boolean;
}

const STEP_LABELS: Record<Exclude<OnboardingStep, 'done'>, string> = {
  welcome: 'Welcome',
  accounts: 'Accounts',
  bills: 'Bills',
  ai: 'AI provider',
  email: 'Bank emails',
  assign: 'Assign money',
};

const STEP_COMPONENTS: Record<Exclude<OnboardingStep, 'done'>, ComponentType<StepProps>> = {
  welcome: WelcomeStep,
  accounts: AccountsStep,
  bills: BillsStep,
  ai: AIStep,
  email: EmailStep,
  assign: AssignStep,
};

const VISIBLE_STEPS = ONBOARDING_STEPS.filter((step) => step !== 'done') as Exclude<OnboardingStep, 'done'>[];

/**
 * First-run setup for a budget. Every step writes real data, and the current
 * step lives on the server, so a reload or another device resumes here.
 */
export default function OnboardingWizard() {
  const dispatch = useAppDispatch();
  const navigate = useNavigate();
  const { onboarding, step: savedStep, update } = useOnboarding();
  const hasAccounts = useAppSelector((state) => state.accounts.allAccounts.length > 0);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    dispatch(fetchAllAccounts());
    dispatch(fetchInflowAmount());
  }, [dispatch]);

  const step = savedStep === 'done' ? 'assign' : savedStep;
  const skipped = onboarding?.skippedSteps ?? [];
  const stepIndex = VISIBLE_STEPS.indexOf(step);

  const goTo = async (target: OnboardingStep, extraSkipped: OnboardingStep[] = []) => {
    setBusy(true);
    try {
      await update({ step: target, skippedSteps: extraSkipped.length ? extraSkipped : undefined });
      if (target === 'done') navigate('/dashboard', { replace: true });
    } catch (err) {
      toast.error('Could not save your progress', { description: errorMessage(err, 'Try again.') });
    } finally {
      setBusy(false);
    }
  };

  const onNext = (options?: { skipped?: boolean }) => goTo(nextStep(step), options?.skipped ? [step] : []);

  // Leaving early marks every step not yet reached as skipped, so the
  // dashboard checklist picks them up.
  const finishLater = () =>
    goTo(
      'done',
      VISIBLE_STEPS.slice(stepIndex).filter((s) => s !== 'welcome'),
    );

  const StepComponent = STEP_COMPONENTS[step];

  return (
    <main className={styles.page}>
      <aside className={styles.rail}>
        <div className={styles.brand}>Pennywise</div>
        <ol className={styles.steps}>
          {VISIBLE_STEPS.map((s, index) => {
            const state = index < stepIndex ? 'done' : index === stepIndex ? 'current' : 'todo';
            return (
              <li
                key={s}
                className={`${styles.stepItem} ${styles[state]}`}
                aria-current={state === 'current' ? 'step' : undefined}
              >
                <span className={styles.stepDot}>
                  {state === 'done' && !skipped.includes(s) ? <Check size={12} weight="bold" /> : index + 1}
                </span>
                <span>{STEP_LABELS[s]}</span>
                {state === 'done' && skipped.includes(s) && <span className={styles.skippedTag}>skipped</span>}
              </li>
            );
          })}
        </ol>
        {hasAccounts && (
          <button type="button" className={styles.finishLater} onClick={finishLater} disabled={busy}>
            Finish setup later
          </button>
        )}
      </aside>

      <section className={styles.card} key={step}>
        <StepComponent
          onNext={onNext}
          onBack={() => goTo(previousStep(step))}
          goTo={(target) => goTo(target)}
          busy={busy}
        />
      </section>
    </main>
  );
}
