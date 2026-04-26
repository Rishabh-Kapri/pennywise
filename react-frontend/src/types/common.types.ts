import type { Transaction } from "@/features/transactions/types/transaction.types";
import type React from "react";

export interface TransactionColumns {
  key: keyof Transaction;
  label: string;
  layout: {
    flex: string;
    textAlign: 'left' | 'right' | 'center' | 'justify';
    minWidth?: number | string;
    overflow?: string;
  };
  className?: string[];
  render?: (txn: Transaction) => React.ReactNode;
}
