import { expect, test } from '@playwright/test';
import Ajv from 'ajv';
import { readFileSync } from 'node:fs';

const schema = JSON.parse(readFileSync(new URL('../../contracts/core.schema.json', import.meta.url), 'utf8'));
const ajv = new Ajv({ allErrors: true });
ajv.addSchema(schema);
function assertContract(name: string, value: unknown) {
  const validate = ajv.getSchema(`${schema.$id}#/definitions/${name}`)!;
  expect(validate(value), JSON.stringify(validate.errors)).toBe(true);
}

// Real API + PostgreSQL: no request interception or preloaded auth state.
test('demo transaction changes balances and survives reload', async ({ page }, testInfo) => {
  await page.goto('/login');
  const loginResponse = page.waitForResponse(r => r.url().endsWith('/api/auth/demo') && r.request().method() === 'POST');
  await page.getByRole('button', { name: 'Try Demo' }).click();
  const response = await loginResponse;
  expect(response.ok()).toBeTruthy();
  const login = await response.json();
  assertContract('login', login);
  expect(login.accessToken).toEqual(expect.any(String));
  await expect(page.getByRole('button', { name: 'Select active budget' })).toHaveText('Demo Budget');

  const budgetsResponse = await page.request.get('/api/budgets', { headers: { Authorization: `Bearer ${login.accessToken}` } });
  expect(budgetsResponse.ok()).toBeTruthy();
  const budgets = await budgetsResponse.json();
  assertContract('budgets', budgets);
  const budget = budgets.find((b: { name: string }) => b.name === 'Demo Budget');
  expect(budget.id).toEqual(expect.any(String));
  const headers = { Authorization: `Bearer ${login.accessToken}`, 'X-Budget-ID': budget.id };
  const accounts = async () => {
    const response = await page.request.get('/api/accounts', { headers });
    expect(response.ok()).toBeTruthy();
    const data = await response.json();
    assertContract('accounts', data);
    return data as { id: string; name: string; balance: number }[];
  };
  const account = (await accounts()).find(a => a.name === 'HDFC Checking')!;
  expect(account.balance).toEqual(expect.any(Number));
  const note = `Smoke ${Date.now()}`;
  let transactionId: string | undefined;
  try {
    await page.getByRole('link', { name: 'Transactions', exact: true }).click();
    await page.getByRole('button', { name: 'Add Expense' }).click();
    await page.getByPlaceholder('0', { exact: true }).fill('123');
    await page.getByPlaceholder('Select payee', { exact: true }).fill('BigBasket');
    await page.getByRole('option', { name: 'BigBasket' }).click();
    await page.getByPlaceholder('Select account', { exact: true }).fill('HDFC');
    await page.getByRole('option', { name: 'HDFC Checking' }).click();
    await page.getByPlaceholder('Add a note...').fill(note);
    const createdResponse = page.waitForResponse(r => r.url().endsWith('/api/transactions') && r.request().method() === 'POST');
    await page.getByRole('button', { name: 'Save', exact: true }).click();
    const created = await createdResponse;
    expect(created.ok()).toBeTruthy();
    const transactions = await created.json();
    assertContract('createdTransactions', transactions);
    expect(transactions).toHaveLength(1);
    const transaction = transactions[0];
    transactionId = transaction.id;
    expect(transactionId).toEqual(expect.any(String));
    await expect.poll(async () => (await accounts()).find(a => a.id === account.id)?.balance).toBeCloseTo(account.balance - 123, 2);
    await page.reload();
    await page.getByPlaceholder('Search transactions').fill(note);
    await expect(page.getByText(note, { exact: true })).toBeVisible();
    // Payee, date, category, account and note edit in place; the amount opens the panel.
    const row = page.getByTestId(`transaction-${transactionId}`);
    // boundingBox doesn't wait: the list can still be re-rendering after the reload.
    await expect(row).toBeVisible();
    const box = await row.boundingBox();
    await row.click({ position: { x: box!.width - 24, y: box!.height / 2 } });
    await page.getByPlaceholder("Anything you'd like to remember about this?").fill(`${note} edited`);
    const updatedResponse = page.waitForResponse(r => r.url().includes(`/api/transactions/${transactionId}`) && r.request().method() === 'PATCH');
    await page.getByRole('button', { name: 'Save', exact: true }).click();
    expect((await updatedResponse).ok()).toBeTruthy();
    await page.reload();
    await page.getByPlaceholder('Search transactions').fill(`${note} edited`);
    await expect(page.getByText(`${note} edited`, { exact: true })).toBeVisible();
  } finally {
    testInfo.setTimeout(testInfo.timeout + 10_000);
    if (transactionId) {
      const deleted = await page.request.delete(`/api/transactions/${transactionId}`, { headers });
      expect(deleted.ok()).toBeTruthy();
      await expect.poll(async () => (await accounts()).find(a => a.id === account.id)?.balance).toBeCloseTo(account.balance, 2);
    }
  }
});

test('budget resources require authentication', async ({ request }) => {
  const response = await request.get('/api/accounts');
  expect(response.status()).toBe(401);
});

async function moveOne(page: import('@playwright/test').Page, positive: boolean) {
  await page.goto('/login');
  await page.getByRole('button', { name: 'Try Demo' }).click();
  await expect(page.getByRole('button', { name: 'Select active budget' })).toHaveText('Demo Budget');
  await page.goto('/budget');

  const pills = page.getByRole('button', { name: /^Move money from / });
  await expect(pills.first()).toBeVisible();
  const pill = pills.filter(positive ? { hasNotText: '-' } : { hasText: '-' }).first();
  const from = (await pill.getAttribute('aria-label'))!.replace('Move money from ', '');
  await pill.click();

  const dialog = page.locator('form').filter({ hasText: `"${from}"` });
  await dialog.getByRole('textbox').fill('1');
  await dialog.getByRole('button', { name: /^(From|To)$/ }).click();
  const target = page
    .getByRole('option')
    .filter({ hasNotText: from })
    .filter({ hasNotText: 'Ready to Assign' })
    .first();
  const to = (await target.locator('span').first().textContent())!.trim();
  await target.click();
  await expect(dialog.getByRole('button', { name: /^(From|To)$/ })).toHaveText(to);

  const patches: string[] = [];
  page.on('response', (r) => {
    if (r.request().method() === 'PATCH' && r.url().includes('/api/categories/') && r.ok()) patches.push(r.url());
  });
  await dialog.getByRole('button', { name: 'Move', exact: true }).click();
  await expect(page.getByText('Money moved')).toBeVisible();
  await expect.poll(() => patches.length).toBe(2);
}

test('covering overspending moves money between categories', async ({ page }) => {
  await moveOne(page, false);
});

test('moving a positive balance moves money between categories', async ({ page }) => {
  await moveOne(page, true);
});

test('moving money works on a narrow window', async ({ page }) => {
  await page.setViewportSize({ width: 1000, height: 800 });
  await moveOne(page, true);
});

test('moving money without a target says what is missing', async ({ page }) => {
  await page.goto('/login');
  await page.getByRole('button', { name: 'Try Demo' }).click();
  await expect(page.getByRole('button', { name: 'Select active budget' })).toHaveText('Demo Budget');
  await page.goto('/budget');
  const pill = page.getByRole('button', { name: /^Move money from / }).first();
  const from = (await pill.getAttribute('aria-label'))!.replace('Move money from ', '');
  await pill.click();
  await page.getByRole('button', { name: 'Move', exact: true }).click();
  await expect(page.getByRole('alert').filter({ hasText: /Choose where to (move it|take it from)/ })).toBeVisible();
  // The list opens to choose from, and the keyboard can pick in it.
  const option = page.getByRole('option').filter({ hasNotText: from }).first();
  const name = (await option.locator('span').first().textContent())!.trim();
  await option.focus();
  await page.keyboard.press('Enter');
  await expect(page.getByRole('button', { name: /^(From|To)$/ })).toHaveText(name);
  await expect(page.getByRole('alert')).toHaveCount(0);
});
