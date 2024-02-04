export interface Transaction {
  id?: string;
  budgetId: string;
  date: string; // locale date string
  amount: number;
  note?: string;
  accountId: string; // account which the transaction belongs to
  payeeId: string; // id of the payee
  categoryId: string | null; // id of the category which the transaction belongs to, null for transfer transaction
  transferTransactionId?: string | null; // id of the tranfer transaction
  transferAccountId?: string | null; // id of the transfer account, only present when transferTransactionId is present
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
  transferTransactionId: string | null,
  transferAccountId: string | null,
  accountName: string;
  accountId: string;
  payeeName: string;
  payeeId: string;
  categoryName: string | null;
  categoryId: string | null;
}

export type TransactionSearchKeys = 'date' | 'payeeName' | 'categoryName' | 'note' | 'accountName';

export const SearchColumns = {
  date: 'date',
  payee: 'payeeName',
  category: 'categoryName',
  memo: 'note',
  account: 'accountName',
};

export const SelectedAccountColumns = [
  { name: 'Date', class: 'flex-[0_0_11%]' },
  { name: 'Payee', class: 'flex-[0_0_19.666%]' },
  { name: 'Category', class: 'flex-[0_0_19.666%]' },
  { name: 'Memo', class: 'flex-[0_0_19.666%]' },
  { name: 'Outflow', class: 'flex-[0_0_10%]' },
  { name: 'Inflow', class: 'flex-[0_0_10%]' },
  { name: 'Balance', class: 'flex-[0_0_10%]' },
];

export const AllAccountsColumns = [
  { name: 'Account', class: 'flex-[0_0_10%]' },
  { name: 'Date', class: 'flex-[0_0_11%]' },
  { name: 'Payee', class: 'flex-[0_0_20.33%]' },
  { name: 'Category', class: 'flex-[0_0_20.33%]' },
  { name: 'Memo', class: 'flex-[0_0_20.33%]' },
  { name: 'Outflow', class: 'flex-[0_0_9%]' },
  { name: 'Inflow', class: 'flex-[0_0_9%]' },
];
