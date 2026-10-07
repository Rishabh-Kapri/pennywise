import { expect, test } from '@playwright/test';
import { formatUtcTimestamp } from '../src/utils/date.utils';

test.use({ timezoneId: 'Asia/Kolkata' });

test('Gmail settings load real connection state and enforce demo restrictions', async ({ page }) => {
  await page.goto('/login');
  const loginResponse = page.waitForResponse(r => r.url().endsWith('/api/auth/demo') && r.request().method() === 'POST');
  await page.getByRole('button', { name: 'Try Demo' }).click();
  const login = await (await loginResponse).json();
  await expect(page.getByRole('button', { name: 'Select active budget' })).toHaveText('Demo Budget');

  const headers = { Authorization: `Bearer ${login.accessToken}` };
  // Mailbox settings do not require a selected budget or a budget header.
  const response = await page.request.get('/api/auth/gmail', { headers });
  expect(response.status()).toBe(200);
  const connections = await response.json();
  expect(connections).toEqual([expect.objectContaining({
    email: 'demo@pennywise.local', oauthClientType: 'web',
    connected: false, paused: false, status: 'needs_reconnect',
  })]);
  expect(JSON.stringify(connections)).not.toContain('refreshToken');

  await page.goto('/settings?section=gmail');
  await expect(page.getByRole('heading', { name: 'Gmail controls' })).toBeVisible();
  await expect(page.getByText('Gmail controls are unavailable in demo mode.')).toBeVisible();
  await expect(page.getByRole('heading', { name: 'demo@pennywise.local' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Pause ingestion' })).toBeDisabled();
  await expect(page.getByRole('button', { name: 'Sync now' })).toBeDisabled();
  await expect(page.getByRole('button', { name: 'Reconnect Gmail' })).toBeDisabled();
  await expect(page.getByRole('alert')).toHaveCount(0);
  await page.getByRole('button', { name: 'Refresh status' }).click();
  await expect(page.getByRole('button', { name: 'Refresh status' })).toBeEnabled();

  const connection = connections[0];
  const expectedLastSync = await page.evaluate((timestamp) => new Date(timestamp).toLocaleString(undefined, {
    year: 'numeric', month: 'numeric', day: 'numeric',
    hour: 'numeric', minute: '2-digit', second: '2-digit', timeZoneName: 'short',
  }), connection.lastGmailSync);
  await expect(page.locator('dl > div').filter({ has: page.getByText('Last sync', { exact: true }) }).locator('dd'))
    .toHaveText(expectedLastSync);
  const pause = await page.request.post('/api/auth/gmail/pause', {
    headers, data: { providerId: connection.providerId, oauthClientType: connection.oauthClientType },
  });
  expect(pause.status()).toBe(400);
  expect((await pause.json()).error).toContain('demo account');
  const unowned = await page.request.post('/api/auth/gmail/pause', {
    headers, data: { providerId: 'another-users-google-account', oauthClientType: 'web' },
  });
  expect(unowned.status()).toBe(404);
  await page.reload();
  await expect(page.getByText('Gmail access needed', { exact: true })).toBeVisible();
  await page.getByRole('link', { name: 'View import activity' }).click();
  await expect(page).toHaveURL(/section=activity/);
});

test('Gmail controls require authentication', async ({ request }) => {
  expect((await request.get('/api/auth/gmail')).status()).toBe(401);
  expect((await request.post('/api/auth/gmail/pause', {
    data: { providerId: 'unowned', oauthClientType: 'web' },
  })).status()).toBe(401);
});


test('Gmail UTC timestamps convert to India time without applying an offset twice', () => {
  const options = { timeZone: 'Asia/Kolkata', hour12: false };
  for (const timestamp of [
    '2026-10-06T16:02:17',
    '2026-10-06 16:02:17',
    '2026-10-06T16:02:17Z',
    '2026-10-06T21:32:17+05:30',
    Date.parse('2026-10-06T16:02:17Z'), // Gmail watch expiration is Unix milliseconds.
  ]) {
    expect(formatUtcTimestamp(timestamp, options)).toContain('21:32:17');
    expect(formatUtcTimestamp(timestamp, options)).toMatch(/IST|GMT\+5:30/);
  }
});

test('Gmail times follow the viewer timezone, including daylight saving', () => {
  expect(formatUtcTimestamp('2026-07-06T16:02:17Z', { timeZone: 'America/New_York', hour12: false }))
    .toContain('12:02:17');
  expect(formatUtcTimestamp('2026-12-06T16:02:17Z', { timeZone: 'America/New_York', hour12: false }))
    .toContain('11:02:17');
  expect(formatUtcTimestamp('2026-10-06T16:02:17Z', { timeZone: 'UTC', hour12: false }))
    .toContain('16:02:17');
  for (const timestamp of [undefined, null, '', 'invalid']) {
    expect(formatUtcTimestamp(timestamp)).toBe('Not available');
  }
});
