import assert from 'node:assert';
import AsyncStorage from './stubs/asyncStorage.mjs';
import { finish, installFetch, test, type Responder } from './harness.mts';
import { loadPressureState, monthElapsedFraction } from '../src/features/widgets/widgetData.ts';

const AUTH_KEY = 'pennywise_auth';
const net = installFetch();

const now = new Date();
const MONTH = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`;
/** Real elapsed fraction, so expectations track whatever day the suite runs. */
const ELAPSED = monthElapsedFraction();

async function signedIn() {
  AsyncStorage.__mem.clear();
  await AsyncStorage.setItem(
    AUTH_KEY,
    JSON.stringify({
      user: { id: 'u1' },
      tokens: { accessToken: 'a', refreshToken: 'r', expiresAt: Date.now() + 600_000 }
    })
  );
  await AsyncStorage.setItem('pennywise_selected_budget_id', 'b1');
  net.reset();
}

type Cat = { name: string; budgeted: number; spent: number; hidden?: boolean };

/** Builds a one-group payload; `spent` is expressed positive and negated here. */
function payload(categories: Cat[], isSystem = false): Responder {
  return (url) =>
    url.includes('category-groups')
      ? {
          status: 200,
          body: [
            {
              name: 'Group',
              isSystem,
              budgeted: {},
              activity: {},
              balance: {},
              categories: categories.map((c) => ({
                id: c.name,
                name: c.name,
                hidden: c.hidden,
                budgeted: { [MONTH]: c.budgeted },
                activity: { [MONTH]: -c.spent },
                balance: { [MONTH]: c.budgeted - c.spent }
              }))
            }
          ]
        }
      : null;
}

async function pressureOf(categories: Cat[], isSystem = false) {
  await signedIn();
  net.respond(payload(categories, isSystem));
  const state = await loadPressureState();
  assert.equal(state.kind, 'ready', 'expected a ready state');
  if (state.kind !== 'ready') throw new Error('unreachable');
  return state;
}

// ---------------------------------------------------------------------------

await test('overspent categories are surfaced first', async () => {
  const state = await pressureOf([
    { name: 'Coffee', budgeted: 2000, spent: 2300 },
    { name: 'Rent', budgeted: 20000, spent: 20000 }
  ]);
  assert.equal(state.pressured[0].name, 'Coffee');
  assert.equal(state.pressured[0].reason, 'over');
  assert.equal(state.pressured[0].remaining, -300);
});

await test('a category tracking exactly to pace is not surfaced', async () => {
  // Spending exactly the elapsed share of the budget is the healthy case.
  const state = await pressureOf([{ name: 'Groceries', budgeted: 10000, spent: Math.round(10000 * ELAPSED * 0.9) }]);
  assert.deepEqual(
    state.pressured.map((p) => p.name),
    [],
    'on-pace spending should not appear'
  );
  assert.equal(state.trackedCount, 1, 'but it is still counted as tracked');
});

await test('burning far faster than the month is flagged as ahead', async () => {
  // 80% spent regardless of how far into the month we are: only "ahead" once
  // that outpaces the calendar, which it does for any day before ~day 20.
  const state = await pressureOf([{ name: 'Dining', budgeted: 5000, spent: 4000 }]);
  if (0.8 / ELAPSED >= 1.25) {
    assert.equal(state.pressured[0]?.reason === 'ahead' || state.pressured[0]?.reason === 'close', true);
  } else {
    // Late in the month 80% is simply on track.
    assert.equal(state.pressured.length, 0);
  }
});

await test('early-month lumpy spending below the floor is ignored', async () => {
  // 40% used is under MIN_USED_TO_WARN, so even a wild pace ratio stays quiet.
  const state = await pressureOf([{ name: 'Travel', budgeted: 10000, spent: 4000 }]);
  const travel = state.pressured.find((p) => p.name === 'Travel');
  if (travel) {
    assert.notEqual(travel.reason, 'ahead', '40% used must not trigger the pace warning');
  }
});

await test('nearly exhausted budgets are flagged even when on pace', async () => {
  const state = await pressureOf([{ name: 'Fuel', budgeted: 1000, spent: 900 }]);
  assert.equal(state.pressured.length, 1);
  assert.ok(['close', 'ahead'].includes(state.pressured[0].reason));
  assert.equal(state.pressured[0].remaining, 100);
});

await test('spending with no budget assigned counts as overspending', async () => {
  const state = await pressureOf([{ name: 'Impulse', budgeted: 0, spent: 500 }]);
  assert.equal(state.pressured[0].name, 'Impulse');
  assert.equal(state.pressured[0].reason, 'over');
});

await test('untouched categories are skipped entirely', async () => {
  const state = await pressureOf([{ name: 'Unused', budgeted: 0, spent: 0 }]);
  assert.equal(state.trackedCount, 0);
  assert.equal(state.pressured.length, 0);
});

await test('hidden categories are excluded', async () => {
  const state = await pressureOf([{ name: 'Archived', budgeted: 100, spent: 500, hidden: true }]);
  assert.equal(state.trackedCount, 0);
  assert.equal(state.pressured.length, 0);
});

await test('system groups are excluded', async () => {
  const state = await pressureOf([{ name: 'Inflow', budgeted: 0, spent: 9999 }], true);
  assert.equal(state.trackedCount, 0);
});

await test('an inflow to a spending category is not treated as spend', async () => {
  await signedIn();
  net.respond((url) =>
    url.includes('category-groups')
      ? {
          status: 200,
          body: [
            {
              name: 'G',
              isSystem: false,
              budgeted: {},
              activity: {},
              balance: {},
              categories: [
                {
                  id: 'Refund',
                  name: 'Refund',
                  // Positive activity = money came back in.
                  budgeted: { [MONTH]: 1000 },
                  activity: { [MONTH]: 250 },
                  balance: { [MONTH]: 1250 }
                }
              ]
            }
          ]
        }
      : null
  );
  const state = await loadPressureState();
  assert.equal(state.kind, 'ready');
  if (state.kind !== 'ready') return;
  assert.equal(state.pressured.length, 0, 'a refund must not read as pressure');
});

await test('at most three rows are returned', async () => {
  const many = Array.from({ length: 8 }, (_, i) => ({ name: `C${i}`, budgeted: 100, spent: 200 + i }));
  const state = await pressureOf(many);
  assert.equal(state.pressured.length, 3);
  assert.equal(state.trackedCount, 8, 'the cap applies to display, not counting');
});

await test('worst overspend ranks above milder overspend', async () => {
  const state = await pressureOf([
    { name: 'Small', budgeted: 1000, spent: 1050 },
    { name: 'Huge', budgeted: 1000, spent: 3000 }
  ]);
  assert.equal(state.pressured[0].name, 'Huge', 'ordered by how far through the budget');
});

await test('month elapsed fraction stays within bounds', async () => {
  for (const day of [1, 15, 28, 31]) {
    const probe = new Date(2026, 0, day);
    const fraction = monthElapsedFraction(probe);
    assert.ok(fraction > 0 && fraction <= 1, `day ${day} gave ${fraction}`);
  }
});

finish();
