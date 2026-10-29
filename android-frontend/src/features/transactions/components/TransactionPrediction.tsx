import { useEffect, useState } from 'react';
import { ActivityIndicator, Pressable, StyleSheet, View } from 'react-native';
import { ChevronDown, ChevronUp, Sparkles } from 'lucide-react-native';
import { useAppSelector } from '../../../app/hooks';
import { AppText } from '../../../components/AppText';
import { apiClient } from '../../../utils/api';
import { formatCurrency } from '../../../utils/date';
import { colors, radii, spacing } from '../../../theme';
import type { TransactionPredictionDetails } from '../types';

function formatConfidence(value?: number | null) {
  if (value === null || value === undefined) return '—';
  const percent = value <= 1 ? value * 100 : value;
  return `${percent.toFixed(percent >= 10 ? 0 : 1)}%`;
}

function PredictionMetric({ label, value, confidence }: { label: string; value?: string | null; confidence?: number | null }) {
  return (
    <View style={styles.predictionMetric}>
      <AppText variant="label" tone="faint">{label}</AppText>
      <AppText variant="caption" weight="semibold" numberOfLines={2}>{value || '—'}</AppText>
      <AppText variant="caption" tone="primary" tabular>{formatConfidence(confidence)}</AppText>
    </View>
  );
}

function PredictionRow({ label, value }: { label: string; value: string }) {
  return (
    <View style={styles.predictionRow}>
      <AppText variant="caption" muted>{label}</AppText>
      <AppText variant="caption" weight="medium" numberOfLines={1} style={styles.predictionRowValue}>{value}</AppText>
    </View>
  );
}

export function PredictionSection({ transactionId }: { transactionId: string }) {
  const payees = useAppSelector((state) => state.payees.allPayees);
  const categories = useAppSelector((state) => state.categories.allCategoryGroups.flatMap((group) => group.categories));
  const [isOpen, setIsOpen] = useState(false);
  // Latches true on the first expand and never clears. The fetch keys off this
  // rather than `isOpen` so that collapsing the panel -- or any state the effect
  // itself sets -- cannot re-run the effect and abort its own in-flight request.
  const [hasRequested, setHasRequested] = useState(false);
  const [details, setDetails] = useState<TransactionPredictionDetails | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!hasRequested) return;
    let ignore = false;
    setIsLoading(true);
    setError(null);
    apiClient
      .get<TransactionPredictionDetails>(`predictions/transactions/${transactionId}`)
      .then((data) => {
        if (!ignore) setDetails(data ?? {});
      })
      .catch((err: unknown) => {
        if (!ignore) {
          setDetails({});
          setError(err instanceof Error ? err.message : 'Failed to load prediction');
        }
      })
      .finally(() => {
        if (!ignore) setIsLoading(false);
      });
    return () => {
      ignore = true;
    };
  }, [hasRequested, transactionId]);

  const cipher = details?.cipherPrediction;
  const legacy = details?.prediction;

  return (
    <View style={styles.predictionSection}>
      <Pressable
        style={({ pressed }) => [styles.predictionHeader, pressed && styles.pressed]}
        onPress={() => {
          setIsOpen((open) => !open);
          setHasRequested(true);
        }}
      >
        <Sparkles size={15} color={colors.primary} />
        <AppText variant="caption" weight="semibold" style={styles.predictionTitle}>AI prediction</AppText>
        {isOpen ? <ChevronUp size={15} color={colors.faint} /> : <ChevronDown size={15} color={colors.faint} />}
      </Pressable>

      {isOpen ? (
        <View style={styles.predictionBody}>
          {isLoading ? (
            <View style={styles.predictionLoading}>
              <ActivityIndicator size="small" color={colors.primary} />
              <AppText variant="caption" muted>Loading prediction</AppText>
            </View>
          ) : error ? (
            <AppText variant="caption" tone="danger">{error}</AppText>
          ) : cipher ? (
            <>
              <View style={styles.predictionGrid}>
                <PredictionMetric label="Account" value={cipher.extractedAccount} confidence={cipher.accountConfidence} />
                <PredictionMetric
                  label="Payee"
                  value={payees.find((payee) => payee.id === cipher.predictedPayeeId)?.name || cipher.extractedPayee}
                  confidence={cipher.payeeConfidence}
                />
                <PredictionMetric
                  label="Category"
                  value={categories.find((category) => category.id === cipher.predictedCategoryId)?.name}
                  confidence={cipher.categoryConfidence}
                />
              </View>
              <PredictionRow label="Predicted amount" value={cipher.amount != null ? formatCurrency(cipher.amount) : '—'} />
              <PredictionRow label="Source" value={cipher.source || '—'} />
              <PredictionRow label="User corrected" value={cipher.hasUserCorrected ? 'Yes' : 'No'} />
              {cipher.llmReasoning ? (
                <View style={styles.reasoningBlock}>
                  <AppText variant="caption" muted>{cipher.llmReasoning}</AppText>
                </View>
              ) : null}
            </>
          ) : legacy ? (
            <>
              <View style={styles.predictionGrid}>
                <PredictionMetric label="Account" value={legacy.account} confidence={legacy.accountPrediction} />
                <PredictionMetric label="Payee" value={legacy.payee} confidence={legacy.payeePrediction} />
                <PredictionMetric label="Category" value={legacy.category} confidence={legacy.categoryPrediction} />
              </View>
              <PredictionRow label="Predicted amount" value={legacy.amount != null ? formatCurrency(legacy.amount) : '—'} />
              <PredictionRow label="User corrected" value={legacy.hasUserCorrected ? 'Yes' : 'No'} />
              {legacy.hasUserCorrected ? (
                <>
                  <PredictionRow label="Corrected account" value={legacy.userCorrectedAccount || '—'} />
                  <PredictionRow label="Corrected payee" value={legacy.userCorrectedPayee || '—'} />
                  <PredictionRow label="Corrected category" value={legacy.userCorrectedCategory || '—'} />
                </>
              ) : null}
            </>
          ) : (
            <AppText variant="caption" muted>No prediction found for this transaction.</AppText>
          )}
        </View>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  predictionSection: {
    backgroundColor: colors.surfaceStrong,
    borderRadius: radii.md,
    overflow: 'hidden'
  },
  predictionHeader: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    minHeight: 44,
    paddingHorizontal: spacing.lg
  },
  predictionTitle: {
    flex: 1
  },
  predictionBody: {
    gap: spacing.sm,
    paddingHorizontal: spacing.lg,
    paddingBottom: spacing.lg
  },
  predictionLoading: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    minHeight: 32
  },
  predictionGrid: {
    flexDirection: 'row',
    gap: spacing.sm,
    marginBottom: spacing.xs
  },
  predictionMetric: {
    flex: 1,
    gap: 3,
    backgroundColor: colors.surface,
    borderRadius: radii.sm,
    padding: spacing.md
  },
  predictionRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: spacing.md
  },
  predictionRowValue: {
    flexShrink: 1,
    textAlign: 'right'
  },
  reasoningBlock: {
    backgroundColor: colors.surface,
    borderRadius: radii.sm,
    padding: spacing.md,
    marginTop: spacing.xs
  },
  pressed: {
    opacity: 0.7
  }
});
