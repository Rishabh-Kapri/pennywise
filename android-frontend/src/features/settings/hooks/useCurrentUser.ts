import { useCallback, useEffect, useState } from 'react';
import { apiClient } from '../../../utils/api';
import type { CurrentUser } from '../types';

/**
 * Loads the authenticated user together with the OAuth providers connected to
 * it. Kept local to the settings screen — nothing else needs the provider list,
 * so it does not earn a slice in the store.
 */
export function useCurrentUser() {
  const [user, setUser] = useState<CurrentUser | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setUser(await apiClient.get<CurrentUser>('auth/users/me'));
      setError(null);
    } catch (err: unknown) {
      setUser(null);
      setError(err instanceof Error ? err.message : 'Failed to load your account');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  return { user, loading, error, reload: load };
}
