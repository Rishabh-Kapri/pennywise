import type { TransactionColumns } from '@/types/common.types';
import styles from './Transaction.module.css';
import { getCurrencyLocaleString, getLocaleDate } from '@/utils/date.utils';

export const allAccountTxnCols: TransactionColumns[] = [
  {
    key: 'accountName',
    label: 'Account',
    layout: {
      gridColumn: '1 / 2',
      textAlign: 'left',
    },
  },
  {
    key: 'date',
    label: 'Date',
    layout: {
      gridColumn: '2 / 3',
      textAlign: 'left',
    },
    render: (txn) => getLocaleDate(txn.date, ['en-GB']),
  },
  {
    key: 'payeeName',
    label: 'Payee',
    layout: {
      gridColumn: '3 / 5',
      textAlign: 'left',
    },
  },
  {
    key: 'categoryName',
    label: 'Category',
    layout: {
      gridColumn: '5 / 7',
      textAlign: 'left',
    },
    render: (txn) =>
      txn.categoryName ?? (
        <span className={styles.note}>Category not required</span>
      ),
  },
  {
    key: 'note',
    label: 'Note',
    layout: {
      gridColumn: '7 / 9',
      textAlign: 'left',
    },
    className: ['note'],
  },
  {
    key: 'outflow',
    label: 'Outflow',
    layout: {
      gridColumn: '9 / 10',
      textAlign: 'center',
    },
    render: (txn) =>
      (txn.outflow ?? 0) !== 0 ? (
        <span className={`${styles.amount} ${styles.outflow}`}>
          {getCurrencyLocaleString(txn.outflow ?? 0)}
        </span>
      ) : (
        ''
      ),
  },
  {
    key: 'inflow',
    label: 'Inflow',
    layout: {
      gridColumn: '10 / 11',
      textAlign: 'right',
    },
    render: (txn) =>
      (txn.inflow ?? 0) !== 0 ? (
        <span className={`${styles.amount} ${styles.inflow}`}>
          {getCurrencyLocaleString(txn.inflow ?? 0)}
        </span>
      ) : (
        ''
      ),
  },
];

export const specificAccountTxnCols: TransactionColumns[] = [
  {
    key: 'date',
    label: 'Date',
    layout: {
      gridColumn: '1 / 2',
      textAlign: 'left',
    },
    render: (txn) => getLocaleDate(txn.date, ['en-GB']),
  },
  {
    key: 'payeeName',
    label: 'Payee',
    layout: {
      gridColumn: '2 / 4',
      textAlign: 'left',
    },
  },
  {
    key: 'categoryName',
    label: 'Category',
    layout: {
      gridColumn: '4 / 6',
      textAlign: 'left',
    },
    render: (txn) =>
      txn.categoryName ?? (
        <span className={styles.note}>Category not required</span>
      ),
  },
  {
    key: 'note',
    label: 'Note',
    layout: {
      gridColumn: '6 / 8',
      textAlign: 'left',
    },
    className: ['note'],
  },
  {
    key: 'outflow',
    label: 'Outflow',
    layout: {
      gridColumn: '8 / 9',
      textAlign: 'center',
    },
    render: (txn) =>
      (txn.outflow ?? 0) !== 0 ? (
        <span className={`${styles.amount} ${styles.outflow}`}>
          {getCurrencyLocaleString(txn.outflow ?? 0)}
        </span>
      ) : (
        ''
      ),
  },
  {
    key: 'inflow',
    label: 'Inflow',
    layout: {
      gridColumn: '9 / 10',
      textAlign: 'center',
    },
    render: (txn) =>
      (txn.inflow ?? 0) !== 0 ? (
        <span className={`${styles.amount} ${styles.inflow}`}>
          {getCurrencyLocaleString(txn.inflow ?? 0)}
        </span>
      ) : (
        ''
      ),
  },
  {
    key: 'balance',
    label: 'Balance',
    layout: {
      gridColumn: '10 / 11',
      textAlign: 'right',
    },
    render: (txn) => getCurrencyLocaleString(txn.balance ?? 0),
  },
];
