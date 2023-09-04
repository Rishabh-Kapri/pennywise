export interface Transaction {
  id?: string;
  budgetId: string;
  date: string;
  amount: number;
  note?: string;
  accountId: string; // account which the transaction belongs to
  payeeId: string; // id of the payee
  categoryId: string; // id of the category which the transaction belongs to
  transferAccountId?: string;
  transferTransactionId?: string;
  createdAt?: string;
  updatedAt?: string;
  deleted: boolean;
}
