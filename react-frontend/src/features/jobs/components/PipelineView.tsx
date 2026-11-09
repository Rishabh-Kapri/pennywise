import { AI_PROVIDERS } from '@/features/onboarding/aiProviders';
import { fetchAIModels, fetchAIProviders } from '@/features/onboarding/api';
import { SearchSelect } from '@/features/transactions/components/popovers/SearchSelect';
import { usableModels, type AIModel, type AIModelKind } from '@/features/onboarding/types';
import { ApiError } from '@/utils/api';
import { errorMessage, toast } from '@/utils';
import { ArrowCounterClockwise, Lock, PencilSimple, Warning } from '@phosphor-icons/react';
import { AnimatePresence, motion } from 'framer-motion';
import { useCallback, useEffect, useState, type FormEvent } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { fetchPipelines, resetPipelineStep, updatePipelineStep } from '../pipelineApi';
import type {
  PipelineAdapter,
  PipelineCatalog,
  PipelineId,
  PipelineModelRef,
  PipelineStep,
  PipelineStepChoice,
} from '../types/pipeline.types';
import jobStyles from './Jobs.module.css';
import styles from './Pipeline.module.css';

const EASE_OUT = [0.22, 1, 0.36, 1] as const;

const KIND_NOUN: Record<AIModelKind, string> = {
  chat: 'chat model',
  decision: 'decision model',
  embedding: 'embedding model',
};

interface FormState {
  adapter: string;
  /** '' = the user's default model of the adapter's kind. */
  modelId: string;
  fallback: boolean;
}

function adapterOf(step: PipelineStep, id: string): PipelineAdapter | undefined {
  return step.adapters.find((adapter) => adapter.id === id);
}

/** A step is locked when there's nothing to choose: one adapter, no model. */
function isLocked(step: PipelineStep): boolean {
  const choosable = step.adapters.filter((adapter) => !adapter.unavailable);
  return choosable.length <= 1 && !choosable.some((adapter) => adapter.usesModel);
}

function providerLabel(model: { provider: PipelineModelRef['provider'] }): string {
  return AI_PROVIDERS[model.provider]?.label ?? model.provider;
}

function describeChoice(step: PipelineStep, choice: PipelineStepChoice): string {
  const adapter = adapterOf(step, choice.adapter);
  const label = adapter?.label ?? choice.adapter;
  if (!adapter?.usesModel) return label;
  if (!choice.model) return `${label} · no ${KIND_NOUN[adapter.modelKind ?? 'chat']} set up`;
  const fallback = choice.fallback ? ' · falls back to your other models' : '';
  return `${label} · ${choice.model.slug} · your ${providerLabel(choice.model)} key${fallback}`;
}

function formFor(step: PipelineStep): FormState {
  return {
    adapter: step.override?.adapter ?? step.defaultAdapter,
    modelId: step.override?.modelId ?? '',
    fallback: step.override?.fallback ?? step.fallback ?? false,
  };
}

interface StepRowProps {
  index: number;
  step: PipelineStep;
  pipelineId: PipelineId;
  defaults: PipelineCatalog['defaults'];
  /** The user's models on enabled keys. */
  models: AIModel[];
  editing: boolean;
  onEdit: () => void;
  onClose: () => void;
  onSaved: (step: PipelineStep) => void;
}

function StepRow({ index, step, pipelineId, defaults, models, editing, onEdit, onClose, onSaved }: StepRowProps) {
  const [form, setForm] = useState<FormState>(() => formFor(step));
  const [saving, setSaving] = useState(false);
  const locked = isLocked(step);
  const changed = step.override !== null;
  const chosenAdapter = adapterOf(step, form.adapter);
  const kind = chosenAdapter?.modelKind;
  const choices = kind ? models.filter((m) => m.kind === kind) : [];
  const kindDefault = kind ? defaults[kind] : undefined;

  const startEditing = () => {
    setForm(formFor(step));
    onEdit();
  };

  const save = async (event: FormEvent) => {
    event.preventDefault();
    const usesModel = !!chosenAdapter?.usesModel;
    const payload = {
      adapter: form.adapter === step.defaultAdapter ? null : form.adapter,
      // A model of another kind can't carry over when the adapter changes.
      modelId: usesModel && choices.some((m) => m.id === form.modelId) ? form.modelId : null,
      fallback: usesModel && form.fallback !== (step.fallback ?? false) ? form.fallback : null,
    };
    setSaving(true);
    try {
      // Choosing every default is the same as having no override.
      const nothingChanged = payload.adapter === null && payload.modelId === null && payload.fallback === null;
      const updated = nothingChanged
        ? await resetPipelineStep(pipelineId, step.id)
        : await updatePipelineStep(pipelineId, step.id, payload);
      onSaved(updated);
      onClose();
      toast.success(`${step.label} updated`, { description: 'New messages will use it.' });
    } catch (err) {
      toast.error(`Could not update ${step.label}`, { description: errorMessage(err, 'Try again.') });
    } finally {
      setSaving(false);
    }
  };

  const reset = async () => {
    setSaving(true);
    try {
      onSaved(await resetPipelineStep(pipelineId, step.id));
      onClose();
      toast.success(`${step.label} reset`, { description: 'It follows the default again.' });
    } catch (err) {
      toast.error(`Could not reset ${step.label}`, { description: errorMessage(err, 'Try again.') });
    } finally {
      setSaving(false);
    }
  };

  const modelItems = [
    { id: '', name: 'Your default', trailing: kindDefault ? kindDefault.slug : 'none set up' },
    ...choices.map((m) => ({ id: m.id, name: m.slug, trailing: `${providerLabel(m)} key` })),
  ];

  const defaultText = kindDefault
    ? `Your default (${kindDefault.slug} · ${providerLabel(kindDefault)})`
    : 'Your default (none set up)';

  return (
    <li className={styles.step}>
      <div className={styles.stepHead}>
        <span className={`${styles.number} ${locked ? styles.numberLocked : ''}`}>{index + 1}</span>
        <div className={styles.stepText}>
          <span className={styles.stepTitle}>
            {step.label}
            {changed && <span className={styles.badge}>Changed</span>}
          </span>
          <span className={styles.stepDescription}>{step.description}</span>
          <span className={styles.choice}>{describeChoice(step, step.effective)}</span>
          {step.warning && (
            <span className={styles.warning}>
              <Warning size={14} weight="fill" />
              {step.warning}
            </span>
          )}
        </div>
        {locked ? (
          <span className={styles.locked} title="This step has nothing to choose.">
            <Lock size={14} />
            Fixed
          </span>
        ) : (
          !editing && (
            <button type="button" className={jobStyles.button} onClick={startEditing}>
              <PencilSimple size={16} />
              Edit
            </button>
          )
        )}
      </div>

      <AnimatePresence initial={false}>
        {editing && (
          <motion.div
            className={styles.editorWrap}
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: 'auto', opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            transition={{ duration: 0.2, ease: EASE_OUT }}
          >
            <form className={styles.editor} onSubmit={save}>
              {step.adapters.length > 1 && (
                <fieldset className={styles.adapters}>
                  <legend className={styles.fieldLabel}>How it runs</legend>
                  {step.adapters.map((adapter) => (
                    <label
                      key={adapter.id}
                      className={`${styles.adapter} ${form.adapter === adapter.id ? styles.adapterOn : ''} ${
                        adapter.unavailable ? styles.adapterOff : ''
                      }`}
                    >
                      <input
                        type="radio"
                        name={`adapter-${step.id}`}
                        value={adapter.id}
                        checked={form.adapter === adapter.id}
                        disabled={adapter.unavailable}
                        onChange={() => setForm({ ...form, adapter: adapter.id, modelId: '' })}
                      />
                      <span className={styles.adapterText}>
                        <span className={styles.adapterLabel}>
                          {adapter.label}
                          {adapter.id === step.defaultAdapter && <span className={styles.muted}> · default</span>}
                          {adapter.unavailable && <span className={styles.muted}> · coming later</span>}
                        </span>
                        <span className={styles.muted}>
                          {adapter.description}
                          {adapter.modelKind && ` Uses a ${KIND_NOUN[adapter.modelKind]}.`}
                        </span>
                      </span>
                    </label>
                  ))}
                </fieldset>
              )}

              {chosenAdapter?.usesModel &&
                kind &&
                (choices.length === 0 ? (
                  <p className={styles.notice}>
                    This needs a {KIND_NOUN[kind]}, and none is set up.{' '}
                    <Link to="/settings?section=ai">Add one in Settings → AI</Link>.
                  </p>
                ) : (
                  <div className={styles.modelFields}>
                    <div className={styles.field}>
                      <span className={styles.fieldLabel}>{kind === 'chat' ? 'Chat model' : 'Decision model'}</span>
                      <SearchSelect
                        variant="form"
                        value={modelItems.find((item) => item.id === form.modelId)?.name ?? defaultText}
                        items={modelItems}
                        placeholder="Your default"
                        emptyText="No models match"
                        onSelect={(item) => setForm({ ...form, modelId: item.id })}
                      />
                      <span className={styles.hint}>
                        Only your {KIND_NOUN[kind]}s are listed. Add others in Settings → AI.
                      </span>
                    </div>
                    <label className={styles.check}>
                      <input
                        type="checkbox"
                        checked={form.fallback}
                        onChange={(event) => setForm({ ...form, fallback: event.target.checked })}
                      />
                      <span>
                        If this model fails, try my other {KIND_NOUN[kind]}s
                        <span className={styles.hint}>In the order set in Settings → AI.</span>
                      </span>
                    </label>
                  </div>
                ))}

              <div className={styles.editorActions}>
                <button type="submit" className={jobStyles.primaryButton} disabled={saving}>
                  Save
                </button>
                <button type="button" className={jobStyles.button} disabled={saving} onClick={onClose}>
                  Cancel
                </button>
                {changed && (
                  <button
                    type="button"
                    className={`${jobStyles.button} ${styles.reset}`}
                    disabled={saving}
                    onClick={reset}
                  >
                    <ArrowCounterClockwise size={16} />
                    Reset to default
                  </button>
                )}
              </div>
            </form>
          </motion.div>
        )}
      </AnimatePresence>
    </li>
  );
}

/**
 * Every system pipeline for this budget (messages today; statements, PDFs and
 * background agents later). Pick one to see its steps in order and change
 * them. Only changed fields are stored, so steps nobody edited keep following
 * the system default as it changes.
 */
export function PipelineView() {
  const [catalog, setCatalog] = useState<PipelineCatalog | null>(null);
  const [models, setModels] = useState<AIModel[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [searchParams, setSearchParams] = useSearchParams();

  const load = useCallback(async () => {
    try {
      const [loaded, keys, aiModels] = await Promise.all([fetchPipelines(), fetchAIProviders(), fetchAIModels()]);
      setCatalog(loaded);
      setModels(usableModels(keys ?? [], aiModels ?? []));
      setError(null);
    } catch (err) {
      setError(
        err instanceof ApiError && err.status === 404
          ? "This server doesn't support pipeline settings yet."
          : errorMessage(err, "Couldn't load the pipelines."),
      );
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  if (error) return <div className={jobStyles.empty}>{error}</div>;
  if (!catalog) return <div className={jobStyles.empty}>Loading pipelines…</div>;
  if (catalog.pipelines.length === 0) return <div className={jobStyles.empty}>No pipelines on this server.</div>;

  const selected = catalog.pipelines.find((p) => p.id === searchParams.get('pipeline')) ?? catalog.pipelines[0];

  const selectPipeline = (id: PipelineId) => {
    setEditingId(null);
    setSearchParams(
      (params) => {
        params.set('pipeline', id);
        return params;
      },
      { replace: true },
    );
  };

  const replaceStep = (updated: PipelineStep) =>
    setCatalog((current) =>
      current
        ? {
            ...current,
            pipelines: current.pipelines.map((p) =>
              p.id === selected.id ? { ...p, steps: p.steps.map((s) => (s.id === updated.id ? updated : s)) } : p,
            ),
          }
        : current,
    );

  const changedIn = (steps: PipelineStep[]) => steps.filter((step) => step.override).length;
  const changedCount = changedIn(selected.steps);

  return (
    <div className={styles.root}>
      <div className={styles.pipelines} role="tablist" aria-label="Pipelines">
        {catalog.pipelines.map((p) => {
          const changed = changedIn(p.steps);
          const warned = p.steps.some((step) => step.warning);
          return (
            <button
              key={p.id}
              type="button"
              role="tab"
              aria-selected={p.id === selected.id}
              className={`${styles.pipelineCard} ${p.id === selected.id ? styles.pipelineCardOn : ''}`}
              onClick={() => selectPipeline(p.id)}
            >
              <span className={styles.pipelineName}>
                {p.label}
                {warned && <Warning size={14} weight="fill" className={styles.warnIcon} />}
              </span>
              <span className={styles.pipelineMeta}>
                {p.steps.length} steps{changed > 0 && ` · ${changed} changed`}
              </span>
            </button>
          );
        })}
      </div>

      <p className={styles.intro}>
        {selected.description} Steps run in this order, and changes apply to what this budget processes next.
        {changedCount > 0 &&
          ` ${changedCount} ${changedCount === 1 ? 'step differs' : 'steps differ'} from the default.`}
      </p>
      <ol className={styles.steps}>
        {selected.steps.map((step, index) => (
          <StepRow
            key={`${selected.id}:${step.id}`}
            index={index}
            step={step}
            pipelineId={selected.id}
            defaults={catalog.defaults ?? {}}
            models={models}
            editing={editingId === step.id}
            onEdit={() => setEditingId(step.id)}
            onClose={() => setEditingId(null)}
            onSaved={replaceStep}
          />
        ))}
      </ol>
      <p className={styles.footnote}>
        {selected.label} pipeline, version {selected.version}
      </p>
    </div>
  );
}
