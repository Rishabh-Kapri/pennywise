import SmsReceiver from '../../../modules/sms-receiver';
import { forwardSms, type ForwardResult } from './smsForwarder';
import { ensureReadSmsPermission } from './smsPermission';

/** How far back an import reaches. Every import covers the full window again. */
export const SMS_IMPORT_DAYS = 30;
const IMPORT_LIMIT = 500;

export type ImportResult = (ForwardResult & { scanned: number }) | { denied: true };

/**
 * Forwards bank SMS from the last {@link SMS_IMPORT_DAYS} days of the inbox.
 * Re-importing resends them, as do messages also caught live; the ingest side
 * dedupes on sender + receivedAt.
 */
export async function importRecentSms(): Promise<ImportResult> {
  if (!SmsReceiver) throw new Error('This build has no SMS support. Install a newer APK.');
  if (!(await ensureReadSmsPermission())) return { denied: true };

  const since = Date.now() - SMS_IMPORT_DAYS * 24 * 60 * 60 * 1000;
  const messages = await SmsReceiver.getInboxMessages(since, IMPORT_LIMIT);
  return { scanned: messages.length, ...(await forwardSms(messages, 'inbox')) };
}
