export type FilterLogicMode = 'AND' | 'OR';

export interface TransactionFilters {
  note: string;
  dateFrom: string;
  dateTo: string;
  accountIds: string[];
  accountNames: string[];
  payeeIds: string[];
  payeeNames: string[];
  categoryIds: string[];
  categoryNames: string[];
  tagIds: string[];
  tagNames: string[];
  /** Only transactions waiting for the user's approval. */
  pendingOnly: boolean;
  logicMode: FilterLogicMode;
}

/** Router state that opens the transactions list pre-filtered. */
export interface TransactionsLocationState {
  payeeFilter?: { id: string; name: string };
  /** Opens the add form with these values filled in. */
  prefill?: TransactionPrefill;
  /** The job to dismiss once the prefilled transaction is saved. */
  dismissJobId?: string;
}

export interface TransactionPrefill {
  date?: string;
  outflow?: number;
  inflow?: number;
  note?: string;
  accountId?: string;
  accountName?: string;
  payeeId?: string;
  payeeName?: string;
}

export const EMPTY_FILTERS: TransactionFilters = {
  note: '',
  dateFrom: '',
  dateTo: '',
  accountIds: [],
  accountNames: [],
  payeeIds: [],
  payeeNames: [],
  categoryIds: [],
  categoryNames: [],
  tagIds: [],
  tagNames: [],
  pendingOnly: false,
  logicMode: 'AND',
};

export function hasActiveFilters(filters: TransactionFilters): boolean {
  return (
    filters.note.trim() !== '' ||
    filters.dateFrom !== '' ||
    filters.dateTo !== '' ||
    filters.accountIds.length > 0 ||
    filters.payeeIds.length > 0 ||
    filters.categoryIds.length > 0 ||
    filters.tagIds.length > 0 ||
    filters.pendingOnly
  );
}

export function countActiveFilters(filters: TransactionFilters): number {
  let count = 0;
  if (filters.note.trim()) count++;
  if (filters.dateFrom || filters.dateTo) count++;
  if (filters.accountIds.length > 0) count++;
  if (filters.payeeIds.length > 0) count++;
  if (filters.categoryIds.length > 0) count++;
  if (filters.tagIds.length > 0) count++;
  if (filters.pendingOnly) count++;
  return count;
}
