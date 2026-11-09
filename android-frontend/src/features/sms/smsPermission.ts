import { PermissionsAndroid, Platform, type Permission } from 'react-native';

const RATIONALE =
  'Pennywise forwards transaction SMS from banks (not from phone numbers) to turn them into transactions.';

/**
 * Android stops showing the dialog after a second denial; the user then has to
 * grant it from system settings. Both SMS permissions share one group, so the
 * second one is usually granted without a dialog.
 */
async function ensure(permission: Permission, title: string): Promise<boolean> {
  if (Platform.OS !== 'android') return false;
  if (await PermissionsAndroid.check(permission)) return true;
  const result = await PermissionsAndroid.request(permission, {
    title,
    message: RATIONALE,
    buttonPositive: 'Allow',
    buttonNegative: 'Not now'
  });
  return result === PermissionsAndroid.RESULTS.GRANTED;
}

/** RECEIVE_SMS: lets the manifest receiver in `modules/sms-receiver` get new messages. */
export const ensureSmsPermission = () => ensure(PermissionsAndroid.PERMISSIONS.RECEIVE_SMS, 'Read bank SMS');

/** READ_SMS: needed only to import messages already in the inbox. */
export const ensureReadSmsPermission = () =>
  ensure(PermissionsAndroid.PERMISSIONS.READ_SMS, 'Import bank SMS');

export const hasSmsPermission = () =>
  Platform.OS === 'android' ? PermissionsAndroid.check(PermissionsAndroid.PERMISSIONS.RECEIVE_SMS) : Promise.resolve(false);
