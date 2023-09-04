export interface Payee {
  id?: string;
  budgetId: string;
  name: string;
  transferAccountId: string | null; // use from the account collection
  createdAt?: string;
  updatedAt?: string;
  deleted: boolean;
}
