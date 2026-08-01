import assert from 'node:assert';
import AsyncStorage from './stubs/asyncStorage.mjs';
import { headlessApi, NotAuthenticatedError, hasStoredSession } from '../src/utils/headlessApi.ts';

const AUTH_KEY = 'pennywise_auth';
const BUDGET_KEY = 'pennywise_selected_budget_id';

let calls: Array<{ url: string; method: string; headers: Record<string, string>; body?: string }> = [];
let responders: Array<(url: string) => { status: number; body: unknown } | null> = [];

globalThis.fetch = (async (url: string, init: any = {}) => {
  calls.push({ url, method: init.method ?? 'GET', headers: init.headers ?? {}, body: init.body });
  for (const r of responders) {
    const res = r(url);
    if (res) {
      return {
        ok: res.status >= 200 && res.status < 300,
        status: res.status,
        statusText: String(res.status),
        text: async () => JSON.stringify(res.body),
        json: async () => res.body
      };
    }
  }
  throw new Error(`unstubbed fetch: ${url}`);
}) as any;

async function reset(auth: unknown, budgetId?: string) {
  AsyncStorage.__mem.clear();
  if (auth) await AsyncStorage.setItem(AUTH_KEY, JSON.stringify(auth));
  if (budgetId) await AsyncStorage.setItem(BUDGET_KEY, budgetId);
  calls = [];
  responders = [];
}

const session = (expiresAt: number) => ({
  user: { id: 'u1' },
  tokens: { accessToken: 'access-old', refreshToken: 'refresh-1', expiresAt }
});

const FUTURE = () => Date.now() + 10 * 60 * 1000;
const PAST = () => Date.now() - 60 * 1000;

const ok = (match: string, body: unknown = { fine: true }) => (url: string) =>
  url.includes(match) ? { status: 200, body } : null;

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

// ---------------------------------------------------------------------------

await test('no stored session throws NotAuthenticatedError', async () => {
  await reset(null);
  await assert.rejects(() => headlessApi.get('transactions'), (e: Error) => e instanceof NotAuthenticatedError);
  assert.equal(calls.length, 0, 'should not hit the network without a session');
});

await test('fresh token is used directly, no refresh round trip', async () => {
  await reset(session(FUTURE()), 'budget-1');
  responders = [ok('transactions')];
  await headlessApi.get('transactions');
  assert.equal(calls.length, 1, 'expected exactly one request');
  assert.equal(calls[0].headers.Authorization, 'Bearer access-old');
  assert.ok(!calls.some((c) => c.url.includes('auth/refresh')), 'must not refresh a fresh token');
});

await test('stale token refreshes proactively before the request', async () => {
  await reset(session(PAST()), 'budget-1');
  responders = [
    (u) => (u.includes('auth/refresh') ? { status: 200, body: { accessToken: 'access-new', expiresIn: 900 } } : null),
    ok('transactions')
  ];
  await headlessApi.get('transactions');
  assert.equal(calls[0].url.includes('auth/refresh'), true, 'refresh should come first');
  assert.equal(calls[1].headers.Authorization, 'Bearer access-new');
  const stored = JSON.parse((await AsyncStorage.getItem(AUTH_KEY))!);
  assert.equal(stored.tokens.accessToken, 'access-new', 'new token must be persisted');
  assert.ok(stored.tokens.expiresAt > Date.now(), 'expiry must be pushed forward');
  assert.deepEqual(stored.user, { id: 'u1' }, 'user must survive the token write');
});

await test('401 despite a fresh token triggers a reactive refresh and retry', async () => {
  await reset(session(FUTURE()), 'budget-1');
  let served = 0;
  responders = [
    (u) => (u.includes('auth/refresh') ? { status: 200, body: { accessToken: 'access-new', expiresIn: 900 } } : null),
    (u) => (u.includes('transactions') ? (served++ === 0 ? { status: 401, body: {} } : { status: 200, body: { ok: 1 } }) : null)
  ];
  const out = await headlessApi.get<{ ok: number }>('transactions');
  assert.equal(out.ok, 1);
  assert.equal(calls.length, 3, 'expected request, refresh, retry');
  assert.equal(calls[2].headers.Authorization, 'Bearer access-new');
});

await test('failed refresh does NOT clear the stored session', async () => {
  await reset(session(PAST()), 'budget-1');
  responders = [
    (u) => (u.includes('auth/refresh') ? { status: 401, body: { error: 'nope' } } : null),
    ok('transactions')
  ];
  await headlessApi.get('transactions').catch(() => undefined);
  const raw = await AsyncStorage.getItem(AUTH_KEY);
  assert.ok(raw, 'session must survive a failed background refresh');
  assert.equal(JSON.parse(raw!).tokens.refreshToken, 'refresh-1');
});

await test('failed refresh falls back to the stored access token', async () => {
  await reset(session(PAST()), 'budget-1');
  responders = [
    (u) => (u.includes('auth/refresh') ? { status: 500, body: {} } : null),
    ok('transactions', { served: true })
  ];
  await headlessApi.get('transactions');
  const attempt = calls.find((c) => c.url.includes('transactions'));
  assert.equal(attempt?.headers.Authorization, 'Bearer access-old');
});

await test('budget id comes from storage', async () => {
  await reset(session(FUTURE()), 'budget-from-storage');
  responders = [ok('transactions')];
  await headlessApi.get('transactions');
  assert.equal(calls[0].headers['x-budget-id'], 'budget-from-storage');
});

await test('explicit budget id overrides the stored one', async () => {
  await reset(session(FUTURE()), 'budget-from-storage');
  responders = [ok('transactions')];
  await headlessApi.patch('transactions/t1/location', { lat: 1 }, { budgetId: 'budget-explicit' });
  assert.equal(calls[0].headers['x-budget-id'], 'budget-explicit');
  assert.equal(calls[0].method, 'PATCH');
  assert.equal(calls[0].body, JSON.stringify({ lat: 1 }));
});

await test('budgets endpoint omits x-budget-id', async () => {
  await reset(session(FUTURE()), 'budget-1');
  responders = [ok('budgets')];
  await headlessApi.get('budgets');
  assert.equal(calls[0].headers['x-budget-id'], undefined);
});

await test('server errors surface their message', async () => {
  await reset(session(FUTURE()), 'budget-1');
  responders = [(u) => (u.includes('transactions') ? { status: 500, body: { error: 'boom' } } : null)];
  await assert.rejects(() => headlessApi.get('transactions'), /boom/);
});

await test('hasStoredSession reflects storage', async () => {
  await reset(null);
  assert.equal(await hasStoredSession(), false);
  await reset(session(FUTURE()));
  assert.equal(await hasStoredSession(), true);
});

console.log(failures === 0 ? '\nALL PASS' : `\n${failures} FAILURE(S)`);
process.exit(failures === 0 ? 0 : 1);
