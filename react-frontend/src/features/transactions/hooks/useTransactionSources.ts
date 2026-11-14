import { useEffect, useState } from 'react';
import { apiClient } from '@/utils';
import type { TransactionSource } from '../types/transaction.types';

/** The bank messages behind a transaction. Empty for one made by hand. */
export function useTransactionSources(transactionId?: string) {
  const [sources, setSources] = useState<TransactionSource[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!transactionId) {
      setSources([]);
      setError(null);
      setIsLoading(false);
      return;
    }

    let ignore = false;
    setIsLoading(true);
    setError(null);

    apiClient
      .get<TransactionSource[]>(`transactions/${transactionId}/sources`)
      .then((list) => {
        if (!ignore) setSources(list ?? []);
      })
      .catch((err: unknown) => {
        if (ignore) return;
        setSources([]);
        setError(err instanceof Error ? err.message : 'Failed to load sources');
      })
      .finally(() => {
        if (!ignore) setIsLoading(false);
      });

    return () => {
      ignore = true;
    };
  }, [transactionId]);

  return { sources, isLoading, error };
}
