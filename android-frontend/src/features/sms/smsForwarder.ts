import AsyncStorage from '@react-native-async-storage/async-storage';
import { config } from '../../config/env';
import { headlessApi, NotAuthenticatedError } from '../../utils/headlessApi';

/**
 * Forwards bank SMS to the ingest service: live ones from the headless task that
 * `modules/sms-receiver` starts (task `SmsReceived`), and inbox imports from
 * Settings. Runs headlessly, so it may only use headlessApi.
 *
 * Messages that fail to send (offline, server down) wait in an AsyncStorage
 * outbox and go out with the next SMS or on the next app start.
 */

/** Must match `SmsHeadlessTaskService.TASK_KEY`. */
export const SMS_TASK_KEY = 'SmsReceived';

export const SMS_INGEST_PATH = 'ingest/sms';
export const SMS_OUTBOX_KEY = 'pennywise_sms_outbox';
const OUTBOX_MAX = 50;

export type SmsMessage = { sender?: string | null; body?: string | null; timestamp?: number };

export type SmsSource = 'live' | 'inbox';

export type SmsPayload = { sender: string; body: string; receivedAt: string; source: SmsSource };

/**
 * Banks send from alphanumeric sender IDs ("AX-HDFCBK", "JD-ICICIT-S"); people
 * send from phone numbers. Only the former leave the device, so personal
 * messages never reach the server.
 */
export function isLikelyBankSender(sender: string): boolean {
  return /[a-z]/i.test(sender);
}

function toPayload(message: SmsMessage, source: SmsSource): SmsPayload | null {
  const sender = message.sender ?? '';
  const body = message.body ?? '';
  if (!body || !isLikelyBankSender(sender)) return null;
  return { sender, body, receivedAt: new Date(message.timestamp || Date.now()).toISOString(), source };
}

async function readOutbox(): Promise<SmsPayload[]> {
  try {
    const raw = await AsyncStorage.getItem(SMS_OUTBOX_KEY);
    return raw ? (JSON.parse(raw) as SmsPayload[]) : [];
  } catch {
    return [];
  }
}

async function writeOutbox(items: SmsPayload[]): Promise<void> {
  if (items.length === 0) await AsyncStorage.removeItem(SMS_OUTBOX_KEY);
  else await AsyncStorage.setItem(SMS_OUTBOX_KEY, JSON.stringify(items.slice(-OUTBOX_MAX)));
}

type SendResult = { unsent: SmsPayload[]; error?: string };

/**
 * Sends `pending` in order, stopping at the first failure. Returns what is
 * still unsent and why. Signed out means nobody to send for, so the messages
 * are dropped, but the error is still reported.
 */
async function sendAll(pending: SmsPayload[]): Promise<SendResult> {
  for (let i = 0; i < pending.length; i++) {
    try {
      await headlessApi.post(SMS_INGEST_PATH, pending[i], { baseUrl: config.ingestBaseUrl });
    } catch (error) {
      if (error instanceof NotAuthenticatedError) return { unsent: [], error: 'Signed out' };
      console.log('[sms] send failed, keeping in outbox', error);
      return { unsent: pending.slice(i), error: error instanceof Error ? error.message : String(error) };
    }
  }
  return { unsent: [] };
}

export type ForwardResult = {
  /** Bank messages among the input. */
  matched: number;
  /** Of those, how many the server accepted. */
  sent: number;
  /** Of those, how many wait in the outbox for a retry. */
  queued: number;
  /** Base URL the messages went to. */
  destination: string;
  /** Why sending stopped, when it did. */
  error?: string;
};

/** Forwards the bank messages among `messages`, after anything already queued. */
export async function forwardSms(messages: SmsMessage[], source: SmsSource): Promise<ForwardResult> {
  const destination = config.ingestBaseUrl;
  const payloads = messages.map((m) => toPayload(m, source)).filter((p): p is SmsPayload => p !== null);
  if (payloads.length === 0) return { matched: 0, sent: 0, queued: 0, destination };

  const outbox = await readOutbox();
  const { unsent, error } = await sendAll([...outbox, ...payloads]);
  await writeOutbox(unsent);

  // The outbox goes first, so unsent messages are the tail: the new ones fail first.
  const queued = Math.min(unsent.length, payloads.length);
  // Signed out drops everything, so nothing new was accepted.
  const sent = error === 'Signed out' ? 0 : payloads.length - queued;
  return { matched: payloads.length, sent, queued, destination, error };
}

export async function smsTask(message: SmsMessage): Promise<void> {
  // Visible in `adb logcat -s ReactNativeJS`. Phone numbers are never logged.
  const sender = message.sender ?? '';
  if (!isLikelyBankSender(sender)) {
    console.log('[sms] live task: skipped, sender is not a bank ID');
    return;
  }
  console.log(`[sms] live task: ${sender}, ${message.body?.length ?? 0} chars`);
  // Counts, destination and error only; no message text.
  console.log('[sms] live task result', JSON.stringify(await forwardSms([message], 'live')));
}

/** Retries anything left in the outbox. Call when the app starts signed in. */
export async function flushSmsOutbox(): Promise<void> {
  const pending = await readOutbox();
  if (pending.length > 0) await writeOutbox((await sendAll(pending)).unsent);
}
