import { useState } from 'react';
import { ActivityIndicator, Pressable, StyleSheet, View } from 'react-native';
import { ChevronDown, ChevronRight, Mail, XCircle } from 'lucide-react-native';
import { AppText } from '../../../components/AppText';
import { colors, radii, spacing } from '../../../theme';
import { formatCurrency } from '../../../utils/date';
import type { EmailGroup } from '../utils';
import { detailNumber, detailString, eventCalls, formatSender, senderAddress } from '../utils';
import { StepUsage } from './RunPieces';

export function EmailCard({ group }: { group: EmailGroup }) {
  const [showReasoning, setShowReasoning] = useState(false);

  const parseSkipped = group.parse?.status === 'skipped';
  const parseFailed = group.parse?.status === 'failed';
  const predictFailed = group.predict?.status === 'failed';
  const amount = detailNumber(group.parse, 'amount');
  const source = detailString(group.predict, 'source');
  const reasoning = detailString(group.predict, 'reasoning');

  // Captured at fetch time, so it is available before extraction runs.
  const from = detailString(group.fetch, 'from');
  const subject = detailString(group.fetch, 'subject');
  const snippet = detailString(group.fetch, 'snippet');
  const address = senderAddress(from);

  return (
    <View style={styles.card}>
      {group.fetch ? (
        <View style={styles.source}>
          <View style={styles.sourceTop}>
            <Mail size={14} color={colors.faint} />
            <AppText variant="caption" weight="semibold" numberOfLines={1} style={styles.sender}>
              {from ? formatSender(from) : 'Unknown sender'}
            </AppText>
            {address ? (
              <AppText variant="caption" tone="faint" numberOfLines={1} style={styles.address}>
                {address}
              </AppText>
            ) : null}
          </View>
          {subject ? (
            <AppText variant="caption" numberOfLines={2}>{subject}</AppText>
          ) : null}
          {snippet ? (
            <AppText variant="caption" tone="faint" numberOfLines={3}>{snippet}</AppText>
          ) : null}
        </View>
      ) : null}

      {!group.parse && !group.predict ? (
        <View style={styles.pendingRow}>
          <ActivityIndicator size="small" color={colors.primary} />
          <AppText variant="caption" muted>Waiting for extraction…</AppText>
        </View>
      ) : null}

      {group.parse || group.predict ? (
        <View style={styles.headerRow}>
          {parseSkipped ? (
            <AppText variant="caption" muted>Skipped — not a transaction email</AppText>
          ) : parseFailed ? (
            <View style={styles.errorRow}>
              <XCircle size={13} color={colors.danger} />
              <AppText variant="caption" tone="danger">Extraction failed for this email</AppText>
            </View>
          ) : (
            <>
              <AppText weight="medium" numberOfLines={1} style={styles.merchant}>
                {detailString(group.parse, 'merchant') || 'Unknown merchant'}
              </AppText>
              {amount !== null ? (
                <AppText weight="semibold" tabular>{formatCurrency(amount)}</AppText>
              ) : null}
            </>
          )}
        </View>
      ) : null}

      {!parseSkipped && !parseFailed && group.parse ? (
        <View style={styles.metaRow}>
          {[
            detailString(group.parse, 'account'),
            detailString(group.parse, 'date'),
            detailString(group.parse, 'transactionType')
          ]
            .filter(Boolean)
            .map((value) => (
              <AppText key={value} variant="caption" tone="faint">{value}</AppText>
            ))}
        </View>
      ) : null}

      {group.predict && !parseSkipped ? (
        <View style={styles.predictionRow}>
          {predictFailed ? (
            <View style={styles.errorRow}>
              <XCircle size={13} color={colors.danger} />
              <AppText variant="caption" tone="danger">Prediction failed for this email</AppText>
            </View>
          ) : (
            <>
              <AppText variant="caption" style={styles.predictionText} numberOfLines={2}>
                {detailString(group.predict, 'payee') || 'Unknown payee'}
                <AppText variant="caption" tone="faint"> → </AppText>
                {detailString(group.predict, 'category') || 'Uncategorized'}
              </AppText>
              {source ? (
                <View style={styles.sourceBadge}>
                  <AppText variant="caption" tone="primary">{source.toLowerCase()}</AppText>
                </View>
              ) : null}
              {detailString(group.predict, 'confidence') ? (
                <AppText variant="caption" tone="faint">
                  {detailString(group.predict, 'confidence')} confidence
                </AppText>
              ) : null}
            </>
          )}
        </View>
      ) : null}

      <StepUsage label="extract" calls={eventCalls(group.parse)} />
      <StepUsage label="predict" calls={eventCalls(group.predict)} />

      {reasoning && !predictFailed && !parseSkipped ? (
        <View style={styles.reasoning}>
          <Pressable
            accessibilityRole="button"
            onPress={() => setShowReasoning((prev) => !prev)}
            style={({ pressed }) => [styles.reasoningToggle, pressed && styles.pressed]}
          >
            {showReasoning ? (
              <ChevronDown size={12} color={colors.muted} />
            ) : (
              <ChevronRight size={12} color={colors.muted} />
            )}
            <AppText variant="caption" muted>reasoning</AppText>
          </Pressable>
          {showReasoning ? (
            <AppText variant="caption" tone="faint">{reasoning}</AppText>
          ) : null}
        </View>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  card: {
    gap: spacing.sm,
    padding: spacing.md,
    borderRadius: radii.md,
    backgroundColor: colors.surfaceStrong
  },
  source: {
    gap: 2
  },
  sourceTop: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.xs
  },
  sender: {
    flexShrink: 1
  },
  address: {
    flexShrink: 1
  },
  pendingRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm
  },
  headerRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: spacing.sm
  },
  merchant: {
    flex: 1
  },
  metaRow: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: spacing.md
  },
  predictionRow: {
    flexDirection: 'row',
    alignItems: 'center',
    flexWrap: 'wrap',
    gap: spacing.sm
  },
  predictionText: {
    flexShrink: 1
  },
  sourceBadge: {
    paddingHorizontal: spacing.sm,
    paddingVertical: 2,
    borderRadius: radii.full,
    backgroundColor: colors.primaryMuted
  },
  errorRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.xs
  },
  reasoning: {
    gap: spacing.xs
  },
  reasoningToggle: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.xs
  },
  pressed: {
    opacity: 0.7
  }
});
