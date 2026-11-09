import assert from 'node:assert';
import AsyncStorage from './stubs/asyncStorage.mjs';
import {
  smsTask,
  forwardSms,
  flushSmsOutbox,
  isLikelyBankSender,
  setSmsTestMode,
  getSmsTestMode,
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

await test('test mode sends to the test URL with the same auth and budget', async () => {
  await reset();
  await setSmsTestMode({ enabled: true, url: ' https://my-pc.tailnet.ts.net/api/ ' });
  await smsTask({ sender: 'AX-HDFCBK', body: 'x', timestamp: 1 });
  assert.strictEqual(posted.length, 1);
  assert.strictEqual(posted[0].url, 'https://my-pc.tailnet.ts.net/api/ingest/sms');
  assert.strictEqual(posted[0].auth, 'Bearer access');
  assert.strictEqual(posted[0].budget, 'budget-1');
});

await test('test mode off goes back to ingest and keeps the URL', async () => {
  await reset();
  await setSmsTestMode({ enabled: true, url: 'https://my-pc.tailnet.ts.net/api' });
  await setSmsTestMode({ enabled: false, url: 'https://my-pc.tailnet.ts.net/api' });
  await smsTask({ sender: 'AX-HDFCBK', body: 'x', timestamp: 1 });
  assert.ok(!posted[0].url.includes('tailnet'));
  assert.deepStrictEqual(await getSmsTestMode(), { enabled: false, url: 'https://my-pc.tailnet.ts.net/api' });
});

await test('test mode with an empty URL falls back to ingest', async () => {
  await reset();
  await setSmsTestMode({ enabled: true, url: '' });
  await smsTask({ sender: 'AX-HDFCBK', body: 'x', timestamp: 1 });
  assert.ok(!posted[0].url.includes('tailnet'));
  assert.ok(posted[0].url.endsWith('/ingest/sms'));
});

await test('result names the destination in test mode', async () => {
  await reset();
  await setSmsTestMode({ enabled: true, url: 'https://my-pc.tailnet.ts.net/api' });
  const result = await forwardSms([{ sender: 'AX-HDFCBK', body: 'a', timestamp: 1 }], 'inbox');
  assert.strictEqual(result.destination, 'https://my-pc.tailnet.ts.net/api');
  assert.strictEqual(result.sent, 1);
});

await test('signed out reports an error instead of counting messages as sent', async () => {
  await reset(false);
  const result = await forwardSms([{ sender: 'AX-HDFCBK', body: 'a', timestamp: 1 }], 'inbox');
  assert.strictEqual(result.sent, 0);
  assert.strictEqual(result.error, 'Signed out');
});

if (failures) {
  console.log(`\n${failures} failed`);
  process.exit(1);
}
