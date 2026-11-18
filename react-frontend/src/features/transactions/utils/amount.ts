import type { Transaction } from '../types/transaction.types';

/** A transaction's effect on its account: inflow positive, outflow negative. */
export const transactionNet = (txn: Pick<Transaction, 'inflow' | 'outflow'>) => (txn.inflow ?? 0) - (txn.outflow ?? 0);
