import { useCallback, useEffect, useState } from 'react';
import { Alert, Modal, Pressable, RefreshControl, ScrollView, StyleSheet, Switch, View } from 'react-native';
import { AppText } from '../../../components/AppText';
import { Button } from '../../../components/Button';
import { Card } from '../../../components/Card';
import { PickerField, PickerOverlay } from '../../../components/Picker';
import { Screen } from '../../../components/Screen';
import { SheetOverlay } from '../../../components/Sheet';
import { StackHeader } from '../../../components/StackHeader';
import { colors, radii, spacing } from '../../../theme';
import { apiClient } from '../../../utils/api';
import { AI_PROVIDERS, errorText, llmApi, usableModels, type AIModel, type AIModelKind } from '../../ai/llmSetup';
import type { PipelineAdapter, PipelineCatalog, PipelineStep, PipelineStepChoice } from '../types';

const KIND_NOUN: Record<AIModelKind, string> = {
  chat: 'chat model',
  decision: 'decision model',
  embedding: 'embedding model'
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

function describeChoice(step: PipelineStep, choice: PipelineStepChoice): string {
  const adapter = adapterOf(step, choice.adapter);
  const label = adapter?.label ?? choice.adapter;
  if (!adapter?.usesModel) return label;
  if (!choice.model) return `${label} · no ${KIND_NOUN[adapter.modelKind ?? 'chat']} set up`;
  const fallback = choice.fallback ? ' · falls back to your other models' : '';
  return `${label} · ${choice.model.slug} · your ${AI_PROVIDERS[choice.model.provider]?.label ?? choice.model.provider} key${fallback}`;
}

function formFor(step: PipelineStep): FormState {
  return {
    adapter: step.override?.adapter ?? step.defaultAdapter,
    modelId: step.override?.modelId ?? '',
    fallback: step.override?.fallback ?? step.fallback ?? false
  };
}

function stepPath(pipeline: string, step: string): string {
  return `pipelines/${encodeURIComponent(pipeline)}/steps/${encodeURIComponent(step)}`;
}

function StepEditor({
  step,
  pipelineId,
  defaults,
  models,
  onClose,
  onSaved
}: {
  step: PipelineStep;
  pipelineId: string;
  defaults: PipelineCatalog['defaults'];
  models: AIModel[];
  onClose: () => void;
  onSaved: (step: PipelineStep) => void;
}) {
  const [form, setForm] = useState<FormState>(() => formFor(step));
  const [saving, setSaving] = useState(false);
  const [picking, setPicking] = useState(false);

  const chosen = adapterOf(step, form.adapter);
  const kind = chosen?.modelKind;
  const choices = kind ? models.filter((m) => m.kind === kind) : [];
  const kindDefault = kind ? defaults[kind] : undefined;
  const modelLabel =
    choices.find((m) => m.id === form.modelId)?.slug ??
    (kindDefault ? `Your default (${kindDefault.slug})` : 'Your default (none set up)');

  const run = async (request: () => Promise<PipelineStep>) => {
    setSaving(true);
    try {
      onSaved(await request());
      onClose();
    } catch (err) {
      Alert.alert(`Could not update ${step.label}`, errorText(err, 'Try again.'));
    } finally {
      setSaving(false);
    }
  };

  const save = () => {
    const usesModel = !!chosen?.usesModel;
    const payload = {
      adapter: form.adapter === step.defaultAdapter ? null : form.adapter,
      // A model of another kind can't carry over when the adapter changes.
      modelId: usesModel && choices.some((m) => m.id === form.modelId) ? form.modelId : null,
      fallback: usesModel && form.fallback !== (step.fallback ?? false) ? form.fallback : null
    };
    const nothingChanged = payload.adapter === null && payload.modelId === null && payload.fallback === null;
    void run(() =>
      nothingChanged
        ? apiClient.delete<PipelineStep>(stepPath(pipelineId, step.id))
        : apiClient.put<PipelineStep>(stepPath(pipelineId, step.id), payload)
    );
  };

  return (
    <Modal visible transparent statusBarTranslucent navigationBarTranslucent animationType="slide" onRequestClose={onClose}>
    <SheetOverlay
      title={step.label}
      subtitle={step.description}
      onClose={onClose}
      footer={
        <View style={styles.footer}>
          {step.override ? (
            <Button
              variant="secondary"
              style={styles.flex}
              disabled={saving}
              onPress={() => void run(() => apiClient.delete<PipelineStep>(stepPath(pipelineId, step.id)))}
            >
              Reset
            </Button>
          ) : null}
          <Button style={styles.flex} disabled={saving} onPress={save}>
            {saving ? 'Saving…' : 'Save'}
          </Button>
        </View>
      }
    >
      <ScrollView keyboardShouldPersistTaps="handled" contentContainerStyle={styles.form}>
        {step.adapters.length > 1 ? (
          <View style={styles.adapters}>
            <AppText variant="label" tone="faint">How it runs</AppText>
            {step.adapters.map((adapter) => {
              const on = form.adapter === adapter.id;
              return (
                <Pressable
                  key={adapter.id}
                  accessibilityRole="radio"
                  accessibilityState={{ selected: on, disabled: adapter.unavailable }}
                  disabled={adapter.unavailable}
                  style={[styles.adapter, on && styles.adapterOn, adapter.unavailable && styles.off]}
                  onPress={() => setForm({ ...form, adapter: adapter.id, modelId: '' })}
                >
                  <AppText weight="medium">
                    {adapter.label}
                    {adapter.id === step.defaultAdapter ? ' · default' : ''}
                    {adapter.unavailable ? ' · coming later' : ''}
                  </AppText>
                  <AppText variant="caption" muted>
                    {adapter.description}
                    {adapter.modelKind ? ` Uses a ${KIND_NOUN[adapter.modelKind]}.` : ''}
                  </AppText>
                </Pressable>
              );
            })}
          </View>
        ) : null}

        {chosen?.usesModel && kind ? (
          choices.length === 0 ? (
            <AppText variant="caption" muted>
              This needs a {KIND_NOUN[kind]}, and none is set up. Add one in AI & Predictions.
            </AppText>
          ) : (
            <>
              <PickerField
                label={kind === 'chat' ? 'Chat model' : kind === 'decision' ? 'Decision model' : 'Embedding model'}
                value={modelLabel}
                onPress={() => setPicking(true)}
              />
              <View style={styles.switchRow}>
                <View style={styles.flex}>
                  <AppText>If this model fails, try my other {KIND_NOUN[kind]}s</AppText>
                  <AppText variant="caption" muted>In the order set in AI & Predictions.</AppText>
                </View>
                <Switch value={form.fallback} onValueChange={(fallback) => setForm({ ...form, fallback })} />
              </View>
            </>
          )
        ) : null}
      </ScrollView>
    </SheetOverlay>
      {picking ? (
        <PickerOverlay
          title="Model"
          options={[
            { id: 'default', label: 'Your default', sublabel: kindDefault?.slug ?? 'none set up' },
            ...choices.map((m) => ({ id: m.id, label: m.slug, sublabel: `${AI_PROVIDERS[m.provider].label} key` }))
          ]}
          selectedId={form.modelId || 'default'}
          onClose={() => setPicking(false)}
          onSelect={(id) => {
            setPicking(false);
            setForm({ ...form, modelId: !id || id === 'default' ? '' : id });
          }}
        />
      ) : null}
    </Modal>
  );
}

/** Every system pipeline for this budget. Only changed fields are stored. */
export function PipelinesScreen() {
  const [catalog, setCatalog] = useState<PipelineCatalog | null>(null);
  const [models, setModels] = useState<AIModel[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [refreshing, setRefreshing] = useState(false);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [editing, setEditing] = useState<PipelineStep | null>(null);

  const load = useCallback(async () => {
    try {
      const [loaded, keys, aiModels] = await Promise.all([
        apiClient.get<PipelineCatalog>('pipelines'),
        llmApi.providers(),
        llmApi.models()
      ]);
      setCatalog(loaded);
      setModels(usableModels(keys ?? [], aiModels ?? []));
      setError(null);
    } catch (err) {
      setError(errorText(err, "Couldn't load the pipelines."));
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const refresh = async () => {
    setRefreshing(true);
    await load();
    setRefreshing(false);
  };

  const selected = catalog?.pipelines.find((p) => p.id === selectedId) ?? catalog?.pipelines[0];

  const replaceStep = (updated: PipelineStep) => {
    setCatalog((current) =>
      current && selected
        ? {
            ...current,
            pipelines: current.pipelines.map((p) =>
              p.id === selected.id ? { ...p, steps: p.steps.map((s) => (s.id === updated.id ? updated : s)) } : p
            )
          }
        : current
    );
  };

  return (
    <Screen
      style={styles.screen}
      refreshControl={<RefreshControl refreshing={refreshing} onRefresh={() => void refresh()} tintColor={colors.primary} />}
    >
      <StackHeader title="Pipelines" subtitle="How this budget's messages are processed" />

      {error ? (
        <Card>
          <AppText variant="caption" tone="danger">{error}</AppText>
        </Card>
      ) : !catalog || !selected ? (
        <Card>
          <AppText variant="caption" muted>{catalog ? 'No pipelines on this server.' : 'Loading pipelines…'}</AppText>
        </Card>
      ) : (
        <>
          {catalog.pipelines.length > 1 ? (
            <View style={styles.chips}>
              {catalog.pipelines.map((p) => (
                <Pressable
                  key={p.id}
                  accessibilityRole="tab"
                  accessibilityState={{ selected: p.id === selected.id }}
                  style={[styles.chip, p.id === selected.id && styles.chipOn]}
                  onPress={() => setSelectedId(p.id)}
                >
                  <AppText variant="caption" weight="medium" tone={p.id === selected.id ? 'primary' : 'muted'}>{p.label}</AppText>
                </Pressable>
              ))}
            </View>
          ) : null}

          <AppText variant="caption" muted>
            {selected.description} Steps run in this order, and changes apply to what this budget processes next.
          </AppText>

          {selected.steps.map((step, index) => {
            const locked = isLocked(step);
            return (
              <Pressable
                key={step.id}
                accessibilityRole="button"
                disabled={locked}
                style={({ pressed }) => pressed && styles.pressed}
                onPress={() => setEditing(step)}
              >
                <Card style={styles.step}>
                  <View style={styles.number}>
                    <AppText variant="caption" weight="semibold">{index + 1}</AppText>
                  </View>
                  <View style={styles.stepText}>
                    <View style={styles.stepTitle}>
                      <AppText weight="medium">{step.label}</AppText>
                      {step.override ? <AppText variant="caption" tone="primary">Changed</AppText> : null}
                      {locked ? <AppText variant="caption" tone="faint">Fixed</AppText> : null}
                    </View>
                    <AppText variant="caption" muted>{step.description}</AppText>
                    <AppText variant="caption">{describeChoice(step, step.effective)}</AppText>
                    {step.warning ? <AppText variant="caption" tone="danger">{step.warning}</AppText> : null}
                  </View>
                </Card>
              </Pressable>
            );
          })}

          <AppText variant="caption" tone="faint">{selected.label} pipeline, version {selected.version}</AppText>
        </>
      )}

      {editing && selected ? (
        <StepEditor
          key={editing.id}
          step={editing}
          pipelineId={selected.id}
          defaults={catalog?.defaults ?? {}}
          models={models}
          onClose={() => setEditing(null)}
          onSaved={replaceStep}
        />
      ) : null}
    </Screen>
  );
}

const styles = StyleSheet.create({
  screen: { gap: spacing.lg },
  flex: { flex: 1, minWidth: 0 },
  footer: { flexDirection: 'row', gap: spacing.sm },
  form: { gap: spacing.lg },
  adapters: { gap: spacing.sm },
  adapter: { gap: 2, padding: spacing.md, borderRadius: radii.md, backgroundColor: colors.surfaceStrong },
  adapterOn: { backgroundColor: colors.primaryMuted },
  off: { opacity: 0.5 },
  switchRow: { flexDirection: 'row', alignItems: 'center', gap: spacing.md },
  chips: { flexDirection: 'row', flexWrap: 'wrap', gap: spacing.sm },
  chip: { paddingHorizontal: spacing.md, paddingVertical: spacing.sm, borderRadius: radii.full, backgroundColor: colors.surface },
  chipOn: { backgroundColor: colors.primaryMuted },
  pressed: { opacity: 0.7 },
  step: { flexDirection: 'row', gap: spacing.md },
  number: {
    width: 26,
    height: 26,
    borderRadius: 13,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.surfaceStrong
  },
  stepText: { flex: 1, minWidth: 0, gap: 2 },
  stepTitle: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm }
});
