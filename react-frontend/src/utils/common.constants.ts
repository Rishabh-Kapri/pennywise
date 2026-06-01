export interface PaginationResponse<T> {
  data: T;
  total: number;
  pagination: {
    prevCursor?: string;
    nextCursor?: string;
  };
}

export const LoadingState = {
  IDLE: 'idle',
  PENDING: 'pending',
  SUCCESS: 'success',
  ERROR: 'error',
} as const;
export type LoadingState = (typeof LoadingState)[keyof typeof LoadingState];
