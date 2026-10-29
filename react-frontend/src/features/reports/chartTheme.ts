import { formatCompactCurrency } from '@/features/dashboard/utils';
import { monthLabel } from './cashFlow';

/**
 * Role colors, defined per theme in Reports.module.css. Every role keeps its
 * color on every tab (spending is always blue, investments always violet).
 * The money-in / money-out orders were validated as adjacent pairs.
 */
export const COLORS = {
  income: 'var(--rp-income)',
  spending: 'var(--rp-spending)',
  oneOff: 'var(--rp-one-off)',
  invested: 'var(--rp-invested)',
  debt: 'var(--rp-debt)',
  cash: 'var(--rp-cash)',
  other: 'var(--rp-other)',
} as const;

/** Fixed categorical order for per-group / per-source series; never cycled. */
export const SERIES = [1, 2, 3, 4, 5, 6, 7].map((n) => `var(--rp-s${n})`);

export const AXIS_TICK = { fill: 'var(--muted)', fontSize: 12 };
export const GRID_COLOR = 'var(--line)';
export const CURSOR_FILL = 'color-mix(in srgb, var(--color-text) 5%, transparent)';

export function axisCurrency(value: number) {
  return formatCompactCurrency(value);
}

export function formatMonthTick(monthKey: string) {
  return monthLabel(monthKey);
}
