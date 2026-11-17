import type { ReactNode } from 'react';
import { StyleSheet, View } from 'react-native';
import { CreditCard, Landmark } from 'lucide-react-native';
import { AppText } from '../../../components/AppText';
import { colors, radii, spacing } from '../../../theme';
import { money, type StatementIdentity } from '../statementIdentity';

/**
 * Whose statement this is, for when, and what's due. "large" is the statement
 * screen's title; the list uses the default.
 */
export function StatementIdentityView({
  identity,
  more = 0,
  large = false,
  children
}: {
  identity: StatementIdentity;
  /** other accounts on the same statement */
  more?: number;
  large?: boolean;
  /** a detail line, e.g. the file name */
  children?: ReactNode;
}) {
  const Icon = identity.isCard ? CreditCard : Landmark;
  return (
    <View style={styles.row}>
      <View style={[styles.icon, large && styles.iconLarge]}>
        <Icon size={large ? 22 : 18} color={colors.primary} />
      </View>
      <View style={styles.text}>
        <View style={styles.nameRow}>
          <AppText variant={large ? 'title' : 'heading'} numberOfLines={1} style={styles.name}>
            {identity.name}
          </AppText>
          {identity.ending ? (
            <AppText variant={large ? 'heading' : 'body'} muted tabular>
              •••• {identity.ending}
            </AppText>
          ) : null}
        </View>
        <AppText variant="caption" muted numberOfLines={1}>
          {identity.period || 'Period not printed'}
          {more > 0 ? ` · +${more} more account${more === 1 ? '' : 's'}` : ''}
        </AppText>
        {identity.totalDue !== undefined ? (
          <AppText variant="caption" numberOfLines={1}>
            Due{' '}
            <AppText variant="caption" weight="semibold" tabular>
              {money(identity.totalDue)}
            </AppText>
            {identity.dueBy ? ` by ${identity.dueBy}` : ''}
          </AppText>
        ) : null}
        {children ? (
          <AppText variant="caption" tone="faint" numberOfLines={1}>
            {children}
          </AppText>
        ) : null}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center', gap: spacing.md },
  icon: {
    width: 40,
    height: 40,
    borderRadius: radii.sm,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.primaryMuted
  },
  iconLarge: { width: 48, height: 48, borderRadius: radii.md },
  text: { flex: 1, minWidth: 0, gap: 2 },
  nameRow: { flexDirection: 'row', alignItems: 'baseline', gap: spacing.sm },
  name: { flexShrink: 1 }
});
