import { headlessApi, hasStoredSession, NotAuthenticatedError } from '../../utils/headlessApi';
import { getCurrentMonthKey } from '../../utils/date';

/**
 * Data loaders for the read-only widgets. Each returns a discriminated state so
 * the render path never has to deal with exceptions -- a headless render that
 * throws leaves a stale widget on the home screen.
 */

export type LoadFailure = { kind: 'signedOut' } | { kind: 'error'; message: string };

/** Wraps a loader so auth and transport failures become renderable states. */
async function guarded<T>(load: () => Promise<T>): Promise<T | LoadFailure> {
  if (!(await hasStoredSession())) return { kind: 'signedOut' };
  try {
    return await load();
  } catch (error) {
    if (error instanceof NotAuthenticatedError) return { kind: 'signedOut' };
    return { kind: 'error', message: error instanceof Error ? error.message : 'Request failed' };
  }
}

// --- Budget: ready to assign -------------------------------------------------

type CategoryGroup = {
  name: string;
  isSystem: boolean;
  budgeted: Record<string, number>;
  activity: Record<string, number>;
  balance: Record<string, number>;
  categories: Array<{ name: string; balance: Record<string, number>; activity: Record<string, number> }>;
};

export type BudgetWidgetState =
  | LoadFailure
  | {
      kind: 'ready';
      readyToAssign: number;
      assigned: number;
      available: number;
      spent: number;
      overspentCount: number;
      month: string;
    };

export async function loadBudgetState(): Promise<BudgetWidgetState> {
  return guarded(async () => {
    const month = getCurrentMonthKey();
    const [inflow, groups] = await Promise.all([
      headlessApi.get<number>('categories/inflow'),
      headlessApi.get<CategoryGroup[]>(`category-groups?month=${month}`)
    ]);

    const list = Array.isArray(groups) ? groups : [];
    const sum = (pick: (g: CategoryGroup) => number) => list.reduce((total, g) => total + (pick(g) || 0), 0);

    // `activity` is negative for spending; report it as a positive figure.
    const spent = Math.abs(sum((g) => g.activity?.[month] ?? 0));
    const overspentCount = list
      .flatMap((g) => g.categories ?? [])
      .filter((category) => (category.balance?.[month] ?? 0) < 0).length;

    return {
      kind: 'ready' as const,
      readyToAssign: typeof inflow === 'number' ? inflow : 0,
      assigned: sum((g) => g.budgeted?.[month] ?? 0),
      available: sum((g) => g.balance?.[month] ?? 0),
      spent,
      overspentCount,
      month
    };
  });
}

// --- Accounts: balances ------------------------------------------------------

type Account = {
  name: string;
  balance?: number;
  closed: boolean;
  deleted: boolean;
};

export type AccountsWidgetState =
  | LoadFailure
  | { kind: 'ready'; total: number; cash: number; debt: number; accountCount: number };

export async function loadAccountsState(): Promise<AccountsWidgetState> {
  return guarded(async () => {
    const accounts = await headlessApi.get<Account[]>('accounts');
    const live = (Array.isArray(accounts) ? accounts : []).filter((a) => !a.closed && !a.deleted);

    const balances = live.map((a) => a.balance ?? 0);
    return {
      kind: 'ready' as const,
      total: balances.reduce((sum, b) => sum + b, 0),
      cash: balances.filter((b) => b > 0).reduce((sum, b) => sum + b, 0),
      debt: Math.abs(balances.filter((b) => b < 0).reduce((sum, b) => sum + b, 0)),
      accountCount: live.length
    };
  });
}

// --- Recent transactions -----------------------------------------------------

type NormalizedTransaction = {
  id?: string;
  date: string;
  payeeName: string;
  categoryName?: string | null;
  accountName: string;
  inflow: number | null;
  outflow: number | null;
};

type PaginatedTransactions = { data?: NormalizedTransaction[] };

export type RecentTransaction = {
  id: string;
  payee: string;
  detail: string;
  amount: number;
};

export type RecentWidgetState = LoadFailure | { kind: 'ready'; transactions: RecentTransaction[] };

/** Rows shown in the list widget. Kept small: RemoteViews cross a size-limited boundary. */
export const RECENT_LIMIT = 5;

export async function loadRecentState(): Promise<RecentWidgetState> {
  return guarded(async () => {
    const page = await headlessApi.get<PaginatedTransactions>(`transactions/normalized?limit=${RECENT_LIMIT}`);
    const rows = Array.isArray(page?.data) ? page.data : [];

    return {
      kind: 'ready' as const,
      transactions: rows.slice(0, RECENT_LIMIT).map((txn, index) => ({
        id: txn.id ?? `${txn.date}-${index}`,
        payee: txn.payeeName || 'Unknown payee',
        detail: txn.categoryName || txn.accountName || '',
        amount: (txn.inflow ?? 0) || -(txn.outflow ?? 0)
      }))
    };
  });
}
