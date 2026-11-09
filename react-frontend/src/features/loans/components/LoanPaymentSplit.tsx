import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { formatCompactCurrency, formatCurrency } from '@/features/dashboard/utils';
import type { LoanMonth } from '../types/loan.types';
import styles from './LoanPaymentSplit.module.css';

// Theme-aware steps live in the CSS module (same blue/orange pair as Reports).
const PRINCIPAL_COLOR = 'var(--loan-principal)';
const INTEREST_COLOR = 'var(--loan-interest)';

function monthLabel(monthKey: string) {
  const [year, month] = monthKey.split('-');
  const date = new Date(parseInt(year, 10), parseInt(month, 10) - 1, 1);
  return `${date.toLocaleString('en-us', { month: 'short' })} ’${year.slice(2)}`;
}

/** How each recorded payment split between interest and principal. */
export default function LoanPaymentSplit({ months }: { months: LoanMonth[] }) {
  const paid = months.filter((m) => m.payment > 0);
  const principal = paid.reduce((sum, m) => sum + m.principal, 0);
  const interest = paid.reduce((sum, m) => sum + m.interest, 0);
  const assumed = paid.some((m) => m.assumed);

  return (
    <section className={styles.card} aria-labelledby="loan-split-title">
      <div className={styles.header}>
        <h3 id="loan-split-title" className={styles.title}>
          Where each payment went
        </h3>
        <div className={styles.legend}>
          <span className={styles.legendItem}>
            <span className={styles.swatch} style={{ background: PRINCIPAL_COLOR }} />
            Principal <strong>{formatCurrency(principal)}</strong>
          </span>
          <span className={styles.legendItem}>
            <span className={styles.swatch} style={{ background: INTEREST_COLOR }} />
            Interest <strong>{formatCurrency(interest)}</strong>
          </span>
        </div>
      </div>
      <ResponsiveContainer width="100%" height={220} initialDimension={{ width: 600, height: 220 }}>
        <BarChart data={paid} barCategoryGap="28%">
          <CartesianGrid vertical={false} stroke="var(--color-border-muted)" />
          <XAxis
            dataKey="month"
            tickFormatter={monthLabel}
            axisLine={false}
            tickLine={false}
            tick={{ fill: 'var(--color-text-secondary)', fontSize: 12 }}
            minTickGap={8}
          />
          <YAxis
            axisLine={false}
            tickLine={false}
            width={52}
            tick={{ fill: 'var(--color-text-secondary)', fontSize: 12 }}
            tickFormatter={(value: number) => formatCompactCurrency(value)}
          />
          <Tooltip
            cursor={{ fill: 'color-mix(in srgb, var(--color-text) 5%, transparent)' }}
            content={({ active, payload }) => {
              const m = payload?.[0]?.payload as LoanMonth | undefined;
              if (!active || !m) return null;
              return (
                <div className={styles.tooltip}>
                  <div className={styles.tooltipTitle}>
                    {monthLabel(m.month)}
                    {m.assumed && <span className={styles.assumed}> · scheduled, not recorded</span>}
                  </div>
                  <div className={styles.tooltipRow}>
                    <span className={styles.swatch} style={{ background: PRINCIPAL_COLOR }} />
                    Principal <strong>{formatCurrency(m.principal)}</strong>
                  </div>
                  <div className={styles.tooltipRow}>
                    <span className={styles.swatch} style={{ background: INTEREST_COLOR }} />
                    Interest <strong>{formatCurrency(m.interest)}</strong>
                  </div>
                  <div className={styles.tooltipFooter}>Balance after: {formatCurrency(m.balance)}</div>
                </div>
              );
            }}
          />
          <Bar dataKey="interest" stackId="p" fill={INTEREST_COLOR} maxBarSize={24} isAnimationActive={false} />
          <Bar
            dataKey="principal"
            stackId="p"
            fill={PRINCIPAL_COLOR}
            radius={[4, 4, 0, 0]}
            maxBarSize={24}
            isAnimationActive={false}
          />
        </BarChart>
      </ResponsiveContainer>
      {assumed && (
        <p className={styles.note}>Months before your first recorded payment assume the scheduled payment was made.</p>
      )}
    </section>
  );
}
