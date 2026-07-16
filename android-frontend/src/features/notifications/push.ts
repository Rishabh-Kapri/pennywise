import * as Device from 'expo-device';
import * as Notifications from 'expo-notifications';
import Constants from 'expo-constants';
import { Platform } from 'react-native';
import { apiClient } from '../../utils/api';

let registeredToken: string | null = null;

// Show pushes while the app is foregrounded (the location snap listener runs
// alongside; the banner doubles as a "new transaction" alert).
Notifications.setNotificationHandler({
  handleNotification: async () => ({
    shouldPlaySound: false,
    shouldSetBadge: false,
    shouldShowBanner: true,
    shouldShowList: true
  })
});

async function getExpoPushToken(): Promise<string | null> {
  if (!Device.isDevice) {
    console.log('[push] skipping registration: not a physical device');
    return null;
  }

  const { status: existingStatus } = await Notifications.getPermissionsAsync();
  let status = existingStatus;
  if (status !== 'granted') {
    ({ status } = await Notifications.requestPermissionsAsync());
  }
  if (status !== 'granted') {
    console.log('[push] notification permission denied');
    return null;
  }

  if (Platform.OS === 'android') {
    await Notifications.setNotificationChannelAsync('default', {
      name: 'Transactions',
      importance: Notifications.AndroidImportance.HIGH
    });
  }

  const projectId = Constants.expoConfig?.extra?.eas?.projectId as string | undefined;
  const token = await Notifications.getExpoPushTokenAsync(projectId ? { projectId } : undefined);
  return token.data;
}

/**
 * Registers this device's Expo push token with the backend so pipeline-created
 * transactions trigger the location snap. Safe to call repeatedly.
 */
export async function registerDevicePushToken(): Promise<void> {
  try {
    const token = await getExpoPushToken();
    if (!token || token === registeredToken) return;
    await apiClient.post('devices/push-token', { expoPushToken: token, platform: Platform.OS });
    registeredToken = token;
    console.log('[push] device push token registered');
  } catch (error) {
    console.log('[push] failed to register push token', error);
  }
}

/** Removes this device's push token from the backend (call on logout). */
export async function unregisterDevicePushToken(): Promise<void> {
  try {
    const token = registeredToken ?? (await getExpoPushToken());
    if (!token) return;
    await apiClient.delete(`devices/push-token?expoPushToken=${encodeURIComponent(token)}`);
    registeredToken = null;
  } catch (error) {
    console.log('[push] failed to unregister push token', error);
  }
}
