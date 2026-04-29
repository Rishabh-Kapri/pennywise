import {
  LucideCalendarDays,
  LucideCheckCircle,
  LucideHandCoins,
  LucideMinus,
  LucideNotebookPen,
  LucidePlus,
  LucideStore,
  LucideThumbsDown,
  LucideThumbsUp,
  Trash2,
  X,
} from 'lucide-react';
import type { ReactNode } from 'react';
import { getCurrencyLocaleString, getLocaleDate, getTodaysDate } from '@/utils/date.utils';
import {
  TransactionStatus,
  type Transaction,
  type TransactionStatus as TransactionStatusType,
} from '../../types/transaction.types';
import { AccountDropdown } from '../popovers/AccountPopover';
import { PayeeDropdown } from '../popovers/PayeePopover';
import { CategoryDropdown } from '../popovers/CategoryPopover';
import { DateDropdown } from '../popovers/DatePopover';
import styles from './TransactionDetailPanel.module.css';

type TransactionStatusAction = Extract<
  TransactionStatusType,
  typeof TransactionStatus.APPROVED | typeof TransactionStatus.REJECTED
>;
type TransactionChangeHandler = (key: keyof Transaction, value: string | number | null) => void;
type TransactionSelectHandler = (
  idKey: keyof Transaction,
  nameKey: keyof Transaction,
) => (id: string, name: string) => void;

interface Props {
  selectedTxn: Transaction | null;
  isAddingNew: boolean;
  isClosing?: boolean;
  onChange: TransactionChangeHandler;
  onSelectChange: TransactionSelectHandler;
  onSave: () => void;
  onDelete: () => void;
  onClose: () => void;
  onStatusChange: (status: TransactionStatusAction) => void;
  onAnimationEnd?: () => void;
}

function getDisplayAmount(txn: Transaction) {
  if ((txn.inflow ?? 0) !== 0) {
    return { value: txn.inflow ?? 0, isInflow: true };
  }
  return { value: txn.outflow ?? 0, isInflow: false };
}

function PanelFrame({
  title,
  isClosing,
  onClose,
  onAnimationEnd,
  children,
  footer,
}: {
  title: string;
  isClosing: boolean;
  onClose: () => void;
  onAnimationEnd?: () => void;
  children: ReactNode;
  footer: ReactNode;
}) {
  return (
    <div
      className={`${styles.panel} ${isClosing ? styles.panelClosing : ''}`}
      onAnimationEnd={onAnimationEnd}>
      <div className={styles.panelHeader}>
        <span className={styles.panelTitle}>{title}</span>
        <button type="button" className={styles.closeButton} onClick={onClose} aria-label="Close">
          <X size={18} />
        </button>
      </div>
      {children}
      <div className={styles.panelFooter}>{footer}</div>
    </div>
  );
}

function AmountField({ txn, isInflow, onChange }: { txn: Transaction; isInflow: boolean; onChange: TransactionChangeHandler }) {
  const setAmountType = (nextType: 'inflow' | 'outflow') => {
    const current = isInflow ? (txn.inflow ?? 0) : (txn.outflow ?? 0);
    onChange(nextType, Math.abs(current));
    onChange(nextType === 'inflow' ? 'outflow' : 'inflow', null);
  };

  return (
    <div className={styles.formGroup}>
      <label className={styles.label}>Amount</label>
      <div className={styles.amountInputRow}>
        <div className={styles.amountTypeControl}>
          <button
            type="button"
            className={`${styles.amountTypeBtn} ${!isInflow ? styles.amountTypeBtnActive : ''}`}
            onClick={() => setAmountType('outflow')}
            aria-pressed={!isInflow}>
            Outflow
          </button>
          <button
            type="button"
            className={`${styles.amountTypeBtn} ${isInflow ? styles.amountTypeBtnActive : ''}`}
            onClick={() => setAmountType('inflow')}
            aria-pressed={isInflow}>
            Inflow
          </button>
        </div>
        <input
          type="text"
          className={styles.amountInput}
          placeholder="0"
          value={isInflow ? (txn.inflow ?? '') : (txn.outflow ?? '')}
          onChange={(event) => onChange(isInflow ? 'inflow' : 'outflow', event.target.value)}
        />
      </div>
    </div>
  );
}

function AddTransactionForm({
  txn,
  isInflow,
  onChange,
  onSelectChange,
}: {
  txn: Transaction;
  isInflow: boolean;
  onChange: TransactionChangeHandler;
  onSelectChange: TransactionSelectHandler;
}) {
  return (
    <div className={`${styles.panelBody} ${styles.addNewPanelBody}`}>
      <AmountField txn={txn} isInflow={isInflow} onChange={onChange} />

      <div className={styles.formGroup}>
        <label className={styles.label}>Date</label>
        <DateDropdown value={txn.date || getTodaysDate()} onClick={onSelectChange('date', 'date')} variant="form" />
      </div>

      <div className={styles.formGroup}>
        <label className={styles.label}>Payee</label>
        <PayeeDropdown value={txn.payeeName} onClick={onSelectChange('payeeId', 'payeeName')} variant="form" />
      </div>

      <div className={styles.formGroup}>
        <label className={styles.label}>Category</label>
        <CategoryDropdown value={txn.categoryName ?? ''} onClick={onSelectChange('categoryId', 'categoryName')} variant="form" />
      </div>

      <div className={styles.formGroup}>
        <label className={styles.label}>Account</label>
        <AccountDropdown value={txn.accountName} onClick={onSelectChange('accountId', 'accountName')} variant="form" />
      </div>

      <div className={styles.formGroup}>
        <label className={styles.label}>Note</label>
        <input
          type="text"
          className={styles.textInput}
          placeholder="Add a note..."
          value={txn.note ?? ''}
          onChange={(event) => onChange('note', event.target.value)}
        />
      </div>
    </div>
  );
}

function TransactionView({
  txn,
  displayAmount,
  isInflow,
  onChange,
  onSelectChange,
  onStatusChange,
}: {
  txn: Transaction;
  displayAmount: number;
  isInflow: boolean;
  onChange: TransactionChangeHandler;
  onSelectChange: TransactionSelectHandler;
  onStatusChange: (status: TransactionStatusAction) => void;
}) {
  return (
    <div className={styles.panelBody}>
      <section className={styles.heroSection}>
        <div className={`${styles.heroAmount} ${isInflow ? styles.heroAmountInflow : styles.heroAmountOutflow}`}>
          {isInflow ? <LucidePlus color="var(--color-text-secondary)" /> : <LucideMinus color="var(--color-text-secondary)" />}
          {getCurrencyLocaleString(displayAmount)}
        </div>
        <div className={styles.heroCategoryControl}>
          <CategoryDropdown
            value={txn.categoryName ?? ''}
            onClick={onSelectChange('categoryId', 'categoryName')}
          />
        </div>
      </section>

      <section className={styles.metaGrid}>
        <MetaItem icon={<LucideCalendarDays color="var(--color-text)" size={18} />}>
          {getLocaleDate(
            txn.date,
            { weekday: 'short', month: 'short', day: 'numeric', year: 'numeric' },
            ['en-US'],
          )}
        </MetaItem>
        <MetaItem icon={<LucideStore color="var(--color-text)" size={18} />}>{txn.payeeName || '-'}</MetaItem>
        <MetaItem icon={<LucideHandCoins color="var(--color-text)" size={18} />}>{txn.accountName || '-'}</MetaItem>
      </section>

      <section className={styles.statusSection}>
        <span className={styles.metaLabel}>
          <LucideCheckCircle size={18} />
          <span>Status</span>
        </span>
        <StatusControl status={txn.status} onStatusChange={onStatusChange} />
      </section>

      <section className={styles.notesSection}>
        <span className={styles.metaLabel}>
          <LucideNotebookPen color="var(--color-text)" size={18} />
          <span>Notes</span>
        </span>
        <textarea
          className={styles.notesInput}
          placeholder="Something about this transaction you would like to recall later?"
          value={txn.note ?? ''}
          onChange={(event) => onChange('note', event.target.value)}
        />
      </section>
    </div>
  );
}

function MetaItem({ icon, children }: { icon: ReactNode; children: ReactNode }) {
  return (
    <div className={styles.metaItem}>
      {icon}
      <strong className={styles.metaValue}>{children}</strong>
    </div>
  );
}

function StatusControl({
  status,
  onStatusChange,
}: {
  status: Transaction['status'];
  onStatusChange: (status: TransactionStatusAction) => void;
}) {
  if (status === TransactionStatus.UNAPPROVED) {
    return (
      <div className={styles.statusActions}>
        <button type="button" className={styles.approveBtn} onClick={() => onStatusChange(TransactionStatus.APPROVED)} aria-label="Approve transaction">
          <LucideThumbsUp size={13} />
          Approve
        </button>
        <button type="button" className={styles.rejectBtn} onClick={() => onStatusChange(TransactionStatus.REJECTED)} aria-label="Reject transaction">
          <LucideThumbsDown size={13} />
          Reject
        </button>
      </div>
    );
  }

  if (!status) {
    return <span className={styles.metaValue}>-</span>;
  }

  return <span className={`${styles.statusPill} ${styles[`status${status}`]}`}>{status.charAt(0) + status.slice(1).toLowerCase()}</span>;
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

  if (isAddingNew) {
    return (
      <PanelFrame
        title="New Transaction"
        isClosing={isClosing}
        onClose={onClose}
        onAnimationEnd={onAnimationEnd}
        footer={(
          <>
            <button type="button" className={styles.cancelBtn} onClick={onClose}>
              Cancel
            </button>
            <button type="button" className={styles.saveBtn} onClick={onSave}>
              Save
            </button>
          </>
        )}>
        <AddTransactionForm txn={selectedTxn} isInflow={isInflow} onChange={onChange} onSelectChange={onSelectChange} />
      </PanelFrame>
    );
  }

  return (
    <PanelFrame
      title="Transaction"
      isClosing={isClosing}
      onClose={onClose}
      onAnimationEnd={onAnimationEnd}
      footer={(
        <>
          <button type="button" className={styles.deleteBtn} onClick={onDelete}>
            <Trash2 size={16} />
            <span>Delete</span>
          </button>
          <button type="button" className={styles.saveBtn} onClick={onSave}>
            Save
          </button>
        </>
      )}>
      <TransactionView
        txn={selectedTxn}
        displayAmount={displayAmount}
        isInflow={isInflow}
        onChange={onChange}
        onSelectChange={onSelectChange}
        onStatusChange={onStatusChange}
      />
    </PanelFrame>
  );
}
