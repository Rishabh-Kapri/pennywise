import { getCurrencyLocaleString } from '@/utils/date.utils';

/** Whole rupees unless there are paise; `|| 0` keeps -0 from rendering as "-₹0". */
export const formatBudgetAmount = (value: number): string =>
  getCurrencyLocaleString(value || 0, 'INR', 'en-IN', {
    minimumFractionDigits: 0,
    maximumFractionDigits: 2,
  });
