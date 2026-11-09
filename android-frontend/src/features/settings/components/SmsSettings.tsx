import { useEffect, useState } from 'react';
import { Alert, Linking, StyleSheet, Switch, TextInput, View } from 'react-native';
import { AppText } from '../../../components/AppText';
import { Button } from '../../../components/Button';
import { getSmsTestMode, setSmsTestMode } from '../../sms/smsForwarder';
import { importRecentSms, SMS_IMPORT_DAYS } from '../../sms/smsInbox';
import { ensureSmsPermission, hasSmsPermission } from '../../sms/smsPermission';
import { colors, radii, spacing } from '../../../theme';

/**
 * Bank SMS forwarding: permission state, a one-off inbox import, and test mode,
 * which points SMS (and only SMS) at another ingest service, e.g. a dev machine on Tailscale.
 */
export function SmsSettings() {
  const [granted, setGranted] = useState<boolean | null>(null);
  const [importing, setImporting] = useState(false);
  const [testEnabled, setTestEnabled] = useState(false);
  const [testUrl, setTestUrl] = useState('');

  useEffect(() => {
    void hasSmsPermission().then(setGranted);
    void getSmsTestMode().then(({ enabled, url }) => {
      setTestEnabled(enabled);
      setTestUrl(url);
    });
  }, []);

  const allow = async () => {
    const ok = await ensureSmsPermission();
    setGranted(ok);
    if (!ok) {
      Alert.alert('SMS access is off', 'Allow SMS for Pennywise in system settings.', [
        { text: 'Cancel', style: 'cancel' },
        { text: 'Open settings', onPress: () => void Linking.openSettings() }
      ]);
    }
  };

  const runImport = async () => {
    setImporting(true);
    try {
      const result = await importRecentSms();
      if ('denied' in result) {
        Alert.alert('SMS access is off', 'Importing needs permission to read SMS.');
        return;
      }
      const found = `${result.matched} bank SMS among ${result.scanned} from the last ${SMS_IMPORT_DAYS} days.`;
      if (result.error) {
        const left = result.queued ? ` ${result.queued} will retry later.` : '';
        Alert.alert(
          "Couldn't send SMS",
          `${found}\nSent ${result.sent} to ${result.destination} before it failed:\n${result.error}${left}`
        );
      } else {
        Alert.alert('Import done', `${found}\nSent ${result.sent} to ${result.destination}.`);
      }
    } catch (err) {
      Alert.alert("Couldn't import SMS", err instanceof Error ? err.message : 'Please try again.');
    } finally {
      setImporting(false);
    }
  };

  const toggleTest = (enabled: boolean) => {
    if (enabled && !/^https?:\/\/\S+$/.test(testUrl.trim())) {
      Alert.alert('Test server needed', 'Enter the test ingest URL first, e.g. https://my-pc.tailnet.ts.net/api');
      return;
    }
    setTestEnabled(enabled);
    void setSmsTestMode({ enabled, url: testUrl });
  };

  const importLabel = `Sends bank SMS from the last ${SMS_IMPORT_DAYS} days, every time.`;

  return (
    <>
      <View style={styles.row}>
        <View style={styles.main}>
          <AppText weight="medium">Forward bank SMS</AppText>
          <AppText variant="caption" muted>
            {granted ? 'On. Messages from phone numbers never leave the phone.' : 'Needs SMS permission.'}
          </AppText>
        </View>
        {granted === false ? (
          <Button size="sm" variant="secondary" onPress={() => void allow()}>
            Allow
          </Button>
        ) : null}
      </View>

      <View style={[styles.action, styles.divider]}>
        <AppText variant="caption" muted>{importLabel}</AppText>
        <Button size="sm" variant="secondary" disabled={importing} onPress={() => void runImport()}>
          {importing ? 'Importing…' : 'Import recent messages'}
        </Button>
      </View>

      <View style={[styles.action, styles.divider]}>
        <View style={styles.toggleRow}>
          <View style={styles.main}>
            <AppText weight="medium">Test mode</AppText>
            <AppText variant="caption" muted>Send SMS to this ingest service instead. Everything else stays on the main API.</AppText>
          </View>
          <Switch
            value={testEnabled}
            onValueChange={toggleTest}
            trackColor={{ true: colors.primary, false: colors.surfaceTertiary }}
            thumbColor={colors.text}
          />
        </View>
        <TextInput
          value={testUrl}
          onChangeText={setTestUrl}
          onEndEditing={() => void setSmsTestMode({ enabled: testEnabled, url: testUrl })}
          placeholder="https://my-pc.tailnet.ts.net/api"
          placeholderTextColor={colors.faint}
          autoCapitalize="none"
          autoCorrect={false}
          keyboardType="url"
          style={styles.input}
        />
        <AppText variant="caption" tone="faint">
          Connect this phone to Tailscale, and use the https URL from `tailscale serve` with /api on the end. Plain http is blocked.
        </AppText>
      </View>
    </>
  );
}

const styles = StyleSheet.create({
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md,
    paddingVertical: spacing.md
  },
  toggleRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md
  },
  main: {
    flex: 1,
    gap: 2
  },
  action: {
    gap: spacing.sm,
    paddingVertical: spacing.md
  },
  divider: {
    borderTopWidth: StyleSheet.hairlineWidth,
    borderTopColor: colors.border
  },
  input: {
    color: colors.text,
    fontSize: 15,
    backgroundColor: colors.surfaceStrong,
    borderRadius: radii.sm,
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.sm
  }
});
