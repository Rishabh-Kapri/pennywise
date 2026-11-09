import assert from 'node:assert';
import AsyncStorage from './stubs/asyncStorage.mjs';
import { test, finish } from './harness.mts';
import { apiClient } from '../src/utils/api.ts';

const AUTH_KEY = 'pennywise_auth';

apiClient.setGetState(
  () =>
    ({
      auth: { tokens: { accessToken: 'stale', refreshToken: 'refresh' } },
      budgets: { selectedBudget: null }
    }) as never
);

const json = (status: number, body: unknown) =>
  ({ ok: status < 400, status, statusText: String(status), text: async () => JSON.stringify(body), json: async () => body });

/** Every call gets a 401 until it reaches auth/refresh, which `refresh` answers. */
function stubRefresh(refresh: () => Promise<unknown>) {
  globalThis.fetch = (async (url: string) =>
    url.endsWith('/auth/refresh') ? refresh() : json(401, { error: 'expired' })) as never;
}

/** Whether the call left the stored session in place. */
async function sessionSurvives() {
  await AsyncStorage.setItem(AUTH_KEY, '{}');
  await apiClient.get('things').catch(() => undefined);
  return (await AsyncStorage.getItem(AUTH_KEY)) !== null;
}

await test('server down during refresh keeps the session', async () => {
  stubRefresh(async () => {
    throw new TypeError('Network request failed');
  });
  assert.strictEqual(await sessionSurvives(), true);
});

await test('server error during refresh keeps the session', async () => {
  stubRefresh(async () => json(503, { error: 'down' }));
  assert.strictEqual(await sessionSurvives(), true);
});

await test('a rejected refresh ends the session', async () => {
  stubRefresh(async () => json(401, { error: 'invalid refresh token' }));
  assert.strictEqual(await sessionSurvives(), false);
});

finish();
