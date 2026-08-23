import { useMemo, useState } from 'react';
import { Pressable, StyleSheet, View } from 'react-native';
import { ChevronDown, ChevronRight } from 'lucide-react-native';
import { useAppSelector } from '../../../app/hooks';
import { AppText } from '../../../components/AppText';
import { Card } from '../../../components/Card';
import { colors, radii, spacing } from '../../../theme';
import { formatCurrency } from '../../../utils/date';
import type { CipherPrediction } from '../../transactions/types';
import { formatConfidence, formatDate } from '../utils';

interface ResolvedPrediction extends CipherPrediction {
  payee?: string;
  category?: string;
  actualPayee?: string | null;
  actualCategory?: string | null;
}

function DetailRow({ label, value }: { label: string; value: string }) {
  return (
    <View style={styles.detailRow}>
      <AppText variant="caption" tone="faint">{label}</AppText>
      <AppText variant="caption" style={styles.detailValue} numberOfLines={2}>{value}</AppText>
    </View>
  );
}

function PredictionRow({ prediction, isLast }: { prediction: ResolvedPrediction; isLast: boolean }) {
  const [isExpanded, setIsExpanded] = useState(false);
  const showCorrection =
    prediction.hasUserCorrected && (prediction.actualPayee || prediction.actualCategory);

  return (
    <View style={[styles.row, !isLast && styles.rowDivider]}>
      <Pressable
        accessibilityRole="button"
        accessibilityState={{ expanded: isExpanded }}
        onPress={() => setIsExpanded((prev) => !prev)}
        style={({ pressed }) => [styles.rowHeader, pressed && styles.pressed]}
      >
        {isExpanded ? (
          <ChevronDown size={14} color={colors.muted} />
        ) : (
          <ChevronRight size={14} color={colors.muted} />
        )}
        <View style={styles.rowMain}>
          <View style={styles.rowTop}>
            <AppText weight="medium" numberOfLines={1} style={styles.rowPayee}>
              {prediction.payee ?? '—'}
            </AppText>
            <AppText variant="caption" muted tabular>
              {formatConfidence(prediction.payeeConfidence)}
            </AppText>
          </View>
          <AppText variant="caption" muted numberOfLines={1}>
            {prediction.category ?? '—'}
          </AppText>
          <View style={styles.rowMeta}>
            <View style={styles.sourceBadge}>
              <AppText variant="caption" tone="primary">{prediction.source ?? '—'}</AppText>
            </View>
            <AppText variant="caption" tone="faint">{formatDate(prediction.createdAt)}</AppText>
            {prediction.hasUserCorrected ? (
              <AppText variant="caption" style={styles.correctedText}>corrected</AppText>
            ) : null}
          </View>
        </View>
      </Pressable>

      {isExpanded ? (
        <View style={styles.detail}>
          {showCorrection ? (
            <View style={styles.detailSection}>
              <AppText variant="label" tone="faint">User correction</AppText>
              {prediction.actualPayee ? (
                <DetailRow label="Actual payee" value={prediction.actualPayee} />
              ) : null}
              {prediction.actualCategory ? (
                <DetailRow label="Actual category" value={prediction.actualCategory} />
              ) : null}
            </View>
          ) : null}

          {prediction.llmReasoning ? (
            <View style={styles.detailSection}>
              <AppText variant="label" tone="faint">AI reasoning</AppText>
              <AppText variant="caption" muted>{prediction.llmReasoning}</AppText>
            </View>
          ) : null}

          <View style={styles.detailSection}>
            <AppText variant="label" tone="faint">Extracted entities</AppText>
            <DetailRow label="Account" value={prediction.extractedAccount || '—'} />
            <DetailRow label="Payee" value={prediction.extractedPayee || '—'} />
            <DetailRow
              label="Amount"
              value={prediction.amount != null ? formatCurrency(prediction.amount) : '—'}
            />
          </View>

          {prediction.metadata ? (
            <View style={styles.detailSection}>
              <AppText variant="label" tone="faint">Metadata</AppText>
              <AppText variant="caption" tone="faint" style={styles.metadata}>
                {JSON.stringify(prediction.metadata, null, 2)}
              </AppText>
            </View>
          ) : null}
        </View>
      ) : null}
    </View>
  );
}

export function PredictionHistory({ predictions }: { predictions: CipherPrediction[] }) {
  const allPayees = useAppSelector((state) => state.payees.allPayees);
  const allCategories = useAppSelector((state) => state.categories.allCategories);

  const resolved = useMemo<ResolvedPrediction[]>(
    () =>
      predictions.map((prediction) => ({
        ...prediction,
        payee: allPayees.find((payee) => payee.id === prediction.predictedPayeeId)?.name,
        category: allCategories.find((category) => category.id === prediction.predictedCategoryId)?.name,
        actualPayee: prediction.actualPayeeId
          ? (allPayees.find((payee) => payee.id === prediction.actualPayeeId)?.name ?? null)
          : null,
        actualCategory: prediction.actualCategoryId
          ? (allCategories.find((category) => category.id === prediction.actualCategoryId)?.name ?? null)
          : null
      })),
    [predictions, allPayees, allCategories]
  );

  return (
    <View style={styles.section}>
      <View style={styles.sectionHead}>
        <AppText variant="label" tone="faint">Prediction history</AppText>
        <AppText variant="caption" muted>{resolved.length} records</AppText>
      </View>
      <Card style={styles.card}>
        {resolved.length === 0 ? (
          <AppText variant="caption" muted>No predictions yet.</AppText>
        ) : (
          resolved.map((prediction, index) => (
            <PredictionRow
              key={prediction.id}
              prediction={prediction}
              isLast={index === resolved.length - 1}
            />
          ))
        )}
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
    paddingVertical: spacing.xs
  },
  row: {
    paddingVertical: spacing.md
  },
  rowDivider: {
    borderBottomWidth: StyleSheet.hairlineWidth,
    borderBottomColor: colors.border
  },
  rowHeader: {
    flexDirection: 'row',
    alignItems: 'flex-start',
    gap: spacing.sm
  },
  rowMain: {
    flex: 1,
    gap: 2
  },
  rowTop: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: spacing.sm
  },
  rowPayee: {
    flex: 1
  },
  rowMeta: {
    flexDirection: 'row',
    alignItems: 'center',
    flexWrap: 'wrap',
    gap: spacing.sm,
    marginTop: 2
  },
  correctedText: {
    color: colors.warning
  },
  sourceBadge: {
    paddingHorizontal: spacing.sm,
    paddingVertical: 1,
    borderRadius: radii.full,
    backgroundColor: colors.primaryMuted
  },
  detail: {
    gap: spacing.md,
    marginTop: spacing.md,
    paddingTop: spacing.md,
    paddingLeft: spacing.lg,
    borderTopWidth: StyleSheet.hairlineWidth,
    borderTopColor: colors.border
  },
  detailSection: {
    gap: spacing.xs
  },
  detailRow: {
    flexDirection: 'row',
    alignItems: 'flex-start',
    justifyContent: 'space-between',
    gap: spacing.md
  },
  detailValue: {
    flexShrink: 1,
    textAlign: 'right'
  },
  metadata: {
    fontFamily: 'monospace'
  },
  pressed: {
    opacity: 0.7
  }
});
