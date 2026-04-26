import type { TransactionColumns } from '@/types/common.types';
import styles from '../Transaction/Transaction.module.css';
import { getCurrencyLocaleString, getLocaleDate } from '@/utils/date.utils';
import { LucideMinus, LucidePlus } from 'lucide-react';

export const allAccountTxnCols: TransactionColumns[] = [
  {
    key: 'accountName',
    label: 'Account',
    layout: { flex: '1 1 12%', textAlign: 'left', minWidth: 0, overflow: 'hidden' },
  },
  {
    key: 'date',
    label: 'Date',
    layout: { flex: '0 0 9rem', textAlign: 'left' },
    render: (txn) =>
      getLocaleDate(
        txn.date,
        { month: 'short', day: 'numeric', year: 'numeric' },
        ['en-GB'],
      ),
  },
  {
    key: 'payeeName',
    label: 'Payee',
    layout: { flex: '1 1 20%', textAlign: 'left', minWidth: 0, overflow: 'hidden' },
    render: (txn) =>
      txn.payeeName ? (
        <span className={styles.payeeName}>
          {txn.payeeName}
        </span>
      ) : (
        <span className={styles.note}>–</span>
      ),
  },
  {
    key: 'categoryName',
    label: 'Category',
    layout: { flex: '1 1 18%', textAlign: 'left', minWidth: 0, overflow: 'hidden' },
    render: (txn) =>
      txn.categoryName ? (
        <span className={styles.categoryPill}>
          {txn.categoryName}
        </span>
      ) : (
        <span className={styles.note}>–</span>
      ),
  },
  {
    key: 'note',
    label: 'Note',
    layout: { flex: '1 1 15%', textAlign: 'left', minWidth: 0, overflow: 'hidden' },
    className: ['note'],
  },
  {
    key: 'outflow',
    label: 'Amount',
    layout: { flex: '0 0 8rem', textAlign: 'right' },
    render: (txn) => {
      if ((txn.inflow ?? 0) !== 0) {
        return (
          <span className={`${styles.amountCell} ${styles.amountInflow}`}>
            <LucidePlus color='var(--color-text-secondary)' size={14} />
            {getCurrencyLocaleString(txn.inflow ?? 0)}
          </span>
        );
      }
      if ((txn.outflow ?? 0) !== 0) {
        return (
          <span className={`${styles.amountCell} ${styles.amountOutflow}`}>
            <LucideMinus color='var(--color-text-secondary)' size={14} />
            <span>{getCurrencyLocaleString(txn.outflow ?? 0)}</span>
          </span>
        );
      }
      return '–';
    },
  },
];

export const specificAccountTxnCols: TransactionColumns[] = [
  {
    key: 'date',
    label: 'Date',
    layout: { flex: '0 0 9rem', textAlign: 'left' },
    render: (txn) =>
      getLocaleDate(
        txn.date,
        { month: 'short', day: 'numeric', year: 'numeric' },
        ['en-GB'],
      ),
  },
  {
    key: 'payeeName',
    label: 'Payee',
    layout: { flex: '1 1 25%', textAlign: 'left', minWidth: 0, overflow: 'hidden' },
  },
  {
    key: 'categoryName',
    label: 'Category',
    layout: { flex: '1 1 22%', textAlign: 'left', minWidth: 0, overflow: 'hidden' },
    render: (txn) =>
      txn.categoryName ? (
        <span className={styles.categoryPill}>
          {txn.categoryName}
        </span>
      ) : (
        <span className={styles.note}>–</span>
      ),
  },
  {
    key: 'note',
    label: 'Note',
    layout: { flex: '1 1 18%', textAlign: 'left', minWidth: 0, overflow: 'hidden' },
    className: ['note'],
  },
  {
    key: 'outflow',
    label: 'Amount',
    layout: { flex: '0 0 8rem', textAlign: 'right' },
    render: (txn) => {
      if ((txn.inflow ?? 0) !== 0) {
        return (
          <span className={`${styles.amountCell} ${styles.amountInflow}`}>
            +{getCurrencyLocaleString(txn.inflow ?? 0)}
          </span>
        );
      }
      if ((txn.outflow ?? 0) !== 0) {
        return (
          <span className={`${styles.amountCell} ${styles.amountOutflow}`}>
            -{getCurrencyLocaleString(txn.outflow ?? 0)}
          </span>
        );
      }
      return '–';
    },
  },
  {
    key: 'balance',
    label: 'Balance',
    layout: { flex: '0 0 8rem', textAlign: 'right' },
    render: (txn) => getCurrencyLocaleString(txn.balance ?? 0),
  },
];
