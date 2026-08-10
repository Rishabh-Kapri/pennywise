import type { LoadingState } from '@/utils';

export type RecurringFrequency = 'DAILY' | 'WEEKLY' | 'MONTHLY' | 'YEARLY';

export interface RecurringTransaction {
  id: string;
  budgetId: string;
  name: string;
  accountId: string;
  payeeId?: string | null;
  categoryId?: string | null;
  amount: number;
  note: string;
  frequency: RecurringFrequency;
  intervalCount: number;
  nextDate: string;
  endDate?: string | null;
  lastRunDate?: string | null;
  paused: boolean;
  deleted: boolean;
  createdAt?: string;
  updatedAt?: string;

  accountName?: string | null;
  payeeName?: string | null;
  categoryName?: string | null;
}

export type RecurringTransactionDraft = Omit<
  RecurringTransaction,
  'id' | 'budgetId' | 'deleted' | 'createdAt' | 'updatedAt' | 'accountName' | 'payeeName' | 'categoryName' | 'lastRunDate'
>;

export interface RecurringRunResult {
  created: number;
  skipped: number;
}

export interface RecurringState {
  rules: RecurringTransaction[];
  loading: LoadingState;
  saving: boolean;
  lastRun: RecurringRunResult | null;
  error: string | null;
}

export const FREQUENCY_LABELS: Record<RecurringFrequency, string> = {
  DAILY: 'day',
  WEEKLY: 'week',
  MONTHLY: 'month',
  YEARLY: 'year',
};

/** "Every month" / "Every 2 weeks" */
export function describeSchedule(frequency: RecurringFrequency, intervalCount: number): string {
  const unit = FREQUENCY_LABELS[frequency];
  return intervalCount <= 1 ? `Every ${unit}` : `Every ${intervalCount} ${unit}s`;
}
