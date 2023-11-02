export interface Transaction {
  id?: string;
  budgetId: string;
  date: string; // locale date string
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

export interface NormalizedTransaction {
  id?: string;
  budgetId: string;
  date: string;
  outflow: number | null;
  inflow: number | null;
  balance: number;
  note?: string;
  accountName: string;
  accountId: string;
  payeeName: string;
  payeeId: string;
  categoryName: string;
  categoryId: string;
}

export const TransactionColumns = [
  { name: 'Account', class: 'flex-[0_0_11%]' },
  { name: 'Date', class: 'flex-[0_0_11%]' },
  { name: 'Payee', class: 'flex-[0_0_18%]' },
  { name: 'Category', class: 'flex-[0_0_18%]' },
  { name: 'Memo', class: 'flex-[0_0_18%]' },
  { name: 'Outflow', class: 'flex-[0_0_8%]' },
  { name: 'Inflow', class: 'flex-[0_0_8%]' },
  { name: 'Balance', class: 'flex-[0_0_8%]' },
];
