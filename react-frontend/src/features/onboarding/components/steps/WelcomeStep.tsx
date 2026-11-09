import { ArrowRight, Envelope, Wallet } from '@phosphor-icons/react';
import type { StepProps } from '../OnboardingWizard';
import styles from './steps.module.css';

export function WelcomeStep({ onNext, busy }: StepProps) {
  return (
    <>
      <p className={styles.eyebrow}>Welcome</p>
      <h1 className={styles.title}>Let's set up your budget</h1>
      <p className={styles.lead}>
        A few minutes now and Pennywise keeps itself up to date. You can skip any step except adding an account.
      </p>

      <ul className={styles.points}>
        <li className={styles.point}>
          <span className={styles.pointIcon}>
            <Wallet size={20} weight="duotone" />
          </span>
          <div>
            <strong>Give every rupee a job</strong>
            <p>
              You budget the money you have today, not money you expect. Each rupee goes to a category until nothing is
              left to assign.
            </p>
          </div>
        </li>
        <li className={styles.point}>
          <span className={styles.pointIcon}>
            <Envelope size={20} weight="duotone" />
          </span>
          <div>
            <strong>Your bank emails become transactions</strong>
            <p>
              Pennywise reads the alert emails your bank sends, works out the payee and category, and adds the
              transaction for you to review.
            </p>
          </div>
        </li>
      </ul>

      <div className={styles.actions}>
        <button type="button" className={styles.primary} onClick={() => onNext()} disabled={busy}>
          Get started <ArrowRight size={16} />
        </button>
      </div>
    </>
  );
}
