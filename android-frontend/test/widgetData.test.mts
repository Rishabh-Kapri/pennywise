import assert from 'node:assert';
import AsyncStorage from './stubs/asyncStorage.mjs';
import { failFor, finish, installFetch, okFor, test, type Responder } from './harness.mts';
import { loadAccountsState, loadBudgetState, loadRecentState } from '../src/features/widgets/widgetData.ts';

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

/** The loaders key group figures by the current month. */
const MONTH = `${new Date().getFullYear()}-${String(new Date().getMonth() + 1).padStart(2, '0')}`;

const inflow = (value: unknown): Responder => (url) =>
  url.includes('categories/inflow') ? { status: 200, body: value } : null;
const groups = (body: unknown): Responder => (url) =>
  url.includes('category-groups') ? { status: 200, body } : null;

// ---------------------------------------------------------------------------

await test('budget: signed out short-circuits before any request', async () => {
  AsyncStorage.__mem.clear();
  net.reset();
  const state = await loadBudgetState();
  assert.equal(state.kind, 'signedOut');
  assert.equal(net.calls.length, 0);
});

await test('budget: totals are summed across groups for the current month', async () => {
  await signedIn();
  net.respond(
    inflow(2500),
    groups([
      {
        name: 'Living',
        isSystem: false,
        budgeted: { [MONTH]: 1000 },
        activity: { [MONTH]: -400 },
        balance: { [MONTH]: 600 },
        categories: [{ name: 'Rent', balance: { [MONTH]: 600 }, activity: { [MONTH]: -400 } }]
      },
      {
        name: 'Fun',
        isSystem: false,
        budgeted: { [MONTH]: 500 },
        activity: { [MONTH]: -700 },
        balance: { [MONTH]: -200 },
        categories: [{ name: 'Dining', balance: { [MONTH]: -200 }, activity: { [MONTH]: -700 } }]
      }
    ])
  );

  const state = await loadBudgetState();
  assert.equal(state.kind, 'ready');
  if (state.kind !== 'ready') return;
  assert.equal(state.readyToAssign, 2500);
  assert.equal(state.assigned, 1500);
  assert.equal(state.available, 400);
  assert.equal(state.spent, 1100, 'activity is negative; spent is reported positive');
  assert.equal(state.overspentCount, 1, 'only the negative-balance category counts');
});

await test('budget: months with no data yield zeroes, not NaN', async () => {
  await signedIn();
  net.respond(
    inflow(0),
    groups([{ name: 'Empty', isSystem: false, budgeted: {}, activity: {}, balance: {}, categories: [] }])
  );

  const state = await loadBudgetState();
  assert.equal(state.kind, 'ready');
  if (state.kind !== 'ready') return;
  for (const value of [state.assigned, state.available, state.spent, state.readyToAssign]) {
    assert.ok(Number.isFinite(value), 'every total must be finite');
  }
  assert.equal(state.overspentCount, 0);
});

await test('budget: non-numeric inflow degrades to zero', async () => {
  await signedIn();
  net.respond(inflow(null), groups([]));
  const state = await loadBudgetState();
  assert.equal(state.kind, 'ready');
  if (state.kind !== 'ready') return;
  assert.equal(state.readyToAssign, 0);
});

await test('budget: server failure becomes an error state', async () => {
  await signedIn();
  net.respond(failFor('categories/inflow', 500, { error: 'boom' }), groups([]));
  const state = await loadBudgetState();
  assert.equal(state.kind, 'error');
});

await test('accounts: closed and deleted accounts are excluded', async () => {
  await signedIn();
  net.respond(
    okFor('accounts', [
      { name: 'Checking', balance: 1000, closed: false, deleted: false },
      { name: 'Card', balance: -250, closed: false, deleted: false },
      { name: 'Old', balance: 9999, closed: true, deleted: false },
      { name: 'Gone', balance: 5555, closed: false, deleted: true }
    ])
  );

  const state = await loadAccountsState();
  assert.equal(state.kind, 'ready');
  if (state.kind !== 'ready') return;
  assert.equal(state.accountCount, 2);
  assert.equal(state.total, 750);
  assert.equal(state.cash, 1000);
  assert.equal(state.debt, 250, 'debt is reported as a positive figure');
});

await test('accounts: missing balances are treated as zero', async () => {
  await signedIn();
  net.respond(okFor('accounts', [{ name: 'New', closed: false, deleted: false }]));
  const state = await loadAccountsState();
  assert.equal(state.kind, 'ready');
  if (state.kind !== 'ready') return;
  assert.equal(state.total, 0);
  assert.equal(state.accountCount, 1);
});

await test('accounts: non-array payload does not crash', async () => {
  await signedIn();
  net.respond(okFor('accounts', { error: 'weird' }));
  const state = await loadAccountsState();
  assert.equal(state.kind, 'ready');
  if (state.kind !== 'ready') return;
  assert.equal(state.accountCount, 0);
  assert.equal(state.total, 0);
});

await test('recent: rows are flattened with a signed amount', async () => {
  await signedIn();
  net.respond(
    okFor('transactions/normalized', {
      data: [
        { id: 't1', date: '2026-08-01', payeeName: 'Blue Tokai', categoryName: 'Dining', accountName: 'Card', inflow: null, outflow: 450 },
        { id: 't2', date: '2026-08-01', payeeName: 'Salary', categoryName: null, accountName: 'Checking', inflow: 90000, outflow: null }
      ]
    })
  );

  const state = await loadRecentState();
  assert.equal(state.kind, 'ready');
  if (state.kind !== 'ready') return;
  assert.equal(state.transactions.length, 2);
  assert.equal(state.transactions[0].amount, -450, 'outflow becomes negative');
  assert.equal(state.transactions[0].detail, 'Dining');
  assert.equal(state.transactions[1].amount, 90000);
  assert.equal(state.transactions[1].detail, 'Checking', 'falls back to account when uncategorised');
});

await test('recent: caps the row count', async () => {
  await signedIn();
  const many = Array.from({ length: 12 }, (_, i) => ({
    id: `t${i}`,
    date: '2026-08-01',
    payeeName: `P${i}`,
    accountName: 'Card',
    inflow: null,
    outflow: 10
  }));
  net.respond(okFor('transactions/normalized', { data: many }));

  const state = await loadRecentState();
  assert.equal(state.kind, 'ready');
  if (state.kind !== 'ready') return;
  assert.equal(state.transactions.length, 5);
});

await test('recent: missing payee and id are tolerated', async () => {
  await signedIn();
  net.respond(
    okFor('transactions/normalized', {
      data: [{ date: '2026-08-01', payeeName: '', accountName: 'Card', inflow: null, outflow: 10 }]
    })
  );

  const state = await loadRecentState();
  assert.equal(state.kind, 'ready');
  if (state.kind !== 'ready') return;
  assert.equal(state.transactions[0].payee, 'Unknown payee');
  assert.ok(state.transactions[0].id.length > 0, 'a key must always exist for the list');
});

await test('recent: missing data array does not crash', async () => {
  await signedIn();
  net.respond(okFor('transactions/normalized', {}));
  const state = await loadRecentState();
  assert.equal(state.kind, 'ready');
  if (state.kind !== 'ready') return;
  assert.deepEqual(state.transactions, []);
});

finish();
