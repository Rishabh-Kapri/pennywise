import type { Transaction } from '@/features/transactions/types/transaction.types';
import type { BalancePoint } from './types';

/**
 * Format an amount as whole-rupee INR, e.g. ₹1,23,456.
 * Pass the raw value; the sign is dropped so callers control +/− display.
 */
export function formatCurrency(amount: number): string {
  return new Intl.NumberFormat('en-IN', {
    style: 'currency',
    currency: 'INR',
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(Math.abs(amount));
}

/**
 * Compact INR for axis ticks and dense labels, e.g. ₹1.2L, ₹45K.
 */
export function formatCompactCurrency(amount: number): string {
  const abs = Math.abs(amount);
  const sign = amount < 0 ? '-' : '';
  if (abs >= 10000000) return `${sign}₹${trimZero(abs / 10000000)}Cr`;
  if (abs >= 100000) return `${sign}₹${trimZero(abs / 100000)}L`;
  if (abs >= 1000) return `${sign}₹${trimZero(abs / 1000)}K`;
  return `${sign}₹${Math.round(abs)}`;
}

/**
 * Compact INR for a chart axis whose ticks are `step` apart: as many decimals
 * as the step needs (₹2.75L, ₹3L), so neighbouring ticks never read the same.
 */
export function formatTickCurrency(amount: number, step: number): string {
  const abs = Math.abs(amount);
  const sign = amount < 0 ? '−' : '';
  const [unit, suffix] = abs >= 1e7 ? [1e7, 'Cr'] : abs >= 1e5 ? [1e5, 'L'] : abs >= 1e3 ? [1e3, 'K'] : [1, ''];
  const ratio = step / unit;
  let decimals = 0;
  while (decimals < 2 && Math.abs(ratio * 10 ** decimals - Math.round(ratio * 10 ** decimals)) > 1e-6) decimals += 1;
  const digits = (abs / unit).toFixed(decimals);
  return `${sign}₹${decimals ? digits.replace(/\.?0+$/, '') : digits}${suffix}`;
}

function trimZero(value: number): string {
  return value.toFixed(1).replace(/\.0$/, '');
}

/** −₹1,191 / +₹500 (with `plus`) / ₹0. Uses a true minus sign. */
export function formatSignedCurrency(amount: number, plus = false): string {
  const rounded = Math.round(amount);
  if (rounded < 0) return `−${formatCurrency(rounded)}`;
  if (rounded > 0 && plus) return `+${formatCurrency(rounded)}`;
  return formatCurrency(rounded);
}

/** Whole rupees without the symbol, e.g. 1,59,006. */
export function formatDigits(amount: number): string {
  return new Intl.NumberFormat('en-IN', { maximumFractionDigits: 0 }).format(Math.abs(Math.round(amount)));
}

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

/** Net effect of a transaction on its account's balance. */
export const transactionNet = (txn: Transaction) => (txn.inflow ?? 0) - (txn.outflow ?? 0);

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
