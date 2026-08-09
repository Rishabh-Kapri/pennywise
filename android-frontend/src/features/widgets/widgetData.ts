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

type Category = {
  id?: string;
  name: string;
  hidden?: boolean;
  budgeted?: Record<string, number>;
  activity?: Record<string, number>;
  balance?: Record<string, number>;
};

type CategoryGroup = {
  name: string;
  isSystem: boolean;
  budgeted: Record<string, number>;
  activity: Record<string, number>;
  balance: Record<string, number>;
  categories: Category[];
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

// --- Pressure: which categories need attention right now ---------------------

/**
 * Why a category is being surfaced.
 * - `over`   : already spent past what was assigned
 * - `ahead`  : on pace to overspend before month end
 * - `close`  : nearly used up, but not obviously running hot
 */
export type PressureReason = 'over' | 'ahead' | 'close';

export type CategoryPressure = {
  id: string;
  name: string;
  reason: PressureReason;
  budgeted: number;
  spent: number;
  /** Negative when overspent. */
  remaining: number;
  /** spent / budgeted, uncapped so callers can tell 1.0 from 3.0. */
  usedFraction: number;
  /** Projected end-of-month spend at the current pace. */
  projected: number;
};

export type PressureWidgetState =
  | LoadFailure
  | { kind: 'ready'; pressured: CategoryPressure[]; trackedCount: number; monthProgress: number };

/** How many rows the widget will show. Kept small for the RemoteViews size budget. */
export const PRESSURE_LIMIT = 3;

/**
 * A category counts as running hot when its spend rate outpaces the month by
 * this much. 1.25 means "burning 25% faster than the calendar", which by month
 * end lands meaningfully over unless something changes.
 */
const PACE_THRESHOLD = 1.25;

/** Below this, an alarming ratio is just noise -- early-month lumpy spending. */
const MIN_USED_TO_WARN = 0.5;

/** Nearly exhausted, flagged regardless of pace. */
const CLOSE_THRESHOLD = 0.85;

/** Fraction of the month elapsed, floored so day 1 does not divide by zero. */
export function monthElapsedFraction(now = new Date()): number {
  const daysInMonth = new Date(now.getFullYear(), now.getMonth() + 1, 0).getDate();
  return Math.min(1, Math.max(now.getDate() / daysInMonth, 1 / daysInMonth));
}

export async function loadPressureState(): Promise<PressureWidgetState> {
  return guarded(async () => {
    const month = getCurrentMonthKey();
    const groups = await headlessApi.get<CategoryGroup[]>(`category-groups?month=${month}`);
    const list = Array.isArray(groups) ? groups : [];
    const elapsed = monthElapsedFraction();

    const tracked: CategoryPressure[] = [];

    for (const group of list) {
      // System groups hold inflow/uncategorised bookkeeping, not spending plans.
      if (group.isSystem) continue;

      for (const category of group.categories ?? []) {
        if (category.hidden) continue;

        const budgeted = category.budgeted?.[month] ?? 0;
        // `activity` is negative for spending; inflows to a spending category
        // would make this positive, which is not overspending.
        const spent = Math.max(0, -(category.activity?.[month] ?? 0));
        if (budgeted <= 0 && spent <= 0) continue;

        const remaining = category.balance?.[month] ?? budgeted - spent;
        const usedFraction = budgeted > 0 ? spent / budgeted : spent > 0 ? Infinity : 0;
        const projected = spent / elapsed;

        tracked.push({
          id: category.id ?? `${group.name}:${category.name}`,
          name: category.name,
          reason: 'close',
          budgeted,
          spent,
          remaining,
          usedFraction,
          projected
        });
      }
    }

    const pressured = tracked
      .map((entry) => {
        // Spending against an unbudgeted category is overspending by definition.
        if (entry.remaining < 0 || (entry.budgeted <= 0 && entry.spent > 0)) {
          return { ...entry, reason: 'over' as const };
        }
        if (entry.usedFraction >= MIN_USED_TO_WARN && entry.usedFraction / elapsed >= PACE_THRESHOLD) {
          return { ...entry, reason: 'ahead' as const };
        }
        if (entry.usedFraction >= CLOSE_THRESHOLD) {
          return { ...entry, reason: 'close' as const };
        }
        return null;
      })
      .filter((entry): entry is CategoryPressure => entry !== null)
      // Worst first: overspent, then whatever is furthest through its budget.
      .sort((a, b) => {
        const rank = { over: 0, ahead: 1, close: 2 };
        if (rank[a.reason] !== rank[b.reason]) return rank[a.reason] - rank[b.reason];
        return b.usedFraction - a.usedFraction;
      });

    return {
      kind: 'ready' as const,
      pressured: pressured.slice(0, PRESSURE_LIMIT),
      trackedCount: tracked.length,
      monthProgress: elapsed
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
