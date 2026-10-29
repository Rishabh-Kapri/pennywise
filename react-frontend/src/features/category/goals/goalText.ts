import { getLocaleDate } from '@/utils/date.utils';
import type { CategoryGoal } from '../types/category.types';
import { formatBudgetAmount } from '../components/budgetFormat';
import type { GoalProgress } from './goalProgress';

export const WEEKDAYS = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];

export const GOAL_TYPE_LABELS: Record<CategoryGoal['type'], string> = {
  NEED: 'Needed for spending',
  TB: 'Savings balance',
  TBD: 'Savings balance by date',
  MF: 'Monthly savings builder',
  DEBT: 'Monthly debt payment',
};

function ordinal(day: number): string {
  const rem100 = day % 100;
  if (rem100 >= 11 && rem100 <= 13) return `${day}th`;
  return `${day}${['th', 'st', 'nd', 'rd'][day % 10] ?? 'th'}`;
}

const dayOfMonth = (day?: number) => (day ? `the ${ordinal(day)}` : 'the last day');
const monthYear = (month: string) => getLocaleDate(month, { month: 'short', year: 'numeric' });

/** One line saying what the target asks for, e.g. "₹5,000 every month by the 15th". */
export function describeGoal(goal: CategoryGoal): string {
  const amount = formatBudgetAmount(goal.target);
  switch (goal.type) {
    case 'MF':
      return `Assign ${amount} every month`;
    case 'DEBT':
      return `Pay ${amount} every month`;
    case 'TB':
      return `Have a balance of ${amount}`;
    case 'TBD':
      return `Have ${amount} by ${monthYear(goal.targetMonth ?? '')}`;
    case 'NEED': {
      const every = goal.cadenceFrequency ?? 1;
      switch (goal.cadence) {
        case 'weekly':
          return `${amount} every ${WEEKDAYS[goal.day ?? 0]}`;
        case 'yearly':
          return every > 1
            ? `${amount} every ${every} years, next ${monthYear(goal.targetMonth ?? '')}`
            : `${amount} every year in ${getLocaleDate(goal.targetMonth ?? '', { month: 'long' })}`;
        case 'none':
          return `${amount} by ${dayOfMonth(goal.day)} of ${monthYear(goal.targetMonth ?? '')}`;
        default:
          return every > 1
            ? `${amount} every ${every} months, next ${monthYear(goal.targetMonth ?? '')}`
            : `${amount} every month by ${dayOfMonth(goal.day)}`;
      }
    }
  }
}

/** The repeat behaviour, for NEED targets that refill each month. */
export function describeRefill(goal: CategoryGoal): string | null {
  if (goal.type !== 'NEED' || !(goal.cadence === 'weekly' || goal.cadence === 'monthly')) return null;
  if ((goal.cadenceFrequency ?? 1) > 1) return null;
  return goal.needsWholeAmount ? 'Refills up to the target' : 'Sets aside another each month';
}

/** Short status for a row or card: "₹1,200 more needed", "Funded", … */
export function describeProgress(progress: GoalProgress): string {
  switch (progress.status) {
    case 'snoozed':
      return 'Snoozed this month';
    case 'overspent':
      return progress.underfunded > 0
        ? `Overspent · ${formatBudgetAmount(progress.underfunded)} more needed`
        : 'Overspent';
    case 'funded':
      return progress.needed > 0 ? 'Funded' : 'Fully funded';
    case 'underfunded':
      return `${formatBudgetAmount(progress.underfunded)} more needed`;
  }
}
