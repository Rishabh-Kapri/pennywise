import {
  CaretRightIcon,
  CheckIcon,
  PlusIcon,
  ReceiptIcon,
  TrashIcon as Trash2,
  XIcon,
} from '@phosphor-icons/react';
import { useEffect, useLayoutEffect, useMemo, useRef, useState, type KeyboardEvent, type ReactNode } from 'react';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { apiClient, errorMessage, toast } from '@/utils';
import { createTag } from '@/features/tags/store/tagSlice';
import { getAutoColor } from '@/features/tags/utils';
import { Popover } from '@/components/common/Popover/Popover';
import { getCurrencyLocaleString, getLocaleDate, getTodaysDate } from '@/utils/date.utils';
import {
  TransactionStatus,
  type CipherPrediction,
  type Transaction,
  type TransactionPrediction,
  type TransactionPredictionDetails,
  type TransactionSource,
  type TransactionStatus as TransactionStatusType,
} from '../../types/transaction.types';
import { AccountDropdown } from '../popovers/AccountPopover';
import { PayeeDropdown } from '../popovers/PayeePopover';
import { CategoryDropdown } from '../popovers/CategoryPopover';
import { DateDropdown } from '../popovers/DatePopover';
import { BankLogo } from '@/features/accounts/components/BankLogo/BankLogo';
import { TransactionLocationSection } from './TransactionLocationSection';
import { TransactionDocumentsSection } from './TransactionDocumentsSection';
import { TransactionSources } from './TransactionSources';
import { formatSourceTime, transactionTime } from '../../utils/sourceTime';
import { useTransactionSources } from '../../hooks/useTransactionSources';
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
  const inflow = Number(txn.inflow) || 0;
  const outflow = Number(txn.outflow) || 0;
  if (inflow !== 0) return { value: inflow, isInflow: true };
  if (outflow !== 0) return { value: outflow, isInflow: false };
  // nothing entered yet: the direction toggle nulls the other side
  return { value: 0, isInflow: txn.inflow != null && txn.outflow == null };
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
  leading,
  eyebrow,
  title,
  isClosing,
  onClose,
  onAnimationEnd,
  children,
  footer,
}: {
  leading?: ReactNode;
  eyebrow?: string;
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
      onAnimationEnd={onAnimationEnd}
      aria-label={`${title} details`}
      role="region">
      <header className={styles.panelHeader}>
        <div className={styles.headingRow}>
          {leading}
          <div className={styles.heading}>
            {eyebrow && <span className={styles.eyebrow}>{eyebrow}</span>}
            <h2 className={styles.title}>{title}</h2>
          </div>
        </div>
        <button type="button" className={styles.closeButton} onClick={onClose} aria-label="Close">
          <XIcon size={18} />
        </button>
      </header>
      {children}
      <footer className={styles.panelFooter}>{footer}</footer>
    </div>
  );
}

/** The big amount at the top of the panel, editable in place, with its direction. */
function AmountHero({
  txn,
  isInflow,
  onChange,
  status,
  autoFocus,
}: {
  txn: Transaction;
  isInflow: boolean;
  onChange: TransactionChangeHandler;
  status?: Transaction['status'];
  autoFocus?: boolean;
}) {
  const key = isInflow ? 'inflow' : 'outflow';
  const amount = Number(txn[key]) || 0;
  // keep the typed text so "12." or "0.5" survive while typing
  const [draft, setDraft] = useState(amount ? String(amount) : '');
  const [isEditing, setIsEditing] = useState(false);
  const inputRef = useRef<HTMLInputElement | null>(null);
  // grouped (1,089) to read, raw while typing
  const shown = isEditing ? draft : amount ? amount.toLocaleString('en-IN', { maximumFractionDigits: 2 }) : '';

  useEffect(() => {
    setDraft((current) => ((Number(current) || 0) === amount ? current : amount ? String(amount) : ''));
  }, [amount]);

  // select once the raw number has replaced the grouped one, before any typing
  useLayoutEffect(() => {
    if (isEditing) inputRef.current?.select();
  }, [isEditing]);

  const setDirection = (nextInflow: boolean) => {
    if (nextInflow === isInflow) return;
    onChange(nextInflow ? 'inflow' : 'outflow', amount);
    onChange(nextInflow ? 'outflow' : 'inflow', null);
  };

  return (
    <>
      <div className={styles.heroTop}>
        <div className={styles.amountTypeControl} role="radiogroup" aria-label="Direction">
          {[false, true].map((inflow) => (
            <button
              key={String(inflow)}
              type="button"
              role="radio"
              aria-checked={isInflow === inflow}
              className={`${styles.amountTypeBtn} ${isInflow === inflow ? styles.amountTypeBtnActive : ''}`}
              onClick={() => setDirection(inflow)}>
              {inflow ? 'Received' : 'Spent'}
            </button>
          ))}
        </div>
        <StatusBadge status={status} />
      </div>
      <label className={`${styles.heroAmount} ${isInflow ? styles.heroAmountInflow : ''}`}>
        <span className={styles.heroAmountPrefix} aria-hidden="true">
          {isInflow ? '+' : '−'}₹
        </span>
        <input
          ref={inputRef}
          type="text"
          inputMode="decimal"
          placeholder="0"
          aria-label={isInflow ? 'Amount received' : 'Amount spent'}
          className={styles.heroAmountInput}
          style={{ width: `${Math.max(shown.length, 1) + 0.6}ch` }}
          value={shown}
          autoFocus={autoFocus}
          onFocus={() => {
            setIsEditing(true);
            setDraft(amount ? String(amount) : '');
          }}
          onBlur={() => setIsEditing(false)}
          onChange={(event) => {
            // digits and one decimal point
            const text = event.target.value.replace(/[^\d.]/g, '').replace(/(\..*)\./g, '$1');
            setDraft(text);
            onChange(key, Number(text) || 0);
          }}
        />
      </label>
    </>
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
      <section className={styles.hero}>
        <AmountHero txn={txn} isInflow={isInflow} onChange={onChange} autoFocus />
      </section>

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
        <CategoryDropdown
          value={txn.categoryName ?? ''}
          onClick={onSelectChange('categoryId', 'categoryName')}
          variant="form"
        />
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
        <TagChip key={tag.id} tag={tag} />
      ))}
    </div>
  );
}

function TagChip({ tag }: { tag: DisplayTag }) {
  return (
    <span className={styles.tagBadge}>
      <span className={styles.tagDot} style={{ backgroundColor: tag.color || '#6366f1' }} aria-hidden="true" />
      {tag.name}
    </span>
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
  const dispatch = useAppDispatch();
  const [isOpen, setIsOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [isCreating, setIsCreating] = useState(false);
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const selectedSet = useMemo(() => new Set(selectedTagIds), [selectedTagIds]);

  const name = query.trim();
  const needle = name.toLowerCase();
  const shownTags = allTags.filter((tag) => tag.name.trim().toLowerCase().includes(needle));
  const exactMatch = allTags.find((tag) => tag.name.trim().toLowerCase() === needle);

  const toggleTag = (tagId: string) => {
    const next = new Set(selectedSet);
    if (next.has(tagId)) {
      next.delete(tagId);
    } else {
      next.add(tagId);
    }
    onChange(Array.from(next));
  };

  const closePicker = () => {
    setIsOpen(false);
    setQuery('');
  };

  const createAndAdd = async () => {
    if (!name || isCreating) return;
    setIsCreating(true);
    try {
      const tag = await dispatch(createTag({ name, color: getAutoColor(name) })).unwrap();
      onChange([...selectedTagIds, tag.id]);
      setQuery('');
    } catch (err) {
      toast.error("Couldn't create that tag", { description: errorMessage(err, 'Please try again.') });
    } finally {
      setIsCreating(false);
    }
  };

  const handleSearchKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key !== 'Enter' || !name) return;
    event.preventDefault();
    if (exactMatch) {
      toggleTag(exactMatch.id);
      setQuery('');
    } else {
      void createAndAdd();
    }
  };

  return (
    <>
      <button
        ref={triggerRef}
        type="button"
        className={`${styles.textButton} ${isOpen ? styles.textButtonOpen : ''}`}
        onClick={() => (isOpen ? closePicker() : setIsOpen(true))}
        aria-expanded={isOpen}>
        <PlusIcon size={13} weight="bold" />
        {selectedTagIds.length > 0 ? 'Edit' : 'Add'}
      </button>
      <Popover
        id="transaction-detail-tags"
        isOpen={isOpen}
        triggerRef={triggerRef}
        width={240}
        onClose={closePicker}>
        <div className={styles.tagPickerSearch}>
          <input
            className={styles.tagPickerInput}
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            onKeyDown={handleSearchKeyDown}
            placeholder="Search or create tags"
            aria-label="Search or create tags"
            autoFocus
          />
        </div>
        <div className={styles.tagPickerMenu}>
          {allTags.length === 0 && !name && <div className={styles.emptyValue}>No tags yet</div>}
          {name && !exactMatch && (
            <button
              type="button"
              className={styles.tagPickerItem}
              disabled={isCreating}
              onClick={() => void createAndAdd()}>
              <PlusIcon size={13} weight="bold" className={styles.tagPickerCheck} />
              <span className={styles.tagPickerName}>Create "{name}"</span>
            </button>
          )}
          {shownTags.map((tag) => (
            <button
              key={tag.id}
              type="button"
              className={styles.tagPickerItem}
              onClick={() => toggleTag(tag.id)}
              aria-pressed={selectedSet.has(tag.id)}>
              <span className={styles.tagDot} style={{ backgroundColor: tag.color || '#6366f1' }} aria-hidden="true" />
              <span className={styles.tagPickerName}>{tag.name}</span>
              {selectedSet.has(tag.id) && <CheckIcon size={14} weight="bold" className={styles.tagPickerCheck} />}
            </button>
          ))}
        </div>
      </Popover>
    </>
  );
}

function DetailRow({ label, children, mono = false }: { label: string; children: ReactNode; mono?: boolean }) {
  return (
    <div className={styles.fact}>
      <dt>{label}</dt>
      <dd className={`${styles.factWrap} ${mono ? styles.monoValue : ''}`}>{children}</dd>
    </div>
  );
}

function confidencePercent(value?: number | null) {
  if (value === null || value === undefined) return null;
  return Math.max(0, Math.min(value <= 1 ? value * 100 : value, 100));
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
  const percent = confidencePercent(confidence);
  const tone = percent === null ? '' : percent >= 80 ? styles.confidenceHigh : percent >= 50 ? styles.confidenceMid : styles.confidenceLow;
  return (
    <div className={`${styles.predictionMetric} ${tone}`}>
      <span className={styles.detailLabel}>{label}</span>
      <strong className={styles.predictionValue}>{value || '-'}</strong>
      <span className={styles.predictionConfidence}>{formatConfidence(confidence)}</span>
      <span className={styles.confidenceMeter} aria-hidden="true">
        <span style={{ width: `${percent ?? 0}%` }} />
      </span>
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
        <PredictionMetric
          label="Account"
          value={prediction.extractedAccount}
          confidence={prediction.accountConfidence}
        />
        <PredictionMetric
          label="Payee"
          value={predictedPayee?.name || prediction.extractedPayee}
          confidence={prediction.payeeConfidence}
        />
        <PredictionMetric label="Category" value={predictedCategory?.name} confidence={prediction.categoryConfidence} />
      </div>
      <dl className={styles.facts}>
        <DetailRow label="Predicted amount">{formatOptionalAmount(prediction.amount)}</DetailRow>
        <DetailRow label="Source">{prediction.source || '-'}</DetailRow>
        <DetailRow label="User corrected">{prediction.hasUserCorrected ? 'Yes' : 'No'}</DetailRow>
      </dl>
      {prediction.llmReasoning && <pre className={styles.rawTextBlock}>{prediction.llmReasoning}</pre>}
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
      <dl className={styles.facts}>
        <DetailRow label="Predicted amount">{formatOptionalAmount(prediction.amount)}</DetailRow>
        <DetailRow label="User corrected">{prediction.hasUserCorrected ? 'Yes' : 'No'}</DetailRow>
        <DetailRow label="Corrected account">{prediction.userCorrectedAccount || '-'}</DetailRow>
        <DetailRow label="Corrected payee">{prediction.userCorrectedPayee || '-'}</DetailRow>
        <DetailRow label="Corrected category">{prediction.userCorrectedCategory || '-'}</DetailRow>
      </dl>
      {prediction.emailText && <pre className={styles.rawTextBlock}>{prediction.emailText}</pre>}
    </div>
  );
}

function AdvancedDetails({
  txn,
  predictionDetails,
  isPredictionLoading,
  predictionError,
  sources,
  isSourcesLoading,
  sourcesError,
}: {
  txn: Transaction;
  predictionDetails: TransactionPredictionDetails | null;
  isPredictionLoading: boolean;
  predictionError: string | null;
  sources: TransactionSource[];
  isSourcesLoading: boolean;
  sourcesError: string | null;
}) {
  return (
    <details className={styles.advancedSection}>
      <summary className={styles.advancedSummary}>
        <CaretRightIcon size={13} weight="bold" className={styles.advancedCaret} />
        <span>Prediction &amp; source</span>
      </summary>

      <div className={styles.advancedContent}>
        <section className={styles.advancedGroup}>
          <span className={styles.sectionTitle}>Sources</span>
          <TransactionSources sources={sources} isLoading={isSourcesLoading} error={sourcesError} />
        </section>

        <section className={styles.advancedGroup}>
          <span className={styles.sectionTitle}>Prediction</span>
          <PredictionDetails details={predictionDetails} isLoading={isPredictionLoading} error={predictionError} />
        </section>

        {txn.summary && (
          <section className={styles.advancedGroup}>
            <span className={styles.sectionTitle}>Summary</span>
            <pre className={styles.rawTextBlock}>{txn?.summary ?? '-'}</pre>
          </section>
        )}
      </div>
    </details>
  );
}

function TransactionView({
  txn,
  isInflow,
  onChange,
  onSelectChange,
  onStatusChange,
}: {
  txn: Transaction;
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
  const { sources, isLoading: isSourcesLoading, error: sourcesError } = useTransactionSources(txn.id);
  // the list already carries it; the sources cover a row opened before it did
  const time = formatSourceTime(txn.occurredAt) || transactionTime(sources);

  const selectedTags = useMemo(
    () => allTags.filter((tag) => (txn.tagIds ?? []).includes(tag.id)),
    [allTags, txn.tagIds],
  );

  const account = allAccounts.find((acc) => acc.id === txn.accountId);

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
      <section className={styles.hero}>
        <AmountHero txn={txn} isInflow={isInflow} onChange={onChange} status={txn.status} />
        {txn.status === TransactionStatus.UNAPPROVED && <ReviewActions onStatusChange={onStatusChange} />}
      </section>

      <dl className={styles.facts}>
        <div className={styles.fact}>
          <dt>Payee</dt>
          <dd className={styles.factEditable}>
            <PayeeDropdown value={txn.payeeName} onClick={onSelectChange('payeeId', 'payeeName')} variant="ghost" />
          </dd>
        </div>
        <div className={styles.fact}>
          <dt>Category</dt>
          <dd className={styles.factControl}>
            <CategoryDropdown value={txn.categoryName ?? ''} onClick={onSelectChange('categoryId', 'categoryName')} />
          </dd>
        </div>
        <div className={styles.fact}>
          <dt>Date</dt>
          <dd className={styles.factEditable}>
            {time && (
              <span className={styles.timeHint} title="Time from the bank's message">
                {time}
              </span>
            )}
            <DateDropdown value={txn.date} onClick={onSelectChange('date', 'date')} variant="ghost" />
          </dd>
        </div>
        <div className={styles.fact}>
          <dt>Account</dt>
          <dd className={styles.factEditable}>
            <BankLogo name={txn.accountName} bank={account?.bank} type={account?.type} size="sm" />
            <AccountDropdown value={txn.accountName} onClick={onSelectChange('accountId', 'accountName')} variant="ghost" />
          </dd>
        </div>
        {hasLinkedTransfer && (
          <>
            <div className={styles.fact}>
              <dt>Transfer {isInflow ? 'from' : 'to'}</dt>
              <dd>{linkedAccount ? linkedAccount.name : txn.transferAccountId || '-'}</dd>
            </div>
            <div className={styles.fact}>
              <dt>Linked</dt>
              <dd className={styles.factWrap}>{linkedTransactionSummary}</dd>
            </div>
          </>
        )}
      </dl>

      <section className={styles.section}>
        <div className={styles.sectionHeader}>
          <span className={styles.sectionTitle}>Tags</span>
          <DetailTagPicker
            allTags={allTags}
            selectedTagIds={txn.tagIds ?? []}
            onChange={(tagIds) => onChange('tagIds', tagIds)}
          />
        </div>
        <TagList tags={selectedTags} />
      </section>

      <section className={styles.section}>
        <label htmlFor={`transaction-note-${txn.id}`} className={styles.sectionTitle}>
          Notes
        </label>
        <textarea
          id={`transaction-note-${txn.id}`}
          className={styles.notesInput}
          placeholder="Anything you'd like to remember about this?"
          value={txn.note ?? ''}
          onChange={(event) => onChange('note', event.target.value)}
        />
      </section>

      <TransactionDocumentsSection txn={txn} />

      <TransactionLocationSection txn={txn} />

      <AdvancedDetails
        txn={txn}
        predictionDetails={predictionDetails}
        isPredictionLoading={isPredictionLoading}
        predictionError={predictionError}
        sources={sources}
        isSourcesLoading={isSourcesLoading}
        sourcesError={sourcesError}
      />
    </div>
  );
}

const statusLabels: Record<string, string> = {
  [TransactionStatus.MANUAL]: 'Manual',
  [TransactionStatus.APPROVED]: 'Approved',
  [TransactionStatus.UNAPPROVED]: 'Needs review',
  [TransactionStatus.REJECTED]: 'Rejected',
};

function StatusBadge({ status }: { status: Transaction['status'] }) {
  if (!status) return null;
  return <span className={`${styles.statusPill} ${styles[`status${status}`]}`}>{statusLabels[status] ?? status}</span>;
}

function ReviewActions({ onStatusChange }: { onStatusChange: (status: TransactionStatusAction) => void }) {
  return (
    <div className={styles.reviewActions}>
      <button
        type="button"
        className={styles.approveBtn}
        onClick={() => onStatusChange(TransactionStatus.APPROVED)}
        aria-label="Approve transaction">
        <CheckIcon size={15} weight="bold" />
        Approve
      </button>
      <button
        type="button"
        className={styles.rejectBtn}
        onClick={() => onStatusChange(TransactionStatus.REJECTED)}
        aria-label="Reject transaction">
        <XIcon size={15} weight="bold" />
        Reject
      </button>
    </div>
  );
}

function PanelAccountLogo({ txn }: { txn: Transaction }) {
  const account = useAppSelector((state) => state.accounts.allAccounts.find((acc) => acc.id === txn.accountId));
  return <BankLogo name={txn.accountName} bank={account?.bank} type={account?.type} size="md" />;
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

  const { isInflow } = getDisplayAmount(selectedTxn);

  if (isAddingNew) {
    return (
      <PanelFrame
        leading={
          <span className={styles.headerIcon}>
            <ReceiptIcon size={19} weight="fill" />
          </span>
        }
        eyebrow="New transaction"
        title={selectedTxn.payeeName || 'Add a transaction'}
        isClosing={isClosing}
        onClose={onClose}
        onAnimationEnd={onAnimationEnd}
        footer={
          <>
            <button type="button" className={styles.cancelBtn} onClick={onClose}>
              Cancel
            </button>
            <button type="button" className={styles.saveBtn} onClick={onSave}>
              Save
            </button>
          </>
        }>
        <AddTransactionForm txn={selectedTxn} isInflow={isInflow} onChange={onChange} onSelectChange={onSelectChange} />
      </PanelFrame>
    );
  }

  return (
    <PanelFrame
      leading={<PanelAccountLogo txn={selectedTxn} />}
      eyebrow={selectedTxn.accountName || 'Transaction'}
      title={selectedTxn.payeeName || 'Transaction'}
      isClosing={isClosing}
      onClose={onClose}
      onAnimationEnd={onAnimationEnd}
      footer={
        <>
          <button type="button" className={styles.deleteBtn} onClick={onDelete}>
            <Trash2 size={16} />
            Delete
          </button>
          <button type="button" className={styles.saveBtn} onClick={onSave}>
            Save
          </button>
        </>
      }>
      <TransactionView
        txn={selectedTxn}
        isInflow={isInflow}
        onChange={onChange}
        onSelectChange={onSelectChange}
        onStatusChange={onStatusChange}
      />
    </PanelFrame>
  );
}
