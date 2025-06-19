/**
 * Stored in DB
 */
export interface CategoryDTO {
  id?: string;
  budgetId: string;
  categoryGroupId: string;
  name: string;
  deleted?: boolean;
  createdAt?: string;
  updatedAt?: string;
  hidden?: boolean;
  note?: string | null;
  goal?: Goal;
  showBudgetInput?: boolean;
  budgeted: Record<string, number> | number;
}

/**
 * Used to show in the UI
 */
export interface Category extends CategoryDTO {
  activity?: Record<string, number>;
  balance?: Record<string, number>;
  budgeted: Record<string, number>;
}

export interface InflowCategory extends CategoryDTO {
  budgeted: number;
}

export interface Goal {
  type: string; // type of goal @TODO: define this
  day: number; // day of the month
  target: string; // needed for spending, savings 
  amount: number; // amount in the goal
  targetMonth: string;
  overallFunded: number;
}
