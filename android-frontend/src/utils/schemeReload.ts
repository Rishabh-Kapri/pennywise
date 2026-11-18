import { useEffect } from 'react';
import { Appearance, DevSettings } from 'react-native';
import * as Updates from 'expo-updates';
import { scheme } from '../theme';

/**
 * Restart the JS bundle when the system light/dark setting changes.
 *
 * `theme.ts` picks its palette once at import and every screen bakes it into a
 * module-scope StyleSheet, so the only way to switch is a reload. The boot
 * splash covers it, and the app comes back on the screen state it restores
 * from the store (unsaved editor drafts are lost).
 */
export function useReloadOnSchemeChange() {
  useEffect(() => {
    const subscription = Appearance.addChangeListener(({ colorScheme }) => {
      const next = colorScheme === 'light' ? 'light' : 'dark';
      if (next === scheme) return;
      if (__DEV__) {
        DevSettings.reload();
        return;
      }
      Updates.reloadAsync().catch((error: unknown) => console.log('[theme] reload failed', error));
    });
    return () => subscription.remove();
  }, []);
}
