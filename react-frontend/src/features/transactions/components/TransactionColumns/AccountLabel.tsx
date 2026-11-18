import { AccountLogo } from '@/features/accounts/components/BankLogo/AccountLogo';
import type { Transaction } from '../../types/transaction.types';
import styles from '../Transaction/Transaction.module.css';

/** Account cell: just the bank's logo (or card art); the name is in the tooltip and the detail panel. */
export function AccountLabel({ txn }: { txn: Transaction }) {
  if (!txn.accountName) return <span className={styles.note}>–</span>;
  return (
    <span className={styles.accountLabel} title={txn.accountName} role="img" aria-label={txn.accountName}>
      <AccountLogo accountId={txn.accountId} name={txn.accountName} size="sm" />
    </span>
  );
}
