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
  logicMode: FilterLogicMode;
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
  logicMode: 'AND',
};

export function hasActiveFilters(filters: TransactionFilters): boolean {
  return (
    filters.note.trim() !== '' ||
    filters.dateFrom !== '' ||
    filters.dateTo !== '' ||
    filters.accountIds.length > 0 ||
    filters.payeeIds.length > 0 ||
    filters.categoryIds.length > 0
  );
}

export function countActiveFilters(filters: TransactionFilters): number {
  let count = 0;
  if (filters.note.trim()) count++;
  if (filters.dateFrom || filters.dateTo) count++;
  if (filters.accountIds.length > 0) count++;
  if (filters.payeeIds.length > 0) count++;
  if (filters.categoryIds.length > 0) count++;
  return count;
}
