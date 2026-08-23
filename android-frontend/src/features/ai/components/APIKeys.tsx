import { useCallback, useEffect, useState } from 'react';
import { StyleSheet, TextInput, View } from 'react-native';
import { AppText } from '../../../components/AppText';
import { Button } from '../../../components/Button';
import { Card } from '../../../components/Card';
import { colors, radii, spacing } from '../../../theme';
import { apiClient } from '../../../utils/api';
import type { APIKey } from '../types';
import { formatDate } from '../utils';

function KeyRow({ apiKey, isLast }: { apiKey: APIKey; isLast: boolean }) {
  return (
    <View style={[styles.keyRow, !isLast && styles.rowDivider]}>
      <View style={styles.keyMain}>
        <AppText weight="medium" numberOfLines={1}>{apiKey.name}</AppText>
        {apiKey.maskedKey ? (
          <AppText variant="caption" tone="faint" style={styles.mono} numberOfLines={1}>
            {apiKey.maskedKey}
          </AppText>
        ) : null}
        {apiKey.description ? (
          <AppText variant="caption" muted numberOfLines={2}>{apiKey.description}</AppText>
        ) : null}
        <View style={styles.keyMeta}>
          <AppText variant="caption" tone="faint">{apiKey.scopes?.join(', ') ?? 'read'}</AppText>
          {apiKey.rateLimit ? (
            <AppText variant="caption" tone="faint">{apiKey.rateLimit}/min</AppText>
          ) : null}
          {apiKey.expiresAt ? (
            <AppText variant="caption" tone="faint">expires {formatDate(apiKey.expiresAt)}</AppText>
          ) : null}
          {apiKey.lastUsedAt ? (
            <AppText variant="caption" tone="faint">used {formatDate(apiKey.lastUsedAt)}</AppText>
          ) : null}
        </View>
      </View>
    </View>
  );
}

export function APIKeys() {
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [newKeyName, setNewKeyName] = useState('');
  const [createdKey, setCreatedKey] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadKeys = useCallback(async () => {
    try {
      setKeys((await apiClient.get<APIKey[]>('keys')) ?? []);
    } catch {
      setKeys([]);
    }
  }, []);

  useEffect(() => {
    void loadKeys().finally(() => setLoading(false));
  }, [loadKeys]);

  const handleCreate = async () => {
    const name = newKeyName.trim();
    if (!name || creating) return;
    setCreating(true);
    setCreatedKey(null);
    setError(null);
    try {
      const fullKey = await apiClient.post<string>('keys', { name });
      setCreatedKey(typeof fullKey === 'string' ? fullKey : JSON.stringify(fullKey));
      setNewKeyName('');
      await loadKeys();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to create API key');
    } finally {
      setCreating(false);
    }
  };

  return (
    <View style={styles.section}>
      <View style={styles.sectionHead}>
        <AppText variant="label" tone="faint">API keys</AppText>
        <AppText variant="caption" muted>
          {keys.length} key{keys.length === 1 ? '' : 's'}
        </AppText>
      </View>

      <Card style={styles.card}>
        {loading ? (
          <AppText variant="caption" muted>Loading keys…</AppText>
        ) : keys.length === 0 ? (
          <AppText variant="caption" muted>No API keys created yet.</AppText>
        ) : (
          keys.map((apiKey, index) => (
            <KeyRow key={apiKey.id} apiKey={apiKey} isLast={index === keys.length - 1} />
          ))
        )}
      </Card>

      {createdKey ? (
        <Card style={styles.newKeyCard}>
          <AppText variant="caption" weight="semibold" tone="primary">
            New key created — copy it now, it won't be shown again
          </AppText>
          <View style={styles.newKeyValue}>
            <AppText variant="caption" style={styles.mono} selectable>{createdKey}</AppText>
          </View>
          <AppText variant="caption" tone="faint">
            Long-press to copy, then store it securely. You won't be able to see it again.
          </AppText>
        </Card>
      ) : null}

      <Card style={styles.createCard}>
        <TextInput
          value={newKeyName}
          onChangeText={setNewKeyName}
          onSubmitEditing={() => void handleCreate()}
          returnKeyType="done"
          editable={!creating}
          accessibilityLabel="New API key name"
          placeholder="Key name (e.g. 'Mobile App')"
          placeholderTextColor={colors.faint}
          style={styles.input}
        />
        {error ? <AppText variant="caption" tone="danger">{error}</AppText> : null}
        <Button size="sm" onPress={() => void handleCreate()} disabled={creating || !newKeyName.trim()}>
          {creating ? 'Creating…' : 'Create key'}
        </Button>
      </Card>
    </View>
  );
}

const styles = StyleSheet.create({
  section: {
    gap: spacing.sm
  },
  sectionHead: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginLeft: spacing.xs
  },
  card: {
    paddingVertical: spacing.xs
  },
  keyRow: {
    paddingVertical: spacing.md
  },
  rowDivider: {
    borderBottomWidth: StyleSheet.hairlineWidth,
    borderBottomColor: colors.border
  },
  keyMain: {
    gap: 2
  },
  keyMeta: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: spacing.md,
    marginTop: 2
  },
  newKeyCard: {
    gap: spacing.sm,
    backgroundColor: colors.primaryMuted
  },
  newKeyValue: {
    padding: spacing.md,
    borderRadius: radii.md,
    backgroundColor: colors.surface
  },
  createCard: {
    gap: spacing.md
  },
  input: {
    minHeight: 44,
    paddingHorizontal: spacing.md,
    borderRadius: radii.md,
    backgroundColor: colors.surfaceStrong,
    color: colors.text,
    fontSize: 15
  },
  mono: {
    fontFamily: 'monospace'
  }
});
