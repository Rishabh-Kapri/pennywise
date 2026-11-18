import type { Transaction } from '@/features/transactions/types/transaction.types';
import { transactionNet } from '@/features/transactions/utils/amount';
import type { BalancePoint } from './types/dashboard.types';

// ── Dates (transaction dates are plain YYYY-MM-DD strings) ──

/** Shifts a YYYY-MM-DD date by whole days. Runs in UTC so DST never skips a day. */
export function addDays(date: string, days: number): string {
  const d = new Date(`${date}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() + days);
  return d.toISOString().slice(0, 10);
}

function parseDay(date: string): Date {
  return new Date(`${date}T00:00:00`);
}

/** "14 Oct" */
export function formatDayMonth(date: string): string {
  return parseDay(date).toLocaleDateString('en-IN', { day: 'numeric', month: 'short' });
}

/** "Wed, 14 Oct" */
export function formatWeekdayDate(date: string): string {
  return parseDay(date).toLocaleDateString('en-IN', { weekday: 'short', day: 'numeric', month: 'short' });
}

/** "Today", "Yesterday", else "Mon, 19 Oct". */
export function formatRelativeDay(date: string, today: string): string {
  if (date === today) return 'Today';
  if (date === addDays(today, -1)) return 'Yesterday';
  return formatWeekdayDate(date);
}

/** "October" or "October 2025" when it isn't this year. */
export function formatMonthName(month: string, style: 'long' | 'short' = 'long'): string {
  const [year, monthNum] = month.split('-').map(Number);
  const date = new Date(year, monthNum - 1, 1);
  const sameYear = year === new Date().getFullYear();
  return date.toLocaleDateString('en-IN', sameYear ? { month: style } : { month: style, year: 'numeric' });
}

// ── Balance history ──


/**
 * End-of-day balance of the given accounts for every day from `start` to
 * `today`, walked backwards from their current balance. Needs every
 * transaction on those accounts dated after `start`.
 */
export function buildBalanceHistory(
  currentBalance: number,
  transactions: Transaction[],
  accountIds: Set<string>,
  start: string,
  today: string,
): BalancePoint[] {
  const netByDate = new Map<string, number>();
  for (const txn of transactions) {
    if (!accountIds.has(txn.accountId)) continue;
    netByDate.set(txn.date, (netByDate.get(txn.date) ?? 0) + transactionNet(txn));
  }

  // Account balances already include anything dated after today.
  let balance = currentBalance;
  for (const [date, net] of netByDate) {
    if (date > today) balance -= net;
  }

  const points: BalancePoint[] = [];
  for (let date = today; date >= start; date = addDays(date, -1)) {
    const change = netByDate.get(date) ?? 0;
    points.push({ date, balance, change });
    balance -= change;
  }
  points.reverse();

  // Before the accounts' first transaction the walk sits at zero; start the
  // line where the money does.
  const firstActive = points.findIndex((point) => point.change !== 0);
  if (firstActive > 0 && points.slice(0, firstActive).every((point) => Math.abs(point.balance) < 0.5)) {
    return points.slice(firstActive);
  }
  return points;
}

/** Evenly spaced round ticks covering [min, max]. */
export function niceTicks(min: number, max: number, count = 3): number[] {
  const span = max - min || Math.abs(max) * 0.1 || 1000;
  const rough = span / count;
  const magnitude = 10 ** Math.floor(Math.log10(rough));
  const step = [1, 2, 2.5, 5, 10].map((m) => m * magnitude).find((s) => s >= rough) ?? 10 * magnitude;
  const lo = Math.floor(min / step) * step;
  const hi = Math.max(Math.ceil(max / step) * step, lo + step);
  const ticks: number[] = [];
  for (let tick = lo; tick <= hi + step / 2; tick += step) ticks.push(tick);
  return ticks;
}
