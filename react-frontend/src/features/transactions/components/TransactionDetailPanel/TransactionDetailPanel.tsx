import { X, Trash2, LucidePlus, LucideMinus, LucideCalendarDays, LucideStore, LucideHandCoins, LucideNotebookPen, LucideCheckCircle, LucideThumbsUp, LucideThumbsDown } from 'lucide-react';
import { getCurrencyLocaleString, getTodaysDate } from '@/utils/date.utils';
import type { Transaction } from '../../types/transaction.types';
import { AccountDropdown } from '../popovers/AccountPopover';
import { PayeeDropdown } from '../popovers/PayeePopover';
import { CategoryDropdown } from '../popovers/CategoryPopover';
import { DateDropdown } from '../popovers/DatePopover';
import styles from './TransactionDetailPanel.module.css';
import { getLocaleDate } from '@/utils/date.utils';

interface Props {
  selectedTxn: Transaction | null;
  isAddingNew: boolean;
  isClosing?: boolean;
  onChange: (key: keyof Transaction, value: string | number | null) => void;
  onSelectChange: (
    idKey: keyof Transaction,
    nameKey: keyof Transaction,
  ) => (id: string, name: string) => void;
  onSave: () => void;
  onDelete: () => void;
  onClose: () => void;
  onStatusChange: (status: 'APPROVED' | 'REJECTED') => void;
  onAnimationEnd?: () => void;
}

function getDisplayAmount(txn: Transaction): {
  value: number;
  isInflow: boolean;
} {
  if ((txn.inflow ?? 0) !== 0)
    return { value: txn.inflow ?? 0, isInflow: true };
  return { value: txn.outflow ?? 0, isInflow: false };
}

export function TransactionDetailPanel({
  selectedTxn,
  isAddingNew,
  isClosing = false,
  onChange,
  onSelectChange,
  onSave,
  onDelete,
  onClose,
  onStatusChange,
  onAnimationEnd,
}: Props) {
  if (!selectedTxn) return null;

  const { value: displayAmount, isInflow } = getDisplayAmount(selectedTxn);

  // ── Add-new mode ─────────────────────────────────────────────────────────
  if (isAddingNew) {
    return (
      <div
        className={`${styles.panel} ${isClosing ? styles.panelClosing : ''}`}
        onAnimationEnd={onAnimationEnd}>
        <div className={styles.panelHeader}>
          <span className={styles.panelTitle}>New Transaction</span>
          <button
            type="button"
            className={styles.closeButton}
            onClick={onClose}
            aria-label="Close">
            <X size={18} />
          </button>
        </div>

        <div className={styles.panelBody}>
          {/* Amount input */}
          <div className={styles.formGroup}>
            <label className={styles.label}>Amount</label>
            <div className={styles.amountInputRow}>
              <button
                type="button"
                className={`${styles.amountTypeBtn} ${!isInflow ? styles.amountTypeBtnActive : ''}`}
                onClick={() => {
                  const current =
                    selectedTxn.outflow ?? selectedTxn.inflow ?? 0;
                  onChange('outflow', Math.abs(current));
                  onChange('inflow', null);
                }}>
                Outflow
              </button>
              <button
                type="button"
                className={`${styles.amountTypeBtn} ${isInflow ? styles.amountTypeBtnActive : ''}`}
                onClick={() => {
                  const current =
                    selectedTxn.inflow ?? selectedTxn.outflow ?? 0;
                  onChange('inflow', Math.abs(current));
                  onChange('outflow', null);
                }}>
                Inflow
              </button>
              <input
                type="text"
                className={styles.amountInput}
                placeholder="0"
                value={
                  isInflow
                    ? (selectedTxn.inflow ?? '')
                    : (selectedTxn.outflow ?? '')
                }
                onChange={(e) => {
                  const key = isInflow ? 'inflow' : 'outflow';
                  onChange(key, e.target.value);
                }}
              />
            </div>
          </div>

          {/* Date */}
          <div className={styles.formGroup}>
            <label className={styles.label}>Date</label>
            <DateDropdown
              value={selectedTxn.date || getTodaysDate()}
              onClick={onSelectChange('date', 'date')}
            />
          </div>

          {/* Payee */}
          <div className={styles.formGroup}>
            <label className={styles.label}>Payee</label>
            <PayeeDropdown
              value={selectedTxn.payeeName}
              onClick={onSelectChange('payeeId', 'payeeName')}
            />
          </div>

          {/* Category */}
          <div className={styles.formGroup}>
            <label className={styles.label}>Category</label>
            <CategoryDropdown
              value={selectedTxn.categoryName ?? ''}
              onClick={onSelectChange('categoryId', 'categoryName')}
            />
          </div>

          {/* Account */}
          <div className={styles.formGroup}>
            <label className={styles.label}>Account</label>
            <AccountDropdown
              value={selectedTxn.accountName}
              onClick={onSelectChange('accountId', 'accountName')}
            />
          </div>

          {/* Note */}
          <div className={styles.formGroup}>
            <label className={styles.label}>Note</label>
            <input
              type="text"
              className={styles.textInput}
              placeholder="Add a note..."
              value={selectedTxn.note ?? ''}
              onChange={(e) => onChange('note', e.target.value)}
            />
          </div>
        </div>

        <div className={styles.panelFooter}>
          <button type="button" className={styles.cancelBtn} onClick={onClose}>
            Cancel
          </button>
          <button type="button" className={styles.saveBtn} onClick={onSave}>
            Save
          </button>
        </div>
      </div>
    );
  }

  // ── View mode (existing transaction) ─────────────────────────────────────
  return (
    <div
      className={`${styles.panel} ${isClosing ? styles.panelClosing : ''}`}
      onAnimationEnd={onAnimationEnd}>
      <div className={styles.panelHeader}>
        <span className={styles.panelTitle}>Transaction</span>
        <button
          type="button"
          className={styles.closeButton}
          onClick={onClose}
          aria-label="Close">
          <X size={18} />
        </button>
      </div>

      <div className={styles.panelBody}>
        {/* Hero */}
        <section className={styles.heroSection}>
          <div
            className={`${styles.heroAmount} ${isInflow ? styles.heroAmountInflow : styles.heroAmountOutflow}`}>
            {isInflow ? (
              <LucidePlus color="var(--color-text-secondary)" />
            ) : (
              <LucideMinus color="var(--color-text-secondary)" />
            )}
            {getCurrencyLocaleString(displayAmount)}
          </div>
          {selectedTxn.categoryName && (
            <span className={styles.heroCategoryPill}>
              {selectedTxn.categoryName}
            </span>
          )}
        </section>

        {/* Meta grid */}
        <section className={styles.metaGrid}>
          <div className={styles.metaItem}>
            <LucideCalendarDays color="var(--color-text)" size={18} />
            <strong className={styles.metaValue}>
              {getLocaleDate(
                selectedTxn.date,
                {
                  weekday: 'short',
                  month: 'short',
                  day: 'numeric',
                  year: 'numeric',
                },
                ['en-US'],
              )}
            </strong>
          </div>
          <div className={styles.metaItem}>
            <LucideStore color="var(--color-text)" size={18} />
            <strong className={styles.metaValue}>
              {selectedTxn.payeeName || '–'}
            </strong>
          </div>
          <div className={styles.metaItem}>
            <LucideHandCoins color="var(--color-text)" size={18} />
            <strong className={styles.metaValue}>
              {selectedTxn.accountName || '–'}
            </strong>
          </div>
        </section>


        {/* Status */}
        <section className={styles.statusSection}>
          <span className={styles.metaLabel}>
            <LucideCheckCircle size={18} />
            <span>Status</span>
          </span>
          {selectedTxn.status === 'UNAPPROVED' ? (
            <div className={styles.statusActions}>
              <button
                type="button"
                className={styles.approveBtn}
                onClick={() => onStatusChange('APPROVED')}
                aria-label="Approve transaction">
                <LucideThumbsUp size={13} />
                Approve
              </button>
              <button
                type="button"
                className={styles.rejectBtn}
                onClick={() => onStatusChange('REJECTED')}
                aria-label="Reject transaction">
                <LucideThumbsDown size={13} />
                Reject
              </button>
            </div>
          ) : selectedTxn.status ? (
            <span className={`${styles.statusPill} ${styles[`status${selectedTxn.status}`]}`}>
              {selectedTxn.status.charAt(0) + selectedTxn.status.slice(1).toLowerCase()}
            </span>
          ) : (
            <span className={styles.metaValue}>–</span>
          )}
        </section>

        {/* Note */}
        <section className={styles.notesSection}>
          <span className={styles.metaLabel}>
            <LucideNotebookPen color="var(--color-text)" size={18} />
            <span>Notes</span>
          </span>
          <p className={styles.notesText}>
            {selectedTxn.note ||
              'Something about this transaction you would like to recall later?'}
          </p>
        </section>
      </div>

      <div className={styles.panelFooter}>
        <button type="button" className={styles.deleteBtn} onClick={onDelete}>
          <Trash2 size={16} />
          <span>Delete</span>
        </button>
      </div>
    </div>
  );
}
