import { Alert, AppState } from 'react-native';
import * as TaskManager from 'expo-task-manager';
import * as Notifications from 'expo-notifications';
import * as Location from 'expo-location';
import AsyncStorage from '@react-native-async-storage/async-storage';
import { headlessApi } from '../../utils/headlessApi';
import { LocationSource } from '../transactions/types';

export const LOCATION_SNAP_TASK = 'pennywise-location-snap';

/** Skip auto-tagging when the cached GPS fix is older than this. */
const MAX_FIX_AGE_MS = 15 * 60 * 1000;

/** Pending "tag this transaction?" prompts expire after this. */
const PROMPT_TTL_MS = 60 * 60 * 1000;

const PENDING_PROMPTS_KEY = 'pennywise_pending_location_prompts';

type TransactionPushData = {
  type?: string;
  transactionId?: string;
  budgetId?: string;
  summary?: string;
};

type PendingLocationPrompt = {
  transactionId: string;
  budgetId: string;
  summary?: string;
  createdAt: number;
};

/**
 * The push payload carries its own budget id, so it is passed explicitly rather
 * than falling back to the persisted selection -- the transaction belongs to
 * whichever budget produced the notification, which need not be the one the
 * user last had open.
 */
async function patchLocation(transactionId: string, budgetId: string, body: unknown): Promise<boolean> {
  try {
    await headlessApi.patch(`transactions/${transactionId}/location`, body, { budgetId });
    return true;
  } catch (error) {
    console.log('[location-snap] request failed', error);
    return false;
  }
}

async function loadPendingPrompts(): Promise<PendingLocationPrompt[]> {
  try {
    const raw = await AsyncStorage.getItem(PENDING_PROMPTS_KEY);
    const list = raw ? (JSON.parse(raw) as PendingLocationPrompt[]) : [];
    return list.filter((entry) => Date.now() - entry.createdAt < PROMPT_TTL_MS);
  } catch {
    return [];
  }
}

async function savePendingPrompts(list: PendingLocationPrompt[]): Promise<void> {
  await AsyncStorage.setItem(PENDING_PROMPTS_KEY, JSON.stringify(list.slice(-10)));
}

async function addPendingPrompt(entry: PendingLocationPrompt): Promise<void> {
  const list = await loadPendingPrompts();
  if (list.some((item) => item.transactionId === entry.transactionId)) return;
  await savePendingPrompts([...list, entry]);
}

async function removePendingPrompt(transactionId: string): Promise<void> {
  const list = await loadPendingPrompts();
  await savePendingPrompts(list.filter((item) => item.transactionId !== transactionId));
}

/**
 * Attach the phone's last-known location to a freshly created transaction.
 * Bank emails land within minutes of a card swipe, so the cached fix is almost
 * always the store the user is standing in. When the fix is stale/missing (or
 * permission wasn't granted) the transaction is queued for a "tag location?"
 * prompt the next time the user interacts with the app instead of guessing.
 * The server reverse-geocodes the place name.
 */
export async function snapLocationToTransaction(data: TransactionPushData): Promise<void> {
  try {
    if (data.type !== 'transaction.created' || !data.transactionId || !data.budgetId) return;

    const queueForPrompt = async (reason: string) => {
      console.log(`[location-snap] ${reason}; queueing prompt for ${data.transactionId}`);
      await addPendingPrompt({
        transactionId: data.transactionId!,
        budgetId: data.budgetId!,
        summary: data.summary,
        createdAt: Date.now()
      });
    };

    const { status } = await Location.getForegroundPermissionsAsync();
    if (status !== 'granted') {
      await queueForPrompt('location permission not granted');
      return;
    }

    const position = await Location.getLastKnownPositionAsync({ maxAge: MAX_FIX_AGE_MS });
    if (!position || Date.now() - position.timestamp > MAX_FIX_AGE_MS) {
      await queueForPrompt('no fresh location fix');
      return;
    }

    const ok = await patchLocation(data.transactionId, data.budgetId, {
      lat: position.coords.latitude,
      lng: position.coords.longitude,
      source: LocationSource.AUTO
    });
    console.log(`[location-snap] ${ok ? 'attached location to' : 'failed to tag'} transaction ${data.transactionId}`);
  } catch (error) {
    console.log('[location-snap] error', error);
  }
}

function promptForPendingLocation(entry: PendingLocationPrompt): Promise<void> {
  return new Promise((resolve) => {
    const label = entry.summary ? `"${entry.summary}"` : 'A new transaction';
    Alert.alert(
      'Tag transaction location?',
      `${label} arrived while your location was unknown. Attach your current location to it?`,
      [
        {
          text: 'Not now',
          style: 'cancel',
          onPress: () => {
            void removePendingPrompt(entry.transactionId).finally(resolve);
          }
        },
        {
          text: 'Attach',
          onPress: () => {
            void (async () => {
              try {
                const { status } = await Location.requestForegroundPermissionsAsync();
                if (status !== 'granted') return;
                const position = await Location.getCurrentPositionAsync({
                  accuracy: Location.Accuracy.Balanced
                });
                // user-confirmed, so stored as manual (no AUTO badge)
                const ok = await patchLocation(entry.transactionId, entry.budgetId, {
                  lat: position.coords.latitude,
                  lng: position.coords.longitude,
                  source: LocationSource.MANUAL
                });
                console.log(`[location-snap] prompt ${ok ? 'tagged' : 'failed to tag'} ${entry.transactionId}`);
              } catch (error) {
                console.log('[location-snap] prompt error', error);
              } finally {
                await removePendingPrompt(entry.transactionId);
                resolve();
              }
            })();
          }
        }
      ],
      { cancelable: true, onDismiss: () => resolve() }
    );
  });
}

let promptsInFlight = false;

/**
 * Shows queued "tag location?" prompts one at a time. Runs when the user taps
 * a transaction notification and whenever the app returns to the foreground.
 */
export async function processPendingLocationPrompts(preferredTransactionId?: string): Promise<void> {
  if (promptsInFlight) return;
  promptsInFlight = true;
  try {
    const prompts = await loadPendingPrompts();
    const ordered = preferredTransactionId
      ? [...prompts].sort((a, b) =>
          (b.transactionId === preferredTransactionId ? 1 : 0) - (a.transactionId === preferredTransactionId ? 1 : 0)
        )
      : prompts;
    for (const entry of ordered) {
      await promptForPendingLocation(entry);
    }
  } finally {
    promptsInFlight = false;
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

  // background task doesn't fire while the app is foregrounded; handle those
  // here — snap silently, then immediately offer prompts for anything queued
  Notifications.addNotificationReceivedListener((notification) => {
    void snapLocationToTransaction(extractPushData(notification.request.content.data)).then(() =>
      processPendingLocationPrompts()
    );
  });

  // user tapped a transaction notification: prompt for that transaction first
  Notifications.addNotificationResponseReceivedListener((response) => {
    const data = extractPushData(response.notification.request.content.data);
    if (data.type === 'transaction.created') {
      void processPendingLocationPrompts(data.transactionId);
    }
  });

  // opening the app any other way also drains the pending-prompt queue
  AppState.addEventListener('change', (state) => {
    if (state === 'active') {
      void processPendingLocationPrompts();
    }
  });
}
