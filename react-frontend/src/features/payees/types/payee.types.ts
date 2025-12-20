import type { LoadingState } from "@/utils";

export interface Payee {
  id?: string;
  budgetId: string;
  name: string;
  transferAccountId: string | null; // id of the account whose transfer payee is this
  createdAt?: string;
  updatedAt?: string;
  deleted: boolean;
}

export interface PayeeState {
  allPayees: Payee[];
  loading: LoadingState;
  error: string | null;
}
