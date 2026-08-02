import assert from 'node:assert';
import AsyncStorage from './stubs/asyncStorage.mjs';
import { failFor, finish, installFetch, okFor, test, type Responder } from './harness.mts';
import { loadPipelineState, retryParkedRuns, type PipelineRun } from '../src/features/widgets/pipelineData.ts';

const AUTH_KEY = 'pennywise_auth';
const BUDGET_KEY = 'pennywise_selected_budget_id';

const net = installFetch();

async function signedIn() {
  AsyncStorage.__mem.clear();
  await AsyncStorage.setItem(
    AUTH_KEY,
    JSON.stringify({
      user: { id: 'u1' },
      tokens: { accessToken: 'a', refreshToken: 'r', expiresAt: Date.now() + 600_000 }
    })
  );
  await AsyncStorage.setItem(BUDGET_KEY, 'budget-1');
  net.reset();
}

const run = (over: Partial<PipelineRun> = {}): PipelineRun => ({
  id: 'run-1',
  status: 'waiting_retry',
  currentStep: 'predict',
  emailsFetched: 3,
  transactionsCreated: 0,
  startedAt: new Date().toISOString(),
  updatedAt: new Date().toISOString(),
  ...over
});

/** Distinguishes the two list calls, which differ only by query string. */
const parkedList = (body: unknown): Responder => (url) =>
  url.includes('status=waiting_retry') ? { status: 200, body } : null;
const latestList = (body: unknown): Responder => (url) =>
  url.includes('pipeline/runs?limit=1') ? { status: 200, body } : null;

// ---------------------------------------------------------------------------

await test('signed out short-circuits without any request', async () => {
  AsyncStorage.__mem.clear();
  net.reset();
  const state = await loadPipelineState();
  assert.equal(state.kind, 'signedOut');
  assert.equal(net.calls.length, 0, 'must not hit the network when signed out');
});

await test('parked runs are surfaced with the latest run', async () => {
  await signedIn();
  net.respond(
    parkedList([run({ id: 'a' }), run({ id: 'b' })]),
    latestList([run({ id: 'c', status: 'completed', transactionsCreated: 4 })])
  );

  const state = await loadPipelineState();
  assert.equal(state.kind, 'ready');
  if (state.kind !== 'ready') return;
  assert.equal(state.parked.length, 2);
  assert.equal(state.latest?.transactionsCreated, 4);
});

await test('no parked runs still reports ready', async () => {
  await signedIn();
  net.respond(parkedList([]), latestList([run({ status: 'completed' })]));

  const state = await loadPipelineState();
  assert.equal(state.kind, 'ready');
  if (state.kind !== 'ready') return;
  assert.equal(state.parked.length, 0);
  assert.ok(state.latest);
});

await test('empty latest list yields a null latest rather than undefined', async () => {
  await signedIn();
  net.respond(parkedList([]), latestList([]));

  const state = await loadPipelineState();
  assert.equal(state.kind, 'ready');
  if (state.kind !== 'ready') return;
  assert.equal(state.latest, null);
});

await test('non-array payload does not crash the widget', async () => {
  await signedIn();
  // The endpoint returns a bare array; anything else (an error object slipping
  // through, say) must not blow up a headless render.
  net.respond(parkedList({ error: 'weird' }), latestList(null));

  const state = await loadPipelineState();
  assert.equal(state.kind, 'ready');
  if (state.kind !== 'ready') return;
  assert.deepEqual(state.parked, []);
  assert.equal(state.latest, null);
});

await test('server failure becomes an error state carrying the message', async () => {
  await signedIn();
  net.respond(failFor('pipeline/runs', 500, { error: 'pipeline_runs missing' }));

  const state = await loadPipelineState();
  assert.equal(state.kind, 'error');
  if (state.kind !== 'error') return;
  assert.match(state.message, /pipeline_runs missing/);
});

await test('expired session degrades to signedOut, not error', async () => {
  AsyncStorage.__mem.clear();
  await AsyncStorage.setItem(
    AUTH_KEY,
    JSON.stringify({ user: { id: 'u1' }, tokens: { accessToken: 'a', refreshToken: 'r', expiresAt: 0 } })
  );
  net.reset();
  net.respond(failFor('auth/refresh', 401), failFor('pipeline/runs', 401));

  const state = await loadPipelineState();
  assert.equal(state.kind, 'signedOut');
});

await test('retry posts once per run and counts acceptances', async () => {
  await signedIn();
  net.respond(okFor('/retry', { ok: true }));

  const accepted = await retryParkedRuns([run({ id: 'a' }), run({ id: 'b' })]);
  assert.equal(accepted, 2);

  const posts = net.calls.filter((c) => c.url.includes('/retry'));
  assert.equal(posts.length, 2);
  assert.equal(posts[0].method, 'POST');
  assert.ok(posts[0].url.includes('pipeline/runs/a/retry'));
  assert.ok(posts[1].url.includes('pipeline/runs/b/retry'));
});

await test('one failing retry does not strand the rest', async () => {
  await signedIn();
  net.respond((url) =>
    url.includes('/a/retry') ? { status: 500, body: { error: 'nope' } } : url.includes('/retry') ? { status: 200, body: {} } : null
  );

  const accepted = await retryParkedRuns([run({ id: 'a' }), run({ id: 'b' })]);
  assert.equal(accepted, 1, 'the healthy run should still be signalled');
  assert.equal(net.calls.filter((c) => c.url.includes('/retry')).length, 2);
});

await test('retry is capped at five runs', async () => {
  await signedIn();
  net.respond(okFor('/retry', {}));

  const many = Array.from({ length: 9 }, (_, i) => run({ id: `r${i}` }));
  const accepted = await retryParkedRuns(many);
  assert.equal(accepted, 5);
  assert.equal(net.calls.filter((c) => c.url.includes('/retry')).length, 5);
});

await test('requests carry the persisted budget id', async () => {
  await signedIn();
  net.respond(parkedList([]), latestList([]));
  await loadPipelineState();
  assert.equal(net.calls[0].headers['x-budget-id'], 'budget-1');
});

finish();
