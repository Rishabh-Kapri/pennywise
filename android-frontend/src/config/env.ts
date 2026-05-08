export const config = {
  apiBaseUrl: process.env.EXPO_PUBLIC_API_URL || 'http://10.0.2.2:5151/api',
  googleClientId: process.env.EXPO_PUBLIC_GOOGLE_CLIENT_ID || '',
  androidGoogleClientId: process.env.EXPO_PUBLIC_ANDROID_GOOGLE_CLIENT_ID || ''
} as const;
