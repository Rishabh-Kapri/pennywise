import { useEffect, useState } from 'react';
import { apiClient } from '@/utils';
import type { LoanHistory } from '../types/loan.types';

/**
 * The loan's amortized history from the API: each recorded payment split
 * into interest and principal. Changing `refreshKey` (a string, so it is
 * stable across renders) refetches, e.g. after a payment or an edit.
 */
export function useLoanHistory(accountId: string | undefined, refreshKey: string) {
  const [history, setHistory] = useState<LoanHistory | null>(null);
  const [error, setError] = useState(false);

  useEffect(() => {
    if (!accountId) return;
    let cancelled = false;
    setError(false);
    apiClient
      .get<LoanHistory[]>(`reports/loans?accountIds=${accountId}`)
      .then((histories) => {
        if (!cancelled) setHistory(histories.find((h) => h.accountId === accountId) ?? null);
      })
      .catch(() => {
        if (!cancelled) setError(true);
      });
    return () => {
      cancelled = true;
    };
  }, [accountId, refreshKey]);

  return { history: history?.accountId === accountId ? history : null, error };
}
