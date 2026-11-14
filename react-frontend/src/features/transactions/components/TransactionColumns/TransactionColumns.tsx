import type { TransactionColumns } from '@/types/common.types';
import styles from '../Transaction/Transaction.module.css';
import { getLocaleDate } from '@/utils/date.utils';
import { formatBudgetAmount } from '@/features/category/components/budgetFormat';
import type { Transaction } from '../../types/transaction.types';
import { formatSourceTime } from '../../utils/sourceTime';
import { AccountLabel } from './AccountLabel';

function renderAmount(txn: Transaction) {
  if ((txn.inflow ?? 0) !== 0) {
    return <span className={`${styles.amountCell} ${styles.amountInflow}`}>+{formatBudgetAmount(txn.inflow ?? 0)}</span>;
  }
  if ((txn.outflow ?? 0) !== 0) {
    return <span className={`${styles.amountCell} ${styles.amountOutflow}`}>-{formatBudgetAmount(txn.outflow ?? 0)}</span>;
  }
  return '–';
}

function renderDate(txn: Transaction) {
  const time = formatSourceTime(txn.occurredAt);
  const date = getLocaleDate(txn.date, { month: 'short', day: 'numeric', year: 'numeric' }, ['en-GB']);
  if (!time) return date;
  return (
    <>
      {date}
      <span className={styles.dateTime}>{time}</span>
    </>
  );
}

function renderPayee(txn: Transaction) {
  return txn.payeeName ? <span className={styles.payeeName}>{txn.payeeName}</span> : <span className={styles.note}>–</span>;
}

export const allAccountTxnCols: TransactionColumns[] = [
  {
    key: 'accountName',
    label: 'Account',
    layout: { flex: '0 0 4.5rem', textAlign: 'left', minWidth: 0, overflow: 'hidden' },
    render: (txn) => <AccountLabel txn={txn} />,
  },
  {
    key: 'date',
    label: 'Date',
    layout: { flex: '0 0 12rem', textAlign: 'left' },
    className: ['dateCell'],
    render: renderDate,
  },
  {
    key: 'payeeName',
    label: 'Payee',
    layout: { flex: '1 1 24%', textAlign: 'left', minWidth: 0, overflow: 'hidden' },
    render: renderPayee,
  },
  {
    key: 'categoryName',
    label: 'Category',
    layout: { flex: '1 1 18%', textAlign: 'left', minWidth: 0, overflow: 'hidden' },
    render: (txn) =>
      txn.categoryName ? (
        <span className={styles.categoryPill}>{txn.categoryName}</span>
      ) : (
        <span className={styles.note}>–</span>
      ),
  },
  {
    key: 'note',
    label: 'Note',
    layout: { flex: '1 1 11%', textAlign: 'left', minWidth: 0, overflow: 'hidden' },
    className: ['note'],
  },
  {
    key: 'outflow',
    label: 'Amount',
    layout: { flex: '0 0 8rem', textAlign: 'right' },
    render: renderAmount,
  },
];

export const specificAccountTxnCols: TransactionColumns[] = [
  {
    key: 'date',
    label: 'Date',
    layout: { flex: '0 0 12rem', textAlign: 'left' },
    className: ['dateCell'],
    render: renderDate,
  },
  {
    key: 'payeeName',
    label: 'Payee',
    layout: { flex: '1 1 28%', textAlign: 'left', minWidth: 0, overflow: 'hidden' },
    render: renderPayee,
  },
  {
    key: 'categoryName',
    label: 'Category',
    layout: { flex: '1 1 22%', textAlign: 'left', minWidth: 0, overflow: 'hidden' },
    render: (txn) =>
      txn.categoryName ? (
        <span className={styles.categoryPill}>{txn.categoryName}</span>
      ) : (
        <span className={styles.note}>–</span>
      ),
  },
  {
    key: 'note',
    label: 'Note',
    layout: { flex: '1 1 13%', textAlign: 'left', minWidth: 0, overflow: 'hidden' },
    className: ['note'],
  },
  {
    key: 'outflow',
    label: 'Amount',
    layout: { flex: '0 0 8rem', textAlign: 'right' },
    render: renderAmount,
  },
  {
    key: 'balance',
    label: 'Balance',
    layout: { flex: '0 0 8rem', textAlign: 'right' },
    className: ['balanceCell'],
    render: (txn) => formatBudgetAmount(txn.balance ?? 0),
  },
];
