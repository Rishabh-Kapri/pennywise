import { useEffect, useState } from 'react';
import { StyleSheet, View } from 'react-native';
import * as Google from 'expo-auth-session/providers/google';
import { Prompt, ResponseType } from 'expo-auth-session';
import * as WebBrowser from 'expo-web-browser';
import { WalletCards } from 'lucide-react-native';
import { useAppDispatch, useAppSelector } from '../../../app/hooks';
import { Button } from '../../../components/Button';
import { Screen } from '../../../components/Screen';
import { AppText } from '../../../components/AppText';
import { LoadingState } from '../../../utils/constants';
import { config } from '../../../config/env';
import { colors, radii, spacing } from '../../../theme';
import { loginWithGoogle } from '../store/authSlice';

WebBrowser.maybeCompleteAuthSession();

export function LoginScreen() {
  const dispatch = useAppDispatch();
  const { loading, error } = useAppSelector((state) => state.auth);
  const googleConfigError =
    !config.googleClientId || !config.androidGoogleClientId
      ? 'Google login is missing an OAuth client ID in the app build.'
      : null;
  const [request, response, promptAsync] = Google.useAuthRequest({
    responseType: ResponseType.Code,
    shouldAutoExchangeCode: false,
    webClientId: config.googleClientId,
    androidClientId: config.androidGoogleClientId,
    scopes: ['https://mail.google.com/', 'https://www.googleapis.com/auth/userinfo.email'],
    prompt: [Prompt.SelectAccount, Prompt.Consent],
    extraParams: {
      access_type: 'offline'
    }
  });

  const [flowMessage, setFlowMessage] = useState<string | null>(null);

  useEffect(() => {
    if (!response) return;
    if (response.type === 'success' && response.params.code) {
      setFlowMessage(null);
      dispatch(loginWithGoogle({
        code: response.params.code,
        redirectUri: request?.redirectUri,
        codeVerifier: request?.codeVerifier
      }));
    } else if (response.type === 'error') {
      setFlowMessage(`Google sign-in failed: ${response.error?.message ?? response.params.error ?? 'unknown error'}`);
    } else if (response.type === 'dismiss' || response.type === 'cancel') {
      setFlowMessage('Sign-in did not complete — the browser closed before returning to the app.');
    }
  }, [dispatch, request?.codeVerifier, request?.redirectUri, response]);

  const isLoading = loading === LoadingState.PENDING;

  return (
    <Screen scroll={false} style={styles.screen}>
      <View style={styles.brand}>
        <View style={styles.logo}>
          <WalletCards size={30} color={colors.primary} />
        </View>
        <AppText variant="display" style={styles.title}>Pennywise</AppText>
        <AppText muted style={styles.subtitle}>
          Budget, track, and classify transactions — automatically.
        </AppText>
      </View>

      <View style={styles.footer}>
        {googleConfigError || error || flowMessage ? (
          <AppText variant="caption" tone="danger" style={styles.error}>
            {googleConfigError ?? error ?? flowMessage}
          </AppText>
        ) : null}
        <Button disabled={isLoading || Boolean(googleConfigError) || !request} onPress={() => void promptAsync()}>
          {isLoading ? 'Signing in...' : 'Continue with Google'}
        </Button>
        <AppText variant="caption" tone="faint" style={styles.hint}>
          Your Google account connects Gmail imports and syncs with the web app.
        </AppText>
      </View>
    </Screen>
  );
}

const styles = StyleSheet.create({
  screen: {
    justifyContent: 'space-between',
    paddingBottom: spacing.xxl
  },
  brand: {
    flex: 1,
    alignItems: 'center',
    justifyContent: 'center',
    gap: spacing.md
  },
  logo: {
    width: 72,
    height: 72,
    borderRadius: radii.lg,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.primaryMuted,
    marginBottom: spacing.sm
  },
  title: {
    fontSize: 34,
    lineHeight: 40
  },
  subtitle: {
    textAlign: 'center',
    maxWidth: 260
  },
  footer: {
    gap: spacing.md
  },
  error: {
    textAlign: 'center'
  },
  hint: {
    textAlign: 'center',
    paddingHorizontal: spacing.lg
  }
});
