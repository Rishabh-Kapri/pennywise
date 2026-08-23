import { StyleSheet, View } from 'react-native';
import { AppText } from '../../../components/AppText';
import { Card } from '../../../components/Card';
import { colors, radii, spacing } from '../../../theme';
import type { CipherPrediction } from '../../transactions/types';
import type { PredictionSource } from '../types';
import { computeAvgConfidence, computeSourceCounts, countLast30Days, formatConfidence } from '../utils';

const SOURCE_COLORS: Record<PredictionSource, string> = {
  RULE: colors.success,
  VECTOR: colors.info,
  LLM: colors.primary,
  MANUAL: colors.warning,
  UNCATEGORIZED: colors.faint
};

function StatCard({ label, value, sub }: { label: string; value: string | number; sub?: string }) {
  return (
    <View style={styles.statCard}>
      <AppText variant="caption" tone="faint">{label}</AppText>
      <AppText variant="heading" tabular>{value}</AppText>
      {sub ? <AppText variant="caption" muted>{sub}</AppText> : null}
    </View>
  );
}

export function PredictionOverview({ predictions }: { predictions: CipherPrediction[] }) {
  const total = predictions.length;
  const corrected = predictions.filter((prediction) => prediction.hasUserCorrected).length;
  const payeeCorrected = predictions.filter(
    (prediction) =>
      prediction.hasUserCorrected &&
      prediction.actualPayeeId != null &&
      prediction.actualPayeeId !== prediction.predictedPayeeId
  ).length;
  const categoryCorrected = predictions.filter(
    (prediction) =>
      prediction.hasUserCorrected &&
      prediction.actualCategoryId != null &&
      prediction.actualCategoryId !== prediction.predictedCategoryId
  ).length;
  const payeeRate = total > 0 ? ((payeeCorrected / total) * 100).toFixed(1) : '0';
  const categoryRate = total > 0 ? ((categoryCorrected / total) * 100).toFixed(1) : '0';
  const avgConfidence = computeAvgConfidence(predictions);
  const sourceCounts = computeSourceCounts(predictions);

  const sources = (Object.keys(sourceCounts) as PredictionSource[])
    .filter((source) => sourceCounts[source] > 0)
    .sort((a, b) => sourceCounts[b] - sourceCounts[a]);

  return (
    <View style={styles.section}>
      <View style={styles.sectionHead}>
        <AppText variant="label" tone="faint">Prediction overview</AppText>
        <AppText variant="caption" muted>{total} total</AppText>
      </View>
      <Card style={styles.card}>
        <View style={styles.statsRow}>
          <StatCard label="Corrected" value={corrected} />
          <View style={styles.statDivider} />
          <StatCard
            label="Avg confidence"
            value={avgConfidence !== null ? formatConfidence(avgConfidence) : '—'}
          />
          <View style={styles.statDivider} />
          <StatCard label="Last 30 days" value={countLast30Days(predictions)} />
        </View>

        <AppText variant="caption" muted>
          {payeeCorrected} payee ({payeeRate}%) · {categoryCorrected} category ({categoryRate}%)
        </AppText>

        {sources.length > 0 ? (
          <View style={styles.sourceBlock}>
            <AppText variant="label" tone="faint">Source breakdown</AppText>
            <View style={styles.sourceRow}>
              {sources.map((source) => (
                <View key={source} style={styles.sourceItem}>
                  <View style={[styles.sourceDot, { backgroundColor: SOURCE_COLORS[source] }]} />
                  <AppText variant="caption" weight="medium">{source}</AppText>
                  <AppText variant="caption" tone="faint" tabular>{sourceCounts[source]}</AppText>
                </View>
              ))}
            </View>
          </View>
        ) : null}
      </Card>
    </View>
  );
}

const styles = StyleSheet.create({
  section: {
    gap: spacing.sm
  },
  sectionHead: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginLeft: spacing.xs
  },
  card: {
    gap: spacing.md
  },
  statsRow: {
    flexDirection: 'row',
    alignItems: 'stretch'
  },
  statCard: {
    flex: 1,
    alignItems: 'center',
    gap: 2
  },
  statDivider: {
    width: StyleSheet.hairlineWidth,
    backgroundColor: colors.border
  },
  sourceBlock: {
    gap: spacing.sm,
    paddingTop: spacing.md,
    borderTopWidth: StyleSheet.hairlineWidth,
    borderTopColor: colors.border
  },
  sourceRow: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: spacing.sm
  },
  sourceItem: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.xs,
    paddingHorizontal: spacing.sm,
    paddingVertical: spacing.xs,
    borderRadius: radii.full,
    backgroundColor: colors.surfaceStrong
  },
  sourceDot: {
    width: 7,
    height: 7,
    borderRadius: 4
  }
});
