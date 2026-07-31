import type { ExpoConfig } from 'expo/config';

/**
 * `google-services.json` is gitignored because this repo is public. EAS builds
 * get it from the `GOOGLE_SERVICES_JSON` file environment variable, which EAS
 * materialises on disk and exposes as a path; local builds fall back to the
 * untracked copy in this directory. Without this file the Android build has no
 * FCM credentials and `expo-notifications` cannot obtain a push token.
 */
const googleServicesFile = process.env.GOOGLE_SERVICES_JSON ?? './google-services.json';

const config: ExpoConfig = {
  name: 'Pennywise',
  slug: 'pennywise-android',
  version: '0.1.0',
  orientation: 'portrait',
  scheme: 'dev.pennywise.cloud',
  userInterfaceStyle: 'automatic',
  assetBundlePatterns: ['**/*'],
  ios: {
    supportsTablet: false
  },
  android: {
    package: 'dev.pennywise.cloud',
    googleServicesFile,
    adaptiveIcon: {
      backgroundColor: '#0B0B0F'
    },
    permissions: [
      'android.permission.ACCESS_COARSE_LOCATION',
      'android.permission.ACCESS_FINE_LOCATION',
      'android.permission.CAMERA',
      'android.permission.RECEIVE_BOOT_COMPLETED',
      'android.permission.POST_NOTIFICATIONS'
    ]
  },
  plugins: [
    [
      'expo-location',
      {
        locationWhenInUsePermission:
          'Pennywise attaches your current location to new transactions so you can remember where they happened.'
      }
    ],
    'expo-notifications',
    [
      'expo-image-picker',
      {
        cameraPermission: 'Pennywise uses the camera to photograph receipts for your transactions.'
      }
    ],
    'expo-document-picker',
    'react-native-document-scanner-plugin'
  ],
  extra: {
    apiUrl: process.env.EXPO_PUBLIC_API_URL,
    googleClientId: process.env.EXPO_PUBLIC_GOOGLE_CLIENT_ID,
    androidGoogleClientId: process.env.EXPO_PUBLIC_ANDROID_GOOGLE_CLIENT_ID,
    router: {
      root: 'app'
    },
    // Read by src/features/notifications/push.ts to request an Expo push token.
    eas: {
      projectId: '285e8ad2-57bb-47e7-85f4-331d05616208'
    }
  }
};

export default config;
