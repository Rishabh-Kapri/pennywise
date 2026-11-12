import assert from 'node:assert';
import AsyncStorage from './stubs/asyncStorage.mjs';
import {
  smsTask,
  forwardSms,
  flushSmsOutbox,
  isLikelyBankSender,
  SMS_OUTBOX_KEY
} from '../src/features/sms/smsForwarder.ts';

const AUTH_KEY = 'pennywise_auth';
const session = {
  user: { id: 'u1' },
  tokens: { accessToken: 'access', refreshToken: 'refresh', expiresAt: Date.now() + 10 * 60 * 1000 }
};

let posted: Array<{ url: string; body: any; auth?: string; budget?: string }> = [];
let serverUp = true;

globalThis.fetch = (async (url: string, init: any = {}) => {
  if (!serverUp) throw new Error('network down');
  posted.push({
    url,
    body: init.body ? JSON.parse(init.body) : undefined,
    auth: init.headers?.Authorization,
    budget: init.headers?.['x-budget-id']
  });
  return { ok: true, status: 202, statusText: '202', text: async () => '{}', json: async () => ({}) };
}) as any;

async function reset(signedIn = true) {
  AsyncStorage.__mem.clear();
  if (signedIn) await AsyncStorage.setItem(AUTH_KEY, JSON.stringify(session));
  await AsyncStorage.setItem('pennywise_selected_budget_id', 'budget-1');
  posted = [];
  serverUp = true;
}

const outbox = async () => JSON.parse((await AsyncStorage.getItem(SMS_OUTBOX_KEY)) ?? '[]');

let failures = 0;
async function test(name: string, fn: () => Promise<void>) {
  try {
    await fn();
    console.log(`  PASS  ${name}`);
  } catch (err) {
    failures++;
    console.log(`  FAIL  ${name}\n        ${(err as Error).message}`);
  }
}

await test('bank sender IDs pass, phone numbers do not', async () => {
  assert.ok(isLikelyBankSender('AX-HDFCBK'));
  assert.ok(isLikelyBankSender('JD-ICICIT-S'));
  assert.ok(!isLikelyBankSender('+919876543210'));
  assert.ok(!isLikelyBankSender('56767'));
  assert.ok(!isLikelyBankSender(''));
});

await test('posts a bank SMS to ingest/sms', async () => {
  await reset();
  await smsTask({ sender: 'AX-HDFCBK', body: 'Rs 500 debited', timestamp: Date.UTC(2026, 9, 30, 10) });
  assert.strictEqual(posted.length, 1);
  assert.ok(posted[0].url.endsWith('/ingest/sms'));
  assert.deepStrictEqual(posted[0].body, {
    sender: 'AX-HDFCBK',
    body: 'Rs 500 debited',
    receivedAt: '2026-10-30T10:00:00.000Z',
    source: 'live'
  });
  assert.deepStrictEqual(await outbox(), []);
});

await test('ignores personal SMS', async () => {
  await reset();
  await smsTask({ sender: '+919876543210', body: 'hey', timestamp: 1 });
  assert.strictEqual(posted.length, 0);
});

await test('failed send is kept and goes out on flush', async () => {
  await reset();
  serverUp = false;
  await smsTask({ sender: 'AX-HDFCBK', body: 'one', timestamp: 1 });
  assert.strictEqual((await outbox()).length, 1);

  serverUp = true;
  await flushSmsOutbox();
  assert.deepStrictEqual(posted.map((p) => p.body.body), ['one']);
  assert.deepStrictEqual(await outbox(), []);
});

await test('queued messages go out before the new one', async () => {
  await reset();
  serverUp = false;
  await smsTask({ sender: 'AX-HDFCBK', body: 'first', timestamp: 1 });
  serverUp = true;
  await smsTask({ sender: 'AX-HDFCBK', body: 'second', timestamp: 2 });
  assert.deepStrictEqual(posted.map((p) => p.body.body), ['first', 'second']);
});

await test('signed out drops the message', async () => {
  await reset(false);
  await smsTask({ sender: 'AX-HDFCBK', body: 'x', timestamp: 1 });
  assert.strictEqual(posted.length, 0);
  assert.deepStrictEqual(await outbox(), []);
});

await test('inbox messages are tagged and counted', async () => {
  await reset();
  const result = await forwardSms(
    [
      { sender: 'AX-HDFCBK', body: 'a', timestamp: 1 },
      { sender: '+919876543210', body: 'personal', timestamp: 2 },
      { sender: 'VM-ICICIT', body: 'b', timestamp: 3 }
    ],
    'inbox'
  );
  assert.strictEqual(result.matched, 2);
  assert.strictEqual(result.sent, 2);
  assert.strictEqual(result.queued, 0);
  assert.strictEqual(result.error, undefined);
  assert.deepStrictEqual(posted.map((p) => [p.body.body, p.body.source]), [['a', 'inbox'], ['b', 'inbox']]);
});

await test('failed sends are reported as queued', async () => {
  await reset();
  serverUp = false;
  const result = await forwardSms([{ sender: 'AX-HDFCBK', body: 'a', timestamp: 1 }], 'inbox');
  assert.strictEqual(result.sent, 0);
  assert.strictEqual(result.queued, 1);
  assert.match(result.error ?? '', /network down/);
});

await test('signed out reports an error instead of counting messages as sent', async () => {
  await reset(false);
  const result = await forwardSms([{ sender: 'AX-HDFCBK', body: 'a', timestamp: 1 }], 'inbox');
  assert.strictEqual(result.sent, 0);
  assert.strictEqual(result.signedOut, true);
  assert.strictEqual(result.dropped, 1);
  assert.strictEqual(result.error, 'Signed out');
});

const bank = (body: string, timestamp: number) => ({ sender: 'AX-HDFCBK', body, timestamp });

await test('concurrent senders lose nothing and send each message once', async () => {
  await reset();
  // The first send fails, the rest succeed: the old read-send-write let one
  // caller's write wipe out what the other had queued.
  let calls = 0;
  const realFetch = globalThis.fetch;
  globalThis.fetch = (async (url: string, init: any) => {
    const call = ++calls;
    await new Promise((r) => setTimeout(r, 5));
    if (call === 1) throw new Error('network down');
    return realFetch(url, init);
  }) as any;
  try {
    await Promise.all([smsTask(bank('x', 1)), smsTask(bank('y', 2))]);
    await flushSmsOutbox();
    const bodies = posted.map((p) => p.body.body).sort();
    assert.deepStrictEqual(bodies, ['x', 'y']);
    assert.deepStrictEqual(await outbox(), []);
  } finally {
    globalThis.fetch = realFetch;
  }
});

await test('an import bigger than the outbox reports what it dropped', async () => {
  await reset();
  serverUp = false;
  const messages = Array.from({ length: 80 }, (_, i) => bank(`m${i}`, i + 1));
  const result = await forwardSms(messages, 'inbox');
  assert.strictEqual(result.matched, 80);
  assert.strictEqual(result.sent, 0);
  assert.strictEqual(result.queued, 50);
  assert.strictEqual(result.dropped, 30);
  assert.strictEqual((await outbox()).length, 50);
});

await test('a full outbox drops imports before live messages', async () => {
  await reset();
  serverUp = false;
  await smsTask(bank('live-1', 1));
  await forwardSms(
    Array.from({ length: 60 }, (_, i) => bank(`inbox-${i}`, i + 10)),
    'inbox'
  );
  const kept = (await outbox()).map((item: any) => item.payload.body);
  assert.strictEqual(kept.length, 50);
  assert.ok(kept.includes('live-1'), 'the live message survives the trim');
});

await test('outboxes written by older builds still send', async () => {
  await reset();
  await AsyncStorage.setItem(
    SMS_OUTBOX_KEY,
    JSON.stringify([{ sender: 'AX-HDFCBK', body: 'old', receivedAt: '2026-10-30T10:00:00.000Z', source: 'live' }])
  );
  await flushSmsOutbox();
  assert.deepStrictEqual(posted.map((p) => p.body.body), ['old']);
  assert.deepStrictEqual(await outbox(), []);
});

if (failures) {
  console.log(`\n${failures} failed`);
  process.exit(1);
}
