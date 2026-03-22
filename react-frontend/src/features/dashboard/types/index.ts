import type { LoadingState } from '@/utils';

/**
 * Dashboard quick stats - key financial metrics
 */
export interface DashboardStats {
  totalIncome: number;
  totalExpenses: number;
  netWorth: number;
}

/**
 * Monthly spending data point for trends chart
 */
export interface SpendingTrend {
  month: string; // Format: "YYYY-MM"
  monthLabel: string; // Display format: "Jan", "Feb", etc.
  income: number;
  expenses: number;
}

/**
 * Individual category health status
 */
export interface CategoryHealth {
  id: string;
  name: string;
  budgeted: number;
  spent: number;
  remaining: number;
  percentUsed: number;
  status: 'healthy' | 'warning' | 'danger';
}

/**
 * Budget health summary
 */
export interface BudgetHealthSummary {
  healthyCount: number;
  warningCount: number;
  dangerCount: number;
  categories: CategoryHealth[];
}

/**
 * Dashboard state for Redux
 */
export interface DashboardState {
  stats: DashboardStats;
  spendingTrends: SpendingTrend[];
  budgetHealth: BudgetHealthSummary;
  loading: LoadingState;
  error: string | null;
}
