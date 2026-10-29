import { useCallback, useState } from 'react';
import { ArrowLeft, ArrowRight, LockSimple } from '@phosphor-icons/react';
import { useAppSelector } from '@/app/hooks';
import { selectIsDemoUser } from '@/features/auth/store';
import { AIProvidersPanel } from '../AIProvidersPanel';
import type { AIProvider } from '../../types';
import type { StepProps } from '../OnboardingWizard';
import styles from './steps.module.css';

export function AIStep({ onNext, onBack, busy }: StepProps) {
  const isDemo = useAppSelector(selectIsDemoUser);
  const [providers, setProviders] = useState<AIProvider[]>([]);
  const handleChange = useCallback((list: AIProvider[]) => setProviders(list), []);
  const hasProvider = providers.some((provider) => provider.enabled);

  return (
    <>
      <p className={styles.eyebrow}>Step 3 · Optional</p>
      <h1 className={styles.title}>Connect your AI provider</h1>
      <p className={styles.lead}>
        Pennywise uses an AI model to read your bank emails, import statements and answer questions about your money.
        Bring your own key: you pay your provider directly, usually a few rupees a month for email parsing.
      </p>

      <div className={styles.callout}>
        <LockSimple size={18} weight="bold" aria-hidden="true" />
        <span>
          Keys are encrypted before they're stored and are never shown again or sent back to your browser. They're only
          used for your own budgets.
        </span>
      </div>

      <div className={styles.section}>
        <AIProvidersPanel onChange={handleChange} readOnly={isDemo} />
      </div>

      {!hasProvider && (
        <div className={`${styles.callout} ${styles.calloutWarning}`}>
          Without a provider, bank emails can't be turned into transactions and the assistant stays off. You can add one
          later in Settings → AI.
        </div>
      )}

      <div className={styles.actions}>
        <button type="button" className={styles.ghost} onClick={onBack} disabled={busy}>
          <ArrowLeft size={16} /> Back
        </button>
        <span className={styles.spacer} />
        {!hasProvider && (
          <button type="button" className={styles.ghost} onClick={() => onNext({ skipped: true })} disabled={busy}>
            Skip
          </button>
        )}
        <button type="button" className={styles.primary} onClick={() => onNext()} disabled={busy || !hasProvider}>
          Continue <ArrowRight size={16} />
        </button>
      </div>
    </>
  );
}
