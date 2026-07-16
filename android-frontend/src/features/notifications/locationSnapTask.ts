import * as TaskManager from 'expo-task-manager';
import * as Notifications from 'expo-notifications';
import * as Location from 'expo-location';
import AsyncStorage from '@react-native-async-storage/async-storage';
import { config } from '../../config/env';
import { LocationSource } from '../transactions/types';

export const LOCATION_SNAP_TASK = 'pennywise-location-snap';

/** Skip auto-tagging when the cached GPS fix is older than this. */
const MAX_FIX_AGE_MS = 15 * 60 * 1000;

const AUTH_STORAGE_KEY = 'pennywise_auth';

type StoredAuth = {
  user?: unknown;
  tokens?: { accessToken: string; refreshToken: string; expiresAt: number };
};

type TransactionPushData = {
  type?: string;
  transactionId?: string;
  budgetId?: string;
};

/**
 * The background task runs outside the React tree (possibly before the Redux
 * store is hydrated), so auth goes straight through AsyncStorage.
 */
async function authenticatedPatch(path: string, budgetId: string, body: unknown): Promise<boolean> {
  const raw = await AsyncStorage.getItem(AUTH_STORAGE_KEY);
  if (!raw) return false;
  const stored = JSON.parse(raw) as StoredAuth;
  if (!stored.tokens?.refreshToken) return false;

  const doPatch = (accessToken: string) =>
    fetch(`${config.apiBaseUrl}/${path}`, {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${accessToken}`,
        'x-budget-id': budgetId
      },
      body: JSON.stringify(body)
    });

  let res = await doPatch(stored.tokens.accessToken);
  if (res.status === 401) {
    const refreshRes = await fetch(`${config.apiBaseUrl}/auth/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refreshToken: stored.tokens.refreshToken })
    });
    if (!refreshRes.ok) return false;
    const refreshed = (await refreshRes.json()) as { accessToken: string; expiresIn: number };
    stored.tokens.accessToken = refreshed.accessToken;
    stored.tokens.expiresAt = Date.now() + refreshed.expiresIn * 1000;
    await AsyncStorage.setItem(AUTH_STORAGE_KEY, JSON.stringify(stored));
    res = await doPatch(refreshed.accessToken);
  }
  return res.ok;
}

/**
 * Attach the phone's last-known location to a freshly created transaction.
 * Bank emails land within minutes of a card swipe, so the cached fix is almost
 * always the store the user is standing in; stale fixes are skipped rather
 * than guessed. The server reverse-geocodes the place name.
 */
export async function snapLocationToTransaction(data: TransactionPushData): Promise<void> {
  try {
    if (data.type !== 'transaction.created' || !data.transactionId || !data.budgetId) return;

    const { status } = await Location.getForegroundPermissionsAsync();
    if (status !== 'granted') {
      console.log('[location-snap] location permission not granted, skipping');
      return;
    }

    const position = await Location.getLastKnownPositionAsync({ maxAge: MAX_FIX_AGE_MS });
    if (!position) {
      console.log('[location-snap] no recent location fix, skipping');
      return;
    }
    if (Date.now() - position.timestamp > MAX_FIX_AGE_MS) {
      console.log('[location-snap] location fix too old, skipping');
      return;
    }

    const ok = await authenticatedPatch(`transactions/${data.transactionId}/location`, data.budgetId, {
      lat: position.coords.latitude,
      lng: position.coords.longitude,
      source: LocationSource.AUTO
    });
    console.log(`[location-snap] ${ok ? 'attached location to' : 'failed to tag'} transaction ${data.transactionId}`);
  } catch (error) {
    console.log('[location-snap] error', error);
  }
}

/** Digs the push `data` payload out of the various shapes Expo/FCM deliver. */
export function extractPushData(payload: unknown): TransactionPushData {
  if (!payload || typeof payload !== 'object') return {};
  const record = payload as Record<string, unknown>;

  if (typeof record.transactionId === 'string') return record as TransactionPushData;

  // Android background delivery: { notification: { data: { body: '<json>' } } }
  const nested =
    (record.notification as Record<string, unknown> | undefined)?.data ?? record.data ?? record.body;
  if (typeof nested === 'string') {
    try {
      return JSON.parse(nested) as TransactionPushData;
    } catch {
      return {};
    }
  }
  if (nested && typeof nested === 'object') {
    const nestedRecord = nested as Record<string, unknown>;
    if (typeof nestedRecord.body === 'string') {
      try {
        return JSON.parse(nestedRecord.body) as TransactionPushData;
      } catch {
        return {};
      }
    }
    return nestedRecord as TransactionPushData;
  }
  return {};
}

TaskManager.defineTask(LOCATION_SNAP_TASK, async ({ data, error }) => {
  if (error) {
    console.log('[location-snap] task error', error.message);
    return;
  }
  await snapLocationToTransaction(extractPushData(data));
});

/**
 * Registers the background push handler and the foreground fallback listener.
 * Must be called once at app startup (module scope of the app root).
 */
export function setupLocationSnap(): void {
  Notifications.registerTaskAsync(LOCATION_SNAP_TASK).catch((error: unknown) => {
    console.log('[location-snap] failed to register background task', error);
  });

  // background task doesn't fire while the app is foregrounded; handle those here
  Notifications.addNotificationReceivedListener((notification) => {
    void snapLocationToTransaction(extractPushData(notification.request.content.data));
  });
}
