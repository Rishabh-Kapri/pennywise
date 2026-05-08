export enum LoadingState {
  IDLE = 'idle',
  PENDING = 'pending',
  SUCCESS = 'success',
  ERROR = 'error'
}

export interface PaginationResponse<T> {
  data: T;
  pagination: {
    nextCursor?: string | null;
  };
  total: number;
}
