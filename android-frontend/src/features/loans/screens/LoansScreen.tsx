import { StyleSheet, View } from 'react-native';
import { Landmark } from 'lucide-react-native';
import { useAppSelector } from '../../../app/hooks';
import { Card } from '../../../components/Card';
import { Screen } from '../../../components/Screen';
import { AppText } from '../../../components/AppText';
import { SectionHeader } from '../../../components/SectionHeader';
import { formatCurrency } from '../../../utils/date';
import { colors, spacing } from '../../../theme';
import { getLoanProjection } from '../utils/payoffCalculator';

export function LoansScreen() {
  const loanAccounts = useAppSelector((state) => state.accounts.loanAccounts);
  const metadata = useAppSelector((state) => state.loans.loanMetadata);

  return (
    <Screen style={styles.screen}>
      <SectionHeader title="Loans" subtitle="Track payoff progress, remaining balance, and interest projection." />
      {loanAccounts.length === 0 ? (
        <Card style={styles.empty}>
          <Landmark size={34} color={colors.primary} />
          <AppText weight="semibold">No loan accounts yet</AppText>
          <AppText muted>Add loan accounts from the web or API, then track payoff progress here.</AppText>
        </Card>
      ) : null}

      {loanAccounts.map((account) => {
        const loan = account.id ? metadata[account.id] : undefined;
        if (!loan) {
          return (
            <Card key={account.id ?? account.name} style={styles.cardGap}>
              <AppText weight="semibold">{account.name}</AppText>
              <AppText muted>Loan details missing.</AppText>
            </Card>
          );
        }
        const projection = getLoanProjection(loan, account.balance ?? 0);
        return (
          <Card key={account.id ?? account.name} style={styles.cardGap}>
            <View style={styles.rowBetween}>
              <View>
                <AppText weight="semibold" style={styles.cardTitle}>{account.name}</AppText>
                <AppText muted>{projection.percentPaid}% paid off</AppText>
              </View>
              <AppText weight="bold">{formatCurrency(projection.currentBalance)}</AppText>
            </View>
            <View style={styles.progressTrack}>
              <View style={[styles.progressFill, { width: `${projection.percentPaid}%` }]} />
            </View>
            <View style={styles.metricRow}>
              <View>
                <AppText muted>Monthly payment</AppText>
                <AppText weight="semibold">{formatCurrency(loan.monthlyPayment)}</AppText>
              </View>
              <View>
                <AppText muted>Interest</AppText>
                <AppText weight="semibold">{loan.interestRate}%</AppText>
              </View>
              <View>
                <AppText muted>Payoff</AppText>
                <AppText weight="semibold">{projection.months} mo</AppText>
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
  empty: {
    alignItems: 'center',
    gap: spacing.sm
  },
  cardGap: {
    gap: spacing.md
  },
  cardTitle: {
    fontSize: 18
  },
  rowBetween: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    gap: spacing.md
  },
  progressTrack: {
    height: 10,
    borderRadius: 5,
    backgroundColor: colors.border,
    overflow: 'hidden'
  },
  progressFill: {
    height: '100%',
    backgroundColor: colors.primary
  },
  metricRow: {
    flexDirection: 'row',
    justifyContent: 'space-between'
  }
});
