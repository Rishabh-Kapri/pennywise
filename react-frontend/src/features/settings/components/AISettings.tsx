import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  CaretDown,
  CheckCircle,
  Copy,
  Key,
  MagnifyingGlass,
  PencilSimpleLine,
  Sparkle,
  WarningCircle,
} from '@phosphor-icons/react';
import { useAppSelector } from '@/app/hooks';
import { selectIsDemoUser } from '@/features/auth/store';
import { AIProvidersPanel } from '@/features/onboarding/components/AIProvidersPanel';
import type { AIProvider } from '@/features/onboarding/types';
import type { CipherPrediction } from '@/features/transactions/types/transaction.types';
import { apiClient, errorMessage, toast } from '@/utils';
import settingsStyles from './Settings.module.css';
import styles from './AISettings.module.css';

/* ────────────────────────────────────────────────────────────── */
/*  Types & constants                                            */
/* ────────────────────────────────────────────────────────────── */

interface APIKey {
  id: string;
  name: string;
  description?: string;
  keyId?: string;
  maskedKey?: string;
  scopes?: string[];
  rateLimit?: number;
  isActive?: boolean;
  expiresAt?: string;
  lastUsedAt?: string;
  createdAt?: string;
}

type PredictionSource = 'RULE' | 'VECTOR' | 'LLM' | 'MANUAL' | 'UNCATEGORIZED';

/**
 * Fixed order and meaning per source. Colour follows the source (CSS
 * `data-source`), never its rank, so filtering never repaints a segment.
 */
const SOURCES: { id: PredictionSource; label: string; hint: string }[] = [
  { id: 'RULE', label: 'Payee rule', hint: 'Matched one of your payee rules' },
  { id: 'VECTOR', label: 'Similar transaction', hint: 'Matched a similar past transaction' },
  { id: 'LLM', label: 'AI model', hint: 'Decided by your AI provider' },
  { id: 'MANUAL', label: 'Manual', hint: 'Entered by hand' },
  { id: 'UNCATEGORIZED', label: 'Uncategorized', hint: 'No prediction was made' },
];

const SOURCE_LABEL = Object.fromEntries(SOURCES.map((s) => [s.id, s.label])) as Record<PredictionSource, string>;

const PAGE_SIZE = 25;
const DAY_MS = 24 * 60 * 60 * 1000;

/* ────────────────────────────────────────────────────────────── */
/*  Helpers                                                      */
/* ────────────────────────────────────────────────────────────── */

function sourceOf(prediction: CipherPrediction): PredictionSource {
  const source = (prediction.source ?? 'UNCATEGORIZED').toUpperCase();
  return SOURCES.some((s) => s.id === source) ? (source as PredictionSource) : 'UNCATEGORIZED';
}

function formatPercent(value?: number | null): string {
  if (value === null || value === undefined) return '—';
  const percent = value <= 1 ? value * 100 : value;
  return `${percent.toFixed(percent >= 10 ? 0 : 1)}%`;
}

function formatDate(dateStr?: string): string {
  if (!dateStr) return '—';
  return new Date(dateStr).toLocaleDateString('en-IN', { day: 'numeric', month: 'short', year: 'numeric' });
}

const formatCurrency = (amount: number): string =>
  new Intl.NumberFormat('en-IN', {
    style: 'currency',
    currency: 'INR',
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(amount);

const formatCount = (n: number) => new Intl.NumberFormat('en-IN').format(n);

function payeeChanged(p: CipherPrediction) {
  return Boolean(p.hasUserCorrected && p.actualPayeeId && p.actualPayeeId !== p.predictedPayeeId);
}

function categoryChanged(p: CipherPrediction) {
  return Boolean(p.hasUserCorrected && p.actualCategoryId && p.actualCategoryId !== p.predictedCategoryId);
}

function averageConfidence(predictions: CipherPrediction[]): number | null {
  const values = predictions.flatMap((p) =>
    [p.payeeConfidence, p.categoryConfidence].filter((v): v is number => v != null),
  );
  return values.length ? values.reduce((sum, v) => sum + v, 0) / values.length : null;
}

/* ────────────────────────────────────────────────────────────── */
/*  Shared bits                                                  */
/* ────────────────────────────────────────────────────────────── */

function Card({
  icon,
  title,
  description,
  aside,
  children,
}: {
  icon: React.ReactNode;
  title: string;
  description?: string;
  aside?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <section className={`${settingsStyles.card} ${styles.card}`}>
      <header className={styles.cardHeader}>
        <span className={styles.cardIcon}>{icon}</span>
        <div className={styles.cardTitle}>
          <h3>{title}</h3>
          {description && <p>{description}</p>}
        </div>
        {aside && <div className={styles.cardAside}>{aside}</div>}
      </header>
      {children}
    </section>
  );
}

function SourceTag({ source }: { source: PredictionSource }) {
  return (
    <span className={styles.sourceTag}>
      <span className={styles.sourceDot} data-source={source} aria-hidden="true" />
      {SOURCE_LABEL[source]}
    </span>
  );
}

/* ────────────────────────────────────────────────────────────── */
/*  AI providers                                                 */
/* ────────────────────────────────────────────────────────────── */

function ProvidersCard() {
  const isDemoUser = useAppSelector(selectIsDemoUser);
  const [providers, setProviders] = useState<AIProvider[] | null>(null);
  const handleChange = useCallback((list: AIProvider[]) => setProviders(list), []);
  const enabled = providers?.filter((p) => p.enabled).length ?? 0;

  const status =
    providers === null ? null : enabled > 0 ? (
      <span className={`${styles.statusPill} ${styles.statusGood}`}>
        <CheckCircle size={14} weight="fill" aria-hidden="true" />
        {enabled} active
      </span>
    ) : (
      <span className={`${styles.statusPill} ${styles.statusWarning}`}>
        <WarningCircle size={14} weight="fill" aria-hidden="true" />
        Not set up
      </span>
    );

  return (
    <Card
      icon={<Sparkle size={18} weight="duotone" />}
      title="AI providers"
      description="Email parsing, statement import and the assistant run on your own keys, tried top to bottom. Keys are encrypted and never shown again."
      aside={status}>
      {providers !== null && enabled === 0 && (
        <p className={styles.notice}>
          Without a provider, bank emails can't become transactions and the assistant is off.
        </p>
      )}
      <AIProvidersPanel onChange={handleChange} readOnly={isDemoUser} />
      <p className={styles.footnote}>
        Matching against similar past transactions uses Pennywise's own embedding model, not your keys.
      </p>
    </Card>
  );
}

/* ────────────────────────────────────────────────────────────── */
/*  Performance                                                  */
/* ────────────────────────────────────────────────────────────── */

function StatTile({ label, value, sub }: { label: string; value: string; sub?: string }) {
  return (
    <div className={styles.statTile}>
      <span className={styles.statLabel}>{label}</span>
      <span className={styles.statValue}>{value}</span>
      {sub && <span className={styles.statSub}>{sub}</span>}
    </div>
  );
}

/** Part-to-whole bar of where predictions came from, with a labelled legend. */
function SourceBreakdown({ predictions }: { predictions: CipherPrediction[] }) {
  const [hovered, setHovered] = useState<PredictionSource | null>(null);
  const total = predictions.length;
  const counts = useMemo(() => {
    const c = Object.fromEntries(SOURCES.map((s) => [s.id, 0])) as Record<PredictionSource, number>;
    for (const p of predictions) c[sourceOf(p)]++;
    return c;
  }, [predictions]);
  const present = SOURCES.filter((s) => counts[s.id] > 0);

  if (total === 0) return null;

  return (
    <div className={styles.breakdown}>
      <div className={styles.breakdownHeader}>
        <span className={styles.subheading}>Where predictions come from</span>
      </div>
      <div className={styles.bar} role="img" aria-label="Predictions by source, see the legend for values">
        {present.map((source) => {
          const share = counts[source.id] / total;
          return (
            <div
              key={source.id}
              className={`${styles.barSegment} ${hovered && hovered !== source.id ? styles.barSegmentDim : ''}`}
              data-source={source.id}
              style={{ flexGrow: share }}
              onMouseEnter={() => setHovered(source.id)}
              onMouseLeave={() => setHovered(null)}>
              {hovered === source.id && (
                <span className={styles.barTooltip} role="tooltip">
                  <strong>{source.label}</strong>
                  {formatCount(counts[source.id])} · {formatPercent(share)}
                  <small>{source.hint}</small>
                </span>
              )}
            </div>
          );
        })}
      </div>
      <ul className={styles.legend}>
        {present.map((source) => (
          <li
            key={source.id}
            className={hovered && hovered !== source.id ? styles.legendDim : undefined}
            onMouseEnter={() => setHovered(source.id)}
            onMouseLeave={() => setHovered(null)}>
            <span className={styles.sourceDot} data-source={source.id} aria-hidden="true" />
            <span className={styles.legendLabel}>{source.label}</span>
            <span className={styles.legendValue}>{formatCount(counts[source.id])}</span>
            <span className={styles.legendShare}>{formatPercent(counts[source.id] / total)}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}

function PerformanceCard({ predictions }: { predictions: CipherPrediction[] }) {
  const total = predictions.length;
  const payeeFixes = predictions.filter(payeeChanged).length;
  const categoryFixes = predictions.filter(categoryChanged).length;
  const changed = predictions.filter((p) => payeeChanged(p) || categoryChanged(p)).length;
  const avgConfidence = averageConfidence(predictions);
  const recent = predictions.filter(
    (p) => p.createdAt && Date.now() - new Date(p.createdAt).getTime() <= 30 * DAY_MS,
  ).length;

  return (
    <Card
      icon={<CheckCircle size={18} weight="duotone" />}
      title="Prediction performance"
      description="How often Pennywise picks the right payee and category for imported transactions.">
      {total === 0 ? (
        <p className={styles.empty}>
          No predictions yet. They appear once bank emails start turning into transactions.
        </p>
      ) : (
        <>
          <div className={styles.statGrid}>
            <StatTile label="Predictions" value={formatCount(total)} sub={`${formatCount(recent)} in the last 30 days`} />
            <StatTile
              label="Kept as predicted"
              value={formatPercent((total - changed) / total)}
              sub={`${formatCount(total - changed)} of ${formatCount(total)}`}
            />
            <StatTile
              label="Corrected"
              value={formatCount(changed)}
              sub={`${formatCount(payeeFixes)} payee · ${formatCount(categoryFixes)} category`}
            />
            <StatTile label="Avg confidence" value={formatPercent(avgConfidence)} sub="Payee and category" />
          </div>
          <SourceBreakdown predictions={predictions} />
        </>
      )}
    </Card>
  );
}

/* ────────────────────────────────────────────────────────────── */
/*  History                                                      */
/* ────────────────────────────────────────────────────────────── */

type HistoryRow = CipherPrediction & {
  source: PredictionSource;
  payee?: string;
  category?: string;
  actualPayee?: string | null;
  actualCategory?: string | null;
  changed: boolean;
};

function HistoryDetails({ row }: { row: HistoryRow }) {
  return (
    <div className={styles.details}>
      {row.changed && (
        <div className={styles.detailBlock}>
          <h4>Your correction</h4>
          <dl className={styles.detailList}>
            {row.actualPayee && payeeChanged(row) && (
              <>
                <dt>Payee</dt>
                <dd>
                  <s>{row.payee ?? '—'}</s> → {row.actualPayee}
                </dd>
              </>
            )}
            {row.actualCategory && categoryChanged(row) && (
              <>
                <dt>Category</dt>
                <dd>
                  <s>{row.category ?? '—'}</s> → {row.actualCategory}
                </dd>
              </>
            )}
          </dl>
        </div>
      )}
      <div className={styles.detailBlock}>
        <h4>Read from the email</h4>
        <dl className={styles.detailList}>
          <dt>Account</dt>
          <dd>{row.extractedAccount || '—'}</dd>
          <dt>Payee</dt>
          <dd>{row.extractedPayee || '—'}</dd>
          <dt>Amount</dt>
          <dd>{row.amount != null ? formatCurrency(row.amount) : '—'}</dd>
          <dt>Confidence</dt>
          <dd>
            payee {formatPercent(row.payeeConfidence)} · category {formatPercent(row.categoryConfidence)}
          </dd>
        </dl>
      </div>
      {row.llmReasoning && (
        <div className={`${styles.detailBlock} ${styles.detailWide}`}>
          <h4>AI reasoning</h4>
          <p className={styles.reasoning}>{row.llmReasoning}</p>
        </div>
      )}
      {!!row.metadata && (
        <details className={`${styles.detailBlock} ${styles.detailWide}`}>
          <summary>Raw metadata</summary>
          <pre className={styles.metadata}>{JSON.stringify(row.metadata, null, 2)}</pre>
        </details>
      )}
    </div>
  );
}

function HistoryCard({ predictions }: { predictions: CipherPrediction[] }) {
  const { allPayees } = useAppSelector((state) => state.payees);
  const { allCategories } = useAppSelector((state) => state.categories);
  const [query, setQuery] = useState('');
  const [sourceFilter, setSourceFilter] = useState<PredictionSource | 'ALL'>('ALL');
  const [correctedOnly, setCorrectedOnly] = useState(false);
  const [limit, setLimit] = useState(PAGE_SIZE);
  const [expandedId, setExpandedId] = useState<string | null>(null);

  const rows = useMemo<HistoryRow[]>(() => {
    const payeeName = (id?: string | null) => (id ? allPayees.find((p) => p.id === id)?.name : undefined);
    const categoryName = (id?: string | null) => (id ? allCategories.find((c) => c.id === id)?.name : undefined);
    return predictions
      .map((p) => ({
        ...p,
        source: sourceOf(p),
        payee: payeeName(p.predictedPayeeId),
        category: categoryName(p.predictedCategoryId),
        actualPayee: payeeName(p.actualPayeeId) ?? null,
        actualCategory: categoryName(p.actualCategoryId) ?? null,
        changed: payeeChanged(p) || categoryChanged(p),
      }))
      .sort((a, b) => (b.createdAt ?? '').localeCompare(a.createdAt ?? ''));
  }, [predictions, allPayees, allCategories]);

  const sourceCounts = useMemo(() => {
    const c: Partial<Record<PredictionSource, number>> = {};
    for (const row of rows) c[row.source] = (c[row.source] ?? 0) + 1;
    return c;
  }, [rows]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return rows.filter(
      (row) =>
        (sourceFilter === 'ALL' || row.source === sourceFilter) &&
        (!correctedOnly || row.changed) &&
        (!q ||
          [row.payee, row.category, row.extractedPayee, row.extractedAccount, row.actualPayee, row.actualCategory]
            .filter(Boolean)
            .some((text) => text!.toLowerCase().includes(q))),
    );
  }, [rows, query, sourceFilter, correctedOnly]);

  // A new filter starts from the first page.
  useEffect(() => setLimit(PAGE_SIZE), [query, sourceFilter, correctedOnly]);

  const shown = filtered.slice(0, limit);

  return (
    <Card
      icon={<MagnifyingGlass size={18} weight="duotone" />}
      title="Prediction history"
      description="Every prediction, newest first. Open one to see what was read from the email and why."
      aside={<span className={styles.countPill}>{formatCount(rows.length)}</span>}>
      {rows.length === 0 ? (
        <p className={styles.empty}>No predictions yet.</p>
      ) : (
        <>
          <div className={styles.toolbar}>
            <label className={styles.searchField}>
              <MagnifyingGlass size={15} aria-hidden="true" />
              <input
                type="search"
                value={query}
                placeholder="Search payee, category or account"
                aria-label="Search predictions"
                onChange={(event) => setQuery(event.target.value)}
              />
            </label>
            <div className={styles.chips} role="group" aria-label="Filter by source">
              <button
                type="button"
                className={`${styles.chip} ${sourceFilter === 'ALL' ? styles.chipActive : ''}`}
                aria-pressed={sourceFilter === 'ALL'}
                onClick={() => setSourceFilter('ALL')}>
                All
              </button>
              {SOURCES.filter((s) => sourceCounts[s.id]).map((source) => (
                <button
                  key={source.id}
                  type="button"
                  className={`${styles.chip} ${sourceFilter === source.id ? styles.chipActive : ''}`}
                  aria-pressed={sourceFilter === source.id}
                  onClick={() => setSourceFilter(sourceFilter === source.id ? 'ALL' : source.id)}>
                  <span className={styles.sourceDot} data-source={source.id} aria-hidden="true" />
                  {source.label}
                  <span className={styles.chipCount}>{sourceCounts[source.id]}</span>
                </button>
              ))}
            </div>
            <label className={styles.toggle}>
              <input type="checkbox" checked={correctedOnly} onChange={(e) => setCorrectedOnly(e.target.checked)} />
              <span className={styles.toggleTrack} aria-hidden="true" />
              Corrected only
            </label>
          </div>

          {filtered.length === 0 ? (
            <p className={styles.empty}>Nothing matches these filters.</p>
          ) : (
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>Date</th>
                    <th>Source</th>
                    <th>Payee</th>
                    <th>Category</th>
                    <th className={styles.numeric}>Confidence</th>
                    <th>Result</th>
                    <th aria-label="Details" />
                  </tr>
                </thead>
                <tbody>
                  {shown.map((row) => {
                    const expanded = expandedId === row.id;
                    const toggle = () => setExpandedId(expanded ? null : row.id);
                    return (
                      <React.Fragment key={row.id}>
                        <tr className={`${styles.row} ${expanded ? styles.rowExpanded : ''}`} onClick={toggle}>
                          <td className={styles.muted}>{formatDate(row.createdAt)}</td>
                          <td>
                            <SourceTag source={row.source} />
                          </td>
                          <td className={styles.truncate}>
                            {row.payee ?? <span className={styles.muted}>—</span>}
                          </td>
                          <td className={styles.truncate}>
                            {row.category ?? <span className={styles.muted}>—</span>}
                          </td>
                          <td className={styles.numeric}>{formatPercent(row.payeeConfidence)}</td>
                          <td>
                            {row.changed ? (
                              <span className={`${styles.statusPill} ${styles.statusWarning}`}>
                                <PencilSimpleLine size={12} weight="bold" aria-hidden="true" />
                                Corrected
                              </span>
                            ) : (
                              <span className={`${styles.statusPill} ${styles.statusGood}`}>
                                <CheckCircle size={12} weight="fill" aria-hidden="true" />
                                Kept
                              </span>
                            )}
                          </td>
                          <td className={styles.expandCell}>
                            <button
                              type="button"
                              className={styles.expandButton}
                              aria-expanded={expanded}
                              aria-label={expanded ? 'Hide details' : 'Show details'}
                              onClick={(event) => {
                                event.stopPropagation();
                                toggle();
                              }}>
                              <CaretDown size={14} weight="bold" />
                            </button>
                          </td>
                        </tr>
                        {expanded && (
                          <tr className={styles.detailsRow}>
                            <td colSpan={7}>
                              <HistoryDetails row={row} />
                            </td>
                          </tr>
                        )}
                      </React.Fragment>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}

          {filtered.length > 0 && (
            <div className={styles.tableFooter}>
              <span>
                Showing {formatCount(shown.length)} of {formatCount(filtered.length)}
              </span>
              {shown.length < filtered.length && (
                <button type="button" className={styles.secondaryButton} onClick={() => setLimit(limit + PAGE_SIZE)}>
                  Show more
                </button>
              )}
            </div>
          )}
        </>
      )}
    </Card>
  );
}

/* ────────────────────────────────────────────────────────────── */
/*  API keys                                                     */
/* ────────────────────────────────────────────────────────────── */

function APIKeysCard() {
  const isDemoUser = useAppSelector(selectIsDemoUser);
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [newKeyName, setNewKeyName] = useState('');
  const [createdKey, setCreatedKey] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);

  const load = useCallback(
    () =>
      apiClient
        .get<APIKey[]>('keys')
        .then((res) => setKeys(res ?? []))
        .catch(() => setKeys([]))
        .finally(() => setLoading(false)),
    [],
  );

  useEffect(() => {
    load();
  }, [load]);

  const handleCreate = async (event: React.FormEvent) => {
    event.preventDefault();
    const name = newKeyName.trim();
    if (!name || creating) return;
    setCreating(true);
    setCreatedKey(null);
    try {
      // The API answers with the full key, once.
      const fullKey = await apiClient.post<string, { name: string }>('keys', { name });
      setCreatedKey(fullKey);
      setNewKeyName('');
      load();
    } catch (err) {
      toast.error('Could not create the key', { description: errorMessage(err, 'Try again.') });
    } finally {
      setCreating(false);
    }
  };

  const copyKey = async () => {
    if (!createdKey) return;
    try {
      await navigator.clipboard.writeText(createdKey);
      toast.success('Key copied');
    } catch {
      toast.error('Could not copy', { description: 'Select the key and copy it by hand.' });
    }
  };

  return (
    <Card
      icon={<Key size={18} weight="duotone" />}
      title="Pennywise API keys"
      description="For scripts and other apps that read or write this budget through the Pennywise API."
      aside={<span className={styles.countPill}>{keys.length}</span>}>
      {createdKey && (
        <div className={styles.newKey} role="status">
          <div>
            <strong>Copy your new key now</strong>
            <span>It won't be shown again.</span>
          </div>
          <code className={styles.keyValue}>{createdKey}</code>
          <button type="button" className={styles.secondaryButton} onClick={copyKey}>
            <Copy size={14} aria-hidden="true" /> Copy
          </button>
        </div>
      )}

      {loading ? (
        <p className={styles.empty}>Loading keys…</p>
      ) : keys.length === 0 ? (
        <p className={styles.empty}>No API keys yet.</p>
      ) : (
        <ul className={styles.keyList}>
          {keys.map((key) => (
            <li key={key.id} className={styles.keyItem}>
              <div className={styles.keyMain}>
                <strong>{key.name}</strong>
                {key.maskedKey && <code>{key.maskedKey}</code>}
                {key.description && <span className={styles.muted}>{key.description}</span>}
              </div>
              <dl className={styles.keyMeta}>
                <div>
                  <dt>Scopes</dt>
                  <dd>{key.scopes?.join(', ') || 'read'}</dd>
                </div>
                <div>
                  <dt>Last used</dt>
                  <dd>{key.lastUsedAt ? formatDate(key.lastUsedAt) : 'Never'}</dd>
                </div>
                <div>
                  <dt>Expires</dt>
                  <dd>{key.expiresAt ? formatDate(key.expiresAt) : 'Never'}</dd>
                </div>
              </dl>
            </li>
          ))}
        </ul>
      )}

      {!isDemoUser && (
        <form onSubmit={handleCreate} className={styles.createKey}>
          <input
            type="text"
            className={styles.input}
            placeholder="Name, e.g. Home Assistant"
            aria-label="New key name"
            value={newKeyName}
            maxLength={80}
            onChange={(event) => setNewKeyName(event.target.value)}
            disabled={creating}
          />
          <button type="submit" className={styles.primaryButton} disabled={creating || !newKeyName.trim()}>
            {creating ? 'Creating…' : 'Create key'}
          </button>
        </form>
      )}
    </Card>
  );
}

/* ────────────────────────────────────────────────────────────── */
/*  Main export                                                  */
/* ────────────────────────────────────────────────────────────── */

export function AISettings() {
  const [predictions, setPredictions] = useState<CipherPrediction[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    apiClient
      .get<CipherPrediction[]>('predictions/cipher')
      .then((data) => setPredictions(data ?? []))
      .catch(() => setPredictions([]))
      .finally(() => setLoading(false));
  }, []);

  return (
    <div className={styles.aiSection}>
      <ProvidersCard />
      {loading ? (
        <div className={`${settingsStyles.card} ${styles.skeleton}`} aria-busy="true">
          Loading predictions…
        </div>
      ) : (
        <>
          <PerformanceCard predictions={predictions} />
          <HistoryCard predictions={predictions} />
        </>
      )}
      <APIKeysCard />
    </div>
  );
}
