export interface PaginationResponse<T> {
  data: T;
  total: number;
  pagination: {
    prevCursor?: string;
    nextCursor?: string;
  };
  /** Money in and out across every matching row, not just this page (transaction lists only). */
  totals?: AmountTotals;
}

export interface AmountTotals {
  inflow: number;
  outflow: number;
}

export const LoadingState = {
  IDLE: 'idle',
  PENDING: 'pending',
  SUCCESS: 'success',
  ERROR: 'error',
} as const;
export type LoadingState = (typeof LoadingState)[keyof typeof LoadingState];
