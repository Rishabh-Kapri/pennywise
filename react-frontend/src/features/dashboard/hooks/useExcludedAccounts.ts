import { useCallback, useEffect, useState } from 'react';

const storageKey = (budgetId: string) => `pennywise.dashboard.excludedAccounts.${budgetId}`;

// Stores the accounts left out rather than the ones kept in, so a new
// account shows up in the total without anyone ticking it.
function readExcluded(budgetId?: string): Set<string> {
  if (!budgetId) return new Set();
  try {
    const ids: unknown = JSON.parse(localStorage.getItem(storageKey(budgetId)) ?? '[]');
    return new Set(Array.isArray(ids) ? ids.filter((id): id is string => typeof id === 'string') : []);
  } catch {
    return new Set();
  }
}

/** Accounts the user unticked from the dashboard balance, remembered per budget. */
export function useExcludedAccounts(budgetId?: string) {
  const [state, setState] = useState(() => ({ budgetId, ids: readExcluded(budgetId) }));

  // Switching budgets swaps in that budget's choices during render.
  let current = state;
  if (state.budgetId !== budgetId) {
    current = { budgetId, ids: readExcluded(budgetId) };
    setState(current);
  }

  useEffect(() => {
    if (!state.budgetId) return;
    try {
      localStorage.setItem(storageKey(state.budgetId), JSON.stringify([...state.ids]));
    } catch {
      // storage full or blocked: the choice just won't persist
    }
  }, [state]);

  const toggle = useCallback((accountId: string) => {
    setState((prev) => {
      const ids = new Set(prev.ids);
      if (ids.has(accountId)) ids.delete(accountId);
      else ids.add(accountId);
      return { ...prev, ids };
    });
  }, []);

  return [current.ids, toggle] as const;
}
