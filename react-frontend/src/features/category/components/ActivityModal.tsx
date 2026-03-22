import { useEffect, useMemo } from 'react';
import { ReceiptText } from 'lucide-react';
import { Popover } from '@/components/common/Popover/Popover';
import { useAppSelector } from '@/app/hooks';
import { getCurrencyLocaleString } from '@/utils/date.utils';
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

export function ActivityPopover({
  isOpen,
  onClose,
  triggerRef,
  categoryId,
  categoryName,
  month,
  activityAmount,
}: ActivityPopoverProps) {
  const { transactions } = useAppSelector((state) => state.transactions);


  const filteredTransactions = useMemo(() => {
    return transactions.filter((txn) => {
      if (txn.categoryId !== categoryId) return false;
      const txnMonth = txn.date.substring(0, 7);
      return txnMonth === month;
    });
  }, [transactions, categoryId, month]);

  useEffect(() => {
    if (categoryName === '🛒 Groceries') {
      console.log('Groceries ActivityPopover transactions:', transactions);
      console.log('Groceries transactions for categoryId', categoryId, 'and month', month, ':', filteredTransactions);
    }
  }, [categoryName, transactions, filteredTransactions, categoryId, month]);

  if (!isOpen) return null;

  return (
    <Popover
      id={`activity-popover-${categoryId}`}
      isOpen={isOpen}
      triggerRef={triggerRef}
      width={520}
      alignment="center"
      onClose={onClose}
    >
      <div className={styles.popoverContainer}>
        {/* Header */}
        <div className={styles.header}>
          <span className={styles.title}>{categoryName}</span>
          <span className={styles.totalAmount}>
            {getCurrencyLocaleString(activityAmount)}
          </span>
        </div>

        {/* Body */}
        {filteredTransactions.length === 0 ? (
          <div className={styles.emptyState}>
            <ReceiptText size={28} className={styles.emptyIcon} />
            <div className={styles.emptyText}>No transactions this month</div>
          </div>
        ) : (
          <table className={styles.txnTable}>
            <thead>
              <tr>
                <th>Date</th>
                <th>Payee</th>
                <th>Memo</th>
                <th className={styles.amountCol}>Amount</th>
              </tr>
            </thead>
            <tbody>
              {filteredTransactions.map((txn) => {
                const amount = txn.outflow ?? txn.inflow ?? 0;
                const isInflow = (txn.inflow ?? 0) > 0;
                return (
                  <tr key={txn.id}>
                    <td>
                      {new Date(txn.date).toLocaleDateString('en-IN', {
                        day: 'numeric',
                        month: 'short',
                      })}
                    </td>
                    <td>{txn.payeeName || '—'}</td>
                    <td>{txn.note || '—'}</td>
                    <td
                      className={`${styles.amountCol} ${
                        isInflow ? styles.amountInflow : styles.amountOutflow
                      }`}>
                      {getCurrencyLocaleString(amount)}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </div>
    </Popover>
  );
}
