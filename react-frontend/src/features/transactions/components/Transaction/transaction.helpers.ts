import { Parser } from 'expr-eval';
import { getTodaysDate } from '@/utils/date.utils';
import type { TransactionColumns } from '@/types/common.types';
import { allAccountTxnCols, specificAccountTxnCols } from '../TransactionColumns';
import type { MobileFilter } from '../TransactionHeader';
import {
  TransactionStatus,
  type ListItem,
  type MonthGroupStats,
  type Transaction,
  type TransactionDTO,
} from '../../types/transaction.types';

const parser = new Parser();

export const HEADER_ROW_HEIGHT = 56;
export const TXN_ROW_HEIGHT = 80;
export const MOBILE_ROW_HEIGHT = 188;

function getTransactionDate(txn: Transaction) {
  return new Date(`${txn.date}T00:00:00`);
}

function isThisWeek(txn: Transaction) {
  const today = new Date();
  const startOfWeek = new Date(today);
  startOfWeek.setHours(0, 0, 0, 0);
  startOfWeek.setDate(today.getDate() - today.getDay());

  const endOfWeek = new Date(startOfWeek);
  endOfWeek.setDate(startOfWeek.getDate() + 7);

  const txnDate = getTransactionDate(txn);
  return txnDate >= startOfWeek && txnDate < endOfWeek;
}

export function getTransactionColumns(accountId: string): TransactionColumns[] {
  return accountId ? specificAccountTxnCols : allAccountTxnCols;
}

export function getTransactionRowHeight(isMobile: boolean, item?: ListItem) {
  if (isMobile) return MOBILE_ROW_HEIGHT;
  return item?.type === 'header' ? HEADER_ROW_HEIGHT : TXN_ROW_HEIGHT;
}

export function groupTransactions(txns: Transaction[]): ListItem[] {
  const groups = new Map<string, { label: string; txns: Transaction[] }>();

  for (const txn of txns) {
    const [year, month] = txn.date.split('-');
    const key = `${year}-${month}`;
    if (!groups.has(key)) {
      const date = new Date(Number(year), Number(month) - 1, 1);
      const label = date.toLocaleDateString('en-US', { month: 'long', year: 'numeric' });
      groups.set(key, { label, txns: [] });
    }
    groups.get(key)!.txns.push(txn);
  }

  const items: ListItem[] = [];
  let runningIndex = 0;

  for (const [key, group] of groups) {
    const stats = group.txns.reduce<MonthGroupStats>(
      (acc, transaction) => ({
        count: acc.count + 1,
        totalInflow: acc.totalInflow + (transaction.inflow ?? 0),
        totalOutflow: acc.totalOutflow + (transaction.outflow ?? 0),
      }),
      { count: 0, totalInflow: 0, totalOutflow: 0 },
    );

    items.push({ type: 'header', key, label: group.label, stats });
    for (const txn of group.txns) {
      items.push({ type: 'row', txn, originalIndex: runningIndex++ });
    }
  }

  return items;
}

export function filterTransactions(transactions: Transaction[], searchTerm: string, mobileFilter: MobileFilter) {
  const search = searchTerm.trim().toLowerCase();

  return transactions.filter((txn) => {
    const matchesSearch =
      !search ||
      [txn.accountName, txn.payeeName, txn.categoryName, txn.note, String(txn.outflow ?? ''), String(txn.inflow ?? '')]
        .filter(Boolean)
        .some((value) => value?.toLowerCase().includes(search));

    if (!matchesSearch) return false;
    if (mobileFilter === 'incoming') return (txn.inflow ?? 0) > 0;
    if (mobileFilter === 'outgoing') return (txn.outflow ?? 0) > 0;
    if (mobileFilter === 'week') return isThisWeek(txn);
    return true;
  });
}

export function buildTransactionPayload(txn: Transaction): TransactionDTO {
  return {
    id: txn.id || undefined,
    budgetId: txn.budgetId,
    accountId: txn.accountId,
    payeeId: txn.payeeId,
    categoryId: txn.categoryId === '' ? null : txn.categoryId,
    date: txn.date,
    amount: txn.outflow ? -txn.outflow : (txn.inflow ?? 0),
    note: txn.note ?? '',
    status: txn.status ?? TransactionStatus.MANUAL,
    tagIds: txn.tagIds ?? [],
  };
}

export function createEmptyTransaction({
  budgetId,
  accountId,
  balance,
}: {
  budgetId: string;
  accountId: string;
  balance: number;
}): Transaction {
  return {
    id: '',
    budgetId,
    date: getTodaysDate(),
    outflow: null,
    inflow: null,
    balance,
    note: '',
    accountName: '',
    accountId,
    payeeName: '',
    payeeId: '',
    categoryName: '',
    categoryId: '',
    transferAccountId: null,
    transferTransactionId: null,
    tagIds: [],
  };
}

export function applyParsedInputValue(txn: Transaction, key: keyof Transaction, value: string | number) {
  const result = (parser.parse(value as string) as { evaluate(): number }).evaluate();

  if (key !== 'outflow') {
    return { ...txn, [key]: result };
  }

  if (result >= 0) {
    return { ...txn, inflow: result || null, outflow: null };
  }

  return { ...txn, outflow: Math.abs(result), inflow: null };
}

export function getStickyHeaderForStartIndex(listItems: ListItem[], startIndex: number) {
  if (listItems[startIndex]?.type === 'header') {
    return null;
  }

  for (let i = startIndex - 1; i >= 0; i--) {
    const item = listItems[i];
    if (item?.type === 'header') {
      return { label: item.label, stats: item.stats };
    }
  }

  return null;
}
