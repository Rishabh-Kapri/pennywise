import { useState } from 'react';
import { Bar, BarChart, CartesianGrid, ReferenceLine, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { Bank, ChartBar, PiggyBank } from '@phosphor-icons/react';
import { DashboardCard, Segmented } from '@/features/dashboard/components/DashboardCard';
import { formatCurrency } from '@/features/dashboard/utils';
import { monthLabel, type AccountFlowSummary, type CashFlowSummary } from '../cashFlow';
import { useCashFlowSummary } from '../hooks/useCashFlowSummary';
import { AXIS_TICK, COLORS, CURSOR_FILL, GRID_COLOR, axisCurrency, formatMonthTick } from '../chartTheme';
import { EmptyState, MonthTable, Stat, StatRow, TooltipCard } from './ReportParts';
import styles from './InvestingReport.module.css';

const VIEW_OPTIONS = [
  { value: 'chart', label: 'Chart' },
  { value: 'table', label: 'Table' },
] as const;

function signed(value: number) {
  const rounded = Math.round(value);
  if (rounded === 0) return formatCurrency(0);
  return `${rounded < 0 ? '−' : '+'}${formatCurrency(rounded)}`;
}

export default function InvestingReport() {
  const { summary, loading } = useCashFlowSummary();

  if (!summary) {
    return <EmptyState>{loading ? 'Loading investments…' : 'No data in the selected period'}</EmptyState>;
  }
  if (summary.invested <= 0 && summary.withdrawn <= 0 && summary.debtAccounts.length === 0) {
    return (
      <EmptyState>
        No money moved to or from asset accounts in this period. Transfers to an asset account (mutual funds, stocks, an
        FD) show up here.
      </EmptyState>
    );
  }

  const net = summary.invested - summary.withdrawn;

  return (
    <div className={styles.stack}>
      <StatRow>
        <Stat
          label="Put in"
          color={COLORS.invested}
          value={summary.invested}
          detail={
            summary.savedInGroups > 0
              ? `Includes ${formatCurrency(summary.savedInGroups)} from saving groups`
              : 'Moved into asset accounts'
          }
        />
        <Stat label="Taken out" value={summary.withdrawn} detail="Moved back into your budget accounts" />
        <Stat
          label={net >= 0 ? 'Net invested' : 'Net withdrawn'}
          value={Math.abs(net)}
          detail={net >= 0 ? 'Your investments grew by this much in new money' : 'More came out than went in'}
        />
      </StatRow>

      <MonthlyCard summary={summary} />
      <AccountsCard summary={summary} />
      {summary.debtAccounts.length > 0 && <DebtCard accounts={summary.debtAccounts} />}
    </div>
  );
}

interface MonthDatum {
  month: string;
  invested: number;
  withdrawn: number;
  net: number;
}

function MonthlyCard({ summary }: { summary: CashFlowSummary }) {
  const [view, setView] = useState<'chart' | 'table'>('chart');
  const data: MonthDatum[] = summary.months.map((month) => {
    const invested = summary.investedMonthly[month] ?? 0;
    const withdrawn = summary.withdrawnMonthly[month] ?? 0;
    return { month, invested, withdrawn, net: invested - withdrawn };
  });

  return (
    <DashboardCard
      title="Net invested by month"
      icon={<ChartBar size={18} />}
      actions={<Segmented label="View" options={VIEW_OPTIONS} value={view} onChange={setView} />}
    >
      {view === 'chart' ? (
        <>
          <p className={styles.help}>
            Above the line: more went in than came out. Below: you drew on your investments.
          </p>
          <ResponsiveContainer width="100%" height={260} initialDimension={{ width: 600, height: 260 }}>
            <BarChart data={data} barCategoryGap="24%">
              <CartesianGrid vertical={false} stroke={GRID_COLOR} />
              <XAxis
                dataKey="month"
                tickFormatter={formatMonthTick}
                axisLine={false}
                tickLine={false}
                tick={AXIS_TICK}
                minTickGap={8}
              />
              <YAxis axisLine={false} tickLine={false} width={56} tick={AXIS_TICK} tickFormatter={axisCurrency} />
              <ReferenceLine y={0} stroke="var(--color-border)" />
              <Tooltip
                cursor={{ fill: CURSOR_FILL }}
                content={({ active, payload }) => {
                  const datum = payload?.[0]?.payload as MonthDatum | undefined;
                  if (!active || !datum) return null;
                  return (
                    <TooltipCard
                      title={monthLabel(datum.month)}
                      rows={[
                        { name: 'Put in', value: formatCurrency(datum.invested) },
                        { name: 'Taken out', value: formatCurrency(datum.withdrawn) },
                      ]}
                      footer={`Net ${signed(datum.net)}`}
                    />
                  );
                }}
              />
              <Bar
                dataKey="net"
                name="Net invested"
                fill={COLORS.invested}
                radius={4}
                maxBarSize={24}
                isAnimationActive={false}
              />
            </BarChart>
          </ResponsiveContainer>
        </>
      ) : (
        <MonthTable
          months={summary.months}
          rows={[
            { key: 'in', label: 'Put in', monthly: summary.investedMonthly },
            { key: 'out', label: 'Taken out', monthly: summary.withdrawnMonthly },
            {
              key: 'net',
              label: 'Net',
              monthly: Object.fromEntries(data.map((d) => [d.month, d.net])),
              emphasis: true,
            },
          ]}
        />
      )}
    </DashboardCard>
  );
}

function AccountsCard({ summary }: { summary: CashFlowSummary }) {
  const savingGroups = summary.groups.filter((group) => group.treatment === 'saving');

  return (
    <DashboardCard title="By account" icon={<PiggyBank size={18} />}>
      <table className={styles.table}>
        <thead>
          <tr>
            <th className={styles.nameCol}>Account</th>
            <th>Put in</th>
            <th>Taken out</th>
            <th>Net</th>
          </tr>
        </thead>
        <tbody>
          {summary.assetAccounts.map((account) => (
            <tr key={account.id}>
              <td className={styles.nameCol}>
                <span className={styles.accountName}>{account.name}</span>
                {account.closed && <span className={styles.tag}>Closed</span>}
                {account.recategorizedCount > 0 && (
                  <span className={styles.hint}>
                    {account.recategorizedCount} withdrawal{account.recategorizedCount === 1 ? '' : 's'} (
                    {formatCurrency(account.recategorizedAmount)}) filed under a category, counted here anyway
                  </span>
                )}
              </td>
              <td>{formatCurrency(account.out)}</td>
              <td>{formatCurrency(account.in)}</td>
              <td className={styles.strong}>{signed(account.out - account.in)}</td>
            </tr>
          ))}
          {savingGroups.map((group) => (
            <tr key={group.id}>
              <td className={styles.nameCol}>
                <span className={styles.accountName}>{group.name}</span>
                <span className={styles.hint}>Category group counted as saving</span>
              </td>
              <td>{formatCurrency(Math.max(group.total, 0))}</td>
              <td>{formatCurrency(Math.max(-group.total, 0))}</td>
              <td className={styles.strong}>{signed(group.total)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </DashboardCard>
  );
}

function DebtCard({ accounts }: { accounts: AccountFlowSummary[] }) {
  return (
    <DashboardCard title="Loans & debt" icon={<Bank size={18} />}>
      <table className={styles.table}>
        <thead>
          <tr>
            <th className={styles.nameCol}>Account</th>
            <th>Paid</th>
            <th>Borrowed</th>
            <th>Net repaid</th>
          </tr>
        </thead>
        <tbody>
          {accounts.map((account) => (
            <tr key={account.id}>
              <td className={styles.nameCol}>
                <span className={styles.accountName}>{account.name}</span>
                {account.closed && <span className={styles.tag}>Closed</span>}
              </td>
              <td>{formatCurrency(account.out)}</td>
              <td>{formatCurrency(account.in)}</td>
              <td className={styles.strong}>{signed(account.out - account.in)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </DashboardCard>
  );
}
