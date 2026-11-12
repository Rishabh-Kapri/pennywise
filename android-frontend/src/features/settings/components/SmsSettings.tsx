import { useEffect, useState } from 'react';
import { Alert, Linking, StyleSheet, View } from 'react-native';
import { AppText } from '../../../components/AppText';
import { Button } from '../../../components/Button';
import { importRecentSms, SMS_IMPORT_DAYS } from '../../sms/smsInbox';
import { ensureSmsPermission, hasSmsPermission } from '../../sms/smsPermission';
import { colors, spacing } from '../../../theme';

/** Bank SMS forwarding: permission state and a one-off inbox import. */
export function SmsSettings() {
  const [granted, setGranted] = useState<boolean | null>(null);
  const [importing, setImporting] = useState(false);

  useEffect(() => {
    void hasSmsPermission().then(setGranted);
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
      if (result.signedOut) {
        Alert.alert("Couldn't send SMS", `${found}\nYou're signed out, so none were sent. Sign in and import again.`);
      } else if (result.error) {
        const left = result.queued ? ` ${result.queued} will retry later.` : '';
        const lost = result.dropped
          ? ` ${result.dropped} didn't fit in the retry queue; import again once sending works.`
          : '';
        Alert.alert(
          "Couldn't send SMS",
          `${found}\nSent ${result.sent} to ${result.destination} before it failed:\n${result.error}${left}${lost}`
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
  }
});
