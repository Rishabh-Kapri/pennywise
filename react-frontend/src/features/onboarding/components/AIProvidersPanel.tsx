import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from 'react';
import {
  CaretDown,
  CaretUp,
  CheckCircle,
  CircleNotch,
  Lightning,
  PencilSimple,
  Plus,
  Trash,
  Warning,
  WarningCircle,
} from '@phosphor-icons/react';
import { SearchSelect } from '@/features/transactions/components/popovers/SearchSelect';
import { errorMessage, toast } from '@/utils';
import {
  createAIModel,
  deleteAIModel,
  deleteAIProvider,
  fetchAIModels,
  fetchAIProviders,
  reorderAIModels,
  saveAIProvider,
  setAIModelEnabled,
  setDefaultAIModel,
  testAIModel,
  updateAIModel,
} from '../api';
import { AI_PROVIDERS } from '../aiProviders';
import type { AIModel, AIModelKind, AIModelTestResult, AIProvider, AIProviderName } from '../types';
import styles from './AIProvidersPanel.module.css';

const PROVIDER_NAMES = Object.keys(AI_PROVIDERS) as AIProviderName[];

/** Two-letter marks so a row says which key it uses at a glance. */
const PROVIDER_MARK: Record<AIProviderName, string> = {
  openai: 'OA',
  anthropic: 'An',
  openrouter: 'OR',
  ollama: 'Ol',
};

const KINDS: { kind: AIModelKind; title: string; description: string; noun: string; empty: string }[] = [
  {
    kind: 'chat',
    title: 'Chat models',
    description: 'Read bank emails, predict payees and categories, import statements and run the assistant.',
    noun: 'chat model',
    empty: 'No chat models yet. Email import and the assistant need one.',
  },
  {
    kind: 'decision',
    title: 'Decision models',
    description:
      'Small, cheap classification models called through a decisions API, for steps like spotting transactions.',
    noun: 'decision model',
    empty: 'No decision models yet.',
  },
  {
    kind: 'embedding',
    title: 'Embedding models',
    description: 'Turn bank text into vectors, to find similar past transactions. Needs an OpenRouter or Ollama key.',
    noun: 'embedding model',
    empty: 'No embedding models yet. Finding similar past transactions needs one.',
  },
];

const EMBEDDING_SWITCH_WARNING =
  'Past transactions were matched using your current model, so switching makes those matches stop working until they are learned again.';

/** What a new model of this kind starts with on this key. */
function defaultSlug(kind: AIModelKind, provider: AIProviderName): string {
  if (kind === 'chat') return AI_PROVIDERS[provider].defaultChatModel;
  if (kind === 'embedding') return AI_PROVIDERS[provider].defaultEmbeddingModel ?? '';
  return '';
}

/** Keys that can run a model of this kind. */
function servesKind(kind: AIModelKind, provider: AIProviderName): boolean {
  return kind !== 'embedding' || !!AI_PROVIDERS[provider].defaultEmbeddingModel;
}

interface KeyForm {
  provider: AIProviderName;
  apiKey: string;
  baseUrl: string;
  /** Editing a saved key: an empty key field keeps the stored one. */
  editing: boolean;
}

interface ModelForm {
  kind: AIModelKind;
  /** Set when editing; the key and kind can't change then. */
  id?: string;
  provider: AIProviderName | '';
  slug: string;
  endpoint: string;
  makeDefault: boolean;
}

export interface AISetup {
  providers: AIProvider[];
  models: AIModel[];
}

interface AIProvidersPanelProps {
  /** Called with the keys and models whenever they load or change. */
  onChange?: (setup: AISetup) => void;
  /** The demo account can look but not change anything. */
  readOnly?: boolean;
}

function ProviderMark({ provider, small }: { provider: AIProviderName; small?: boolean }) {
  return (
    <span className={`${styles.mark} ${small ? styles.markSmall : ''}`} data-provider={provider} aria-hidden="true">
      {PROVIDER_MARK[provider]}
    </span>
  );
}

function GroupHeader({
  title,
  description,
  count,
  action,
}: {
  title: string;
  description: string;
  count?: number;
  action?: ReactNode;
}) {
  return (
    <header className={styles.groupHeader}>
      <div className={styles.groupTitle}>
        <h4>
          {title}
          {count !== undefined && count > 0 && <span className={styles.count}>{count}</span>}
        </h4>
        <p>{description}</p>
      </div>
      {action}
    </header>
  );
}

/**
 * The user's own AI setup: keys (one per provider) and the models run with
 * them. One key can back several models, of either kind. Models are tested
 * before they're saved. Keys are write-only: the API only says one is saved.
 */
export function AIProvidersPanel({ onChange, readOnly }: AIProvidersPanelProps) {
  const [providers, setProviders] = useState<AIProvider[]>([]);
  const [models, setModels] = useState<AIModel[]>([]);
  const [loading, setLoading] = useState(true);
  const [keyForm, setKeyForm] = useState<KeyForm | null>(null);
  const [modelForm, setModelForm] = useState<ModelForm | null>(null);
  const [modelError, setModelError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState<string | null>(null);
  const [results, setResults] = useState<Record<string, AIModelTestResult>>({});

  const load = useCallback(async () => {
    try {
      const [keys, list] = await Promise.all([fetchAIProviders(), fetchAIModels()]);
      setProviders(keys ?? []);
      setModels(list ?? []);
      onChange?.({ providers: keys ?? [], models: list ?? [] });
    } catch (err) {
      toast.error('Could not load your AI setup', { description: errorMessage(err, 'Try again.') });
    } finally {
      setLoading(false);
    }
  }, [onChange]);

  useEffect(() => {
    load();
  }, [load]);

  const unconfigured = PROVIDER_NAMES.filter((name) => !providers.some((p) => p.provider === name));
  const keyOf = (name: AIProviderName | '') => providers.find((p) => p.provider === name);
  const modelsOn = (provider: AIProvider) => models.filter((m) => m.providerId === provider.id);

  /* ── keys ─────────────────────────────────────────────── */

  const openNewKey = () => {
    if (unconfigured.length === 0) return;
    setModelForm(null);
    setKeyForm({ provider: unconfigured[0], apiKey: '', baseUrl: '', editing: false });
  };

  const submitKey = async (event: FormEvent) => {
    event.preventDefault();
    if (!keyForm) return;
    const info = AI_PROVIDERS[keyForm.provider];
    const existing = keyOf(keyForm.provider);
    setSaving(true);
    try {
      await saveAIProvider(keyForm.provider, {
        // Left blank while editing = keep the saved key.
        apiKey: keyForm.editing && !keyForm.apiKey.trim() ? undefined : keyForm.apiKey.trim(),
        baseUrl: info.selfHosted ? keyForm.baseUrl.trim() : undefined,
        enabled: existing?.enabled ?? true,
      });
      const added = !keyForm.editing;
      const provider = keyForm.provider;
      setKeyForm(null);
      await load();
      // A new key does nothing without a model: offer the first one straight away.
      if (added) {
        setModelError(null);
        setModelForm({ kind: 'chat', provider, slug: info.defaultChatModel, endpoint: '', makeDefault: false });
      }
    } catch (err) {
      toast.error(`Could not save the ${info.label} key`, { description: errorMessage(err, 'Try again.') });
    } finally {
      setSaving(false);
    }
  };

  const removeKey = async (provider: AIProvider) => {
    const count = modelsOn(provider).length;
    const label = AI_PROVIDERS[provider.provider].label;
    const also = count > 0 ? ` and the ${count === 1 ? 'model' : `${count} models`} that use it` : '';
    if (!window.confirm(`Remove your ${label} key${also}?`)) return;
    try {
      await deleteAIProvider(provider.provider);
      await load();
    } catch (err) {
      toast.error('Could not remove the key', { description: errorMessage(err, 'Try again.') });
    }
  };

  const toggleKey = async (provider: AIProvider) => {
    try {
      await saveAIProvider(provider.provider, { baseUrl: provider.baseUrl, enabled: !provider.enabled });
      await load();
    } catch (err) {
      toast.error('Could not update the key', { description: errorMessage(err, 'Try again.') });
    }
  };

  /* ── models ───────────────────────────────────────────── */

  const openNewModel = (kind: AIModelKind) => {
    const eligible = providers.filter((p) => servesKind(kind, p.provider));
    const first = eligible.find((p) => p.enabled) ?? eligible[0];
    setModelError(null);
    setKeyForm(null);
    setModelForm({
      kind,
      provider: first?.provider ?? '',
      slug: first ? defaultSlug(kind, first.provider) : '',
      endpoint: '',
      makeDefault: false,
    });
  };

  const closeModelForm = () => {
    setModelForm(null);
    setModelError(null);
  };

  const submitModel = async (event: FormEvent) => {
    event.preventDefault();
    if (!modelForm || !modelForm.provider) return;
    setSaving(true);
    setModelError(null);
    try {
      const endpoint = modelForm.kind === 'decision' ? modelForm.endpoint.trim() : undefined;
      if (modelForm.id) {
        await updateAIModel(modelForm.id, { slug: modelForm.slug.trim(), endpoint });
      } else {
        await createAIModel({
          provider: modelForm.provider,
          kind: modelForm.kind,
          slug: modelForm.slug.trim(),
          endpoint,
          makeDefault: modelForm.makeDefault,
        });
      }
      toast.success(modelForm.id ? 'Model updated' : 'Model added', { description: 'It answered the test call.' });
      setModelForm(null);
      await load();
    } catch (err) {
      // A failed test (422) is the common case: show it next to the fields.
      setModelError(errorMessage(err, 'Could not save the model.'));
    } finally {
      setSaving(false);
    }
  };

  const runTest = async (model: AIModel) => {
    setTesting(model.id);
    try {
      const result = await testAIModel(model.id);
      setResults((current) => ({ ...current, [model.id]: result }));
      await load();
    } catch (err) {
      setResults((current) => ({
        ...current,
        [model.id]: { ok: false, error: errorMessage(err, 'Test failed'), latencyMs: 0 },
      }));
    } finally {
      setTesting(null);
    }
  };

  const makeDefault = async (model: AIModel) => {
    if (model.kind === 'embedding' && !window.confirm(EMBEDDING_SWITCH_WARNING)) return;
    try {
      await setDefaultAIModel(model.id);
      await load();
    } catch (err) {
      toast.error('Could not change the default', { description: errorMessage(err, 'Try again.') });
    }
  };

  const toggleModel = async (model: AIModel) => {
    try {
      await setAIModelEnabled(model.id, !model.enabled);
      await load();
    } catch (err) {
      toast.error(`Could not ${model.enabled ? 'disable' : 'enable'} the model`, {
        description: errorMessage(err, 'Try again.'),
      });
    }
  };

  /** Swaps a model with its neighbour. The default always stays first, so it can't be moved or passed. */
  const moveModel = async (list: AIModel[], index: number, direction: -1 | 1) => {
    const ids = list.map((m) => m.id);
    [ids[index], ids[index + direction]] = [ids[index + direction], ids[index]];
    try {
      await reorderAIModels(list[index].kind, ids);
      await load();
    } catch (err) {
      toast.error('Could not change the order', { description: errorMessage(err, 'Try again.') });
    }
  };

  const removeModel = async (model: AIModel) => {
    if (!window.confirm(`Remove ${model.slug}? Pipeline steps that used it go back to your default.`)) return;
    try {
      await deleteAIModel(model.id);
      await load();
    } catch (err) {
      toast.error('Could not remove the model', { description: errorMessage(err, 'Try again.') });
    }
  };

  /** The status pill, plus the error text when the last test failed. */
  const status = (model: AIModel): { pill: ReactNode; error?: string } => {
    if (testing === model.id) {
      return {
        pill: (
          <span className={`${styles.pill} ${styles.pillNeutral}`}>
            <CircleNotch size={12} weight="bold" className={styles.spin} aria-hidden="true" />
            Testing
          </span>
        ),
      };
    }
    const result = results[model.id];
    const error = result && !result.ok ? result.error : model.lastError;
    if (error) {
      return {
        pill: (
          <span className={`${styles.pill} ${styles.pillBad}`}>
            <WarningCircle size={12} weight="fill" aria-hidden="true" />
            Failing
          </span>
        ),
        error,
      };
    }
    if (model.lastVerifiedAt) {
      return {
        pill: (
          <span className={`${styles.pill} ${styles.pillGood}`}>
            <CheckCircle size={12} weight="fill" aria-hidden="true" />
            Working{result?.latencyMs ? ` · ${(result.latencyMs / 1000).toFixed(1)}s` : ''}
          </span>
        ),
      };
    }
    return { pill: <span className={`${styles.pill} ${styles.pillNeutral}`}>Not tested</span> };
  };

  if (loading) return <p className={styles.muted}>Loading your AI setup…</p>;

  const formKey = modelForm ? keyOf(modelForm.provider) : undefined;
  const formKeyLabel = formKey ? AI_PROVIDERS[formKey.provider].label : 'This provider';
  const keyItems = providers.map((p) => ({
    id: p.provider,
    name: `${AI_PROVIDERS[p.provider].label} key${p.enabled ? '' : ' (disabled)'}`,
    leading: <ProviderMark provider={p.provider} small />,
    trailing: p.apiKeyHint ? `••••${p.apiKeyHint}` : undefined,
  }));
  const keyName = (name: AIProviderName | '') => keyItems.find((item) => item.id === name)?.name ?? '';

  return (
    <div className={styles.panel}>
      {/* ── Keys ── */}
      <section className={styles.group}>
        <GroupHeader
          title="Keys"
          description="One per provider. A key can back several models."
          count={providers.length}
          action={
            !readOnly &&
            unconfigured.length > 0 &&
            !(keyForm && !keyForm.editing) && (
              <button type="button" className={styles.secondaryButton} onClick={openNewKey}>
                <Plus size={14} weight="bold" aria-hidden="true" />
                Add key
              </button>
            )
          }
        />

        {providers.length === 0 && !keyForm ? (
          <p className={styles.empty}>
            {readOnly ? 'No keys are saved for this account.' : 'No keys yet. Add one to start.'}
          </p>
        ) : (
          providers.length > 0 && (
            <ul className={styles.list}>
              {providers.map((provider) => {
                const count = modelsOn(provider).length;
                const label = AI_PROVIDERS[provider.provider].label;
                return (
                  <li key={provider.id} className={`${styles.row} ${provider.enabled ? '' : styles.rowOff}`}>
                    <ProviderMark provider={provider.provider} />
                    <div className={styles.rowText}>
                      <span className={styles.rowTitle}>
                        {label}
                        {!provider.enabled && <span className={`${styles.pill} ${styles.pillNeutral}`}>Disabled</span>}
                      </span>
                      <span className={styles.rowMeta}>
                        {provider.apiKeyHint
                          ? `••••${provider.apiKeyHint}`
                          : provider.hasApiKey
                            ? 'Key saved'
                            : 'No key'}
                        {provider.baseUrl ? ` · ${provider.baseUrl}` : ''}
                        {` · ${count === 0 ? 'no models' : count === 1 ? '1 model' : `${count} models`}`}
                      </span>
                    </div>
                    {!readOnly && (
                      <div className={styles.actions}>
                        <button type="button" className={styles.textButton} onClick={() => toggleKey(provider)}>
                          {provider.enabled ? 'Disable' : 'Enable'}
                        </button>
                        <button
                          type="button"
                          className={styles.iconButton}
                          aria-label={`Edit the ${label} key`}
                          title="Edit"
                          onClick={() => {
                            setModelForm(null);
                            setKeyForm({
                              provider: provider.provider,
                              apiKey: '',
                              baseUrl: provider.baseUrl ?? '',
                              editing: true,
                            });
                          }}
                        >
                          <PencilSimple size={16} />
                        </button>
                        <button
                          type="button"
                          className={`${styles.iconButton} ${styles.danger}`}
                          aria-label={`Remove the ${label} key`}
                          title="Remove"
                          onClick={() => removeKey(provider)}
                        >
                          <Trash size={16} />
                        </button>
                      </div>
                    )}
                  </li>
                );
              })}
            </ul>
          )
        )}

        {keyForm && (
          <form className={styles.editor} onSubmit={submitKey}>
            <strong className={styles.editorTitle}>
              {keyForm.editing ? `Edit your ${AI_PROVIDERS[keyForm.provider].label} key` : 'Add a key'}
            </strong>

            {!keyForm.editing && (
              <div className={styles.field}>
                <span>Provider</span>
                <div className={styles.segmented} role="radiogroup" aria-label="Provider">
                  {unconfigured.map((name) => (
                    <button
                      key={name}
                      type="button"
                      role="radio"
                      aria-checked={keyForm.provider === name}
                      className={keyForm.provider === name ? styles.segmentActive : undefined}
                      onClick={() => setKeyForm({ ...keyForm, provider: name, apiKey: '', baseUrl: '' })}
                    >
                      {AI_PROVIDERS[name].label}
                    </button>
                  ))}
                </div>
              </div>
            )}

            <div className={styles.fieldGrid}>
              {AI_PROVIDERS[keyForm.provider].selfHosted && (
                <label className={styles.field}>
                  <span>Server URL</span>
                  <input
                    className={styles.input}
                    value={keyForm.baseUrl}
                    placeholder="https://ollama.example.com"
                    onChange={(event) => setKeyForm({ ...keyForm, baseUrl: event.target.value })}
                  />
                  <small>Must be reachable from the internet; private addresses are refused.</small>
                </label>
              )}
              <label className={styles.field}>
                <span>
                  API key
                  {AI_PROVIDERS[keyForm.provider].keyUrl && (
                    <a href={AI_PROVIDERS[keyForm.provider].keyUrl} target="_blank" rel="noreferrer">
                      Get one
                    </a>
                  )}
                </span>
                <input
                  className={styles.input}
                  type="password"
                  autoComplete="off"
                  value={keyForm.apiKey}
                  placeholder={
                    keyForm.editing
                      ? 'Leave blank to keep the saved key'
                      : AI_PROVIDERS[keyForm.provider].keyPlaceholder
                  }
                  onChange={(event) => setKeyForm({ ...keyForm, apiKey: event.target.value })}
                />
                <small>Encrypted before it's stored, and never shown again.</small>
              </label>
            </div>

            <div className={styles.editorActions}>
              <button
                type="submit"
                className={styles.primaryButton}
                disabled={
                  saving ||
                  (AI_PROVIDERS[keyForm.provider].selfHosted
                    ? !keyForm.baseUrl.trim()
                    : !keyForm.editing && !keyForm.apiKey.trim())
                }
              >
                {saving ? 'Saving…' : 'Save key'}
              </button>
              <button type="button" className={styles.ghostButton} onClick={() => setKeyForm(null)} disabled={saving}>
                Cancel
              </button>
            </div>
          </form>
        )}
      </section>

      {/* ── Models, by kind ── */}
      {KINDS.map(({ kind, title, description, noun, empty }) => {
        const list = models.filter((m) => m.kind === kind);
        const formHere = modelForm?.kind === kind;
        return (
          <section key={kind} className={styles.group}>
            <GroupHeader
              title={title}
              description={description}
              count={list.length}
              action={
                !readOnly &&
                !(formHere && !modelForm?.id) && (
                  <button
                    type="button"
                    className={styles.secondaryButton}
                    disabled={!providers.some((p) => servesKind(kind, p.provider))}
                    title={
                      providers.some((p) => servesKind(kind, p.provider))
                        ? undefined
                        : kind === 'embedding'
                          ? 'Add an OpenRouter or Ollama key first'
                          : 'Add a key first'
                    }
                    onClick={() => openNewModel(kind)}
                  >
                    <Plus size={14} weight="bold" aria-hidden="true" />
                    Add {noun}
                  </button>
                )
              }
            />

            {list.length === 0 && !formHere ? (
              <p className={styles.empty}>{providers.length === 0 && !readOnly ? 'Add a key first.' : empty}</p>
            ) : (
              list.length > 0 && (
                <ul className={styles.list}>
                  {list.map((model, index) => {
                    const { pill, error } = status(model);
                    const canMoveUp = !model.isDefault && index > 0 && !list[index - 1].isDefault;
                    const canMoveDown = !model.isDefault && index < list.length - 1;
                    return (
                      <li key={model.id} className={`${styles.row} ${model.enabled ? '' : styles.rowOff}`}>
                        <ProviderMark provider={model.provider} />
                        <div className={styles.rowText}>
                          <span className={styles.rowTitle}>
                            <span className={styles.slug}>{model.slug}</span>
                            {model.isDefault && <span className={`${styles.pill} ${styles.pillAccent}`}>Default</span>}
                            {!model.enabled && <span className={`${styles.pill} ${styles.pillNeutral}`}>Disabled</span>}
                          </span>
                          <span className={styles.rowMeta}>
                            {AI_PROVIDERS[model.provider].label} key
                            {model.endpoint ? ` · ${model.endpoint}` : ''}
                          </span>
                          {error && <span className={styles.rowError}>{error}</span>}
                        </div>
                        <div className={styles.rowStatus}>{pill}</div>
                        {!readOnly && (
                          <div className={styles.actions}>
                            {!model.isDefault && model.enabled && (
                              <button type="button" className={styles.textButton} onClick={() => makeDefault(model)}>
                                Make default
                              </button>
                            )}
                            <button type="button" className={styles.textButton} onClick={() => toggleModel(model)}>
                              {model.enabled ? 'Disable' : 'Enable'}
                            </button>
                            {!model.isDefault && list.length > 1 && (
                              <>
                                <button
                                  type="button"
                                  className={styles.iconButton}
                                  aria-label={`Move ${model.slug} up`}
                                  title="Move up"
                                  disabled={!canMoveUp}
                                  onClick={() => moveModel(list, index, -1)}
                                >
                                  <CaretUp size={16} />
                                </button>
                                <button
                                  type="button"
                                  className={styles.iconButton}
                                  aria-label={`Move ${model.slug} down`}
                                  title="Move down"
                                  disabled={!canMoveDown}
                                  onClick={() => moveModel(list, index, 1)}
                                >
                                  <CaretDown size={16} />
                                </button>
                              </>
                            )}
                            <button
                              type="button"
                              className={styles.iconButton}
                              aria-label={`Test ${model.slug}`}
                              title="Test"
                              disabled={testing !== null}
                              onClick={() => runTest(model)}
                            >
                              <Lightning size={16} />
                            </button>
                            <button
                              type="button"
                              className={styles.iconButton}
                              aria-label={`Edit ${model.slug}`}
                              title="Edit"
                              onClick={() => {
                                setKeyForm(null);
                                setModelError(null);
                                setModelForm({
                                  kind,
                                  id: model.id,
                                  provider: model.provider,
                                  slug: model.slug,
                                  endpoint: model.endpoint ?? '',
                                  makeDefault: false,
                                });
                              }}
                            >
                              <PencilSimple size={16} />
                            </button>
                            <button
                              type="button"
                              className={`${styles.iconButton} ${styles.danger}`}
                              aria-label={`Remove ${model.slug}`}
                              title="Remove"
                              onClick={() => removeModel(model)}
                            >
                              <Trash size={16} />
                            </button>
                          </div>
                        )}
                      </li>
                    );
                  })}
                </ul>
              )
            )}

            {formHere && modelForm && (
              <form className={styles.editor} onSubmit={submitModel}>
                <strong className={styles.editorTitle}>{modelForm.id ? `Edit ${noun}` : `Add a ${noun}`}</strong>

                <div className={styles.fieldGrid}>
                  <div className={styles.field}>
                    <span>Key</span>
                    {modelForm.id ? (
                      <input className={styles.input} value={keyName(modelForm.provider)} disabled />
                    ) : (
                      <SearchSelect
                        variant="form"
                        value={keyName(modelForm.provider)}
                        items={keyItems.filter((item) => servesKind(kind, item.id))}
                        placeholder="Choose a key"
                        emptyText="No keys match"
                        onSelect={(item) => {
                          const provider = item.id as AIProviderName;
                          setModelForm({
                            ...modelForm,
                            provider,
                            slug: defaultSlug(kind, provider) || modelForm.slug,
                          });
                        }}
                      />
                    )}
                    {modelForm.id && <small>To use another key, add the model again.</small>}
                  </div>

                  <label className={styles.field}>
                    <span>Model</span>
                    <input
                      className={`${styles.input} ${styles.mono}`}
                      value={modelForm.slug}
                      placeholder={kind === 'decision' ? 'provider/model' : ''}
                      onChange={(event) => setModelForm({ ...modelForm, slug: event.target.value })}
                    />
                    {kind === 'chat' && <small>A small, cheap model is plenty for reading bank emails.</small>}
                  </label>

                  {kind === 'decision' && (
                    <label className={`${styles.field} ${styles.fieldWide}`}>
                      <span>Endpoint</span>
                      <input
                        className={`${styles.input} ${styles.mono}`}
                        value={modelForm.endpoint}
                        placeholder="https://…/decisions"
                        onChange={(event) => setModelForm({ ...modelForm, endpoint: event.target.value })}
                      />
                      <small>
                        The decisions API URL. https only; plain http works only for a server on your own network.
                      </small>
                    </label>
                  )}
                </div>

                {kind === 'decision' && (
                  <p className={`${styles.callout} ${styles.calloutWarn}`}>
                    <Warning size={16} weight="fill" aria-hidden="true" />
                    <span>
                      {formKeyLabel} must serve decision models at this endpoint. Your {formKeyLabel} key is sent to
                      this address, so only use one you trust.
                    </span>
                  </p>
                )}

                {kind === 'embedding' && (
                  <p className={`${styles.callout} ${styles.calloutWarn}`}>
                    <Warning size={16} weight="fill" aria-hidden="true" />
                    <span>The model must return 1024 dimensions. {EMBEDDING_SWITCH_WARNING}</span>
                  </p>
                )}

                {!modelForm.id && list.length > 0 && (
                  <label className={styles.check}>
                    <input
                      type="checkbox"
                      checked={modelForm.makeDefault}
                      onChange={(event) => setModelForm({ ...modelForm, makeDefault: event.target.checked })}
                    />
                    Make this the default {noun}
                  </label>
                )}

                {modelError && (
                  <p className={`${styles.callout} ${styles.calloutBad}`}>
                    <WarningCircle size={16} weight="fill" aria-hidden="true" />
                    <span>{modelError}</span>
                  </p>
                )}

                <div className={styles.editorActions}>
                  <button
                    type="submit"
                    className={styles.primaryButton}
                    disabled={
                      saving ||
                      !modelForm.provider ||
                      !modelForm.slug.trim() ||
                      (kind === 'decision' && !modelForm.endpoint.trim())
                    }
                  >
                    {saving ? (
                      <>
                        <CircleNotch size={14} weight="bold" className={styles.spin} aria-hidden="true" />
                        Testing…
                      </>
                    ) : (
                      'Test and save'
                    )}
                  </button>
                  <button type="button" className={styles.ghostButton} onClick={closeModelForm} disabled={saving}>
                    Cancel
                  </button>
                  <span className={styles.editorNote}>We send one tiny test call before saving.</span>
                </div>
              </form>
            )}
          </section>
        );
      })}
    </div>
  );
}
