import { useEffect, useMemo, useState } from 'react';
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { Coins } from '@phosphor-icons/react';
import { Segmented } from '@/components/common/Card/Segmented';
import { apiClient, errorMessage } from '@/utils';
import { formatCount, formatTokens, formatUsd } from '@/utils/format';
import settingsStyles from './Settings.module.css';
import aiStyles from './AISettings.module.css';
import styles from './AIUsageCard.module.css';

interface UsageTotals {
  calls: number;
  failedCalls: number;
  /** Calls whose provider said nothing about cost; not in costMicros. */
  unpricedCalls: number;
  inputTokens: number;
  outputTokens: number;
  cacheReadTokens: number;
  costMicros: number;
}

interface UsageDay extends UsageTotals {
  date: string;
}

interface UsageGroup extends UsageTotals {
  key: string;
  provider?: string;
}

interface UsageSummary {
  since: string;
  totals: UsageTotals;
  days: UsageDay[];
  byPurpose: UsageGroup[];
  byModel: UsageGroup[];
}

const RANGES = [
  { value: '7', label: '7D' },
  { value: '30', label: '30D' },
  { value: '90', label: '90D' },
] as const;
type Range = (typeof RANGES)[number]['value'];

/** What each llm_usage purpose is, in words. Unknown ones show as they are. */
const PURPOSES: Record<string, string> = {
  agent_turn: 'Chat answers',
  title: 'Chat titles',
  memory_observe: 'Chat memory',
  pipeline_check: 'Message check',
  pipeline_extract: 'Message extract',
  pipeline_verify: 'Message verify',
  pipeline_predict: 'Message predict',
  statement_extract: 'Statement reading',
  statement_fallback: 'Statement re-read',
  embedding: 'Embeddings',
  decision: 'Decisions',
  model_test: 'Model tests',
  category_describe: 'Category descriptions',
  parse_extract: 'Email extract (old flow)',
  predict_summarize: 'Email summary (old flow)',
  predict_llm_fallback: 'Email predict (old flow)',
  other: 'Other',
};

const CHART_HEIGHT = 160;
const TOP_GROUPS = 6;

function purposeLabel(key: string): string {
  return PURPOSES[key] ?? key.replace(/_/g, ' ');
}

function isoDay(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

function dayLabel(iso: string): string {
  const [y, m, d] = iso.split('-').map(Number);
  return new Date(y, m - 1, d).toLocaleDateString('en-IN', { day: 'numeric', month: 'short' });
}

/** Every day of the range, with zeros for days without calls, so the bars keep time. */
function fillDays(days: UsageDay[], range: number): { date: string; cost: number; calls: number }[] {
  const byDate = new Map(days.map((d) => [d.date, d]));
  const out = [];
  const today = new Date();
  for (let i = range - 1; i >= 0; i--) {
    const d = new Date(today.getFullYear(), today.getMonth(), today.getDate() - i);
    const key = isoDay(d);
    const day = byDate.get(key);
    out.push({ date: key, cost: day?.costMicros ?? 0, calls: day?.calls ?? 0 });
  }
  return out;
}

interface TooltipProps {
  active?: boolean;
  payload?: { payload: { date: string; cost: number; calls: number } }[];
}

function DayTooltip({ active, payload }: TooltipProps) {
  const day = active ? payload?.[0]?.payload : undefined;
  if (!day) return null;
  return (
    <div className={styles.tooltip}>
      <span className={styles.tooltipDate}>{dayLabel(day.date)}</span>
      <span className={styles.tooltipValue}>{formatUsd(day.cost)}</span>
      <span className={styles.tooltipMeta}>{formatCount(day.calls)} calls</span>
    </div>
  );
}

/** A ranked list: name, a bar for its share of the cost, the cost. */
function GroupList({ title, groups, total, label }: {
  title: string;
  groups: UsageGroup[];
  total: number;
  label: (g: UsageGroup) => string;
}) {
  const shown = groups.slice(0, TOP_GROUPS);
  const rest = groups.slice(TOP_GROUPS);
  const restCost = rest.reduce((sum, g) => sum + g.costMicros, 0);
  return (
    <div className={styles.group}>
      <h4 className={styles.groupTitle}>{title}</h4>
      <ul className={styles.groupList}>
        {shown.map((g) => (
          <li key={`${g.provider ?? ''}/${g.key}`} className={styles.groupRow}>
            <span className={styles.groupName} title={label(g)}>{label(g)}</span>
            <span className={styles.groupBar} aria-hidden="true">
              <span style={{ width: `${total > 0 ? Math.max((g.costMicros / total) * 100, g.costMicros > 0 ? 2 : 0) : 0}%` }} />
            </span>
            <span className={styles.groupCost}>{formatUsd(g.costMicros)}</span>
            <span className={styles.groupCalls}>{formatCount(g.calls)} calls</span>
          </li>
        ))}
        {rest.length > 0 && (
          <li className={`${styles.groupRow} ${styles.groupRest}`}>
            <span className={styles.groupName}>{rest.length} more</span>
            <span className={styles.groupBar} aria-hidden="true" />
            <span className={styles.groupCost}>{formatUsd(restCost)}</span>
            <span className={styles.groupCalls} />
          </li>
        )}
      </ul>
    </div>
  );
}

/**
 * What the budget's AI cost: every model call (chat, the message pipeline,
 * statements, embeddings) from llm_usage, as the providers reported it.
 * Owner only, like the keys it ran on.
 */
export function AIUsageCard() {
  const [range, setRange] = useState<Range>('30');
  const [summary, setSummary] = useState<UsageSummary | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
    apiClient
      .get<UsageSummary>(`ai-usage?days=${range}&tz=${encodeURIComponent(tz)}`)
      .then((res) => {
        if (cancelled) return;
        setSummary(res);
        setError(null);
      })
      .catch((err) => !cancelled && setError(errorMessage(err, "Couldn't load AI usage")));
    return () => {
      cancelled = true;
    };
  }, [range]);

  const days = useMemo(() => (summary ? fillDays(summary.days, Number(range)) : []), [summary, range]);
  const totals = summary?.totals;
  const loading = !summary && !error;

  return (
    <section className={`${settingsStyles.card} ${aiStyles.card}`}>
      <header className={aiStyles.cardHeader}>
        <span className={aiStyles.cardIcon}>
          <Coins size={18} />
        </span>
        <div className={aiStyles.cardTitle}>
          <h3>AI usage & cost</h3>
          <p>Every model call this budget made, priced as your providers reported it.</p>
        </div>
        <div className={aiStyles.cardAside}>
          <Segmented label="Usage range" options={RANGES} value={range} onChange={setRange} />
        </div>
      </header>

      {error && <p className={styles.empty}>{error}</p>}
      {loading && <div className={styles.placeholder} aria-busy="true" />}

      {totals && totals.calls === 0 && (
        <p className={styles.empty}>No model calls in the last {range} days.</p>
      )}

      {totals && totals.calls > 0 && summary && (
        <>
          <div className={styles.hero}>
            <div>
              <span className={styles.heroValue}>{formatUsd(totals.costMicros)}</span>
              <span className={styles.heroLabel}>spent in the last {range} days</span>
            </div>
            <dl className={styles.stats}>
              <div>
                <dt>Calls</dt>
                <dd>{formatCount(totals.calls)}</dd>
              </div>
              <div>
                <dt>Tokens in / out</dt>
                <dd>
                  {formatTokens(totals.inputTokens)} / {formatTokens(totals.outputTokens)}
                </dd>
              </div>
              {totals.inputTokens > 0 && (
                <div>
                  <dt>From cache</dt>
                  <dd>{Math.round((totals.cacheReadTokens / totals.inputTokens) * 100)}%</dd>
                </div>
              )}
              {totals.failedCalls > 0 && (
                <div>
                  <dt>Failed</dt>
                  <dd>{formatCount(totals.failedCalls)}</dd>
                </div>
              )}
            </dl>
          </div>
          {totals.unpricedCalls > 0 && (
            <p className={styles.note}>
              {formatCount(totals.unpricedCalls)} of {formatCount(totals.calls)} calls had no cost reported (local
              models, or a provider that doesn't say), so the total may be low.
            </p>
          )}

          <div className={styles.chart}>
            <ResponsiveContainer width="100%" height={CHART_HEIGHT} initialDimension={{ width: 600, height: CHART_HEIGHT }}>
              <BarChart data={days} margin={{ top: 8, right: 4, bottom: 0, left: 4 }} barCategoryGap={2}>
                <CartesianGrid vertical={false} stroke="var(--color-border-muted)" />
                <XAxis
                  dataKey="date"
                  tickFormatter={dayLabel}
                  tickLine={false}
                  axisLine={false}
                  minTickGap={24}
                  tick={{ fill: 'var(--color-text-secondary)', fontSize: 11 }}
                />
                <YAxis
                  width={56}
                  tickFormatter={(v: number) => formatUsd(v)}
                  tickLine={false}
                  axisLine={false}
                  allowDecimals
                  tick={{ fill: 'var(--color-text-secondary)', fontSize: 11 }}
                />
                <Tooltip content={<DayTooltip />} cursor={{ fill: 'var(--color-border-muted)', opacity: 0.5 }} />
                <Bar dataKey="cost" fill="var(--accent)" radius={[4, 4, 0, 0]} maxBarSize={24} minPointSize={0} />
              </BarChart>
            </ResponsiveContainer>
          </div>
          <details className={styles.table}>
            <summary>Daily cost as a table</summary>
            <table>
              <thead>
                <tr>
                  <th scope="col">Day</th>
                  <th scope="col">Calls</th>
                  <th scope="col">Cost</th>
                </tr>
              </thead>
              <tbody>
                {summary.days.map((d) => (
                  <tr key={d.date}>
                    <td>{dayLabel(d.date)}</td>
                    <td>{formatCount(d.calls)}</td>
                    <td>{formatUsd(d.costMicros)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </details>

          <div className={styles.groups}>
            <GroupList
              title="By what it was for"
              groups={summary.byPurpose}
              total={totals.costMicros}
              label={(g) => purposeLabel(g.key)}
            />
            <GroupList
              title="By model"
              groups={summary.byModel}
              total={totals.costMicros}
              label={(g) => g.key}
            />
          </div>
        </>
      )}
    </section>
  );
}

export default AIUsageCard;
