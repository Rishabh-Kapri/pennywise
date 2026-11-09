import type { ReactNode } from 'react';
import { Amount } from '@/features/dashboard/components/Amount';
import { formatCurrency } from '@/features/dashboard/utils';
import { formatPercent, monthLabel, shareOfIncome, type Monthly } from '../cashFlow';
import styles from './ReportParts.module.css';

// ── Tooltip ──

interface TooltipRow {
  name: string;
  color?: string;
  value: string;
}

export function TooltipCard({ title, rows, footer }: { title: string; rows: TooltipRow[]; footer?: ReactNode }) {
  return (
    <div className={styles.tooltip}>
      <div className={styles.tooltipTitle}>{title}</div>
      {rows.map((row) => (
        <div key={row.name} className={styles.tooltipRow}>
          {row.color && <span className={styles.swatch} style={{ background: row.color }} />}
          <span className={styles.tooltipName}>{row.name}</span>
          <span className={styles.tooltipValue}>{row.value}</span>
        </div>
      ))}
      {footer && <div className={styles.tooltipFooter}>{footer}</div>}
    </div>
  );
}

// ── Stat tile ──

interface StatProps {
  label: string;
  value: number;
  signed?: boolean;
  detail?: ReactNode;
  color?: string;
}

export function Stat({ label, value, signed, detail, color }: StatProps) {
  return (
    <div className={styles.stat}>
      <span className={styles.statLabel}>
        {color && <span className={styles.swatch} style={{ background: color }} />}
        {label}
      </span>
      <Amount value={value} signed={signed} className={styles.statValue} />
      {detail && <span className={styles.statDetail}>{detail}</span>}
    </div>
  );
}

export function StatRow({ children }: { children: ReactNode }) {
  return <div className={styles.statRow}>{children}</div>;
}

// ── Flow bar: one stacked horizontal bar + its labeled legend ──

export interface FlowSegment {
  key: string;
  label: string;
  value: number;
  color: string;
  hint?: string;
}

interface FlowBarProps {
  title: string;
  segments: FlowSegment[];
  /** Bar length = scale; share figures are of `income`. */
  scale: number;
  income: number;
}

export function FlowBar({ title, segments, scale, income }: FlowBarProps) {
  const visible = segments.filter((segment) => segment.value > 0.5);
  const total = visible.reduce((sum, segment) => sum + segment.value, 0);

  return (
    <div className={styles.flow}>
      <div className={styles.flowHead}>
        <span className={styles.flowTitle}>{title}</span>
        <span className={styles.flowTotal}>{formatCurrency(total)}</span>
      </div>
      <div
        className={styles.flowTrack}
        role="img"
        aria-label={`${title}: ${visible.map((s) => `${s.label} ${formatCurrency(s.value)}`).join(', ')}`}
      >
        {visible.map((segment) => (
          <span
            key={segment.key}
            className={styles.flowSegment}
            style={{ width: `${(segment.value / scale) * 100}%`, background: segment.color }}
            title={`${segment.label}: ${formatCurrency(segment.value)}`}
          />
        ))}
      </div>
      <ul className={styles.flowLegend}>
        {visible.map((segment) => (
          <li key={segment.key} className={styles.flowItem}>
            <span className={styles.swatch} style={{ background: segment.color }} />
            <span className={styles.flowLabel}>
              {segment.label}
              {segment.hint && <span className={styles.flowHint}>{segment.hint}</span>}
            </span>
            <span className={styles.flowAmount}>{formatCurrency(segment.value)}</span>
            <span className={styles.flowShare}>{formatPercent(shareOfIncome(segment.value, income))}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}

// ── Month table: the table view of every trend chart ──

export interface MonthTableRow {
  key: string;
  label: ReactNode;
  monthly: Monthly;
  /** Shown instead of currency, e.g. a rate. */
  format?: (value: number, month: string | null) => string;
  total?: number | null;
  emphasis?: boolean;
  indent?: boolean;
}

export function MonthTable({ months, rows }: { months: string[]; rows: MonthTableRow[] }) {
  return (
    <div className={styles.tableScroll}>
      <table className={styles.table}>
        <thead>
          <tr>
            <th className={styles.nameCol} />
            {months.map((month) => (
              <th key={month}>{monthLabel(month)}</th>
            ))}
            <th>Total</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => {
            const format = row.format ?? ((value: number) => formatSignedRounded(value));
            const total =
              row.total === undefined ? months.reduce((sum, m) => sum + (row.monthly[m] ?? 0), 0) : row.total;
            return (
              <tr key={row.key} className={row.emphasis ? styles.rowEmphasis : undefined}>
                <td className={row.indent ? styles.nameColIndent : styles.nameCol}>{row.label}</td>
                {months.map((month) => (
                  <td key={month}>{format(row.monthly[month] ?? 0, month)}</td>
                ))}
                <td>{total === null ? '—' : format(total, null)}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

function formatSignedRounded(value: number) {
  const rounded = Math.round(value);
  if (rounded === 0) return '–';
  return `${rounded < 0 ? '−' : ''}${formatCurrency(rounded)}`;
}

// ── Misc ──

export function EmptyState({ children }: { children: ReactNode }) {
  return <div className={styles.empty}>{children}</div>;
}

export function Note({ icon, children }: { icon?: ReactNode; children: ReactNode }) {
  return (
    <div className={styles.note}>
      {icon && <span className={styles.noteIcon}>{icon}</span>}
      <div className={styles.noteBody}>{children}</div>
    </div>
  );
}

/** A horizontal bar showing value against the largest row. */
export function RowBar({ value, max, color }: { value: number; max: number; color: string }) {
  const width = max > 0 ? Math.max(0, Math.min(value / max, 1)) * 100 : 0;
  return (
    <span className={styles.rowBarTrack} aria-hidden="true">
      <span className={styles.rowBarFill} style={{ width: `${width}%`, background: color }} />
    </span>
  );
}
