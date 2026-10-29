import type { LoadingState } from "@/utils";

/**
 * YNAB's target types (its API's goal_type values):
 * - NEED: needed for spending, on a cadence or once by a date
 * - TB:   savings balance, no deadline
 * - TBD:  savings balance by a date
 * - MF:   monthly savings builder (assign the same amount every month)
 * - DEBT: monthly debt payment
 */
export type GoalType = 'NEED' | 'TB' | 'TBD' | 'MF' | 'DEBT';

/** How often a NEED target repeats; `none` is once, by targetMonth. */
export type GoalCadence = 'none' | 'weekly' | 'monthly' | 'yearly';

/** Mirrors model.CategoryGoal; validated and normalized by the API. */
export interface CategoryGoal {
  type: GoalType;
  target: number;
  cadence?: GoalCadence;
  /** every N months / years */
  cadenceFrequency?: number;
  /** weekday (0 = Sunday) for weekly, else day of month; absent = last day */
  day?: number;
  /** true: refill up to target; false: set aside another target */
  needsWholeAmount: boolean;
  /** YYYY-MM: TBD deadline, or first due month of a once/yearly/every-N target */
  targetMonth?: string;
  /** YYYY-MM: the first month the target applies to */
  creationMonth: string;
  /** YYYY-MM: the one month the target is snoozed for */
  snoozedMonth?: string;
}

export interface Category {
  id?: string;
  budgetId: string;
  categoryGroupId: string;
  name: string;
  deleted?: boolean;
  createdAt?: string;
  updatedAt?: string;
  hidden?: boolean;
  // system categories (Inflow: Ready to Assign) cannot be renamed, moved or hidden
  isSystem?: boolean;
  note?: string | null;
  // what belongs in the category; separate from the free-form note
  description?: string;
  goal?: CategoryGoal | null;
  showBudgetInput?: boolean;
  budgeted: Record<string, number>;
  activity?: Record<string, number>;
  balance?: Record<string, number>;
}

export interface CategoryGroup {
  id?: string;
  name: string;
  hidden?: boolean;
  collapsed: boolean;
  balance: Record<string, number>;
  budgeted: Record<string, number>;
  activity: Record<string, number>;
  categories: Category[];
  isSystem: boolean;
}

export interface CategoryState {
  allCategories: Category[];
  allCategoryGroups: CategoryGroup[];
  collapseAllGroups: boolean;
  inflowAmount: number;
  loading: LoadingState;
  inflowLoading: LoadingState;
  error: string | null;
}

