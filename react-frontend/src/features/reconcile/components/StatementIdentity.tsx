import type { ReactNode } from 'react';
import { BankLogo } from '@/features/accounts/components/BankLogo/BankLogo';
import { money, type StatementIdentity as Identity } from '../statementIdentity';
import styles from './StatementReconcile.module.css';

/**
 * Whose statement this is, for when, and what's due: the card or account
 * with its logo and last digits, the period, and the amount due.
 * `size` "lg" is the detail page's title, "md" a row in the list.
 */
export function StatementIdentity({
  identity,
  more = 0,
  size = 'md',
  children,
}: {
  identity: Identity;
  /** other accounts on the same statement */
  more?: number;
  size?: 'md' | 'lg';
  /** a line under the period, e.g. the file name */
  children?: ReactNode;
}) {
  const Title = size === 'lg' ? 'h2' : 'strong';
  return (
    <div className={`${styles.identity} ${size === 'lg' ? styles.identityLarge : ''}`}>
      <BankLogo
        name={identity.account?.name ?? identity.name}
        bank={identity.account?.bank}
        type={identity.logoType}
        size={size === 'lg' ? 'lg' : 'md'}
        className={styles.identityLogo}
      />
      <div className={styles.identityText}>
        <div className={styles.identityName}>
          <Title title={identity.name}>{identity.name}</Title>
          {identity.ending && <span className={styles.identityEnding}>•••• {identity.ending}</span>}
          {more > 0 && (
            <span className={styles.badgeMuted}>
              +{more} more account{more === 1 ? '' : 's'}
            </span>
          )}
        </div>
        <div className={styles.identityFacts}>
          {identity.period ? <span>{identity.period}</span> : <span className={styles.muted}>Period not printed</span>}
          {identity.totalDue !== undefined && (
            <span className={styles.identityDue}>
              Due <strong>{money(identity.totalDue)}</strong>
              {identity.dueBy && <> by {identity.dueBy}</>}
            </span>
          )}
        </div>
        {children && <div className={styles.identityMeta}>{children}</div>}
      </div>
    </div>
  );
}
