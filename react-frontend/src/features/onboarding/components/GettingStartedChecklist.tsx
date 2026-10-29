import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { CheckCircle, Circle, X } from '@phosphor-icons/react';
import { useAppSelector } from '@/app/hooks';
import type { Category } from '@/features/category/types/category.types';
import { apiClient, errorMessage, toast } from '@/utils';
import { fetchAIProviders } from '../api';
import { useOnboarding } from '../hooks/useOnboarding';
import styles from './GettingStartedChecklist.module.css';

interface Item {
  label: string;
  hint: string;
  to: string;
  done: boolean;
}

const EXPLORE = [
  { label: 'Recurring transactions', to: '/settings?section=recurring' },
  { label: 'Reconcile a statement', to: '/settings?section=reconcile' },
  { label: 'Payee rules', to: '/payees' },
];

/**
 * Dashboard card for whatever setup is still open after the wizard: skipped
 * steps and anything undone since. Each item is checked against real data,
 * not the wizard's record, so doing it elsewhere ticks it too.
 */
export function GettingStartedChecklist() {
  const { onboarding, update } = useOnboarding();
  const hasAccounts = useAppSelector((state) => state.accounts.allAccounts.length > 0);
  const readyToAssign = useAppSelector((state) => state.categories.inflowAmount);
  const [facts, setFacts] = useState<{ bills: boolean; ai: boolean; emails: boolean } | null>(null);

  const visible = Boolean(onboarding?.completedAt) && !onboarding?.checklistDismissed;

  useEffect(() => {
    if (!visible) return;
    Promise.all([
      apiClient.get<Category[]>('categories').catch(() => []),
      fetchAIProviders().catch(() => []),
      apiClient.get<unknown[]>('pipeline/runs?limit=1').catch(() => []),
    ]).then(([categories, providers, runs]) =>
      setFacts({
        bills: (categories ?? []).some((category) => category.goal),
        ai: (providers ?? []).some((provider) => provider.enabled),
        emails: (runs ?? []).length > 0,
      }),
    );
  }, [visible]);

  if (!visible || !facts) return null;

  const items: Item[] = [
    { label: 'Add your accounts', hint: 'With last digits, so emails match', to: '/accounts', done: hasAccounts },
    { label: 'Set targets for your bills', hint: 'Know what to set aside each month', to: '/budget', done: facts.bills },
    { label: 'Connect an AI provider', hint: 'Needed for email import and the assistant', to: '/settings?section=ai', done: facts.ai },
    { label: 'Import transactions from email', hint: 'Bank alerts become transactions', to: '/settings?section=activity', done: facts.emails },
    { label: 'Assign all your money', hint: 'Get Ready to Assign to zero', to: '/budget', done: readyToAssign <= 0 },
  ];
  const doneCount = items.filter((item) => item.done).length;
  if (doneCount === items.length) return null;

  const dismiss = () =>
    update({ checklistDismissed: true }).catch((err) =>
      toast.error('Could not hide the checklist', { description: errorMessage(err, 'Try again.') }),
    );

  return (
    <section className={styles.card} aria-labelledby="getting-started-title">
      <header className={styles.header}>
        <div>
          <h2 id="getting-started-title">Finish setting up</h2>
          <p>
            {doneCount} of {items.length} done
          </p>
        </div>
        <button type="button" className={styles.dismiss} onClick={dismiss} aria-label="Hide the checklist">
          <X size={16} />
        </button>
      </header>
      <div className={styles.progress}>
        <div className={styles.progressBar} style={{ width: `${(doneCount / items.length) * 100}%` }} />
      </div>
      <ul className={styles.items}>
        {items.map((item) => (
          <li key={item.label}>
            <Link to={item.to} className={`${styles.item} ${item.done ? styles.done : ''}`}>
              {item.done ? (
                <CheckCircle size={20} weight="fill" className={styles.check} />
              ) : (
                <Circle size={20} className={styles.todo} />
              )}
              <span>
                {item.label}
                <small>{item.hint}</small>
              </span>
            </Link>
          </li>
        ))}
      </ul>
      <footer className={styles.explore}>
        <span>Also worth a look:</span>
        {EXPLORE.map((link) => (
          <Link key={link.to} to={link.to}>
            {link.label}
          </Link>
        ))}
      </footer>
    </section>
  );
}
