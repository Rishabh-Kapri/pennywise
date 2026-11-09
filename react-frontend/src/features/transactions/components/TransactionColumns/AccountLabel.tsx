import { useAppSelector } from '@/app/hooks';
import { BankLogo } from '@/features/accounts/components/BankLogo/BankLogo';
import type { Transaction } from '../../types/transaction.types';
import styles from '../Transaction/Transaction.module.css';

/** Account cell: just the bank's logo (or card art); the name is in the tooltip and the detail panel. */
export function AccountLabel({ txn }: { txn: Transaction }) {
  const account = useAppSelector((state) => state.accounts.allAccounts.find((acc) => acc.id === txn.accountId));
  if (!txn.accountName) return <span className={styles.note}>–</span>;
  return (
    <span className={styles.accountLabel} title={txn.accountName} role="img" aria-label={txn.accountName}>
      <BankLogo name={txn.accountName} bank={account?.bank} type={account?.type} size="sm" />
    </span>
  );
}
