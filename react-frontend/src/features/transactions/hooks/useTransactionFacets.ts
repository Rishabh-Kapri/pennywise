import { useEffect, useState } from 'react';
import { apiClient } from '@/utils';
import { type FetchTransactionArgs, transactionListQuery } from '../store/transactionSlice';

interface FacetRow {
  id: string;
  count: number;
}

interface FacetResponse {
  payees: FacetRow[];
  accounts: FacetRow[];
  categories: FacetRow[];
  tags: FacetRow[];
}

/** Per value, how many filtered transactions carry it. */
export interface TransactionFacets {
  payees: Record<string, number>;
  accounts: Record<string, number>;
  categories: Record<string, number>;
  tags: Record<string, number>;
}

const toCounts = (rows: FacetRow[] | null | undefined) =>
  Object.fromEntries((rows ?? []).map((row) => [row.id, row.count]));

/**
 * The payees, accounts, categories and tags found in the transactions the filters match,
 * from `GET /transactions/facets`. Each list ignores its own filter, so with
 * one payee picked the others under the same category still show.
 * Null while off or not loaded yet.
 */
export function useTransactionFacets(
  args: FetchTransactionArgs,
  enabled: boolean,
  refreshKey: unknown,
): TransactionFacets | null {
  const key = transactionListQuery({ ...args, cursor: undefined, limit: 1 });
  const [loaded, setLoaded] = useState<{ key: string; facets: TransactionFacets } | null>(null);

  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    const timer = setTimeout(async () => {
      try {
        const res = await apiClient.get<FacetResponse>(`transactions/facets?${key}`);
        if (cancelled) return;
        setLoaded({
          key,
          facets: {
            payees: toCounts(res.payees),
            accounts: toCounts(res.accounts),
            categories: toCounts(res.categories),
            tags: toCounts(res.tags),
          },
        });
      } catch {
        if (!cancelled) setLoaded(null);
      }
    }, 300);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [enabled, key, refreshKey]);

  // Keep the last counts while new ones load, so sections don't jump around.
  return enabled ? (loaded?.facets ?? null) : null;
}
