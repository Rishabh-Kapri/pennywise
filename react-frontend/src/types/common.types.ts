import type { Transaction } from "@/features/transactions/types/transaction.types";
import type React from "react";

export interface TransactionColumns {
  key: keyof Transaction;
  label: string;
  layout: {
    gridColumn: string;
    textAlign: 'left' | 'right' | 'center' | 'justify';
  };
  className?: string[];
  render?: (txn: Transaction) => React.ReactNode;
}
