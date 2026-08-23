import { StyleSheet, View } from 'react-native';
import { ChevronRight } from 'lucide-react-native';
import { AppText } from '../../../components/AppText';
import { Card } from '../../../components/Card';
import { colors, radii, spacing } from '../../../theme';
import type { CipherPrediction } from '../../transactions/types';
import { computeAvgConfidenceBySource, formatConfidence } from '../utils';

/**
 * The stages an email passes through in cipher's classification pipeline, with
 * the average confidence the predictions from each stage carry.
 */
export function ClassificationPipeline({ predictions }: { predictions: CipherPrediction[] }) {
  const stages = [
    { step: '0', label: 'Data extraction', sub: 'Raw parsing' },
    { step: '1', label: 'Payee rules', sub: confidenceSub(predictions, 'RULE') },
    { step: '2', label: 'Semantic search', sub: confidenceSub(predictions, 'VECTOR') },
    { step: '3', label: 'LLM fallback', sub: confidenceSub(predictions, 'LLM') }
  ];

  return (
    <View style={styles.section}>
      <AppText variant="label" tone="faint" style={styles.sectionLabel}>Classification pipeline</AppText>
      <Card style={styles.card}>
        {stages.map((stage, index) => (
          <View key={stage.step} style={styles.stageWrap}>
            {index > 0 ? (
              <View style={styles.arrowRow}>
                <ChevronRight size={13} color={colors.faint} style={styles.arrow} />
              </View>
            ) : null}
            <View style={styles.stage}>
              <View style={styles.stepBadge}>
                <AppText variant="caption" weight="semibold" tone="primary">{stage.step}</AppText>
              </View>
              <View style={styles.stageMain}>
                <AppText weight="medium">{stage.label}</AppText>
                <AppText variant="caption" tone="faint">{stage.sub}</AppText>
              </View>
            </View>
          </View>
        ))}
      </Card>
    </View>
  );
}

function confidenceSub(predictions: CipherPrediction[], source: string): string {
  const confidence = computeAvgConfidenceBySource(predictions, source);
  return confidence !== null ? `${formatConfidence(confidence)} avg conf` : 'No data';
}

const styles = StyleSheet.create({
  section: {
    gap: spacing.sm
  },
  sectionLabel: {
    marginLeft: spacing.xs
  },
  card: {
    paddingVertical: spacing.md
  },
  stageWrap: {
    gap: spacing.xs
  },
  arrowRow: {
    paddingLeft: 11
  },
  arrow: {
    transform: [{ rotate: '90deg' }]
  },
  stage: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md
  },
  stepBadge: {
    width: 24,
    height: 24,
    borderRadius: radii.full,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.primaryMuted
  },
  stageMain: {
    flex: 1,
    gap: 1
  }
});
