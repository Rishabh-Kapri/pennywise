import { Warning } from '@phosphor-icons/react';
import type { ReactNode } from 'react';
import { formatAgo } from '../jobTime';
import type { MessageReview } from '../types/job.types';
import styles from './Jobs.module.css';

const CHANNEL_LABELS: Record<string, string> = {
  UPI: 'UPI',
  CARD: 'Card',
  NEFT: 'NEFT',
  IMPS: 'IMPS',
  NACH: 'Auto-debit',
};

function formatAmount(amount: number, currency?: string): string {
  const value = Math.abs(amount);
  if (currency && /^[A-Z]{3}$/.test(currency)) {
    return new Intl.NumberFormat(undefined, { style: 'currency', currency }).format(value);
  }
  return value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

/** Go's zero time means no date was read. */
function formatDate(value?: string): string | undefined {
  if (!value || value.startsWith('0001')) return undefined;
  return new Date(`${value.slice(0, 10)}T00:00:00`).toLocaleDateString(undefined, { day: 'numeric', month: 'short' });
}

function Fact({ label, value, unsure }: { label: string; value?: string; unsure: boolean }) {
  if (!value && !unsure) return null;
  return (
    <div className={styles.reviewFact}>
      <dt>{label}</dt>
      <dd>
        {value || 'Not found'}
        {unsure && (
          <span className={styles.reviewUnsure}>
            <Warning size={13} weight="fill" aria-hidden />
            unsure
          </span>
        )}
      </dd>
    </div>
  );
}

interface MessageReviewPanelProps {
  reason: string;
  sender?: string;
  receivedAt?: string;
  review?: MessageReview;
  loading: boolean;
  /** The actions, shown under what they act on. */
  children: ReactNode;
}

/** A parked message: what it says and what was read from it, ready to decide on. */
export function MessageReviewPanel({ reason, sender, receivedAt, review, loading, children }: MessageReviewPanelProps) {
  const entities = review?.entities;
  const unsure = (field: string) => review?.unsure?.includes(field) ?? false;
  const amount = entities?.amount ? formatAmount(entities.amount, entities.currency) : undefined;

  return (
    <div className={styles.review}>
      <div>
        <p className={styles.reviewReason}>{reason}</p>
        {(sender || receivedAt) && (
          <p className={styles.muted}>
            {sender && `From ${sender}`}
            {sender && receivedAt && ' · '}
            {receivedAt && formatAgo(receivedAt)}
          </p>
        )}
      </div>

      {loading ? (
        <p className={styles.muted}>Loading the message…</p>
      ) : (
        review?.text && <div className={styles.reviewText}>{review.text}</div>
      )}

      {entities && (
        <dl className={styles.reviewFacts}>
          <Fact label="Amount" value={amount} unsure={unsure('amount')} />
          <Fact label="Account" value={entities.account} unsure={unsure('account')} />
          <Fact label="Date" value={formatDate(entities.date)} unsure={unsure('date')} />
          <Fact label="Payee" value={entities.merchant} unsure={unsure('merchant')} />
          <Fact label="Paid by" value={entities.channel && (CHANNEL_LABELS[entities.channel] ?? entities.channel)} unsure={false} />
          <Fact label="UPI ID" value={entities.vpa} unsure={false} />
          <Fact label="Ref" value={entities.ref} unsure={false} />
        </dl>
      )}

      {children}
    </div>
  );
}
