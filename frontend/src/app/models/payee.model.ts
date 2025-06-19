export interface Payee {
  id?: string;
  budgetId: string;
  name: string;
  transferAccountId: string | null; // id of the account whose transfer payee is this
  createdAt?: string;
  updatedAt?: string;
  deleted: boolean;
}
