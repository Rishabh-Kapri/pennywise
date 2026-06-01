import type { LoadingState } from '../../utils/constants';

export interface Tag {
  id?: string;
  name: string;
  color: string;
  budgetId?: string;
}

export interface TagState {
  tags: Tag[];
  loading: LoadingState;
  error: string | null;
}
