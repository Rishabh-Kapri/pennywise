
import {
  ArrowLeft,
  CalendarDays,
  ChevronLeft,
  ChevronRight,
  FileText,
  Info,
  PencilLine,
  ReceiptText,
} from 'lucide-react';
import { getCurrencyLocaleString } from '@/utils/date.utils';
import type { Transaction } from '../types/transaction.types';
import styles from './Transaction.module.css';
import { useEffect, useMemo, useState } from 'react';

interface TransactionMobileProps {
  transactions: Transaction[];
  selectedTransactionId?: string;
  showAccountName: boolean;
  onSelectTransaction: (index: number, transaction: Transaction) => void;
}

function getTransactionAmount(txn: Transaction) {
  if ((txn.inflow ?? 0) !== 0) {
    return txn.inflow ?? 0;
  }

  return -(txn.outflow ?? 0);
}

function getTransactionDate(txn: Transaction) {
  return new Date(`${txn.date}T00:00:00`);
}

function getDateGroupLabel(txn: Transaction) {
  return getTransactionDate(txn).toLocaleDateString('en-GB', {
    day: '2-digit',
    month: 'long',
  });
}

function getCardDateLabel(txn: Transaction) {
  return getTransactionDate(txn).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
  });
}

function getDetailDateLabel(txn: Transaction) {
  return getTransactionDate(txn).toLocaleDateString('en-US', {
    weekday: 'short',
    month: 'short',
    day: 'numeric',
    year: '2-digit',
  });
}

interface TransactionCardView {
  amount: number;
  cardDateLabel: string;
  index: number;
  isInflow: boolean;
  txn: Transaction;
}

interface TransactionDateGroup {
  key: string;
  label: string;
  transactions: TransactionCardView[];
}

function groupTransactionsByDate(transactions: Transaction[]) {
  const groups = new Map<string, TransactionDateGroup>();

  transactions.forEach((txn, index) => {
    const key = txn.date;
    const amount = getTransactionAmount(txn);
    const cardView: TransactionCardView = {
      amount,
      cardDateLabel: getCardDateLabel(txn),
      index,
      isInflow: amount > 0,
      txn,
    };

    const existingGroup = groups.get(key);

    if (existingGroup) {
      existingGroup.transactions.push(cardView);
      return;
    }

    groups.set(key, {
      key,
      label: getDateGroupLabel(txn),
      transactions: [cardView],
    });
  });

  return Array.from(groups.values());
}

export function TransactionMobile({
  transactions,
  selectedTransactionId,
  showAccountName,
  onSelectTransaction,
}: TransactionMobileProps) {
  const [activeIndex, setActiveIndex] = useState<number | null>(null);
  const groupedTransactions = useMemo(
    () => groupTransactionsByDate(transactions),
    [transactions],
  );
  const hasTransactions = useMemo(
    () => groupedTransactions.length > 0,
    [groupedTransactions],
  );
  const activeTransaction = useMemo(
    () => (activeIndex === null ? null : transactions[activeIndex]),
    [activeIndex, transactions],
  );
  const activeAmount = useMemo(
    () => (activeTransaction ? getTransactionAmount(activeTransaction) : 0),
    [activeTransaction],
  );
  const canGoPrevious = useMemo(
    () => activeIndex !== null && activeIndex > 0,
    [activeIndex],
  );
  const canGoNext = useMemo(
    () => activeIndex !== null && activeIndex < transactions.length - 1,
    [activeIndex, transactions.length],
  );

  useEffect(() => {
    if (activeIndex !== null && activeIndex >= transactions.length) {
      setActiveIndex(null);
    }
  }, [activeIndex, transactions.length]);

  if (!hasTransactions) {
    return (
      <div className={styles.mobileEmptyState}>
        <span>No transactions found</span>
      </div>
    );
  }

  if (activeTransaction && activeIndex !== null) {
    const isInflow = activeAmount > 0;

    return (
      <div className={styles.mobileDetail}>
        <div className={styles.mobileDetailNav}>
          <button
            type="button"
            className={styles.mobileIconButton}
            aria-label="Back to transactions"
            onClick={() => setActiveIndex(null)}>
            <ArrowLeft size={24} />
          </button>
          <div className={styles.mobileDetailStepper}>
            <button
              type="button"
              className={styles.mobileIconButton}
              aria-label="Previous transaction"
              disabled={!canGoPrevious}
              onClick={() => {
                const nextIndex = activeIndex - 1;
                setActiveIndex(nextIndex);
                onSelectTransaction(nextIndex, transactions[nextIndex]);
              }}>
              <ChevronLeft size={24} />
            </button>
            <button
              type="button"
              className={styles.mobileIconButton}
              aria-label="Next transaction"
              disabled={!canGoNext}
              onClick={() => {
                const nextIndex = activeIndex + 1;
                setActiveIndex(nextIndex);
                onSelectTransaction(nextIndex, transactions[nextIndex]);
              }}>
              <ChevronRight size={24} />
            </button>
          </div>
        </div>

        <section className={styles.mobileDetailHero}>
          <div
            className={`${styles.mobileDetailAmount} ${
              isInflow ? styles.mobileTxnInflow : ''
            }`}>
            {isInflow ? '+' : '-'}{' '}
            {getCurrencyLocaleString(Math.abs(activeAmount))}
          </div>
          <div className={styles.mobileDetailCategory}>
            {activeTransaction.categoryName || 'Uncategorized'}
          </div>
        </section>

        <section className={styles.mobileDetailMetaGrid}>
          <div>
            <span>From</span>
            <strong>{activeTransaction.accountName || 'Account'}</strong>
          </div>
          <div>
            <span>On</span>
            <strong>{getDetailDateLabel(activeTransaction)}</strong>
          </div>
        </section>

        <section className={styles.mobileDetailPanel}>
          <div className={styles.mobileDetailRow}>
            <PencilLine size={22} />
            <div>
              <span>Paid to</span>
              <strong>{activeTransaction.payeeName || 'Transaction'}</strong>
            </div>
            <ChevronRight size={22} />
          </div>
        </section>

        <section className={styles.mobileDetailPanel}>
          <div className={styles.mobileDetailRow}>
            <Info size={22} />
            <strong>More Details</strong>
            <ChevronRight size={22} />
          </div>
        </section>

        <section className={styles.mobileDetailNotes}>
          <div className={styles.mobileDetailNotesHeader}>
            <span>
              <CalendarDays size={20} />
              Notes
            </span>
            <span>
              <ReceiptText size={20} />
              Add receipt
            </span>
          </div>
          <p>
            <FileText size={20} />
            {activeTransaction.note ||
              'Something about this transaction you would like to recall later?'}
          </p>
        </section>
      </div>
    );
  }

  return (
    <div className={styles.mobileTxnContainer}>
      {groupedTransactions.map((group) => (
        <section key={group.key} className={styles.mobileTxnGroup}>
          <div className={styles.mobileDateHeader}>
            <span>{group.label}</span>
          </div>
          <div className={styles.mobileTxnCards}>
            {group.transactions.map((card) => {
              return (
                <button
                  type="button"
                  key={card.txn.id ?? `${card.txn.date}-${card.index}`}
                  className={`${styles.mobileTxnCard} ${
                    selectedTransactionId === card.txn.id ? styles.selected : ''
                  }`}
                  onClick={() => {
                    setActiveIndex(card.index);
                    onSelectTransaction(card.index, card.txn);
                  }}>
                  <div className={styles.mobileTxnTopRow}>
                    <span className={styles.mobileTxnPayee}>
                      {card.txn.payeeName ||
                        card.txn.accountName ||
                        'Transaction'}
                    </span>
                    <span className={styles.mobileTxnDate}>
                      {card.cardDateLabel}
                    </span>
                  </div>
                  <div className={styles.mobileTxnBottomRow}>
                    <span
                      className={`${styles.mobileTxnAmount} ${
                        card.isInflow ? styles.mobileTxnInflow : ''
                      }`}>
                      {card.isInflow ? '+' : '-'}{' '}
                      {getCurrencyLocaleString(Math.abs(card.amount))}
                    </span>
                    <span className={styles.mobileTxnCategory}>
                      {/* <Banknote size={18} /> */}
                      {card.txn.categoryName || 'Uncategorized'}
                    </span>
                  </div>
                  {showAccountName && card.txn.accountName && (
                    <div className={styles.mobileTxnAccount}>
                      {card.txn.accountName}
                    </div>
                  )}
                </button>
              );
            })}
          </div>
        </section>
      ))}
    </div>
  );
}
