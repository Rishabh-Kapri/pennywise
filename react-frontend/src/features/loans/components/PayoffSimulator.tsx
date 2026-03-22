import { useState, useMemo } from 'react';
import { X } from 'lucide-react';
import {
  calculateAmortizationSchedule,
  compareScenarios,
  formatPayoffDuration,
} from '../utils/payoffCalculator';
import type { PayoffSimulatorInput } from '../types/loan.types';
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts';
import styles from './PayoffSimulator.module.css';

interface PayoffSimulatorProps {
  isOpen: boolean;
  onClose: () => void;
  currentBalance: number;
  interestRate: number;
  minimumPayment: number;
}

function formatCurrency(value: number): string {
  return new Intl.NumberFormat('en-IN', {
    style: 'currency',
    currency: 'INR',
    maximumFractionDigits: 0,
  }).format(value);
}

export default function PayoffSimulator({
  isOpen,
  onClose,
  currentBalance,
  interestRate,
  minimumPayment,
}: PayoffSimulatorProps) {
  const [monthlyPayment, setMonthlyPayment] = useState(minimumPayment.toString());
  const [extraOneTime, setExtraOneTime] = useState('');
  const [extraOneTimeMonth, setExtraOneTimeMonth] = useState('1');

  const paymentAmount = parseFloat(monthlyPayment) || minimumPayment;
  const oneTimeAmount = parseFloat(extraOneTime) || 0;
  const oneTimeMonth = parseInt(extraOneTimeMonth) || 1;

  const baseInput: PayoffSimulatorInput = useMemo(
    () => ({
      currentBalance,
      interestRate,
      monthlyPayment: minimumPayment,
    }),
    [currentBalance, interestRate, minimumPayment],
  );

  const targetInput: PayoffSimulatorInput = useMemo(
    () => ({
      currentBalance,
      interestRate,
      monthlyPayment: paymentAmount,
      extraMonthlyPayment: 0,
      oneTimeExtraPayment: oneTimeAmount,
      oneTimeExtraPaymentMonth: oneTimeMonth,
    }),
    [currentBalance, interestRate, paymentAmount, oneTimeAmount, oneTimeMonth],
  );

  const comparison = useMemo(
    () => compareScenarios(baseInput, targetInput),
    [baseInput, targetInput],
  );

  const baseSchedule = useMemo(
    () => calculateAmortizationSchedule(baseInput),
    [baseInput],
  );

  const targetSchedule = useMemo(
    () => calculateAmortizationSchedule(targetInput),
    [targetInput],
  );

  // Build chart data — merge both schedules by month
  const chartData = useMemo(() => {
    const maxMonths = Math.max(baseSchedule.length, targetSchedule.length);
    const data = [];
    // Sample every N months for readability
    const step = maxMonths > 60 ? Math.ceil(maxMonths / 60) : 1;
    for (let i = 0; i < maxMonths; i += step) {
      const basePoint = baseSchedule[i];
      const targetPoint = targetSchedule[i];
      data.push({
        month: i + 1,
        label: basePoint?.date ?? targetPoint?.date ?? '',
        baseline: basePoint?.remainingBalance ?? 0,
        target: targetPoint?.remainingBalance ?? 0,
      });
    }
    // Always include the last point
    const lastBase = baseSchedule[baseSchedule.length - 1];
    const lastTarget = targetSchedule[targetSchedule.length - 1];
    if (data.length > 0 && data[data.length - 1]?.month !== maxMonths) {
      data.push({
        month: maxMonths,
        label: lastBase?.date ?? lastTarget?.date ?? '',
        baseline: lastBase?.remainingBalance ?? 0,
        target: lastTarget?.remainingBalance ?? 0,
      });
    }
    return data;
  }, [baseSchedule, targetSchedule]);

  if (!isOpen) return null;

  return (
    <div className={styles.overlay} onClick={onClose}>
      <div className={styles.modal} onClick={(e) => e.stopPropagation()}>
        <div className={styles.header}>
          <h2>Loan Payoff Simulator</h2>
          <button className={styles.closeBtn} onClick={onClose}>
            <X size={20} />
          </button>
        </div>

        {/* Inputs */}
        <div className={styles.inputs}>
          <div className={styles.staticField}>
            <label>Minimum Payment</label>
            <div className={styles.staticValue}>{formatCurrency(minimumPayment)}</div>
          </div>

          <div className={styles.field}>
            <label htmlFor="sim-payment">Monthly Payment (₹)</label>
            <input
              id="sim-payment"
              type="number"
              min={minimumPayment}
              step="100"
              value={monthlyPayment}
              onChange={(e) => setMonthlyPayment(e.target.value)}
            />
          </div>

          <div className={styles.field}>
            <label htmlFor="sim-extra">One-time Extra Payment (₹)</label>
            <input
              id="sim-extra"
              type="number"
              min="0"
              step="100"
              placeholder="0"
              value={extraOneTime}
              onChange={(e) => setExtraOneTime(e.target.value)}
            />
          </div>

          <div className={styles.field}>
            <label htmlFor="sim-extra-month">Apply in Month #</label>
            <input
              id="sim-extra-month"
              type="number"
              min="1"
              value={extraOneTimeMonth}
              onChange={(e) => setExtraOneTimeMonth(e.target.value)}
            />
          </div>
        </div>

        {/* Savings Summary */}
        <div className={styles.savingsGrid}>
          <div className={styles.savingsCard}>
            <span className={styles.savingsLabel}>Interest Savings</span>
            <span
              className={
                comparison.interestSaved > 0 ? styles.savingsValue : styles.savingsNeutral
              }
            >
              {comparison.interestSaved > 0
                ? formatCurrency(comparison.interestSaved)
                : '₹0'}
            </span>
          </div>
          <div className={styles.savingsCard}>
            <span className={styles.savingsLabel}>Time Savings</span>
            <span
              className={
                comparison.monthsSaved > 0 ? styles.savingsValue : styles.savingsNeutral
              }
            >
              {comparison.monthsSaved > 0
                ? formatPayoffDuration(comparison.monthsSaved)
                : 'None'}
            </span>
          </div>
        </div>

        {/* Breakdown */}
        <div className={styles.breakdown}>
          <div className={styles.breakdownItem}>
            <span className={styles.breakdownLabel}>Payoff in</span>
            <span className={styles.breakdownValue}>
              {formatPayoffDuration(comparison.targetPayoffMonths)}
            </span>
          </div>
          <div className={styles.breakdownItem}>
            <span className={styles.breakdownLabel}>Total Interest</span>
            <span className={styles.breakdownValue}>
              {formatCurrency(comparison.targetTotalInterest)}
            </span>
          </div>
          <div className={styles.breakdownItem}>
            <span className={styles.breakdownLabel}>Total Cost</span>
            <span className={styles.breakdownValue}>
              {formatCurrency(currentBalance + comparison.targetTotalInterest)}
            </span>
          </div>
          <div className={styles.breakdownItem}>
            <span className={styles.breakdownLabel}>vs. Minimum Only</span>
            <span className={styles.breakdownValue}>
              {formatPayoffDuration(comparison.basePayoffMonths)}
            </span>
          </div>
        </div>

        {/* Burndown Chart */}
        <div className={styles.chartContainer}>
          <div className={styles.chartTitle}>Balance Over Time</div>
          <ResponsiveContainer width="100%" height={280}>
            <LineChart data={chartData}>
              <CartesianGrid strokeDasharray="3 3" stroke="var(--color-surface-secondary)" />
              <XAxis
                dataKey="month"
                tick={{ fill: 'var(--color-text-secondary)', fontSize: 12 }}
                label={{
                  value: 'Months',
                  position: 'insideBottom',
                  offset: -5,
                  fill: 'var(--color-text-secondary)',
                }}
              />
              <YAxis
                tick={{ fill: 'var(--color-text-secondary)', fontSize: 12 }}
                tickFormatter={(v: number) =>
                  v >= 100000 ? `${(v / 100000).toFixed(1)}L` : `${(v / 1000).toFixed(0)}K`
                }
              />
              <Tooltip
                contentStyle={{
                  background: 'var(--color-surface)',
                  border: '1px solid var(--color-border)',
                  borderRadius: '0.5rem',
                  color: 'var(--color-text)',
                }}
                formatter={(value) => formatCurrency(value as number)}
                labelFormatter={(label) => `Month ${label}`}
              />
              <Line
                type="monotone"
                dataKey="baseline"
                name="Minimum Only"
                stroke="var(--color-text-secondary)"
                strokeWidth={2}
                strokeDasharray="6 4"
                dot={false}
              />
              <Line
                type="monotone"
                dataKey="target"
                name="Your Plan"
                stroke="var(--color-primary)"
                strokeWidth={2.5}
                dot={false}
              />
            </LineChart>
          </ResponsiveContainer>
          <div className={styles.chartLegend}>
            <div className={styles.legendItem}>
              <span className={styles.legendDashed} />
              Minimum Only
            </div>
            <div className={styles.legendItem}>
              <span className={styles.legendSolid} />
              Your Plan
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
