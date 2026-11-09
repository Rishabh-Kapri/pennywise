import { useState } from 'react';
import { Alert, StyleSheet, View } from 'react-native';
import * as Updates from 'expo-updates';
import { AppText } from '../../../components/AppText';
import { Button } from '../../../components/Button';
import { colors, spacing } from '../../../theme';

/**
 * Manual OTA updates. The app never checks on its own
 * (`updates.checkAutomatically: 'NEVER'` in app.config.ts), so this is the
 * only way a published update reaches the device.
 */
export function AppUpdates() {
  const [busy, setBusy] = useState(false);

  const running = Updates.isEmbeddedLaunch
    ? 'Built-in'
    : `${Updates.updateId?.slice(0, 8) ?? '—'}${
        Updates.createdAt
          ? ` · ${Updates.createdAt.toLocaleDateString('en-IN', { day: 'numeric', month: 'short' })}`
          : ''
      }`;

  const check = async () => {
    setBusy(true);
    try {
      const result = await Updates.checkForUpdateAsync();
      if (!result.isAvailable) {
        Alert.alert('Up to date', `No newer update on ${Updates.channel ?? 'this channel'}.`);
        return;
      }
      await Updates.fetchUpdateAsync();
      Alert.alert('Update downloaded', 'Restart now to use it, or it applies the next time the app starts.', [
        { text: 'Later', style: 'cancel' },
        { text: 'Restart', onPress: () => void Updates.reloadAsync() }
      ]);
    } catch (err) {
      Alert.alert("Couldn't check for updates", err instanceof Error ? err.message : 'Please try again.');
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <View style={[styles.row, styles.divider]}>
        <AppText variant="caption" muted>Running</AppText>
        <AppText variant="caption" weight="medium" tabular>
          {running}
          {Updates.channel ? ` (${Updates.channel})` : ''}
        </AppText>
      </View>
      <View style={[styles.action, styles.divider]}>
        <AppText variant="caption" muted>
          {Updates.isEnabled ? 'Updates install only when you ask.' : 'Updates are off in development.'}
        </AppText>
        <Button size="sm" variant="secondary" disabled={busy || !Updates.isEnabled} onPress={() => void check()}>
          {busy ? 'Checking…' : 'Check for updates'}
        </Button>
      </View>
    </>
  );
}

const styles = StyleSheet.create({
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: spacing.lg,
    paddingVertical: spacing.md
  },
  // stacked: side by side, the caption pushed the button out of the card
  action: {
    gap: spacing.sm,
    paddingVertical: spacing.md
  },
  divider: {
    borderTopWidth: StyleSheet.hairlineWidth,
    borderTopColor: colors.border
  }
});
