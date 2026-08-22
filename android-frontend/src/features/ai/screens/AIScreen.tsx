import { useCallback, useEffect, useState } from 'react';
import { Pressable, RefreshControl, StyleSheet, View } from 'react-native';
import { useNavigation } from '@react-navigation/native';
import { ChevronLeft } from 'lucide-react-native';
import { AppText } from '../../../components/AppText';
import { Card } from '../../../components/Card';
import { Screen } from '../../../components/Screen';
import { colors, spacing } from '../../../theme';
import { apiClient } from '../../../utils/api';
import type { CipherPrediction } from '../../transactions/types';
import { APIKeys } from '../components/APIKeys';
import { ClassificationPipeline } from '../components/ClassificationPipeline';
import { PredictionHistory } from '../components/PredictionHistory';
import { PredictionOverview } from '../components/PredictionOverview';

export function AIScreen() {
  const navigation = useNavigation();
  const [predictions, setPredictions] = useState<CipherPrediction[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      setPredictions((await apiClient.get<CipherPrediction[]>('predictions/cipher')) ?? []);
      setError(null);
    } catch (err: unknown) {
      setPredictions([]);
      setError(err instanceof Error ? err.message : 'Failed to load prediction data');
    }
  }, []);

  useEffect(() => {
    void load().finally(() => setLoading(false));
  }, [load]);

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    await load();
    setRefreshing(false);
  }, [load]);

  return (
    <Screen
      style={styles.screen}
      refreshControl={
        <RefreshControl refreshing={refreshing} onRefresh={() => void onRefresh()} tintColor={colors.primary} />
      }
    >
      <View style={styles.headerRow}>
        <Pressable
          accessibilityRole="button"
          accessibilityLabel="Go back"
          style={({ pressed }) => [styles.backButton, pressed && styles.pressed]}
          onPress={() => navigation.goBack()}
        >
          <ChevronLeft size={20} color={colors.text} />
        </Pressable>
        <View style={styles.headerMain}>
          <AppText variant="title">AI & Predictions</AppText>
          <AppText variant="caption" muted>
            How the classifier is performing, and the keys that reach this budget
          </AppText>
        </View>
      </View>

      {error ? (
        <Card style={styles.errorCard}>
          <AppText variant="caption" tone="danger">{error}</AppText>
        </Card>
      ) : null}

      {loading ? (
        <Card>
          <AppText variant="caption" muted>Loading prediction data…</AppText>
        </Card>
      ) : (
        <>
          <PredictionOverview predictions={predictions} />
          <ClassificationPipeline predictions={predictions} />
          <PredictionHistory predictions={predictions} />
        </>
      )}

      <APIKeys />
    </Screen>
  );
}

const styles = StyleSheet.create({
  screen: {
    gap: spacing.xl
  },
  headerRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md
  },
  headerMain: {
    flex: 1,
    gap: 2
  },
  backButton: {
    width: 38,
    height: 38,
    borderRadius: 19,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.surfaceStrong
  },
  errorCard: {
    backgroundColor: colors.dangerMuted
  },
  pressed: {
    opacity: 0.7
  }
});
