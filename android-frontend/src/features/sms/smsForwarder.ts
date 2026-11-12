import AsyncStorage from '@react-native-async-storage/async-storage';
import { config } from '../../config/env';
import { headlessApi, NotAuthenticatedError } from '../../utils/headlessApi';

/**
 * Forwards bank SMS to the ingest service: live ones from the headless task that
 * `modules/sms-receiver` starts (task `SmsReceived`), and inbox imports from
 * Settings. Runs headlessly, so it may only use headlessApi.
 *
 * Every message goes into an AsyncStorage outbox first, then the outbox is
 * drained. A message stays in the outbox until ingest accepts it, the user is
 * signed out, or the outbox is full. Two rules keep concurrent callers (the live
 * task, the app-start flush and an inbox import) from losing messages:
 *  - every read-modify-write of the outbox runs under one lock (`mutateOutbox`);
 *  - only one drain sends at a time (`drain`), removing each message by id as
 *    soon as ingest accepts it.
 */

/** Must match `SmsHeadlessTaskService.TASK_KEY`. */
export const SMS_TASK_KEY = 'SmsReceived';

export const SMS_INGEST_PATH = 'ingest/sms';
export const SMS_OUTBOX_KEY = 'pennywise_sms_outbox';
export const SMS_OUTBOX_MAX = 50;

export type SmsMessage = { sender?: string | null; body?: string | null; timestamp?: number };

export type SmsSource = 'live' | 'inbox';

export type SmsPayload = { sender: string; body: string; receivedAt: string; source: SmsSource };

/** A message waiting in the outbox. Leaving the outbox means sent or dropped. */
export type OutboxItem = { id: string; payload: SmsPayload; attempts: number; lastError?: string };

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

let idCounter = 0;
function newItemId(): string {
  idCounter += 1;
  return `${Date.now().toString(36)}-${idCounter.toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
}

function isOutboxItem(value: unknown): value is OutboxItem {
  return Boolean(value && typeof value === 'object' && 'id' in value && 'payload' in value);
}

async function readOutbox(): Promise<OutboxItem[]> {
  try {
    const raw = await AsyncStorage.getItem(SMS_OUTBOX_KEY);
    const list = raw ? (JSON.parse(raw) as unknown[]) : [];
    // Builds before the outbox had ids stored bare payloads. Their id comes
    // from the payload, so it is the same on every read until it's rewritten.
    return list.map((entry) =>
      isOutboxItem(entry) ? entry : { id: `legacy:${JSON.stringify(entry)}`, payload: entry as SmsPayload, attempts: 0 }
    );
  } catch {
    return [];
  }
}

async function writeOutbox(items: OutboxItem[]): Promise<void> {
  if (items.length === 0) await AsyncStorage.removeItem(SMS_OUTBOX_KEY);
  else await AsyncStorage.setItem(SMS_OUTBOX_KEY, JSON.stringify(items));
}

let outboxLock: Promise<unknown> = Promise.resolve();

/**
 * Reads the outbox, applies `change` and writes the result, with no other
 * change in between. Every write to the outbox goes through here.
 */
function mutateOutbox<T>(change: (items: OutboxItem[]) => { items: OutboxItem[]; result: T }): Promise<T> {
  const run = outboxLock.then(async () => {
    const { items, result } = change(await readOutbox());
    await writeOutbox(items);
    return result;
  });
  outboxLock = run.catch(() => undefined);
  return run;
}

/**
 * Keeps the outbox within `SMS_OUTBOX_MAX`. Inbox imports go first, since
 * importing again brings them back; then the oldest live messages. Returns the
 * kept items and the ids it dropped.
 */
export function trimOutbox(items: OutboxItem[], max = SMS_OUTBOX_MAX): { kept: OutboxItem[]; dropped: string[] } {
  let excess = items.length - max;
  if (excess <= 0) return { kept: items, dropped: [] };
  const dropped = new Set<string>();
  for (const source of ['inbox', 'live'] as const) {
    for (const item of items) {
      if (excess === 0) break;
      if (item.payload.source === source) {
        dropped.add(item.id);
        excess--;
      }
    }
  }
  return { kept: items.filter((item) => !dropped.has(item.id)), dropped: [...dropped] };
}

/** Adds items to the end of the outbox, trimming it to its cap. */
function enqueue(added: OutboxItem[]): Promise<void> {
  return mutateOutbox((items) => ({ items: trimOutbox([...items, ...added]).kept, result: undefined }));
}

/** How a drain ended. */
export type DrainResult = { kind: 'empty' } | { kind: 'stopped'; error: string } | { kind: 'signedOut' };

/**
 * Sends the outbox in order, removing each message once ingest accepts it, and
 * stops at the first failure. Signed out means nobody to send for, so the
 * outbox is cleared.
 */
async function runDrain(): Promise<DrainResult> {
  for (;;) {
    const [next] = await readOutbox();
    if (!next) return { kind: 'empty' };
    try {
      await headlessApi.post(SMS_INGEST_PATH, next.payload, { baseUrl: config.ingestBaseUrl });
    } catch (error) {
      if (error instanceof NotAuthenticatedError) {
        await mutateOutbox(() => ({ items: [], result: undefined }));
        return { kind: 'signedOut' };
      }
      const message = error instanceof Error ? error.message : String(error);
      console.log('[sms] send failed, keeping in outbox', message);
      await mutateOutbox((items) => ({
        items: items.map((item) =>
          item.id === next.id ? { ...item, attempts: item.attempts + 1, lastError: message } : item
        ),
        result: undefined
      }));
      return { kind: 'stopped', error: message };
    }
    await mutateOutbox((items) => ({ items: items.filter((item) => item.id !== next.id), result: undefined }));
    if (watched.has(next.id)) accepted.add(next.id);
  }
}

let drainChain: Promise<unknown> = Promise.resolve();

/**
 * Ids a `forwardSms` call is waiting on, and those of them ingest accepted.
 * Lets each call count its own sent messages exactly, even when another call's
 * drain sent them or another call's enqueue trimmed them.
 */
const watched = new Set<string>();
const accepted = new Set<string>();

/**
 * Runs a drain after any drain already running, so only one sends at a time.
 * The later drain finds whatever the earlier one left.
 */
function drain(): Promise<DrainResult> {
  const run = drainChain.then(runDrain);
  drainChain = run.catch(() => undefined);
  return run;
}

export type ForwardResult = {
  /** Bank messages among the input. */
  matched: number;
  /** Of those, how many the server accepted. */
  sent: number;
  /** Of those, how many wait in the outbox for a retry. */
  queued: number;
  /** Of those, how many were thrown away: the outbox was full, or signed out. */
  dropped: number;
  /** True when the user was signed out, so nothing could be sent. */
  signedOut: boolean;
  /** Base URL the messages went to. */
  destination: string;
  /** Why sending stopped, when it did. */
  error?: string;
};

/** Forwards the bank messages among `messages`, after anything already queued. */
export async function forwardSms(messages: SmsMessage[], source: SmsSource): Promise<ForwardResult> {
  const destination = config.ingestBaseUrl;
  const payloads = messages.map((m) => toPayload(m, source)).filter((p): p is SmsPayload => p !== null);
  const empty = { matched: 0, sent: 0, queued: 0, dropped: 0, signedOut: false, destination };
  if (payloads.length === 0) return empty;

  const added = payloads.map((payload) => ({ id: newItemId(), payload, attempts: 0 }));
  const ids = added.map((item) => item.id);
  ids.forEach((id) => watched.add(id));
  try {
    await enqueue(added);
    const outcome = await drain();

    const sent = ids.filter((id) => accepted.has(id)).length;
    if (outcome.kind === 'signedOut') {
      return {
        ...empty,
        matched: payloads.length,
        sent,
        dropped: payloads.length - sent,
        signedOut: true,
        error: 'Signed out'
      };
    }
    // Ours still in the outbox wait for a retry; any others were trimmed.
    const waiting = new Set((await readOutbox()).map((item) => item.id));
    const queued = ids.filter((id) => waiting.has(id)).length;
    return {
      ...empty,
      matched: payloads.length,
      sent,
      queued,
      dropped: payloads.length - sent - queued,
      error: outcome.kind === 'stopped' ? outcome.error : undefined
    };
  } finally {
    ids.forEach((id) => {
      watched.delete(id);
      accepted.delete(id);
    });
  }
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
export async function flushSmsOutbox(): Promise<DrainResult> {
  return drain();
}
