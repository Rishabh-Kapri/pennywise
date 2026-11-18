import { formatTokens, formatUsd } from '@/utils/format';
import { stepUsage, totalUsage } from '../jobUsage';
import type { JobSummary } from '../types/job.types';
import styles from './Jobs.module.css';

/** "$0.0021", or "not reported" for a provider that doesn't say what it charged. */
function costText(priced: boolean, micros: number): string {
  return priced ? formatUsd(micros) : 'cost not reported';
}

/**
 * What a message's model calls cost, step by step, like a till receipt:
 * each step that asked a model, the model, its tokens and its cost, then the
 * total. Renders nothing when no step called a model.
 */
export function UsageReceipt({ jobs }: { jobs: JobSummary[] }) {
  const total = totalUsage(jobs);
  if (!total) return null;
  const steps = stepUsage(jobs);

  return (
    <div className={styles.usage}>
      <h4 className={styles.detailHeading}>AI cost</h4>
      <dl className={styles.usageLines}>
        {steps.map((step) => (
          <div key={step.jobId} className={styles.usageLine}>
            <dt>
              {step.label}
              {step.model && <span className={styles.usageModel}>{step.model}</span>}
            </dt>
            <dd>
              <span className={styles.usageTokens}>
                {formatTokens(step.inputTokens)} in · {formatTokens(step.outputTokens)} out
                {step.calls > 1 && ` · ${step.calls} calls`}
              </span>
              <span className={styles.usageCost}>{costText(step.priced, step.costMicros)}</span>
            </dd>
          </div>
        ))}
        {steps.length > 1 && (
          <div className={`${styles.usageLine} ${styles.usageTotal}`}>
            <dt>Total</dt>
            <dd>
              <span className={styles.usageTokens}>
                {formatTokens(total.inputTokens + total.outputTokens)} tokens · {total.calls} calls
              </span>
              <span className={styles.usageCost}>{costText(total.priced, total.costMicros)}</span>
            </dd>
          </div>
        )}
      </dl>
    </div>
  );
}

/** The row's cost, small, beside its status. Nothing when no step was priced. */
export function RowCost({ jobs }: { jobs: JobSummary[] }) {
  const total = totalUsage(jobs);
  if (!total?.priced) return null;
  return (
    <span className={styles.rowCost} title={`AI cost of this message: ${total.calls} model calls`}>
      {formatUsd(total.costMicros)}
    </span>
  );
}
