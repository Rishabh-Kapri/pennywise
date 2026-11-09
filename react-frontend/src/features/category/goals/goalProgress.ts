import type { Category, CategoryGoal } from '../types/category.types';

/**
 * How a category's target stands in one month. Only the target's definition
 * is stored (CategoryGoal); everything here is derived from the month's
 * budgeted/activity/balance, so it follows optimistic assign edits instantly.
 *
 * `needed` is what the month asks for and `funded` what counts toward it.
 * Every target type measures `funded` so that assigning X more raises it by
 * exactly X, which is what makes `budgeted + underfunded` the right amount
 * for "Assign underfunded".
 */
export interface GoalProgress {
  goal: CategoryGoal;
  needed: number;
  funded: number;
  underfunded: number;
  /** 0–100, how much of `needed` is funded */
  percent: number;
  status: 'funded' | 'underfunded' | 'overspent' | 'snoozed';
  /** YYYY-MM the money is due by, for dated and repeating-beyond-monthly targets */
  dueMonth?: string;
  /** months left to fund it, the current one included */
  monthsLeft?: number;
  /** balance targets report the whole target, not just this month's share */
  overall?: { target: number; funded: number };
}

/** Months from `from` to `to` (both YYYY-MM); negative when `to` is earlier. */
export function monthsBetween(from: string, to: string): number {
  const [fy, fm] = from.split('-').map(Number);
  const [ty, tm] = to.split('-').map(Number);
  return (ty - fy) * 12 + (tm - fm);
}

function shiftMonth(month: string, delta: number): string {
  const [y, m] = month.split('-').map(Number);
  const index = y * 12 + (m - 1) + delta;
  return `${Math.floor(index / 12)}-${String((index % 12) + 1).padStart(2, '0')}`;
}

/** How many times `weekday` (0 = Sunday) falls in the month. */
export function weekdayCount(month: string, weekday: number): number {
  const [y, m] = month.split('-').map(Number);
  const days = new Date(Date.UTC(y, m, 0)).getUTCDate();
  const first = new Date(Date.UTC(y, m - 1, 1)).getUTCDay();
  // days 1..days; the first `weekday` is offset days in
  const offset = (weekday - first + 7) % 7;
  return offset >= days ? 0 : Math.floor((days - 1 - offset) / 7) + 1;
}

/** Rounds up to the paisa so a spread-out target is never left a rupee short. */
const ceilCents = (value: number) => Math.ceil(Math.round(value * 1e6) / 1e4) / 100;
const roundCents = (value: number) => Math.round(value * 100) / 100;

/**
 * The month the money is next due for a once/yearly/every-N-months target, or
 * null once a one-time target's month has passed (it no longer asks for
 * anything).
 */
export function nextDueMonth(goal: CategoryGoal, month: string): string | null {
  const anchor = goal.targetMonth;
  if (!anchor) return null;
  if (goal.cadence === 'none') {
    return monthsBetween(anchor, month) > 0 ? null : anchor;
  }
  const step = (goal.cadence === 'yearly' ? 12 : 1) * Math.max(goal.cadenceFrequency ?? 1, 1);
  const behind = monthsBetween(anchor, month);
  if (behind <= 0) return anchor;
  return shiftMonth(anchor, Math.ceil(behind / step) * step);
}

/** True when the NEED target asks for a fixed amount every month. */
export function isMonthlyNeed(goal: CategoryGoal): boolean {
  return (
    goal.type === 'NEED' &&
    (goal.cadence === 'weekly' ||
      ((goal.cadence ?? 'monthly') === 'monthly' && (goal.cadenceFrequency ?? 1) <= 1))
  );
}

export function getGoalProgress(category: Category, month: string): GoalProgress | null {
  const goal = category.goal;
  if (!goal || monthsBetween(goal.creationMonth, month) < 0) return null;

  const budgeted = category.budgeted?.[month] ?? 0;
  const activity = category.activity?.[month] ?? 0;
  const available = category.balance?.[month] ?? 0;
  // what was carried in from earlier months, before this month's assignment
  const leftover = roundCents(available - budgeted - activity);

  if (goal.snoozedMonth === month) {
    return { goal, needed: 0, funded: 0, underfunded: 0, percent: 100, status: 'snoozed' };
  }

  let needed: number;
  let funded: number;
  let dueMonth: string | undefined;
  let monthsLeft: number | undefined;
  let overall: GoalProgress['overall'];

  switch (goal.type) {
    case 'MF':
    case 'DEBT':
      needed = goal.target;
      funded = budgeted;
      break;

    case 'TB':
      // a savings balance is met by what is available, so spending from it
      // asks for a top-up
      needed = goal.target;
      funded = available;
      overall = { target: goal.target, funded: available };
      break;

    case 'TBD': {
      dueMonth = goal.targetMonth;
      monthsLeft = dueMonth ? Math.max(monthsBetween(month, dueMonth) + 1, 1) : 1;
      needed = ceilCents(Math.max(goal.target - leftover, 0) / monthsLeft);
      funded = budgeted;
      overall = { target: goal.target, funded: available };
      break;
    }

    case 'NEED': {
      if (isMonthlyNeed(goal)) {
        needed =
          goal.cadence === 'weekly'
            ? roundCents(goal.target * weekdayCount(month, goal.day ?? 0))
            : goal.target;
        // "refill up to" counts what was left over; "set aside another" does not
        funded = goal.needsWholeAmount ? Math.max(leftover, 0) + budgeted : budgeted;
        break;
      }
      const due = nextDueMonth(goal, month);
      if (!due) return null;
      dueMonth = due;
      monthsLeft = monthsBetween(month, due) + 1;
      needed = ceilCents(Math.max(goal.target - Math.max(leftover, 0), 0) / monthsLeft);
      funded = budgeted;
      overall = { target: goal.target, funded: Math.max(leftover, 0) + budgeted };
      break;
    }

    default:
      return null;
  }

  const underfunded = roundCents(Math.max(needed - funded, 0));
  const percent = needed > 0 ? Math.min(Math.max((funded / needed) * 100, 0), 100) : 100;
  const status: GoalProgress['status'] =
    available < 0 ? 'overspent' : underfunded > 0 ? 'underfunded' : 'funded';

  return { goal, needed, funded, underfunded, percent, status, dueMonth, monthsLeft, overall };
}
