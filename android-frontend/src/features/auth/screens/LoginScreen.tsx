import { useEffect } from 'react';
import { StyleSheet, View } from 'react-native';
import * as Google from 'expo-auth-session/providers/google';
import { Prompt, ResponseType } from 'expo-auth-session';
import * as WebBrowser from 'expo-web-browser';
import { WalletCards } from 'lucide-react-native';
import { useAppDispatch, useAppSelector } from '../../../app/hooks';
import { Button } from '../../../components/Button';
import { Card } from '../../../components/Card';
import { Screen } from '../../../components/Screen';
import { AppText } from '../../../components/AppText';
import { LoadingState } from '../../../utils/constants';
import { config } from '../../../config/env';
import { colors, spacing } from '../../../theme';
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

  useEffect(() => {
    if (response?.type === 'success' && response.params.code) {
      dispatch(loginWithGoogle({
        code: response.params.code,
        redirectUri: request?.redirectUri,
        codeVerifier: request?.codeVerifier
      }));
    }
  }, [dispatch, request?.codeVerifier, request?.redirectUri, response]);

  const isLoading = loading === LoadingState.PENDING;

  return (
    <Screen scroll={false} style={styles.screen}>
      <View style={styles.brand}>
        <View style={styles.logo}>
          <WalletCards size={30} color="#fff" />
        </View>
        <AppText weight="bold" style={styles.title}>
          Pennywise
        </AppText>
        <AppText muted style={styles.subtitle}>
          Budget, track, and classify transactions from Gmail.
        </AppText>
      </View>

      <Card style={styles.card}>
        <AppText weight="semibold" style={styles.cardTitle}>
          Continue with Google
        </AppText>
        <AppText muted>
          The API exchanges your Google auth code for Pennywise access and refresh tokens, matching the web login flow.
        </AppText>
        {googleConfigError || error ? <AppText style={styles.error}>{googleConfigError ?? error}</AppText> : null}
        <Button disabled={isLoading || Boolean(googleConfigError) || !request} onPress={() => void promptAsync()}>
          {isLoading ? 'Signing in...' : 'Sign in'}
        </Button>
      </Card>
    </Screen>
  );
}

const styles = StyleSheet.create({
  screen: {
    justifyContent: 'center',
    gap: spacing.xl
  },
  brand: {
    alignItems: 'center',
    gap: spacing.sm
  },
  logo: {
    width: 64,
    height: 64,
    borderRadius: 16,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.primary
  },
  title: {
    fontSize: 36,
    lineHeight: 42
  },
  subtitle: {
    textAlign: 'center'
  },
  card: {
    gap: spacing.lg
  },
  cardTitle: {
    fontSize: 18
  },
  error: {
    color: colors.danger
  }
});
