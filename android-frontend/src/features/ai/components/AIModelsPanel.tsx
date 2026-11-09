import { useCallback, useEffect, useState } from 'react';
import { Alert, Modal, Pressable, ScrollView, StyleSheet, Switch, TextInput, View } from 'react-native';
import { AppText } from '../../../components/AppText';
import { Button } from '../../../components/Button';
import { Card } from '../../../components/Card';
import { PickerField, PickerOverlay } from '../../../components/Picker';
import { Sheet, SheetOverlay } from '../../../components/Sheet';
import { colors, radii, spacing } from '../../../theme';
import {
  AI_PROVIDERS,
  EMBEDDING_SWITCH_WARNING,
  MODEL_KINDS,
  PROVIDER_NAMES,
  defaultSlug,
  errorText,
  llmApi,
  servesKind,
  type AIModel,
  type AIModelKind,
  type AIModelTestResult,
  type AIProvider,
  type AIProviderName
} from '../llmSetup';

interface KeyForm {
  provider: AIProviderName;
  apiKey: string;
  baseUrl: string;
  editing: boolean;
}

interface ModelForm {
  kind: AIModelKind;
  id?: string;
  provider: AIProviderName | '';
  slug: string;
  endpoint: string;
  makeDefault: boolean;
}

type Tone = 'good' | 'bad' | 'neutral' | 'accent';

function Pill({ label, tone }: { label: string; tone: Tone }) {
  return (
    <View style={[styles.pill, tone === 'good' && styles.pillGood, tone === 'bad' && styles.pillBad, tone === 'accent' && styles.pillAccent]}>
      <AppText
        variant="caption"
        weight="medium"
        tone={tone === 'good' ? 'success' : tone === 'bad' ? 'danger' : tone === 'accent' ? 'primary' : 'muted'}
      >
        {label}
      </AppText>
    </View>
  );
}

function Mark({ provider }: { provider: AIProviderName }) {
  return (
    <View style={styles.mark}>
      <AppText variant="caption" weight="semibold">{AI_PROVIDERS[provider].mark}</AppText>
    </View>
  );
}

function confirm(title: string, message: string, action: string, onConfirm: () => void) {
  Alert.alert(title, message, [
    { text: 'Cancel', style: 'cancel' },
    { text: action, style: 'destructive', onPress: onConfirm }
  ]);
}

function Field({
  label,
  hint,
  ...input
}: {
  label: string;
  hint?: string;
} & React.ComponentProps<typeof TextInput>) {
  return (
    <View style={styles.field}>
      <AppText variant="label" tone="faint">{label}</AppText>
      <TextInput
        {...input}
        autoCapitalize="none"
        autoCorrect={false}
        placeholderTextColor={colors.faint}
        style={styles.input}
      />
      {hint ? <AppText variant="caption" muted>{hint}</AppText> : null}
    </View>
  );
}

/** The user's AI keys and the models run with them. Models are tested before they're saved. */
export function AIModelsPanel() {
  const [providers, setProviders] = useState<AIProvider[]>([]);
  const [models, setModels] = useState<AIModel[]>([]);
  const [loading, setLoading] = useState(true);
  const [keyForm, setKeyForm] = useState<KeyForm | null>(null);
  const [modelForm, setModelForm] = useState<ModelForm | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [pickingKey, setPickingKey] = useState(false);
  const [actionsFor, setActionsFor] = useState<AIModel | null>(null);
  const [testing, setTesting] = useState<string | null>(null);
  const [results, setResults] = useState<Record<string, AIModelTestResult>>({});

  const load = useCallback(async () => {
    try {
      const [keys, list] = await Promise.all([llmApi.providers(), llmApi.models()]);
      setProviders(keys ?? []);
      setModels(list ?? []);
    } catch (err) {
      Alert.alert('Could not load your AI setup', errorText(err, 'Try again.'));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  /** Runs one change, reloads, and reports a failure. */
  const act = async (failure: string, change: () => Promise<unknown>) => {
    try {
      await change();
      await load();
    } catch (err) {
      Alert.alert(failure, errorText(err, 'Try again.'));
    }
  };

  const unconfigured = PROVIDER_NAMES.filter((name) => !providers.some((p) => p.provider === name));

  const openNewModel = (kind: AIModelKind) => {
    const eligible = providers.filter((p) => servesKind(kind, p.provider));
    const first = eligible.find((p) => p.enabled) ?? eligible[0];
    setFormError(null);
    setModelForm({
      kind,
      provider: first?.provider ?? '',
      slug: first ? defaultSlug(kind, first.provider) : '',
      endpoint: '',
      makeDefault: false
    });
  };

  const submitKey = async () => {
    if (!keyForm) return;
    const info = AI_PROVIDERS[keyForm.provider];
    const existing = providers.find((p) => p.provider === keyForm.provider);
    const apiKey = keyForm.apiKey.trim();
    setSaving(true);
    try {
      await llmApi.saveProvider(keyForm.provider, {
        // Left blank while editing = keep the saved key.
        apiKey: keyForm.editing && !apiKey ? undefined : apiKey,
        baseUrl: info.selfHosted ? keyForm.baseUrl.trim() : undefined,
        enabled: existing?.enabled ?? true
      });
      const added = !keyForm.editing;
      const provider = keyForm.provider;
      setKeyForm(null);
      await load();
      // A new key does nothing without a model: offer the first one straight away.
      if (added) {
        setFormError(null);
        setModelForm({ kind: 'chat', provider, slug: info.defaultChatModel, endpoint: '', makeDefault: false });
      }
    } catch (err) {
      Alert.alert(`Could not save the ${info.label} key`, errorText(err, 'Try again.'));
    } finally {
      setSaving(false);
    }
  };

  const submitModel = async () => {
    if (!modelForm?.provider) return;
    setSaving(true);
    setFormError(null);
    try {
      const endpoint = modelForm.kind === 'decision' ? modelForm.endpoint.trim() : undefined;
      if (modelForm.id) {
        await llmApi.updateModel(modelForm.id, { slug: modelForm.slug.trim(), endpoint });
      } else {
        await llmApi.createModel({
          provider: modelForm.provider,
          kind: modelForm.kind,
          slug: modelForm.slug.trim(),
          endpoint,
          makeDefault: modelForm.makeDefault
        });
      }
      setModelForm(null);
      await load();
    } catch (err) {
      // A failed test (422) is the common case: show it next to the fields.
      setFormError(errorText(err, 'Could not save the model.'));
    } finally {
      setSaving(false);
    }
  };

  const runTest = async (model: AIModel) => {
    setTesting(model.id);
    try {
      const result = await llmApi.testModel(model.id);
      setResults((current) => ({ ...current, [model.id]: result }));
      await load();
    } catch (err) {
      setResults((current) => ({
        ...current,
        [model.id]: { ok: false, error: errorText(err, 'Test failed'), latencyMs: 0 }
      }));
    } finally {
      setTesting(null);
    }
  };

  const openKeyActions = (provider: AIProvider) => {
    const label = AI_PROVIDERS[provider.provider].label;
    const count = models.filter((m) => m.providerId === provider.id).length;
    const also = count > 0 ? ` and the ${count === 1 ? 'model' : `${count} models`} that use it` : '';
    Alert.alert(`${label} key`, undefined, [
      {
        text: 'Replace key',
        onPress: () =>
          setKeyForm({ provider: provider.provider, apiKey: '', baseUrl: provider.baseUrl ?? '', editing: true })
      },
      {
        text: provider.enabled ? 'Disable' : 'Enable',
        onPress: () =>
          void act('Could not update the key', () =>
            llmApi.saveProvider(provider.provider, { baseUrl: provider.baseUrl, enabled: !provider.enabled })
          )
      },
      {
        text: 'Remove',
        style: 'destructive',
        onPress: () =>
          confirm('Remove key', `Remove your ${label} key${also}?`, 'Remove', () =>
            void act('Could not remove the key', () => llmApi.deleteProvider(provider.provider))
          )
      }
    ]);
  };

  const statusOf = (model: AIModel): { pill: { label: string; tone: Tone }; error?: string } => {
    if (testing === model.id) return { pill: { label: 'Testing', tone: 'neutral' } };
    const result = results[model.id];
    const error = result && !result.ok ? result.error : model.lastError;
    if (error) return { pill: { label: 'Failing', tone: 'bad' }, error };
    if (model.lastVerifiedAt) return { pill: { label: 'Working', tone: 'good' } };
    return { pill: { label: 'Not tested', tone: 'neutral' } };
  };

  if (loading) {
    return (
      <Card>
        <AppText variant="caption" muted>Loading your AI setup…</AppText>
      </Card>
    );
  }

  const keyLabel = (name: AIProviderName | '') => {
    const key = providers.find((p) => p.provider === name);
    return key ? `${AI_PROVIDERS[key.provider].label} key${key.enabled ? '' : ' (disabled)'}` : undefined;
  };

  const actionList = actionsFor ? models.filter((m) => m.kind === actionsFor.kind) : [];
  const actionIndex = actionsFor ? actionList.findIndex((m) => m.id === actionsFor.id) : -1;
  const canMoveUp = !!actionsFor && !actionsFor.isDefault && actionIndex > 0 && !actionList[actionIndex - 1].isDefault;
  const canMoveDown = !!actionsFor && !actionsFor.isDefault && actionIndex >= 0 && actionIndex < actionList.length - 1;

  const move = (direction: -1 | 1) => {
    if (!actionsFor) return;
    const ids = actionList.map((m) => m.id);
    [ids[actionIndex], ids[actionIndex + direction]] = [ids[actionIndex + direction], ids[actionIndex]];
    const kind = actionsFor.kind;
    setActionsFor(null);
    void act('Could not change the order', () => llmApi.reorder(kind, ids));
  };

  return (
    <View style={styles.panel}>
      <View style={styles.group}>
        <View style={styles.groupHeader}>
          <View style={styles.groupText}>
            <AppText variant="heading">Keys</AppText>
            <AppText variant="caption" muted>One per provider. A key can back several models.</AppText>
          </View>
          {unconfigured.length > 0 ? (
            <Button
              size="sm"
              variant="secondary"
              onPress={() => setKeyForm({ provider: unconfigured[0], apiKey: '', baseUrl: '', editing: false })}
            >
              Add key
            </Button>
          ) : null}
        </View>
        {providers.length === 0 ? (
          <AppText variant="caption" muted>No keys yet. Add one to use your own AI.</AppText>
        ) : (
          <Card style={styles.list}>
            {providers.map((provider, index) => (
              <Pressable
                key={provider.id}
                accessibilityRole="button"
                style={({ pressed }) => [styles.row, index > 0 && styles.divider, pressed && styles.pressed, !provider.enabled && styles.off]}
                onPress={() => openKeyActions(provider)}
              >
                <Mark provider={provider.provider} />
                <View style={styles.rowText}>
                  <AppText weight="medium" numberOfLines={1}>{AI_PROVIDERS[provider.provider].label}</AppText>
                  <AppText variant="caption" muted numberOfLines={1}>
                    {provider.baseUrl || (provider.apiKeyHint ? `••••${provider.apiKeyHint}` : 'No key saved')}
                  </AppText>
                </View>
                {!provider.enabled ? <Pill label="Disabled" tone="neutral" /> : null}
              </Pressable>
            ))}
          </Card>
        )}
      </View>

      {MODEL_KINDS.map(({ kind, title, description, noun, empty }) => {
        const list = models.filter((m) => m.kind === kind);
        const canAdd = providers.some((p) => servesKind(kind, p.provider));
        return (
          <View key={kind} style={styles.group}>
            <View style={styles.groupHeader}>
              <View style={styles.groupText}>
                <AppText variant="heading">{title}</AppText>
                <AppText variant="caption" muted>{description}</AppText>
              </View>
              <Button size="sm" variant="secondary" disabled={!canAdd} onPress={() => openNewModel(kind)}>
                Add
              </Button>
            </View>
            {list.length === 0 ? (
              <AppText variant="caption" muted>
                {canAdd ? empty : kind === 'embedding' ? 'Add an OpenRouter or Ollama key first.' : 'Add a key first.'}
              </AppText>
            ) : (
              <Card style={styles.list}>
                {list.map((model, index) => {
                  const { pill, error } = statusOf(model);
                  return (
                    <Pressable
                      key={model.id}
                      accessibilityRole="button"
                      accessibilityLabel={`${model.slug} ${noun}`}
                      style={({ pressed }) => [styles.row, index > 0 && styles.divider, pressed && styles.pressed, !model.enabled && styles.off]}
                      onPress={() => setActionsFor(model)}
                    >
                      <Mark provider={model.provider} />
                      <View style={styles.rowText}>
                        <AppText weight="medium" numberOfLines={1}>{model.slug}</AppText>
                        <View style={styles.pills}>
                          {model.isDefault ? <Pill label="Default" tone="accent" /> : null}
                          {!model.enabled ? <Pill label="Disabled" tone="neutral" /> : null}
                          <Pill label={pill.label} tone={pill.tone} />
                        </View>
                        {model.endpoint ? (
                          <AppText variant="caption" tone="faint" numberOfLines={1}>{model.endpoint}</AppText>
                        ) : null}
                        {error ? (
                          <AppText variant="caption" tone="danger" numberOfLines={3}>{error}</AppText>
                        ) : null}
                      </View>
                    </Pressable>
                  );
                })}
              </Card>
            )}
          </View>
        );
      })}

      {actionsFor ? (
        <Sheet visible title={actionsFor.slug} subtitle={AI_PROVIDERS[actionsFor.provider].label} onClose={() => setActionsFor(null)}>
          <View style={styles.actions}>
            <Button
              variant="secondary"
              disabled={testing !== null}
              onPress={() => {
                const model = actionsFor;
                setActionsFor(null);
                void runTest(model);
              }}
            >
              Test
            </Button>
            {!actionsFor.isDefault && actionsFor.enabled ? (
              <Button
                variant="secondary"
                onPress={() => {
                  const model = actionsFor;
                  const apply = () => void act('Could not change the default', () => llmApi.setDefault(model.id));
                  setActionsFor(null);
                  if (model.kind === 'embedding') confirm('Switch embedding model', EMBEDDING_SWITCH_WARNING, 'Switch', apply);
                  else apply();
                }}
              >
                Make default
              </Button>
            ) : null}
            <Button
              variant="secondary"
              onPress={() => {
                const model = actionsFor;
                setActionsFor(null);
                void act(`Could not ${model.enabled ? 'disable' : 'enable'} the model`, () =>
                  llmApi.setEnabled(model.id, !model.enabled)
                );
              }}
            >
              {actionsFor.enabled ? 'Disable' : 'Enable'}
            </Button>
            {canMoveUp ? <Button variant="secondary" onPress={() => move(-1)}>Move up</Button> : null}
            {canMoveDown ? <Button variant="secondary" onPress={() => move(1)}>Move down</Button> : null}
            <Button
              variant="secondary"
              onPress={() => {
                const model = actionsFor;
                setActionsFor(null);
                setFormError(null);
                setModelForm({
                  kind: model.kind,
                  id: model.id,
                  provider: model.provider,
                  slug: model.slug,
                  endpoint: model.endpoint ?? '',
                  makeDefault: false
                });
              }}
            >
              Edit
            </Button>
            <Button
              variant="danger"
              onPress={() => {
                const model = actionsFor;
                setActionsFor(null);
                confirm('Remove model', `Remove ${model.slug}? Pipeline steps that used it go back to your default.`, 'Remove', () =>
                  void act('Could not remove the model', () => llmApi.deleteModel(model.id))
                );
              }}
            >
              Remove
            </Button>
          </View>
        </Sheet>
      ) : null}

      {keyForm ? (
        <Sheet
          visible
          title={keyForm.editing ? `Replace ${AI_PROVIDERS[keyForm.provider].label} key` : 'Add a key'}
          onClose={() => setKeyForm(null)}
          footer={
            <Button
              disabled={saving || (!keyForm.editing && !AI_PROVIDERS[keyForm.provider].selfHosted && !keyForm.apiKey.trim())}
              onPress={() => void submitKey()}
            >
              {saving ? 'Saving…' : 'Save key'}
            </Button>
          }
        >
          <ScrollView keyboardShouldPersistTaps="handled" contentContainerStyle={styles.form}>
            {!keyForm.editing ? (
              <View style={styles.segments}>
                {unconfigured.map((name) => (
                  <Pressable
                    key={name}
                    accessibilityRole="button"
                    accessibilityState={{ selected: keyForm.provider === name }}
                    style={[styles.segment, keyForm.provider === name && styles.segmentOn]}
                    onPress={() => setKeyForm({ ...keyForm, provider: name })}
                  >
                    <AppText variant="caption" weight="medium" tone={keyForm.provider === name ? 'primary' : 'muted'}>
                      {AI_PROVIDERS[name].label}
                    </AppText>
                  </Pressable>
                ))}
              </View>
            ) : null}
            <Field
              label={AI_PROVIDERS[keyForm.provider].selfHosted ? 'API key (optional)' : 'API key'}
              value={keyForm.apiKey}
              secureTextEntry
              placeholder={keyForm.editing ? 'Leave blank to keep the saved key' : undefined}
              onChangeText={(apiKey) => setKeyForm({ ...keyForm, apiKey })}
            />
            {AI_PROVIDERS[keyForm.provider].selfHosted ? (
              <Field
                label="Server URL"
                value={keyForm.baseUrl}
                placeholder="http://192.168.1.20:11434"
                keyboardType="url"
                onChangeText={(baseUrl) => setKeyForm({ ...keyForm, baseUrl })}
              />
            ) : null}
          </ScrollView>
        </Sheet>
      ) : null}

      {modelForm ? (
        <Modal visible transparent statusBarTranslucent navigationBarTranslucent animationType="slide" onRequestClose={() => setModelForm(null)}>
        <SheetOverlay
          title={`${modelForm.id ? 'Edit' : 'Add a'} ${MODEL_KINDS.find((k) => k.kind === modelForm.kind)?.noun}`}
          onClose={() => setModelForm(null)}
          footer={
            <Button
              disabled={saving || !modelForm.provider || !modelForm.slug.trim() || (modelForm.kind === 'decision' && !modelForm.endpoint.trim())}
              onPress={() => void submitModel()}
            >
              {saving ? 'Testing…' : 'Test and save'}
            </Button>
          }
        >
          <ScrollView keyboardShouldPersistTaps="handled" contentContainerStyle={styles.form}>
            {modelForm.id ? (
              <AppText variant="caption" muted>
                {keyLabel(modelForm.provider)}. To use another key, add the model again.
              </AppText>
            ) : (
              <PickerField label="Key" value={keyLabel(modelForm.provider)} placeholder="Choose a key" onPress={() => setPickingKey(true)} />
            )}
            <Field
              label="Model"
              value={modelForm.slug}
              placeholder={modelForm.kind === 'decision' ? 'provider/model' : undefined}
              onChangeText={(slug) => setModelForm({ ...modelForm, slug })}
            />
            {modelForm.kind === 'decision' ? (
              <Field
                label="Endpoint"
                value={modelForm.endpoint}
                placeholder="https://…/decisions"
                keyboardType="url"
                hint="https only; plain http works only for a server on your own network."
                onChangeText={(endpoint) => setModelForm({ ...modelForm, endpoint })}
              />
            ) : null}
            {modelForm.kind !== 'chat' ? (
              <View style={styles.warning}>
                <AppText variant="caption">
                  {modelForm.kind === 'decision'
                    ? 'This provider must serve decision models at this endpoint. Your key is sent to this address, so only use one you trust.'
                    : `The model must return 1024 dimensions. ${EMBEDDING_SWITCH_WARNING}`}
                </AppText>
              </View>
            ) : null}
            {!modelForm.id && models.some((m) => m.kind === modelForm.kind) ? (
              <View style={styles.switchRow}>
                <AppText style={styles.groupText}>Make this the default</AppText>
                <Switch value={modelForm.makeDefault} onValueChange={(makeDefault) => setModelForm({ ...modelForm, makeDefault })} />
              </View>
            ) : null}
            {formError ? (
              <View style={styles.error}>
                <AppText variant="caption" tone="danger">{formError}</AppText>
              </View>
            ) : null}
          </ScrollView>
        </SheetOverlay>
          {pickingKey ? (
            <PickerOverlay
              title="Key"
              options={providers
                .filter((p) => servesKind(modelForm.kind, p.provider))
                .map((p) => ({ id: p.provider, label: keyLabel(p.provider) ?? p.provider, sublabel: p.apiKeyHint ? `••••${p.apiKeyHint}` : undefined }))}
              selectedId={modelForm.provider}
              onClose={() => setPickingKey(false)}
              onSelect={(id) => {
                setPickingKey(false);
                if (!id) return;
                const provider = id as AIProviderName;
                setModelForm({ ...modelForm, provider, slug: defaultSlug(modelForm.kind, provider) || modelForm.slug });
              }}
            />
          ) : null}
        </Modal>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  panel: { gap: spacing.xl },
  group: { gap: spacing.md },
  groupHeader: { flexDirection: 'row', alignItems: 'center', gap: spacing.md },
  groupText: { flex: 1, minWidth: 0 },
  list: { paddingVertical: spacing.xs },
  row: { flexDirection: 'row', alignItems: 'center', gap: spacing.md, paddingVertical: spacing.md },
  divider: { borderTopWidth: StyleSheet.hairlineWidth, borderTopColor: colors.border },
  pressed: { opacity: 0.7 },
  off: { opacity: 0.55 },
  rowText: { flex: 1, minWidth: 0, gap: spacing.xs },
  mark: {
    width: 34,
    height: 34,
    borderRadius: radii.sm,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.surfaceStrong
  },
  pills: { flexDirection: 'row', flexWrap: 'wrap', gap: spacing.xs },
  pill: { paddingHorizontal: spacing.sm, paddingVertical: 2, borderRadius: radii.full, backgroundColor: colors.surfaceStrong },
  pillGood: { backgroundColor: colors.successMuted },
  pillBad: { backgroundColor: colors.dangerMuted },
  pillAccent: { backgroundColor: colors.primaryMuted },
  actions: { gap: spacing.sm },
  form: { gap: spacing.lg },
  field: { gap: spacing.xs },
  input: {
    minHeight: 46,
    paddingHorizontal: spacing.md,
    borderRadius: radii.md,
    backgroundColor: colors.surfaceStrong,
    color: colors.text
  },
  segments: { flexDirection: 'row', flexWrap: 'wrap', gap: spacing.sm },
  segment: { paddingHorizontal: spacing.md, paddingVertical: spacing.sm, borderRadius: radii.full, backgroundColor: colors.surfaceStrong },
  segmentOn: { backgroundColor: colors.primaryMuted },
  warning: { padding: spacing.md, borderRadius: radii.md, backgroundColor: colors.warningMuted },
  error: { padding: spacing.md, borderRadius: radii.md, backgroundColor: colors.dangerMuted },
  switchRow: { flexDirection: 'row', alignItems: 'center', gap: spacing.md }
});
