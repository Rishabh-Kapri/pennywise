import {
  ArrowsLeftRight,
  CalendarDots as CalendarDays,
  CheckCircle,
  Code,
  Database,
  HandCoins,
  Minus,
  NotePencil,
  Plus,
  Robot,
  Storefront,
  Tag as TagIcon,
  ThumbsDown,
  ThumbsUp,
  Trash as Trash2,
  X,
} from '@phosphor-icons/react';
import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { useAppSelector } from '@/app/hooks';
import { apiClient } from '@/utils';
import { Popover } from '@/components/common/Popover/Popover';
import { getCurrencyLocaleString, getLocaleDate, getTodaysDate } from '@/utils/date.utils';
import {
  TransactionStatus,
  type CipherPrediction,
  type Transaction,
  type TransactionPrediction,
  type TransactionPredictionDetails,
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
type TransactionChangeHandler = (key: keyof Transaction, value: Transaction[keyof Transaction]) => void;
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

function getSignedDisplayAmount(txn: Transaction) {
  const { value, isInflow } = getDisplayAmount(txn);
  const sign = isInflow ? '+' : '-';
  return `${sign}${getCurrencyLocaleString(value)}`;
}

function formatConfidence(value?: number | null) {
  if (value === null || value === undefined) return '-';
  const percent = value <= 1 ? value * 100 : value;
  return `${percent.toFixed(percent >= 10 ? 0 : 1)}%`;
}

function formatOptionalAmount(value?: number | null) {
  if (value === null || value === undefined) return '-';
  const sign = value >= 0 ? '+' : '-';
  return `${sign}${getCurrencyLocaleString(Math.abs(value))}`;
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

type DisplayTag = { id: string; name: string; color?: string | null };

function TagList({ tags }: { tags: DisplayTag[] }) {
  if (tags.length === 0) {
    return <span className={styles.emptyValue}>No tags</span>;
  }

  return (
    <div className={styles.tagList}>
      {tags.map((tag) => (
        <span
          key={tag.id}
          className={styles.tagBadge}
          style={{ backgroundColor: tag.color || '#6366f1' }}>
          {tag.name}
        </span>
      ))}
    </div>
  );
}

function DetailTagPicker({
  allTags,
  selectedTagIds,
  onChange,
}: {
  allTags: DisplayTag[];
  selectedTagIds: string[];
  onChange: (tagIds: string[]) => void;
}) {
  const [isOpen, setIsOpen] = useState(false);
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const selectedSet = useMemo(() => new Set(selectedTagIds), [selectedTagIds]);

  const toggleTag = (tagId: string) => {
    const next = new Set(selectedSet);
    if (next.has(tagId)) {
      next.delete(tagId);
    } else {
      next.add(tagId);
    }
    onChange(Array.from(next));
  };

  return (
    <>
      <button
        ref={triggerRef}
        type="button"
        className={`${styles.tagPickerButton} ${isOpen ? styles.tagPickerButtonOpen : ''}`}
        onClick={() => setIsOpen((value) => !value)}
        aria-label="Edit tags"
        aria-expanded={isOpen}
        title="Edit tags">
        <Plus size={18} />
      </button>
      <Popover
        id="transaction-detail-tags"
        isOpen={isOpen}
        triggerRef={triggerRef}
        width={240}
        onClose={() => setIsOpen(false)}>
        <div className={styles.tagPickerMenu}>
          {allTags.length === 0 && <div className={styles.emptyValue}>No tags yet</div>}
          {allTags.map((tag) => (
            <button
              key={tag.id}
              type="button"
              className={styles.tagPickerItem}
              onClick={() => toggleTag(tag.id)}>
              <input
                type="checkbox"
                checked={selectedSet.has(tag.id)}
                readOnly
                className={styles.tagPickerCheckbox}
              />
              <span
                className={styles.tagBadge}
                style={{ backgroundColor: tag.color || '#6366f1' }}>
                {tag.name}
              </span>
            </button>
          ))}
        </div>
      </Popover>
    </>
  );
}

function DetailRow({
  label,
  children,
  mono = false,
}: {
  label: string;
  children: ReactNode;
  mono?: boolean;
}) {
  return (
    <div className={styles.detailRow}>
      <span className={styles.detailLabel}>{label}</span>
      <span className={`${styles.detailValue} ${mono ? styles.monoValue : ''}`}>{children}</span>
    </div>
  );
}

function PredictionMetric({
  label,
  value,
  confidence,
}: {
  label: string;
  value?: string | null;
  confidence?: number | null;
}) {
  return (
    <div className={styles.predictionMetric}>
      <span className={styles.detailLabel}>{label}</span>
      <strong>{value || '-'}</strong>
      <span className={styles.predictionConfidence}>{formatConfidence(confidence)}</span>
    </div>
  );
}

function PredictionDetails({
  details,
  isLoading,
  error,
}: {
  details: TransactionPredictionDetails | null;
  isLoading: boolean;
  error: string | null;
}) {
  if (isLoading) {
    return <span className={styles.emptyValue}>Loading prediction...</span>;
  }

  if (error) {
    return <span className={styles.errorText}>{error}</span>;
  }

  if (!details?.cipherPrediction && !details?.prediction) {
    return <span className={styles.emptyValue}>No prediction found for this transaction.</span>;
  }

  if (details.cipherPrediction) {
    return <CipherPredictionDetails prediction={details.cipherPrediction} />;
  }

  return <LegacyPredictionDetails prediction={details.prediction!} />;
}

function CipherPredictionDetails({ prediction }: { prediction: CipherPrediction }) {
  const payees = useAppSelector((state) => state.payees.allPayees);
  const categories = useAppSelector((state) => state.categories.allCategories);
  const predictedPayee = payees.find((payee) => payee.id === prediction.predictedPayeeId);
  const predictedCategory = categories.find((category) => category.id === prediction.predictedCategoryId);

  return (
    <div className={styles.predictionDetails}>
      <div className={styles.predictionGrid}>
        <PredictionMetric label="Account" value={prediction.extractedAccount} confidence={prediction.accountConfidence} />
        <PredictionMetric label="Payee" value={predictedPayee?.name || prediction.extractedPayee} confidence={prediction.payeeConfidence} />
        <PredictionMetric label="Category" value={predictedCategory?.name} confidence={prediction.categoryConfidence} />
      </div>
      <div className={styles.detailRows}>
        <DetailRow label="Predicted amount">{formatOptionalAmount(prediction.amount)}</DetailRow>
        <DetailRow label="Source">{prediction.source || '-'}</DetailRow>
        <DetailRow label="User corrected">{prediction.hasUserCorrected ? 'Yes' : 'No'}</DetailRow>
      </div>
      {prediction.llmReasoning && (
        <pre className={styles.rawTextBlock}>{prediction.llmReasoning}</pre>
      )}
    </div>
  );
}

function LegacyPredictionDetails({ prediction }: { prediction: TransactionPrediction }) {
  return (
    <div className={styles.predictionDetails}>
      <div className={styles.predictionGrid}>
        <PredictionMetric label="Account" value={prediction.account} confidence={prediction.accountPrediction} />
        <PredictionMetric label="Payee" value={prediction.payee} confidence={prediction.payeePrediction} />
        <PredictionMetric label="Category" value={prediction.category} confidence={prediction.categoryPrediction} />
      </div>
      <div className={styles.detailRows}>
        <DetailRow label="Predicted amount">{formatOptionalAmount(prediction.amount)}</DetailRow>
        <DetailRow label="User corrected">{prediction.hasUserCorrected ? 'Yes' : 'No'}</DetailRow>
        <DetailRow label="Corrected account">{prediction.userCorrectedAccount || '-'}</DetailRow>
        <DetailRow label="Corrected payee">{prediction.userCorrectedPayee || '-'}</DetailRow>
        <DetailRow label="Corrected category">{prediction.userCorrectedCategory || '-'}</DetailRow>
      </div>
      {prediction.emailText && (
        <pre className={styles.rawTextBlock}>{prediction.emailText}</pre>
      )}
    </div>
  );
}

function AdvancedDetails({
  txn,
  predictionDetails,
  isPredictionLoading,
  predictionError,
}: {
  txn: Transaction;
  predictionDetails: TransactionPredictionDetails | null;
  isPredictionLoading: boolean;
  predictionError: string | null;
}) {
  const rawText = txn.rawBankText || predictionDetails?.cipherPrediction?.emailText || predictionDetails?.prediction?.emailText || '-';

  return (
    <details className={styles.advancedSection}>
      <summary className={styles.advancedSummary}>
        <span>
          <Database size={18} />
          More Details
        </span>
      </summary>

      <div className={styles.advancedContent}>
        <section className={styles.advancedGroup}>
          <span className={styles.metaLabel}>
            <Robot size={18} />
            <span>Prediction</span>
          </span>
          <PredictionDetails details={predictionDetails} isLoading={isPredictionLoading} error={predictionError} />
        </section>

        <section className={styles.advancedGroup}>
          <span className={styles.metaLabel}>
            <Code size={18} />
            <span>Raw Bank Text</span>
          </span>
          <pre className={styles.rawTextBlock}>{rawText}</pre>
        </section>
      </div>
    </details>
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
  const allTags = useAppSelector((state) => state.tags.allTags);
  const allAccounts = useAppSelector((state) => state.accounts.allAccounts);
  const loadedTransactions = useAppSelector((state) => state.transactions.transactions);
  const [predictionDetails, setPredictionDetails] = useState<TransactionPredictionDetails | null>(null);
  const [isPredictionLoading, setIsPredictionLoading] = useState(false);
  const [predictionError, setPredictionError] = useState<string | null>(null);

  const selectedTags = useMemo(
    () => allTags.filter((tag) => (txn.tagIds ?? []).includes(tag.id)),
    [allTags, txn.tagIds],
  );

  const linkedAccount = useMemo(
    () => allAccounts.find((account) => account.id === txn.transferAccountId) ?? null,
    [allAccounts, txn.transferAccountId],
  );

  const linkedTransaction = useMemo(
    () => loadedTransactions.find((transaction) => transaction.id === txn.transferTransactionId) ?? null,
    [loadedTransactions, txn.transferTransactionId],
  );

  useEffect(() => {
    if (!txn.id) {
      setPredictionDetails(null);
      setPredictionError(null);
      setIsPredictionLoading(false);
      return;
    }

    let ignore = false;
    setIsPredictionLoading(true);
    setPredictionError(null);

    apiClient
      .get<TransactionPredictionDetails>(`predictions/transactions/${txn.id}`)
      .then((details) => {
        if (ignore) return;
        setPredictionDetails(details);
      })
      .catch((error: unknown) => {
        if (ignore) return;
        setPredictionDetails(null);
        setPredictionError(error instanceof Error ? error.message : 'Failed to load prediction');
      })
      .finally(() => {
        if (!ignore) setIsPredictionLoading(false);
      });

    return () => {
      ignore = true;
    };
  }, [txn.id]);

  const linkedTransactionSummary = linkedTransaction
    ? `${getLocaleDate(linkedTransaction.date, { month: 'short', day: 'numeric', year: 'numeric' }, ['en-US'])} · ${linkedTransaction.payeeName || linkedTransaction.accountName || 'Transaction'} · ${getSignedDisplayAmount(linkedTransaction)}`
    : txn.transferTransactionId || '-';
  const hasLinkedTransfer = Boolean(txn.transferAccountId || txn.transferTransactionId);

  return (
    <div className={styles.panelBody}>
      <section className={styles.heroSection}>
        <div className={`${styles.heroAmount} ${isInflow ? styles.heroAmountInflow : styles.heroAmountOutflow}`}>
          {isInflow ? <Plus color="var(--color-text-secondary)" /> : <Minus color="var(--color-text-secondary)" />}
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
        <MetaItem icon={<CalendarDays color="var(--color-text)" size={18} />}>
          {getLocaleDate(
            txn.date,
            { weekday: 'short', month: 'short', day: 'numeric', year: 'numeric' },
            ['en-US'],
          )}
        </MetaItem>
        <MetaItem icon={<Storefront color="var(--color-text)" size={18} />}>{txn.payeeName || '-'}</MetaItem>
        <MetaItem icon={<HandCoins color="var(--color-text)" size={18} />}>{txn.accountName || '-'}</MetaItem>
      </section>

      <section className={styles.tagsSection}>
        <span className={styles.metaLabel}>
          <TagIcon size={18} />
          <span>Tags</span>
        </span>
        <div className={styles.tagEditor}>
          <TagList tags={selectedTags} />
          <DetailTagPicker
            allTags={allTags}
            selectedTagIds={txn.tagIds ?? []}
            onChange={(tagIds) => onChange('tagIds', tagIds)}
          />
        </div>
      </section>

      {hasLinkedTransfer && (
        <section className={styles.linkedSection}>
          <span className={styles.metaLabel}>
            <ArrowsLeftRight size={18} />
            <span>Linked Transfer</span>
          </span>
          <div className={styles.detailRows}>
            <DetailRow label="Linked account">
              {linkedAccount ? `${linkedAccount.name} (${linkedAccount.type})` : txn.transferAccountId || '-'}
            </DetailRow>
            <DetailRow label="Linked transaction">{linkedTransactionSummary}</DetailRow>
          </div>
        </section>
      )}

      <section className={styles.statusSection}>
        <span className={styles.metaLabel}>
          <CheckCircle size={18} />
          <span>Status</span>
        </span>
        <StatusControl status={txn.status} onStatusChange={onStatusChange} />
      </section>

      <section className={styles.notesSection}>
        <span className={styles.metaLabel}>
          <NotePencil color="var(--color-text)" size={18} />
          <span>Notes</span>
        </span>
        <textarea
          className={styles.notesInput}
          placeholder="Something about this transaction you would like to recall later?"
          value={txn.note ?? ''}
          onChange={(event) => onChange('note', event.target.value)}
        />
      </section>

      <AdvancedDetails
        txn={txn}
        predictionDetails={predictionDetails}
        isPredictionLoading={isPredictionLoading}
        predictionError={predictionError}
      />
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
          <ThumbsUp size={13} />
          Approve
        </button>
        <button type="button" className={styles.rejectBtn} onClick={() => onStatusChange(TransactionStatus.REJECTED)} aria-label="Reject transaction">
          <ThumbsDown size={13} />
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
