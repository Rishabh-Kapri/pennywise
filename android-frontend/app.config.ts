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
  icon: './assets/icon.png',
  ios: {
    supportsTablet: false
  },
  android: {
    package: 'dev.pennywise.cloud',
    googleServicesFile,
    adaptiveIcon: {
      // Foreground art sits inside the 66dp safe zone; the background colour is
      // the app canvas, so the knocked-out "P" reads the same under every
      // launcher mask.
      foregroundImage: './assets/adaptive-icon.png',
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
    [
      'expo-notifications',
      {
        // Android renders the small icon as a flat silhouette from the alpha
        // channel only, so this asset is white-on-transparent; anything with
        // colour would come out as a solid blob.
        icon: './assets/notification-icon.png',
        color: '#8B93FF'
      }
    ],
    [
      'expo-image-picker',
      {
        cameraPermission: 'Pennywise uses the camera to photograph receipts for your transactions.'
      }
    ],
    'expo-document-picker',
    'react-native-document-scanner-plugin',
    [
      'react-native-android-widget',
      {
        // Widget `name` must match the switch in
        // src/features/widgets/widgetTaskHandler.tsx.
        //
        // updatePeriodMillis is Android's 30-minute floor throughout, treated as
        // a fallback: freshness is meant to come from requestWidgetUpdate when
        // the app knows something changed.
        widgets: [
          {
            name: 'Budget',
            label: 'Pennywise budget',
            description: 'Ready to assign this month, with assigned, spent and available.',
            minWidth: '180dp',
            minHeight: '110dp',
            targetCellWidth: 3,
            targetCellHeight: 2,
            resizeMode: 'horizontal|vertical',
            updatePeriodMillis: 1800000
          },
          {
            name: 'Accounts',
            label: 'Pennywise net worth',
            description: 'Total balance across open accounts, split into cash and debt.',
            minWidth: '180dp',
            minHeight: '110dp',
            targetCellWidth: 3,
            targetCellHeight: 2,
            resizeMode: 'horizontal|vertical',
            updatePeriodMillis: 1800000
          },
          {
            name: 'Recent',
            label: 'Pennywise recent transactions',
            description: 'The latest transactions, scrollable when the widget is made taller.',
            minWidth: '250dp',
            minHeight: '150dp',
            targetCellWidth: 4,
            targetCellHeight: 3,
            resizeMode: 'horizontal|vertical',
            updatePeriodMillis: 1800000
          },
          {
            name: 'Pipeline',
            label: 'Pennywise ingestion',
            description: 'Email-to-transaction pipeline health, with one-tap retry for parked runs.',
            minWidth: '180dp',
            minHeight: '110dp',
            targetCellWidth: 3,
            targetCellHeight: 2,
            resizeMode: 'horizontal|vertical',
            updatePeriodMillis: 1800000
          }
        ]
      }
    ]
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
