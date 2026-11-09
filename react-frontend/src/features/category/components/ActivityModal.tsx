import { useEffect, useState } from 'react';
import { Receipt as ReceiptText } from '@phosphor-icons/react';
import { Popover } from '@/components/common/Popover/Popover';
import type { Transaction } from '@/features/transactions/types/transaction.types';
import { apiClient } from '@/utils';
import { type PaginationResponse } from '@/utils/common.constants';
import { getLocaleDate } from '@/utils/date.utils';
import { formatBudgetAmount } from './budgetFormat';
import styles from './ActivityModal.module.css';

interface ActivityPopoverProps {
  isOpen: boolean;
  onClose: () => void;
  triggerRef: React.RefObject<HTMLElement | null>;
  categoryId: string;
  categoryName: string;
  month: string;
  activityAmount: number;
}

// The transactions slice only ever holds the page the transactions screen last
// fetched, so filtering it client-side showed "no transactions" for any month
// outside that page. Fetch the category's month directly instead.
const TXN_LIMIT = 200;

function monthBounds(month: string): { startDate: string; endDate: string } {
  const [year, monthNumber] = month.split('-').map(Number);
  // day 0 of the next month is the last day of this one
  const lastDay = new Date(Date.UTC(year, monthNumber, 0)).getUTCDate();
  return {
    startDate: `${month}-01`,
    endDate: `${month}-${String(lastDay).padStart(2, '0')}`,
  };
}

export function ActivityPopover({
  isOpen,
  onClose,
  triggerRef,
  categoryId,
  categoryName,
  month,
  activityAmount,
}: ActivityPopoverProps) {
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!isOpen || !categoryId || !month) return;

    let cancelled = false;
    const { startDate, endDate } = monthBounds(month);
    const params = new URLSearchParams({
      'categoryId[]': categoryId,
      startDate,
      endDate,
      limit: String(TXN_LIMIT),
    });

    setLoading(true);
    setError(null);
    apiClient
      .get<PaginationResponse<Transaction[]>>(`transactions/normalized?${params.toString()}`)
      .then((response) => {
        if (cancelled) return;
        setTransactions(response.data ?? []);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setTransactions([]);
        setError(err instanceof Error ? err.message : 'Failed to load transactions');
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [isOpen, categoryId, month]);

  if (!isOpen) return null;

  return (
    <Popover
      id={`activity-popover-${categoryId}`}
      isOpen={isOpen}
      triggerRef={triggerRef}
      width={380}
      alignment="center"
      onClose={onClose}
    >
      <div className={styles.popoverContainer}>
        <header className={styles.header}>
          <div className={styles.heading}>
            <span className={styles.eyebrow}>
              {getLocaleDate(month, { month: 'long' })} activity
              {!loading && transactions.length > 0 &&
                ` · ${transactions.length} transaction${transactions.length === 1 ? '' : 's'}`}
            </span>
            <span className={styles.title}>{categoryName}</span>
          </div>
          <span className={styles.totalAmount}>{formatBudgetAmount(activityAmount)}</span>
        </header>

        {loading || error || transactions.length === 0 ? (
          <div className={styles.emptyState}>
            <ReceiptText size={24} className={styles.emptyIcon} />
            <span className={styles.emptyText}>
              {loading ? 'Loading transactions…' : (error ?? 'No transactions this month')}
            </span>
          </div>
        ) : (
            <ul className={styles.list}>
              {transactions.map((txn) => {
                const isInflow = (txn.inflow ?? 0) > 0;
                const amount = isInflow ? (txn.inflow ?? 0) : -(txn.outflow ?? 0);
                const date = new Date(txn.date).toLocaleDateString('en-IN', {
                  day: 'numeric',
                  month: 'short',
                });
                return (
                  <li key={txn.id} className={styles.item}>
                    <div className={styles.itemMain}>
                      <span className={styles.payee}>{txn.payeeName || 'No payee'}</span>
                      <span className={styles.meta}>
                        {date}
                        {txn.note ? ` · ${txn.note}` : ''}
                      </span>
                    </div>
                    <span className={`${styles.amount} ${isInflow ? styles.amountInflow : ''}`}>
                      {isInflow ? '+' : ''}
                      {formatBudgetAmount(amount)}
                    </span>
                  </li>
                );
              })}
            </ul>
        )}
      </div>
    </Popover>
  );
}
