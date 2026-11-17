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
  /** points card statements printed for the matching transactions; absent when none had any */
  rewardPoints?: number;
  /** how many of them came with points */
  rewardPointsCount?: number;
}

export const LoadingState = {
  IDLE: 'idle',
  PENDING: 'pending',
  SUCCESS: 'success',
  ERROR: 'error',
} as const;
export type LoadingState = (typeof LoadingState)[keyof typeof LoadingState];
