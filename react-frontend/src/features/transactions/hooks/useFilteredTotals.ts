import { useEffect, useState } from 'react';
import { apiClient } from '@/utils';
import type { AmountTotals, PaginationResponse } from '@/utils/common.constants';
import { type FetchTransactionArgs, transactionListQuery } from '../store/transactionSlice';
import type { Transaction } from '../types/transaction.types';

export interface FilteredTotals extends AmountTotals {
  count: number;
}

interface Loaded {
  key: string;
  totals: FilteredTotals;
}

/**
 * Money in and out across every transaction the filters match, not just the
 * loaded page. Asked for on its own (one row, the server sums the rest),
 * because app start and the websocket reload the shared list without filters.
 *
 * Returns null while off, loading, or after a failure. `refreshKey` reloads
 * it when the list changes (an edit, a new transaction).
 */
export function useFilteredTotals(
  args: FetchTransactionArgs,
  enabled: boolean,
  refreshKey: unknown,
): FilteredTotals | null {
  const key = transactionListQuery({ ...args, cursor: undefined, limit: 1 });
  const [loaded, setLoaded] = useState<Loaded | null>(null);

  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    const timer = setTimeout(async () => {
      try {
        const res = await apiClient.get<PaginationResponse<Transaction[]>>(`transactions/normalized?${key}`);
        if (cancelled) return;
        // An API without totals: show nothing rather than a wrong ₹0.
        setLoaded(res.totals ? { key, totals: { ...res.totals, count: res.total } } : null);
      } catch {
        if (!cancelled) setLoaded(null);
      }
    }, 300);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [enabled, key, refreshKey]);

  return enabled && loaded?.key === key ? loaded.totals : null;
}
