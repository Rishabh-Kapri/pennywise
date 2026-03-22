import type { LoadingState } from "@/utils";

export interface Tag {
  id: string;
  budgetId: string;
  name: string;
  color: string;
  deleted: boolean;
  createdAt?: string;
  updatedAt?: string;
}

export interface TagState {
  allTags: Tag[];
  loading: LoadingState;
  error: string | null;
}
