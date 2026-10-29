import { useEffect, useMemo, useState } from 'react';
import { Alert, Pressable, RefreshControl, ScrollView, StyleSheet, TextInput, View } from 'react-native';
import { Landmark, Lightbulb, Pencil, Rocket } from 'lucide-react-native';
import { useAppDispatch, useAppSelector } from '../../../app/hooks';
import { AmountSheet } from '../../../components/AmountKeypad';
import { AppText } from '../../../components/AppText';
import { Button } from '../../../components/Button';
import { Card } from '../../../components/Card';
import { BarChart, Legend, Ring } from '../../../components/charts';
import { EmptyState } from '../../../components/EmptyState';
import { Screen } from '../../../components/Screen';
import { Sheet } from '../../../components/Sheet';
import { StackHeader } from '../../../components/StackHeader';
import { apiClient } from '../../../utils/api';
import { formatCurrency, formatMonthShort } from '../../../utils/date';
import { chartColors, colors, radii, spacing } from '../../../theme';
import { fetchAllAccounts } from '../../accounts/store/accountSlice';
import { fetchAllLoanMetadata, updateLoanMetadata } from '../store/loanSlice';
import type { LoanHistory, LoanMetadata } from '../types';
import { formatPayoffDuration, projectPayoff } from '../utils/payoffCalculator';

/**
 * The loan's amortized history from the API. `refreshKey` changes when a
 * payment lands (the account balance moves) or the terms are edited.
 */
function useLoanHistory(accountId: string | undefined, refreshKey: string) {
  const [history, setHistory] = useState<LoanHistory | null>(null);
  const [error, setError] = useState(false);

  useEffect(() => {
    if (!accountId) return;
    let cancelled = false;
    setError(false);
    apiClient
      .get<LoanHistory[]>(`reports/loans?accountIds=${accountId}`)
      .then((histories) => {
        if (!cancelled) setHistory((histories ?? []).find((h) => h.accountId === accountId) ?? null);
      })
      .catch(() => {
        if (!cancelled) setError(true);
      });
    return () => {
      cancelled = true;
    };
  }, [accountId, refreshKey]);

  return { history: history?.accountId === accountId ? history : null, error };
}

function Row({ label, value, tone, sub }: { label: string; value: string; tone?: 'success' | 'danger'; sub?: boolean }) {
  return (
    <View style={[styles.row, sub && styles.subRow]}>
      <AppText variant={sub ? 'caption' : 'body'} muted={sub}>
        {label}
      </AppText>
      <AppText variant={sub ? 'caption' : 'body'} weight={sub ? 'regular' : 'semibold'} tone={tone} tabular>
        {value}
      </AppText>
    </View>
  );
}

function PaymentSplit({ history }: { history: LoanHistory }) {
  const paid = history.months.filter((m) => m.payment > 0);
  const [selected, setSelected] = useState<number | null>(null);
  if (!paid.length) return null;
  const principal = paid.reduce((sum, m) => sum + m.principal, 0);
  const interest = paid.reduce((sum, m) => sum + m.interest, 0);
  const pick = selected != null ? paid[selected] : null;

  return (
    <Card style={styles.card}>
      <AppText variant="heading">Where each payment went</AppText>
      <Legend
        items={[
          { label: 'Principal', color: chartColors.principal, value: formatCurrency(principal) },
          { label: 'Interest', color: chartColors.interest, value: formatCurrency(interest) }
        ]}
      />
      <BarChart
        height={170}
        series={[
          { key: 'interest', label: 'Interest', color: chartColors.interest },
          { key: 'principal', label: 'Principal', color: chartColors.principal }
        ]}
        columns={paid.map((m) => ({
          key: m.month,
          label: formatMonthShort(m.month),
          values: { interest: m.interest, principal: m.principal }
        }))}
        selected={selected}
        onSelect={(index) => setSelected((current) => (current === index ? null : index))}
      />
      {pick ? (
        <View style={styles.detailBox}>
          <AppText variant="caption" weight="semibold">
            {formatMonthShort(pick.month)}
            {pick.assumed ? ' · scheduled, not recorded' : ''}
          </AppText>
          <Row sub label="Principal" value={formatCurrency(pick.principal)} />
          <Row sub label="Interest" value={formatCurrency(pick.interest)} />
          <Row sub label="Balance after" value={formatCurrency(pick.balance)} />
        </View>
      ) : (
        <AppText variant="caption" tone="faint">
          Tap a month for its split.
          {paid.some((m) => m.assumed) ? ' Months before your first recorded payment assume the scheduled one.' : ''}
        </AppText>
      )}
    </Card>
  );
}

function PayoffSimulator({ balance, loan }: { balance: number; loan: LoanMetadata }) {
  const [extra, setExtra] = useState(0);
  const [lumpSum, setLumpSum] = useState(0);
  const [editing, setEditing] = useState<'extra' | 'lump' | null>(null);
  const base = projectPayoff({ currentBalance: balance, interestRate: loan.interestRate, monthlyPayment: loan.monthlyPayment });
  const target = projectPayoff({
    currentBalance: balance,
    interestRate: loan.interestRate,
    monthlyPayment: loan.monthlyPayment,
    extraMonthlyPayment: extra,
    oneTimeExtraPayment: lumpSum
  });
  const changed = extra > 0 || lumpSum > 0;
  const payoffDate = (months: number) => {
    if (!Number.isFinite(months)) return '—';
    // day 1, or adding months to the 31st spills into the month after
    const date = new Date();
    date.setDate(1);
    date.setMonth(date.getMonth() + months);
    return date.toLocaleDateString('en-IN', { month: 'short', year: 'numeric' });
  };

  return (
    <Card style={styles.card}>
      <View style={styles.titleRow}>
        <Rocket size={18} color={colors.primary} />
        <AppText variant="heading">Pay it off faster</AppText>
      </View>
      <View style={styles.inputs}>
        <Pressable style={styles.inputTile} onPress={() => setEditing('extra')}>
          <AppText variant="caption" muted>
            Extra every month
          </AppText>
          <AppText weight="semibold" tabular tone={extra ? undefined : undefined}>
            {extra ? formatCurrency(extra) : 'Add'}
          </AppText>
        </Pressable>
        <Pressable style={styles.inputTile} onPress={() => setEditing('lump')}>
          <AppText variant="caption" muted>
            One-time payment now
          </AppText>
          <AppText weight="semibold" tabular>
            {lumpSum ? formatCurrency(lumpSum) : 'Add'}
          </AppText>
        </Pressable>
      </View>
      {changed ? (
        target.paysOff ? (
          <View style={styles.savings}>
            <View style={styles.savingCell}>
              <AppText variant="caption" muted>
                Interest saved
              </AppText>
              <AppText variant="heading" tone="success" tabular>
                {formatCurrency(base.totalInterest - target.totalInterest)}
              </AppText>
            </View>
            <View style={styles.savingCell}>
              <AppText variant="caption" muted>
                Debt-free sooner by
              </AppText>
              <AppText variant="heading" tone="success">
                {formatPayoffDuration(base.months - target.months)}
              </AppText>
            </View>
            <AppText variant="caption" tone="faint" style={styles.fullWidth}>
              Paid off {payoffDate(target.months)} instead of {payoffDate(base.months)}.
            </AppText>
          </View>
        ) : (
          <AppText variant="caption" tone="danger">
            That payment doesn't cover the interest.
          </AppText>
        )
      ) : (
        <AppText variant="caption" tone="faint">
          Try an extra amount to see the interest and time it saves.
        </AppText>
      )}
      <AmountSheet
        visible={editing !== null}
        title={editing === 'lump' ? 'One-time payment' : 'Extra each month'}
        initialValue={editing === 'lump' ? lumpSum : extra}
        onClose={() => setEditing(null)}
        onSave={(value) => {
          if (editing === 'lump') setLumpSum(Math.max(value, 0));
          else setExtra(Math.max(value, 0));
          setEditing(null);
        }}
      />
    </Card>
  );
}

function EditLoanSheet({ loan, onClose }: { loan: LoanMetadata | null; onClose: () => void }) {
  const dispatch = useAppDispatch();
  const [rate, setRate] = useState('');
  const [payment, setPayment] = useState('');
  const [original, setOriginal] = useState('');
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!loan) return;
    setRate(String(loan.interestRate));
    setPayment(String(loan.monthlyPayment));
    setOriginal(String(loan.originalBalance));
  }, [loan]);

  const save = async () => {
    if (!loan) return;
    const updates = {
      interestRate: Number(rate),
      monthlyPayment: Number(payment.replace(/,/g, '')),
      originalBalance: Number(original.replace(/,/g, ''))
    };
    if (Object.values(updates).some((value) => !Number.isFinite(value) || value < 0)) return;
    setSaving(true);
    try {
      await dispatch(updateLoanMetadata({ accountId: loan.accountId, updates })).unwrap();
      onClose();
    } catch (err) {
      Alert.alert("Couldn't save the loan", err instanceof Error ? err.message : 'Please try again.');
    } finally {
      setSaving(false);
    }
  };

  const field = (label: string, value: string, set: (text: string) => void, suffix?: string) => (
    <View style={styles.editField}>
      <AppText variant="label" tone="faint">
        {label}
      </AppText>
      <View style={styles.editInputRow}>
        <TextInput
          value={value}
          onChangeText={set}
          keyboardType="decimal-pad"
          style={styles.editInput}
          placeholderTextColor={colors.faint}
        />
        {suffix ? <AppText muted>{suffix}</AppText> : null}
      </View>
    </View>
  );

  return (
    <Sheet
      visible={loan !== null}
      title="Loan terms"
      onClose={onClose}
      footer={
        <Button onPress={() => void save()} disabled={saving}>
          {saving ? 'Saving…' : 'Save'}
        </Button>
      }
    >
      <ScrollView keyboardShouldPersistTaps="handled" contentContainerStyle={styles.editBody}>
        {field('Interest rate', rate, setRate, '% a year')}
        {field('Monthly payment (EMI)', payment, setPayment)}
        {field('Original loan amount', original, setOriginal)}
      </ScrollView>
    </Sheet>
  );
}

export function LoansScreen() {
  const dispatch = useAppDispatch();
  const loanAccounts = useAppSelector((state) => state.accounts.loanAccounts);
  const metadata = useAppSelector((state) => state.loans.loanMetadata);
  const refreshing = useAppSelector((state) => state.loans.loading === 'pending');
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [editing, setEditing] = useState(false);

  const account = loanAccounts.find((item) => item.id === selectedId) ?? loanAccounts[0] ?? null;
  const loan = account?.id ? metadata[account.id] ?? null : null;
  const { history, error } = useLoanHistory(account?.id, `${account?.balance ?? 0}:${JSON.stringify(loan)}`);

  // Until the amortized history loads (or if it fails), every payment counts
  // as principal, which overstates progress.
  const data = useMemo(() => {
    if (!loan || !account) return null;
    const payments = Math.abs(account.balance ?? 0);
    const currentBalance = history ? history.balance : Math.max(loan.originalBalance - payments, 0);
    const principalPaid = history ? history.principalPaid : payments;
    const interestPaid = history ? history.interestPaid : 0;
    const projection = projectPayoff({
      currentBalance,
      interestRate: loan.interestRate,
      monthlyPayment: loan.monthlyPayment
    });
    const payoffDate = new Date();
    payoffDate.setDate(1); // see PayoffSimulator
    payoffDate.setMonth(payoffDate.getMonth() + (Number.isFinite(projection.months) ? projection.months : 0));
    return {
      currentBalance,
      principalPaid,
      interestPaid,
      projection,
      payoffDate,
      percentPaid:
        loan.originalBalance > 0 ? Math.min(Math.round((principalPaid / loan.originalBalance) * 100), 100) : 0,
      exact: history !== null
    };
  }, [account, loan, history]);

  const refresh = () => {
    dispatch(fetchAllAccounts());
    dispatch(fetchAllLoanMetadata());
  };

  return (
    <Screen
      style={styles.screen}
      refreshControl={<RefreshControl refreshing={refreshing} onRefresh={refresh} tintColor={colors.primary} />}
    >
      <StackHeader
        title="Loans"
        subtitle={account?.name}
        right={
          loan ? (
            <Pressable
              accessibilityRole="button"
              accessibilityLabel="Edit loan terms"
              style={({ pressed }) => [styles.iconButton, pressed && styles.pressed]}
              onPress={() => setEditing(true)}
            >
              <Pencil size={17} color={colors.text} />
            </Pressable>
          ) : null
        }
      />

      {loanAccounts.length > 1 ? (
        <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={styles.pills}>
          {loanAccounts.map((item) => {
            const active = item.id === account?.id;
            return (
              <Pressable
                key={item.id ?? item.name}
                style={[styles.pill, active && styles.pillActive]}
                onPress={() => item.id && setSelectedId(item.id)}
              >
                <AppText variant="caption" weight="semibold" style={active ? styles.pillActiveText : styles.pillText}>
                  {item.name}
                </AppText>
              </Pressable>
            );
          })}
        </ScrollView>
      ) : null}

      {loanAccounts.length === 0 ? (
        <EmptyState
          icon={<Landmark size={26} color={colors.primary} />}
          title="No loan accounts yet"
          body="Add a loan account on the web, then track its payoff here."
        />
      ) : !loan || !data ? (
        <EmptyState
          icon={<Landmark size={26} color={colors.primary} />}
          title="Loan details missing"
          body="This account has no interest rate or payment yet. Re-create it with the loan form on the web."
        />
      ) : (
        <>
          <Card style={styles.heroCard}>
            <Ring percent={data.percentPaid} size={112} stroke={10} color={colors.success}>
              <AppText variant="heading" tabular>
                {data.percentPaid}%
              </AppText>
              <AppText variant="caption" tone="faint">
                repaid
              </AppText>
            </Ring>
            <View style={styles.heroText}>
              <AppText variant="caption" muted>
                Principal outstanding
              </AppText>
              <AppText variant="title" tabular>
                {formatCurrency(data.currentBalance)}
              </AppText>
              <AppText variant="caption" tone="faint">
                Debt-free{' '}
                {Number.isFinite(data.projection.months)
                  ? data.payoffDate.toLocaleDateString('en-IN', { month: 'short', year: 'numeric' })
                  : 'never at this payment'}
              </AppText>
            </View>
          </Card>

          <Card style={styles.card}>
            <Row label="Paid so far" value={formatCurrency(data.principalPaid + data.interestPaid)} />
            <Row sub label="Principal" value={formatCurrency(data.principalPaid)} tone="success" />
            <Row sub label="Interest" value={formatCurrency(data.interestPaid)} />
            <View style={styles.divider} />
            <Row
              label="Interest still to pay"
              value={Number.isFinite(data.projection.totalInterest) ? formatCurrency(data.projection.totalInterest) : '—'}
            />
            <Row
              label="Total interest over the loan"
              value={
                Number.isFinite(data.projection.totalInterest)
                  ? formatCurrency(data.interestPaid + data.projection.totalInterest)
                  : '—'
              }
            />
            {!data.exact ? (
              <AppText variant="caption" tone="faint">
                {error
                  ? "Couldn't load the payment history, so every payment is counted as principal for now."
                  : 'Loading the payment history…'}
              </AppText>
            ) : null}
          </Card>

          <Card style={styles.card}>
            <View style={styles.titleRow}>
              <Lightbulb size={18} color={colors.warning} />
              <AppText variant="heading">Payoff insight</AppText>
            </View>
            {data.exact && data.projection.nextInterest > 0 && loan.monthlyPayment > 0 ? (
              <AppText muted>
                Right now{' '}
                <AppText weight="semibold">{formatCurrency(data.projection.nextInterest)}</AppText> of each{' '}
                {formatCurrency(loan.monthlyPayment)} payment (
                {Math.round(Math.min(data.projection.nextInterest / loan.monthlyPayment, 1) * 100)}%) goes to interest.
                That share shrinks as the principal does, so extra payments early on save the most.
              </AppText>
            ) : null}
            <AppText muted>
              At <AppText weight="semibold">{formatCurrency(loan.monthlyPayment)}</AppText> a month you'll be done in{' '}
              <AppText weight="semibold">{formatPayoffDuration(data.projection.months)}</AppText>
              {Number.isFinite(data.projection.totalInterest) ? (
                <>
                  , paying <AppText weight="semibold">{formatCurrency(data.projection.totalInterest)}</AppText> more in
                  interest.
                </>
              ) : (
                '.'
              )}
            </AppText>
          </Card>

          {history ? <PaymentSplit key={account?.id} history={history} /> : null}

          <PayoffSimulator key={account?.id} balance={data.currentBalance} loan={loan} />

          <Card style={styles.detailsGrid}>
            {[
              ['Interest rate', `${loan.interestRate}%`],
              ['Monthly payment', formatCurrency(loan.monthlyPayment)],
              ['Original amount', formatCurrency(loan.originalBalance)],
              [
                'Started',
                loan.loanStartDate
                  ? new Date(loan.loanStartDate).toLocaleDateString('en-IN', { month: 'short', year: 'numeric' })
                  : '—'
              ]
            ].map(([label, value]) => (
              <View key={label} style={styles.detailCell}>
                <AppText variant="label" tone="faint">
                  {label}
                </AppText>
                <AppText weight="semibold" tabular>
                  {value}
                </AppText>
              </View>
            ))}
          </Card>
        </>
      )}

      <EditLoanSheet loan={editing ? loan : null} onClose={() => setEditing(false)} />
    </Screen>
  );
}

const styles = StyleSheet.create({
  screen: {
    gap: spacing.lg
  },
  pressed: {
    opacity: 0.7
  },
  iconButton: {
    width: 38,
    height: 38,
    borderRadius: 19,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.surfaceStrong
  },
  pills: {
    gap: spacing.sm
  },
  pill: {
    minHeight: 34,
    justifyContent: 'center',
    paddingHorizontal: spacing.md,
    borderRadius: radii.full,
    backgroundColor: colors.surface
  },
  pillActive: {
    backgroundColor: colors.primaryMuted
  },
  pillText: {
    color: colors.muted
  },
  pillActiveText: {
    color: colors.primary
  },
  heroCard: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.xl
  },
  heroText: {
    flex: 1,
    gap: 2
  },
  card: {
    gap: spacing.md
  },
  titleRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm
  },
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: spacing.md
  },
  subRow: {
    paddingLeft: spacing.md
  },
  divider: {
    height: StyleSheet.hairlineWidth,
    backgroundColor: colors.border
  },
  detailBox: {
    gap: spacing.xs,
    backgroundColor: colors.surfaceStrong,
    borderRadius: radii.md,
    padding: spacing.md
  },
  inputs: {
    flexDirection: 'row',
    gap: spacing.sm
  },
  inputTile: {
    flex: 1,
    gap: 2,
    backgroundColor: colors.surfaceStrong,
    borderRadius: radii.md,
    padding: spacing.md
  },
  savings: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: spacing.md
  },
  savingCell: {
    flex: 1,
    gap: 2
  },
  fullWidth: {
    width: '100%'
  },
  detailsGrid: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    rowGap: spacing.lg
  },
  detailCell: {
    width: '50%',
    gap: spacing.xs
  },
  editBody: {
    gap: spacing.lg,
    paddingBottom: spacing.md
  },
  editField: {
    gap: spacing.sm
  },
  editInputRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    minHeight: 48,
    borderRadius: radii.md,
    paddingHorizontal: spacing.lg,
    backgroundColor: colors.surfaceStrong
  },
  editInput: {
    flex: 1,
    color: colors.text,
    fontSize: 15
  }
});
