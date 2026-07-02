import { ArrowRight as ArrowRightIcon, Brain as BrainIcon, ChartBar as ChartBarIcon, Lightning as LightningIcon, TreeStructure as TreeStructureIcon, CaretDown } from '@phosphor-icons/react';
import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { apiClient } from '@/utils';
import type { CipherPrediction } from '@/features/transactions/types/transaction.types';
import settingsStyles from './Settings.module.css';
import styles from './AISettings.module.css';
import { useAppSelector } from '@/app/hooks';
import { Popover } from '@/components/common/Popover/Popover';

/* ────────────────────────────────────────────────────────────── */
/*  Types                                                        */
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



/* ────────────────────────────────────────────────────────────── */
/*  Helpers                                                      */
/* ────────────────────────────────────────────────────────────── */

function formatConfidence(value?: number | null): string {
  if (value === null || value === undefined) return '—';
  const percent = value <= 1 ? value * 100 : value;
  return `${percent.toFixed(percent >= 10 ? 0 : 1)}%`;
}

function formatDate(dateStr?: string): string {
  if (!dateStr) return '—';
  return new Date(dateStr).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  });
}

const formatCurrency = (amount: number): string => {
  return new Intl.NumberFormat('en-IN', {
    style: 'currency',
    currency: 'INR',
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(amount);
};

function computeSourceCounts(predictions: CipherPrediction[]): Record<PredictionSource, number> {
  const counts: Record<string, number> = {
    RULE: 0,
    VECTOR: 0,
    LLM: 0,
    MANUAL: 0,
    UNCATEGORIZED: 0,
  };
  for (const p of predictions) {
    const source = (p.source ?? 'UNCATEGORIZED').toUpperCase();
    if (source in counts) {
      counts[source]++;
    } else {
      counts['UNCATEGORIZED']++;
    }
  }
  return counts as Record<PredictionSource, number>;
}

function computeAvgConfidence(predictions: CipherPrediction[]): number | null {
  const values: number[] = [];
  for (const p of predictions) {
    if (p.payeeConfidence != null) values.push(p.payeeConfidence);
    if (p.categoryConfidence != null) values.push(p.categoryConfidence);
  }
  if (values.length === 0) return null;
  return values.reduce((sum, v) => sum + v, 0) / values.length;
}

function computeAvgConfidenceBySource(predictions: CipherPrediction[], source: string): number | null {
  const values: number[] = [];
  for (const p of predictions) {
    if (p.source === source) {
      if (p.payeeConfidence != null) values.push(p.payeeConfidence);
      if (p.categoryConfidence != null) values.push(p.categoryConfidence);
    }
  }
  if (values.length === 0) return null;
  return values.reduce((sum, v) => sum + v, 0) / values.length;
}

/* ────────────────────────────────────────────────────────────── */
/*  Sub-components                                               */
/* ────────────────────────────────────────────────────────────── */

function SectionLabel({ icon, label }: { icon: React.ReactNode; label: string }) {
  return (
    <div className={styles.sectionHeader}>
      <span className={styles.sectionHeaderIcon}>{icon}</span>
      {label}
    </div>
  );
}

function StatCard({ label, value, sub }: { label: string; value: string | number; sub?: string }) {
  return (
    <div className={styles.statCard}>
      <span className={styles.statLabel}>{label}</span>
      <span className={styles.statValue}>{value}</span>
      {sub && <span className={styles.statSub}>{sub}</span>}
    </div>
  );
}



function PredictionStatsSection({ predictions }: { predictions: CipherPrediction[] }) {
  const total = predictions.length;
  const corrected = predictions.filter((p) => p.hasUserCorrected).length;
  const payeeCorrected = predictions.filter((p) => p.hasUserCorrected && p.actualPayeeId != null && p.actualPayeeId !== p.predictedPayeeId).length;
  const categoryCorrected = predictions.filter((p) => p.hasUserCorrected && p.actualCategoryId != null && p.actualCategoryId !== p.predictedCategoryId).length;
  const payeeRate = total > 0 ? ((payeeCorrected / total) * 100).toFixed(1) : '0';
  const categoryRate = total > 0 ? ((categoryCorrected / total) * 100).toFixed(1) : '0';
  const avgConf = computeAvgConfidence(predictions);
  const sourceCounts = computeSourceCounts(predictions);
  const last30 = predictions.filter((p) => {
    if (!p.createdAt) return false;
    const diff = Date.now() - new Date(p.createdAt).getTime();
    return diff <= 30 * 24 * 60 * 60 * 1000;
  }).length;

  return (
    <div className={settingsStyles.card}>
      <div className={settingsStyles.cardHeader}>
        <h2>Prediction Overview</h2>
        <span>{total} total</span>
      </div>

      <div className={styles.statsGrid}>
        <StatCard label="Corrected" value={corrected} sub={`${payeeCorrected} payee (${payeeRate}%) • ${categoryCorrected} category (${categoryRate}%)`} />
        <StatCard label="Avg Confidence" value={avgConf !== null ? formatConfidence(avgConf) : '—'} />
        <StatCard label="Last 30 Days" value={last30} />
      </div>

      <div style={{ marginTop: '1rem' }}>
        <SectionLabel icon={<ChartBarIcon size={16} />} label="Source Breakdown" />
        <div className={styles.sourceBadgeRow}>
          {(Object.keys(sourceCounts) as PredictionSource[])
            .filter((s) => sourceCounts[s] > 0)
            .sort((a, b) => sourceCounts[b] - sourceCounts[a])
            .map((source) => (
              <div key={source} className={styles.sourceBadgeItem}>
                <span className={styles.sourceBadge} data-source={source}>
                  {source}
                </span>
                <span className={styles.sourceBadgeCount}>{sourceCounts[source]}</span>
              </div>
            ))}
        </div>
      </div>
    </div>
  );
}

function PredictionHistorySection({ predictions }: { predictions: CipherPrediction[] }) {
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const { allPayees } = useAppSelector((state) => state.payees);
  const { allCategories } = useAppSelector((state) => state.categories);
  const sorted = useMemo(
    () => [...predictions.map((p) => {
      return {
        ...p,
        payee: allPayees.find((payee) => payee.id === p.predictedPayeeId)?.name,
        category: allCategories.find((category) => category.id === p.predictedCategoryId)?.name,
        actualPayee: p.actualPayeeId ? allPayees.find((payee) => payee.id === p.actualPayeeId)?.name : null,
        actualCategory: p.actualCategoryId ? allCategories.find((category) => category.id === p.actualCategoryId)?.name : null,
      }
    })],
    [predictions, allPayees, allCategories],
  );

  return (
    <div className={settingsStyles.card}>
      <div className={settingsStyles.cardHeader}>
        <h2>Prediction History</h2>
        <span>{sorted.length} records</span>
      </div>

      {sorted.length === 0 ? (
        <p className={styles.emptyText}>No predictions yet.</p>
      ) : (
        <div className={styles.tableWrap}>
          <table className={styles.predictionTable}>
            <thead>
              <tr>
                <th>Date</th>
                <th>Source</th>
                <th>Predicted Payee</th>
                <th>Predicted Category</th>
                <th>Confidence</th>
                <th>Corrected</th>
                <th>Reasoning</th>
              </tr>
            </thead>
            <tbody>
              {sorted.map((p) => (
                <React.Fragment key={p.id}>
                  <tr 
                    className={`${styles.predictionRow} ${expandedId === p.id ? styles.expandedRow : ''}`} 
                    onClick={() => setExpandedId(expandedId === p.id ? null : p.id)}
                  >
                    <td className={styles.dimText}>{formatDate(p.createdAt)}</td>
                    <td>
                      <span className={styles.sourceBadge} data-source={p.source ?? 'UNCATEGORIZED'}>
                        {p.source ?? '—'}
                      </span>
                    </td>
                    <td className={styles.truncate}>{p.payee ?? '—'}</td>
                    <td className={styles.truncate}>{p.category ?? '—'}</td>
                    <td className={styles.confidenceCell}>{formatConfidence(p.payeeConfidence)}</td>
                    <td>
                      <span className={p.hasUserCorrected ? styles.correctedYes : styles.correctedNo}>
                        {p.hasUserCorrected ? 'Yes' : 'No'}
                      </span>
                    </td>
                    <td className={`${styles.truncate} ${styles.dimText}`} title={p.llmReasoning ?? ''}>
                      {p.llmReasoning ? p.llmReasoning.slice(0, 60) + (p.llmReasoning.length > 60 ? '…' : '') : ''}
                    </td>
                  </tr>
                  {expandedId === p.id && (
                    <tr className={styles.detailsRow}>
                      <td colSpan={7}>
                        <div className={styles.detailsContent}>
                          {p.hasUserCorrected && (p.actualPayee || p.actualCategory) && (
                            <div className={styles.detailsSection}>
                              <h4 className={styles.detailsSectionTitle}>User Correction</h4>
                              <div className={styles.correctionGrid}>
                                {p.actualPayee && (
                                  <div>
                                    <span className={styles.correctionLabel}>Actual Payee:</span>
                                    <span className={styles.correctionValue}>{p.actualPayee}</span>
                                  </div>
                                )}
                                {p.actualCategory && (
                                  <div>
                                    <span className={styles.correctionLabel}>Actual Category:</span>
                                    <span className={styles.correctionValue}>{p.actualCategory}</span>
                                  </div>
                                )}
                              </div>
                            </div>
                          )}
                          {p.llmReasoning && (
                            <div className={styles.detailsSection}>
                              <h4 className={styles.detailsSectionTitle}>AI Reasoning</h4>
                              <div className={styles.detailsBox}>{p.llmReasoning}</div>
                            </div>
                          )}
                          <div className={styles.detailsSectionRow}>
                            <div className={styles.detailsSection}>
                              <h4 className={styles.detailsSectionTitle}>Extracted Entities</h4>
                              <div className={styles.entityGrid}>
                                <div>
                                  <span className={styles.entityLabel}>Account:</span>
                                  <span className={styles.entityValue}>{p.extractedAccount || '—'}</span>
                                </div>
                                <div>
                                  <span className={styles.entityLabel}>Payee:</span>
                                  <span className={styles.entityValue}>{p.extractedPayee || '—'}</span>
                                </div>
                                <div>
                                  <span className={styles.entityLabel}>Amount:</span>
                                  <span className={styles.entityValue}>{p.amount != null ? formatCurrency(p.amount) : '—'}</span>
                                </div>
                              </div>
                            </div>
                            {!!p.metadata && (
                              <div className={styles.detailsSection}>
                                <h4 className={styles.detailsSectionTitle}>Metadata</h4>
                                <pre className={styles.metadataBox}>
                                  {JSON.stringify(p.metadata, null, 2)}
                                </pre>
                              </div>
                            )}
                          </div>
                        </div>
                      </td>
                    </tr>
                  )}
                </React.Fragment>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

interface BudgetConfig {
  aiEmbeddingProvider: string;
  aiEmbeddingModel: string;
  aiExtractionProvider: string;
  aiExtractionModel: string;
  aiClassificationProvider: string;
  aiClassificationModel: string;
  openaiApiKey: string;
  anthropicApiKey: string;
  openrouterApiKey: string;
  aiVectorSimilarityThreshold: number;
  aiExactAmountThreshold: number;
}

const mockOllamaModels = ['gemma4:12b', 'llama3:8b', 'bge-m3'];
const providers = ['ollama', 'openai', 'anthropic', 'openrouter'];

function CustomSelect({ value, options, onChange, placeholder }: { value: string, options: string[], onChange: (val: string) => void, placeholder?: string }) {
  const [isOpen, setIsOpen] = useState(false);
  const triggerRef = React.useRef<HTMLButtonElement>(null);

  return (
    <>
      <button 
        type="button" 
        ref={triggerRef} 
        className={styles.configInput} 
        onClick={() => setIsOpen(!isOpen)}
        style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', width: '100%', textAlign: 'left', cursor: 'pointer' }}
      >
        <span>{value || placeholder || 'Select...'}</span>
        <CaretDown size={14} weight="bold" />
      </button>
      <Popover id={`select-${value}`} isOpen={isOpen} triggerRef={triggerRef} onClose={() => setIsOpen(false)}>
        <div style={{ display: 'flex', flexDirection: 'column', padding: '0.25rem', gap: '0.2rem', minWidth: '150px' }}>
          {options.map(opt => (
            <button 
              key={opt}
              type="button"
              className={`${styles.popoverOption} ${value === opt ? styles.popoverOptionActive : ''}`}
              onClick={() => { onChange(opt); setIsOpen(false); }}
            >
              {opt}
            </button>
          ))}
        </div>
      </Popover>
    </>
  );
}

function AIConfigSection({ predictions }: { predictions: CipherPrediction[] }) {
  const ruleConf = computeAvgConfidenceBySource(predictions, 'RULE');
  const vectorConf = computeAvgConfidenceBySource(predictions, 'VECTOR');
  const llmConf = computeAvgConfidenceBySource(predictions, 'LLM');

  const [isEditing, setIsEditing] = useState(false);
  const [config, setConfig] = useState<BudgetConfig>({
    aiEmbeddingProvider: 'ollama',
    aiEmbeddingModel: 'bge-m3',
    aiExtractionProvider: 'ollama',
    aiExtractionModel: 'gemma4:12b',
    aiClassificationProvider: 'ollama',
    aiClassificationModel: 'gemma4:12b',
    openaiApiKey: '',
    anthropicApiKey: '',
    openrouterApiKey: '',
    aiVectorSimilarityThreshold: 0.80,
    aiExactAmountThreshold: 0.70,
  });

  const [editForm, setEditForm] = useState<BudgetConfig>(config);

  const handleEdit = () => {
    setEditForm(config);
    setIsEditing(true);
  };

  const handleCancel = () => {
    setIsEditing(false);
  };

  const handleSave = () => {
    setConfig(editForm);
    setIsEditing(false);
  };

  const handleChange = (field: keyof BudgetConfig, value: string | number) => {
    setEditForm(prev => ({ ...prev, [field]: value }));
  };

  return (
    <div className={settingsStyles.card}>
      <div className={styles.configHeaderRow}>
        <div className={settingsStyles.cardHeader} style={{ padding: 0, border: 'none', margin: 0, paddingBottom: '1rem' }}>
          <h2>AI Configuration</h2>
        </div>
        {!isEditing ? (
          <button className={styles.editButton} onClick={handleEdit}>Edit Config</button>
        ) : (
          <div className={styles.actionButtonGroup}>
            <button className={styles.cancelButton} onClick={handleCancel}>Cancel</button>
            <button className={styles.saveButton} onClick={handleSave}>Save</button>
          </div>
        )}
      </div>

      <SectionLabel icon={<TreeStructureIcon size={16} />} label="Classification Pipeline" />
      <div className={styles.pipelineSteps}>
        <div className={styles.pipelineStep}>
          <div className={styles.pipelineStepHeader}>
            <span className={styles.pipelineStepIcon} data-step="0">0</span>
            Data Extraction
          </div>
          <span className={styles.pipelineStepSub}>Raw parsing</span>
        </div>
        <span className={styles.pipelineArrow}>
          <ArrowRightIcon size={16} />
        </span>
        <div className={styles.pipelineStep}>
          <div className={styles.pipelineStepHeader}>
            <span className={styles.pipelineStepIcon} data-step="1">1</span>
            Payee Rules
          </div>
          <span className={styles.pipelineStepSub}>
            {ruleConf !== null ? `${formatConfidence(ruleConf)} avg conf` : 'No data'}
          </span>
        </div>
        <span className={styles.pipelineArrow}>
          <ArrowRightIcon size={16} />
        </span>
        <div className={styles.pipelineStep}>
          <div className={styles.pipelineStepHeader}>
            <span className={styles.pipelineStepIcon} data-step="2">2</span>
            Semantic Search
          </div>
          <span className={styles.pipelineStepSub}>
            {vectorConf !== null ? `${formatConfidence(vectorConf)} avg conf` : 'No data'}
          </span>
        </div>
        <span className={styles.pipelineArrow}>
          <ArrowRightIcon size={16} />
        </span>
        <div className={styles.pipelineStep}>
          <div className={styles.pipelineStepHeader}>
            <span className={styles.pipelineStepIcon} data-step="3">3</span>
            LLM Fallback
          </div>
          <span className={styles.pipelineStepSub}>
            {llmConf !== null ? `${formatConfidence(llmConf)} avg conf` : 'No data'}
          </span>
        </div>
      </div>

      <div style={{ marginTop: '1rem' }}>
        <SectionLabel icon={<BrainIcon size={16} />} label="Models & Providers" />
        <div className={styles.configGrid}>
          <div className={styles.configItem}>
            <span className={styles.configLabel}>Embedding</span>
            {!isEditing ? (
              <>
                <span className={styles.configValue}>{config.aiEmbeddingModel}</span>
                <span className={styles.configSub}>via {config.aiEmbeddingProvider}</span>
              </>
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '0.4rem', width: '100%' }}>
                <input type="text" className={styles.configInput} value={config.aiEmbeddingProvider} disabled style={{ opacity: 0.6, cursor: 'not-allowed' }} />
                <input type="text" className={styles.configInput} value={config.aiEmbeddingModel} disabled style={{ opacity: 0.6, cursor: 'not-allowed' }} />
                <span className={styles.configSub} style={{ color: 'var(--color-warning)' }}>Cannot change (breaks existing vector data)</span>
              </div>
            )}
          </div>
          
          <div className={styles.configItem}>
            <span className={styles.configLabel}>Extraction</span>
            {!isEditing ? (
              <>
                <span className={styles.configValue}>{config.aiExtractionModel}</span>
                <span className={styles.configSub}>via {config.aiExtractionProvider}</span>
              </>
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '0.4rem', width: '100%' }}>
                <CustomSelect value={editForm.aiExtractionProvider} options={providers} onChange={v => handleChange('aiExtractionProvider', v)} />
                {editForm.aiExtractionProvider === 'ollama' ? (
                  <CustomSelect value={editForm.aiExtractionModel} options={mockOllamaModels} onChange={v => handleChange('aiExtractionModel', v)} />
                ) : (
                  <input type="text" className={styles.configInput} value={editForm.aiExtractionModel} onChange={e => handleChange('aiExtractionModel', e.target.value)} />
                )}
              </div>
            )}
          </div>

          <div className={styles.configItem}>
            <span className={styles.configLabel}>Classification</span>
            {!isEditing ? (
              <>
                <span className={styles.configValue}>{config.aiClassificationModel}</span>
                <span className={styles.configSub}>via {config.aiClassificationProvider}</span>
              </>
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '0.4rem', width: '100%' }}>
                <CustomSelect value={editForm.aiClassificationProvider} options={providers} onChange={v => handleChange('aiClassificationProvider', v)} />
                {editForm.aiClassificationProvider === 'ollama' ? (
                  <CustomSelect value={editForm.aiClassificationModel} options={mockOllamaModels} onChange={v => handleChange('aiClassificationModel', v)} />
                ) : (
                  <input type="text" className={styles.configInput} value={editForm.aiClassificationModel} onChange={e => handleChange('aiClassificationModel', e.target.value)} />
                )}
              </div>
            )}
          </div>
        </div>
      </div>

      <div style={{ marginTop: '1rem' }}>
        <SectionLabel icon={<LightningIcon size={16} />} label="API Keys" />
        <div className={styles.configGrid}>
          <div className={styles.configItem}>
            <span className={styles.configLabel}>OpenAI Key</span>
            {!isEditing ? (
              <span className={styles.configValue}>{config.openaiApiKey ? '********' : 'Not set'}</span>
            ) : (
              <input type="password" placeholder="sk-..." className={styles.configInput} value={editForm.openaiApiKey} onChange={e => handleChange('openaiApiKey', e.target.value)} />
            )}
            <span className={styles.configSub}>For OpenAI models</span>
          </div>
          <div className={styles.configItem}>
            <span className={styles.configLabel}>Anthropic Key</span>
            {!isEditing ? (
              <span className={styles.configValue}>{config.anthropicApiKey ? '********' : 'Not set'}</span>
            ) : (
              <input type="password" placeholder="sk-ant-..." className={styles.configInput} value={editForm.anthropicApiKey} onChange={e => handleChange('anthropicApiKey', e.target.value)} />
            )}
            <span className={styles.configSub}>For Claude models</span>
          </div>
          <div className={styles.configItem}>
            <span className={styles.configLabel}>OpenRouter Key</span>
            {!isEditing ? (
              <span className={styles.configValue}>{config.openrouterApiKey ? '********' : 'Not set'}</span>
            ) : (
              <input type="password" placeholder="sk-or-v1-..." className={styles.configInput} value={editForm.openrouterApiKey} onChange={e => handleChange('openrouterApiKey', e.target.value)} />
            )}
            <span className={styles.configSub}>For OpenRouter models</span>
          </div>
        </div>
      </div>

      <div style={{ marginTop: '1rem' }}>
        <SectionLabel icon={<TreeStructureIcon size={16} />} label="Thresholds" />
        <div className={styles.configGrid}>
          <div className={styles.configItem}>
            <span className={styles.configLabel}>Vector Similarity</span>
            {!isEditing ? (
              <span className={styles.configValue}>{(config.aiVectorSimilarityThreshold * 100).toFixed(0)}%</span>
            ) : (
              <input type="number" step="0.01" min="0" max="1" className={styles.configInput} value={editForm.aiVectorSimilarityThreshold} onChange={e => handleChange('aiVectorSimilarityThreshold', parseFloat(e.target.value))} />
            )}
            <span className={styles.configSub}>Min cosine similarity</span>
          </div>
          <div className={styles.configItem}>
            <span className={styles.configLabel}>Exact Amount Match</span>
            {!isEditing ? (
              <span className={styles.configValue}>{(config.aiExactAmountThreshold * 100).toFixed(0)}%</span>
            ) : (
              <input type="number" step="0.01" min="0" max="1" className={styles.configInput} value={editForm.aiExactAmountThreshold} onChange={e => handleChange('aiExactAmountThreshold', parseFloat(e.target.value))} />
            )}
            <span className={styles.configSub}>Threshold when amount matches</span>
          </div>
        </div>
      </div>
    </div>
  );
}

function APIKeysSection() {
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [newKeyName, setNewKeyName] = useState('');
  const [createdKey, setCreatedKey] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    apiClient
      .get<APIKey[]>('keys')
      .then(setKeys)
      .catch(() => setKeys([]))
      .finally(() => setLoading(false));
  }, []);

  const handleCreate = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      if (!newKeyName.trim() || creating) return;
      setCreating(true);
      setCreatedKey(null);
      try {
        const fullKey = await apiClient.post<any>('keys', { name: newKeyName.trim() } as any);
        setCreatedKey(fullKey as string);
        setNewKeyName('');
        // Reload keys list
        apiClient
          .get<APIKey[]>('keys')
          .then(setKeys)
          .catch(() => { });
      } catch (err) {
        console.error('Failed to create API key:', err);
      } finally {
        setCreating(false);
      }
    },
    [newKeyName, creating],
  );

  return (
    <div className={settingsStyles.card}>
      <div className={settingsStyles.cardHeader}>
        <h2>API Keys</h2>
        <span>
          {keys.length} key{keys.length !== 1 ? 's' : ''}
        </span>
      </div>

      <div className={styles.apiKeySection}>
        {loading ? (
          <span className={styles.loadingText}>Loading keys…</span>
        ) : keys.length === 0 && !createdKey ? (
          <span className={styles.emptyText}>No API keys created yet.</span>
        ) : (
          <div className={styles.tableWrap}>
            <table className={styles.apiKeyTable}>
              <thead>
                <tr>
                  <th>Key</th>
                  <th>Description</th>
                  <th>Scopes</th>
                  <th>Rate Limit</th>
                  <th>Expires</th>
                  <th>Last Used</th>
                </tr>
              </thead>
              <tbody>
                {keys.map((key) => (
                  <tr key={key.id}>
                    <td className={styles.apiKeyName}>
                      <span>{key.name}</span>
                      <span className={styles.maskedKey}>{key.maskedKey}</span>
                    </td>
                    <td>{key.description ?? '—'}</td>
                    <td className={styles.apiKeyMeta}>{key.scopes?.join(', ') ?? 'read'}</td>
                    <td>{key.rateLimit ?? '—'}</td>
                    <td>{(key.expiresAt && formatDate(key.expiresAt)) ?? '—'}</td>
                    <td>{key.lastUsedAt && formatDate(key.lastUsedAt)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {createdKey && (
          <div className={styles.newKeyDisplay}>
            <span className={styles.newKeyLabel}>New key created — copy it now, it won't be shown again</span>
            <span className={styles.apiKeyValue}>{createdKey}</span>
            <span className={styles.newKeySub}>Store this securely. You won't be able to see it again.</span>
          </div>
        )}

        <form onSubmit={handleCreate} className={styles.createKeyForm}>
          <input
            type="text"
            className={styles.createKeyInput}
            placeholder="Key name (e.g. 'Mobile App')"
            value={newKeyName}
            onChange={(e) => setNewKeyName(e.target.value)}
            disabled={creating}
          />
          <button type="submit" className={styles.createKeyBtn} disabled={creating || !newKeyName.trim()}>
            {creating ? 'Creating…' : 'Create Key'}
          </button>
        </form>
      </div>
    </div>
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
      {loading ? (
        <div className={settingsStyles.card}>
          <span className={styles.loadingText}>Loading prediction data…</span>
        </div>
      ) : (
        <>
          <PredictionStatsSection predictions={predictions} />
          <PredictionHistorySection predictions={predictions} />
        </>
      )}
      <AIConfigSection predictions={predictions} />
      <APIKeysSection />
    </div>
  );
}
