import type { ReactNode } from 'react';
import { getCurrencyLocaleString } from '@/utils/date.utils';
import type { TransactionSource, TransactionSourceKind } from '../../types/transaction.types';
import { formatSourceTime } from '../../utils/sourceTime';
import styles from './TransactionDetailPanel.module.css';

const KIND_LABELS: Record<TransactionSourceKind, string> = {
  EMAIL: 'Email',
  SMS: 'SMS',
  STATEMENT_LINE: 'Statement line',
};

function SourceFact({ label, children }: { label: string; children?: ReactNode }) {
  if (children === undefined || children === null || children === '') return null;
  return (
    <div className={styles.fact}>
      <dt>{label}</dt>
      <dd className={styles.factWrap}>{children}</dd>
    </div>
  );
}

function sourceHeading(source: TransactionSource): string {
  const time = formatSourceTime(source.occurredAt);
  const parts = [KIND_LABELS[source.kind] ?? source.kind, source.occurredOn, time].filter(Boolean);
  return parts.join(' · ');
}

function SourceCard({ source }: { source: TransactionSource }) {
  return (
    <div className={styles.sourceCard}>
      <span className={styles.sectionTitle}>{sourceHeading(source)}</span>
      <dl className={styles.facts}>
        <SourceFact label="Channel">{source.channel}</SourceFact>
        <SourceFact label="Merchant">{source.merchant}</SourceFact>
        <SourceFact label="UPI ID">{source.vpa}</SourceFact>
        <SourceFact label="Reference">{source.ref}</SourceFact>
        <SourceFact label="Account">{source.accountHint ? `••${source.accountHint}` : undefined}</SourceFact>
        <SourceFact label="Amount">
          {source.amount !== undefined
            ? `${source.direction === 'CREDIT' ? '+' : source.direction === 'DEBIT' ? '-' : ''}${getCurrencyLocaleString(source.amount)}`
            : undefined}
        </SourceFact>
      </dl>
      {/* bank text, shown as text and never as HTML */}
      <pre className={styles.rawTextBlock}>{source.rawText}</pre>
    </div>
  );
}

export function TransactionSources({
  sources,
  isLoading,
  error,
}: {
  sources: TransactionSource[];
  isLoading: boolean;
  error: string | null;
}) {
  if (isLoading) return <span className={styles.emptyValue}>Loading sources...</span>;
  if (error) return <span className={styles.errorText}>{error}</span>;
  if (sources.length === 0) return <span className={styles.emptyValue}>No bank message behind this transaction.</span>;

  return (
    <div className={styles.advancedGroup}>
      {sources.map((source) => (
        <SourceCard key={source.id} source={source} />
      ))}
    </div>
  );
}
