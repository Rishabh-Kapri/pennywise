import { useEffect, useState } from 'react';
import { CheckIcon, PencilSimpleIcon, RobotIcon, XIcon } from '@phosphor-icons/react';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { LoadingState } from '@/utils';
import { getCurrencyLocaleString, getLocaleDate } from '@/utils/date.utils';
import { PayeeDropdown } from '@/features/transactions/components/popovers/PayeePopover';
import { CategoryDropdown } from '@/features/transactions/components/popovers/CategoryPopover';
import {
  fetchReviewQueue,
  reviewPrediction,
  selectReviewError,
  selectReviewIncludeReviewed,
  selectReviewItems,
  selectReviewLoading,
  selectReviewSubmittingId,
  setIncludeReviewed,
} from '../store/predictionReviewSlice';
import {
  confidenceLevel,
  formatConfidence,
  type PredictionReviewItem,
} from '../types/predictionReview.types';
import styles from './PredictionReview.module.css';

function ConfidenceChip({ label, value }: { label: string; value?: number | null }) {
  const level = confidenceLevel(value);
  return (
    <span className={`${styles.chip} ${styles[`chip_${level}`]}`}>
      {label} {formatConfidence(value)}
    </span>
  );
}

function ValueLine({ label, value }: { label: string; value?: string | null }) {
  return (
    <div className={styles.valueLine}>
      <span className={styles.valueLabel}>{label}</span>
      <span className={value ? styles.value : styles.valueEmpty}>{value || '—'}</span>
    </div>
  );
}

function ReviewCard({ item }: { item: PredictionReviewItem }) {
  const dispatch = useAppDispatch();
  const submittingId = useAppSelector(selectReviewSubmittingId);
  const [isCorrecting, setIsCorrecting] = useState(false);
  const [payee, setPayee] = useState({ id: '', name: '' });
  const [category, setCategory] = useState({ id: '', name: '' });

  const isSubmitting = submittingId === item.id;

  const startCorrecting = () => {
    setPayee({ id: item.currentPayeeId ?? '', name: item.currentPayeeName ?? '' });
    setCategory({ id: item.currentCategoryId ?? '', name: item.currentCategoryName ?? '' });
    setIsCorrecting(true);
  };

  const submitCorrection = async () => {
    // only send what actually changed; the API rejects an empty correction
    const request = {
      action: 'correct' as const,
      payeeId: payee.id && payee.id !== item.currentPayeeId ? payee.id : undefined,
      categoryId: category.id && category.id !== item.currentCategoryId ? category.id : undefined,
    };
    if (!request.payeeId && !request.categoryId) {
      setIsCorrecting(false);
      return;
    }
    await dispatch(reviewPrediction({ id: item.id, request }));
    setIsCorrecting(false);
  };

  return (
    <li className={item.reviewedAt ? styles.cardReviewed : styles.card}>
      <div className={styles.cardHeader}>
        <div className={styles.txnSummary}>
          <span className={styles.txnDate}>
            {getLocaleDate(item.transactionDate, { month: 'short', day: 'numeric', year: 'numeric' })}
          </span>
          <span className={styles.txnAmount}>
            {item.transactionAmount > 0 ? '+' : '−'}
            {getCurrencyLocaleString(Math.abs(item.transactionAmount))}
          </span>
          {item.accountName && <span className={styles.txnAccount}>{item.accountName}</span>}
          {item.reviewedAt && <span className={styles.reviewedBadge}>Reviewed</span>}
        </div>
        <div className={styles.chips}>
          <ConfidenceChip label="payee" value={item.payeeConfidence} />
          <ConfidenceChip label="category" value={item.categoryConfidence} />
          <span className={styles.sourceChip}>{item.source}</span>
        </div>
      </div>

      <div className={styles.comparison}>
        <div className={styles.column}>
          <span className={styles.columnTitle}>
            <RobotIcon size={14} /> Predicted
          </span>
          <ValueLine label="Payee" value={item.predictedPayeeName ?? item.extractedPayee} />
          <ValueLine label="Category" value={item.predictedCategoryName} />
        </div>
        <div className={styles.column}>
          <span className={styles.columnTitle}>On transaction</span>
          <ValueLine label="Payee" value={item.currentPayeeName} />
          <ValueLine label="Category" value={item.currentCategoryName} />
        </div>
      </div>

      {item.transactionNote && <div className={styles.note}>{item.transactionNote}</div>}

      {isCorrecting ? (
        <div className={styles.correctRow}>
          <div className={styles.correctField}>
            <span className={styles.valueLabel}>Payee</span>
            <PayeeDropdown
              value={payee.name}
              variant="form"
              onClick={(id, name) => setPayee({ id, name })}
            />
          </div>
          <div className={styles.correctField}>
            <span className={styles.valueLabel}>Category</span>
            <CategoryDropdown
              value={category.name}
              variant="form"
              onClick={(id, name) => setCategory({ id, name })}
            />
          </div>
          <div className={styles.correctActions}>
            <button
              type="button"
              className={styles.primaryBtn}
              onClick={submitCorrection}
              disabled={isSubmitting}>
              <CheckIcon size={15} />
              Save
            </button>
            <button
              type="button"
              className={styles.iconBtn}
              onClick={() => setIsCorrecting(false)}
              aria-label="Cancel correction">
              <XIcon size={16} />
            </button>
          </div>
        </div>
      ) : (
        <div className={styles.actions}>
          <button
            type="button"
            className={styles.acceptBtn}
            onClick={() => dispatch(reviewPrediction({ id: item.id, request: { action: 'accept' } }))}
            disabled={isSubmitting}>
            <CheckIcon size={15} />
            Looks right
          </button>
          <button
            type="button"
            className={styles.correctBtn}
            onClick={startCorrecting}
            disabled={isSubmitting}>
            <PencilSimpleIcon size={15} />
            Correct
          </button>
        </div>
      )}
    </li>
  );
}

export default function PredictionReview() {
  const dispatch = useAppDispatch();
  const items = useAppSelector(selectReviewItems);
  const loading = useAppSelector(selectReviewLoading);
  const includeReviewed = useAppSelector(selectReviewIncludeReviewed);
  const error = useAppSelector(selectReviewError);

  useEffect(() => {
    if (loading === LoadingState.IDLE) {
      dispatch(fetchReviewQueue(includeReviewed));
    }
  }, [dispatch, loading, includeReviewed]);

  const pendingCount = items.filter((item) => !item.reviewedAt).length;

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <div>
          <h1>Prediction review</h1>
          <p className={styles.subtitle}>
            Confirm or correct how transactions from email were classified. Corrections train the
            payee rules.
          </p>
        </div>
        <label className={styles.toggle}>
          <input
            type="checkbox"
            checked={includeReviewed}
            onChange={(e) => dispatch(setIncludeReviewed(e.target.checked))}
          />
          Show reviewed
        </label>
      </div>

      {error && <div className={styles.error}>{error}</div>}

      {loading === LoadingState.PENDING && items.length === 0 ? (
        <div className={styles.emptyState}>Loading…</div>
      ) : items.length === 0 ? (
        <div className={styles.emptyState}>
          Nothing to review — every prediction has been triaged.
        </div>
      ) : (
        <>
          <div className={styles.queueMeta}>
            {includeReviewed
              ? `${items.length} prediction${items.length === 1 ? '' : 's'} · ${pendingCount} pending`
              : `${items.length} awaiting review · least confident first`}
          </div>
          <ul className={styles.list}>
            {items.map((item) => (
              <ReviewCard key={item.id} item={item} />
            ))}
          </ul>
        </>
      )}
    </div>
  );
}
