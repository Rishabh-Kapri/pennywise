import { useCallback, useEffect, useState, type FormEvent } from 'react';
import { ArrowDown, ArrowUp, CheckCircle, Lightning, PencilSimple, Trash, WarningCircle } from '@phosphor-icons/react';
import { errorMessage, toast } from '@/utils';
import { deleteAIProvider, fetchAIProviders, saveAIProvider, testAIProvider } from '../api';
import type { AIProvider, AIProviderName, AIProviderTestResult } from '../types';
import styles from './steps/steps.module.css';

interface ProviderInfo {
  label: string;
  defaultModel: string;
  keyPlaceholder: string;
  keyUrl?: string;
  /** Ollama: the user's own server, key optional. */
  selfHosted?: boolean;
}

const AI_PROVIDERS: Record<AIProviderName, ProviderInfo> = {
  openai: {
    label: 'OpenAI',
    defaultModel: 'gpt-4o-mini',
    keyPlaceholder: 'sk-…',
    keyUrl: 'https://platform.openai.com/api-keys',
  },
  anthropic: {
    label: 'Anthropic',
    defaultModel: 'claude-haiku-4-5',
    keyPlaceholder: 'sk-ant-…',
    keyUrl: 'https://console.anthropic.com/settings/keys',
  },
  openrouter: {
    label: 'OpenRouter',
    defaultModel: 'google/gemini-2.5-flash',
    keyPlaceholder: 'sk-or-…',
    keyUrl: 'https://openrouter.ai/keys',
  },
  ollama: {
    label: 'Ollama (self-hosted)',
    defaultModel: 'gemma4:12b',
    keyPlaceholder: 'Optional',
    selfHosted: true,
  },
};

const PROVIDER_NAMES = Object.keys(AI_PROVIDERS) as AIProviderName[];

interface FormState {
  provider: AIProviderName;
  apiKey: string;
  baseUrl: string;
  model: string;
  /** Editing a saved provider: an empty key keeps the stored one. */
  editing: boolean;
}

const newForm = (provider: AIProviderName): FormState => ({
  provider,
  apiKey: '',
  baseUrl: '',
  model: AI_PROVIDERS[provider].defaultModel,
  editing: false,
});

interface AIProvidersPanelProps {
  /** Called with the list whenever it loads or changes. */
  onChange?: (providers: AIProvider[]) => void;
  /** The demo account can look but not change anything. */
  readOnly?: boolean;
}

/**
 * The user's own AI providers, tried in order for email parsing, statement
 * import and the assistant. Keys are write-only: the API only says one is saved.
 */
export function AIProvidersPanel({ onChange, readOnly }: AIProvidersPanelProps) {
  const [providers, setProviders] = useState<AIProvider[]>([]);
  const [loading, setLoading] = useState(true);
  const [form, setForm] = useState<FormState | null>(null);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState<AIProviderName | null>(null);
  const [results, setResults] = useState<Partial<Record<AIProviderName, AIProviderTestResult>>>({});

  const load = useCallback(async () => {
    try {
      const list = await fetchAIProviders();
      setProviders(list ?? []);
      onChange?.(list ?? []);
    } catch (err) {
      toast.error('Could not load AI providers', { description: errorMessage(err, 'Try again.') });
    } finally {
      setLoading(false);
    }
  }, [onChange]);

  useEffect(() => {
    load();
  }, [load]);

  const unconfigured = PROVIDER_NAMES.filter((name) => !providers.some((p) => p.provider === name));

  const runTest = async (provider: AIProviderName) => {
    setTesting(provider);
    try {
      const result = await testAIProvider(provider);
      setResults((current) => ({ ...current, [provider]: result }));
      await load();
    } catch (err) {
      setResults((current) => ({
        ...current,
        [provider]: { ok: false, error: errorMessage(err, 'Test failed'), latencyMs: 0 },
      }));
    } finally {
      setTesting(null);
    }
  };

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!form) return;
    const info = AI_PROVIDERS[form.provider];
    const existing = providers.find((p) => p.provider === form.provider);
    setSaving(true);
    try {
      await saveAIProvider(form.provider, {
        // Left blank while editing = keep the saved key.
        apiKey: form.editing && !form.apiKey.trim() ? undefined : form.apiKey.trim(),
        baseUrl: info.selfHosted ? form.baseUrl.trim() : undefined,
        model: form.model.trim(),
        priority: existing?.priority ?? Math.max(-1, ...providers.map((p) => p.priority)) + 1,
        enabled: existing?.enabled ?? true,
      });
      setForm(null);
      await load();
      // Check it straight away so a typo shows up here, not as a failed email.
      await runTest(form.provider);
    } catch (err) {
      toast.error(`Could not save ${info.label}`, { description: errorMessage(err, 'Try again.') });
    } finally {
      setSaving(false);
    }
  };

  const remove = async (provider: AIProvider) => {
    if (!window.confirm(`Remove ${AI_PROVIDERS[provider.provider].label} and its saved key?`)) return;
    try {
      await deleteAIProvider(provider.provider);
      await load();
    } catch (err) {
      toast.error('Could not remove the provider', { description: errorMessage(err, 'Try again.') });
    }
  };

  // Saves a changed order/enabled flag without touching the key.
  const resave = (provider: AIProvider, changes: Partial<Pick<AIProvider, 'priority' | 'enabled'>>) =>
    saveAIProvider(provider.provider, {
      baseUrl: provider.baseUrl,
      model: provider.model,
      priority: changes.priority ?? provider.priority,
      enabled: changes.enabled ?? provider.enabled,
    });

  const move = async (index: number, delta: -1 | 1) => {
    const a = providers[index];
    const b = providers[index + delta];
    if (!a || !b) return;
    try {
      // Positions, not the stored numbers, so ties can't leave the order unchanged.
      await Promise.all([resave(a, { priority: index + delta }), resave(b, { priority: index })]);
      await load();
    } catch (err) {
      toast.error('Could not reorder', { description: errorMessage(err, 'Try again.') });
    }
  };

  const toggle = async (provider: AIProvider) => {
    try {
      await resave(provider, { enabled: !provider.enabled });
      await load();
    } catch (err) {
      toast.error('Could not update the provider', { description: errorMessage(err, 'Try again.') });
    }
  };

  const status = (provider: AIProvider) => {
    const result = results[provider.provider];
    if (testing === provider.provider) return <small>Testing…</small>;
    if (result && !result.ok) {
      return (
        <small className={styles.negative}>
          <WarningCircle size={12} weight="bold" /> {result.error}
        </small>
      );
    }
    if (provider.lastError) {
      return (
        <small className={styles.negative}>
          <WarningCircle size={12} weight="bold" /> {provider.lastError}
        </small>
      );
    }
    if (provider.lastVerifiedAt) {
      return (
        <small>
          <CheckCircle size={12} weight="fill" color="var(--color-success)" /> Working
          {result?.latencyMs ? ` · ${(result.latencyMs / 1000).toFixed(1)}s` : ''}
        </small>
      );
    }
    return <small>Not tested yet</small>;
  };

  if (loading) return <p className={styles.hint}>Loading AI providers…</p>;

  return (
    <div className={styles.form}>
      {providers.length === 0 && readOnly && (
        <p className={styles.hint}>No AI providers are set up for this account.</p>
      )}

      {providers.length > 0 && (
        <ul className={styles.list}>
          {providers.map((provider, index) => (
            <li key={provider.id} className={styles.listItem} style={provider.enabled ? undefined : { opacity: 0.6 }}>
              <div className={styles.grow}>
                <strong>{AI_PROVIDERS[provider.provider].label}</strong>
                {index === 0 && provider.enabled && <small style={{ display: 'inline', marginLeft: 6 }}>primary</small>}
                <small>
                  {provider.model}
                  {provider.apiKeyHint ? ` · key ••••${provider.apiKeyHint}` : ''}
                  {provider.baseUrl ? ` · ${provider.baseUrl}` : ''}
                </small>
                {status(provider)}
              </div>
              {!readOnly && (
                <>
                  <button type="button" className={styles.iconButton} aria-label="Move up" disabled={index === 0} onClick={() => move(index, -1)}>
                    <ArrowUp size={16} />
                  </button>
                  <button
                    type="button"
                    className={styles.iconButton}
                    aria-label="Move down"
                    disabled={index === providers.length - 1}
                    onClick={() => move(index, 1)}>
                    <ArrowDown size={16} />
                  </button>
                  <button type="button" className={styles.iconButton} aria-label="Test" disabled={testing !== null} onClick={() => runTest(provider.provider)}>
                    <Lightning size={16} />
                  </button>
                  <button
                    type="button"
                    className={styles.iconButton}
                    aria-label="Edit"
                    onClick={() =>
                      setForm({
                        provider: provider.provider,
                        apiKey: '',
                        baseUrl: provider.baseUrl ?? '',
                        model: provider.model,
                        editing: true,
                      })
                    }>
                    <PencilSimple size={16} />
                  </button>
                  <button type="button" className={styles.ghost} onClick={() => toggle(provider)}>
                    {provider.enabled ? 'Disable' : 'Enable'}
                  </button>
                  <button type="button" className={styles.iconButton} aria-label="Remove" onClick={() => remove(provider)}>
                    <Trash size={16} />
                  </button>
                </>
              )}
            </li>
          ))}
        </ul>
      )}

      {providers.length > 1 && (
        <p className={styles.hint}>Providers are tried top to bottom; if one fails, the next takes over.</p>
      )}

      {!readOnly && !form && unconfigured.length > 0 && (
        <div className={styles.chips}>
          {unconfigured.map((name) => (
            <button key={name} type="button" className={styles.segment} onClick={() => setForm(newForm(name))}>
              + {AI_PROVIDERS[name].label}
            </button>
          ))}
        </div>
      )}

      {form && (
        <form className={`${styles.panel} ${styles.form}`} onSubmit={submit}>
          <strong>
            {form.editing ? 'Edit' : 'Add'} {AI_PROVIDERS[form.provider].label}
          </strong>
          {AI_PROVIDERS[form.provider].selfHosted && (
            <label className={styles.field}>
              <span className={styles.label}>Server URL</span>
              <input
                className={styles.input}
                value={form.baseUrl}
                placeholder="https://ollama.example.com"
                onChange={(event) => setForm({ ...form, baseUrl: event.target.value })}
              />
              <span className={styles.hint}>Must be reachable from the internet; private addresses are refused.</span>
            </label>
          )}
          <label className={styles.field}>
            <span className={styles.label}>
              API key
              {AI_PROVIDERS[form.provider].keyUrl && (
                <>
                  {' · '}
                  <a href={AI_PROVIDERS[form.provider].keyUrl} target="_blank" rel="noreferrer">
                    get one
                  </a>
                </>
              )}
            </span>
            <input
              className={styles.input}
              type="password"
              autoComplete="off"
              value={form.apiKey}
              placeholder={form.editing ? 'Leave blank to keep the saved key' : AI_PROVIDERS[form.provider].keyPlaceholder}
              onChange={(event) => setForm({ ...form, apiKey: event.target.value })}
            />
          </label>
          <label className={styles.field}>
            <span className={styles.label}>Model</span>
            <input
              className={styles.input}
              value={form.model}
              onChange={(event) => setForm({ ...form, model: event.target.value })}
            />
            <span className={styles.hint}>A small, cheap model is plenty for reading bank emails.</span>
          </label>
          <div className={styles.segmented}>
            <button
              type="submit"
              className={styles.primary}
              disabled={
                saving ||
                !form.model.trim() ||
                (AI_PROVIDERS[form.provider].selfHosted ? !form.baseUrl.trim() : !form.editing && !form.apiKey.trim())
              }>
              {saving ? 'Saving…' : 'Save and test'}
            </button>
            <button type="button" className={styles.ghost} onClick={() => setForm(null)} disabled={saving}>
              Cancel
            </button>
          </div>
        </form>
      )}
    </div>
  );
}
