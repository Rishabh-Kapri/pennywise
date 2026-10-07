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
    await page.getByPlaceholder('Select Payee', { exact: true }).fill('BigBasket');
    await page.getByText('BigBasket', { exact: true }).last().click();
    await page.getByRole('combobox', { name: 'Select Account' }).fill('HDFC');
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
    await page.getByTestId(`transaction-${transactionId}`).getByText('₹123.00', { exact: true }).click();
    await page.getByPlaceholder('Something about this transaction you would like to recall later?').fill(`${note} edited`);
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
