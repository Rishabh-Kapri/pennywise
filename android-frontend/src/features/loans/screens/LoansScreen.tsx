import { StyleSheet, View } from 'react-native';
import { Landmark } from 'lucide-react-native';
import { useAppSelector } from '../../../app/hooks';
import { Card } from '../../../components/Card';
import { Screen } from '../../../components/Screen';
import { AppText } from '../../../components/AppText';
import { EmptyState } from '../../../components/EmptyState';
import { ProgressBar } from '../../../components/ProgressBar';
import { SectionHeader } from '../../../components/SectionHeader';
import { formatCurrency } from '../../../utils/date';
import { colors, radii, spacing } from '../../../theme';
import { getLoanProjection } from '../utils/payoffCalculator';

export function LoansScreen() {
  const loanAccounts = useAppSelector((state) => state.accounts.loanAccounts);
  const metadata = useAppSelector((state) => state.loans.loanMetadata);

  return (
    <Screen style={styles.screen}>
      <SectionHeader title="Loans" subtitle="Payoff progress, balances, and interest projections" />
      {loanAccounts.length === 0 ? (
        <EmptyState
          icon={<Landmark size={26} color={colors.primary} />}
          title="No loan accounts yet"
          body="Add loan accounts from the web or API, then track payoff progress here."
        />
      ) : null}

      {loanAccounts.map((account) => {
        const loan = account.id ? metadata[account.id] : undefined;
        if (!loan) {
          return (
            <Card key={account.id ?? account.name} style={styles.cardGap}>
              <AppText variant="heading">{account.name}</AppText>
              <AppText variant="caption" muted>Loan details missing.</AppText>
            </Card>
          );
        }
        const projection = getLoanProjection(loan, account.balance ?? 0);
        return (
          <Card key={account.id ?? account.name} style={styles.cardGap}>
            <View style={styles.rowBetween}>
              <View style={styles.titleBlock}>
                <AppText variant="heading">{account.name}</AppText>
                <AppText variant="caption" tone="success">{projection.percentPaid}% paid off</AppText>
              </View>
              <AppText variant="heading" tabular>{formatCurrency(projection.currentBalance)}</AppText>
            </View>
            <ProgressBar percent={projection.percentPaid} color={colors.success} />
            <View style={styles.metricRow}>
              <View style={styles.metric}>
                <AppText variant="label" tone="faint">Monthly</AppText>
                <AppText variant="caption" weight="semibold" tabular>{formatCurrency(loan.monthlyPayment)}</AppText>
              </View>
              <View style={styles.metric}>
                <AppText variant="label" tone="faint">Interest</AppText>
                <AppText variant="caption" weight="semibold" tabular>{loan.interestRate}%</AppText>
              </View>
              <View style={styles.metric}>
                <AppText variant="label" tone="faint">Payoff</AppText>
                <AppText variant="caption" weight="semibold" tabular>{projection.months} mo</AppText>
              </View>
            </View>
          </Card>
        );
      })}
    </Screen>
  );
}

const styles = StyleSheet.create({
  screen: {
    gap: spacing.lg
  },
  cardGap: {
    gap: spacing.lg
  },
  rowBetween: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'flex-start',
    gap: spacing.md
  },
  titleBlock: {
    flex: 1,
    gap: 2
  },
  metricRow: {
    flexDirection: 'row',
    gap: spacing.sm
  },
  metric: {
    flex: 1,
    gap: spacing.xs,
    backgroundColor: colors.surfaceStrong,
    borderRadius: radii.sm,
    padding: spacing.md
  }
});
