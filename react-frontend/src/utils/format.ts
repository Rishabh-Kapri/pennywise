/**
 * How the app writes amounts, counts and sizes. Money is Indian rupees in the
 * en-IN grouping (1,23,456) everywhere; negative amounts use a true minus (−).
 */

const LOCALE = 'en-IN';

const rupees = (options: Intl.NumberFormatOptions = {}) =>
  new Intl.NumberFormat(LOCALE, { style: 'currency', currency: 'INR', ...options });

const wholeRupees = rupees({ minimumFractionDigits: 0, maximumFractionDigits: 0 });

/** ₹1,23,456 in whole rupees. The sign is dropped so callers control +/− display. */
export function formatCurrency(amount: number): string {
  return wholeRupees.format(Math.abs(amount));
}

/** −₹1,191 / +₹500 (with `plus`) / ₹0, in whole rupees. */
export function formatSignedCurrency(amount: number, plus = false): string {
  const rounded = Math.round(amount);
  if (rounded < 0) return `−${formatCurrency(rounded)}`;
  if (rounded > 0 && plus) return `+${formatCurrency(rounded)}`;
  return formatCurrency(rounded);
}

/** ₹1,23,456.50, with paise. */
export function formatAmount(amount: number): string {
  return rupees().format(amount);
}

/** Whole rupees unless there are paise; `|| 0` keeps -0 from rendering as "-₹0". */
export function formatBudgetAmount(amount: number): string {
  return rupees({ minimumFractionDigits: 0, maximumFractionDigits: 2 }).format(amount || 0);
}

/** Compact INR for axis ticks and dense labels, e.g. ₹1.2L, ₹45K. */
export function formatCompactCurrency(amount: number): string {
  const abs = Math.abs(amount);
  const sign = amount < 0 ? '-' : '';
  if (abs >= 10000000) return `${sign}₹${trimZero(abs / 10000000)}Cr`;
  if (abs >= 100000) return `${sign}₹${trimZero(abs / 100000)}L`;
  if (abs >= 1000) return `${sign}₹${trimZero(abs / 1000)}K`;
  return `${sign}₹${Math.round(abs)}`;
}

/**
 * Compact INR for a chart axis whose ticks are `step` apart: as many decimals
 * as the step needs (₹2.75L, ₹3L), so neighbouring ticks never read the same.
 */
export function formatTickCurrency(amount: number, step: number): string {
  const abs = Math.abs(amount);
  const sign = amount < 0 ? '−' : '';
  const [unit, suffix] = abs >= 1e7 ? [1e7, 'Cr'] : abs >= 1e5 ? [1e5, 'L'] : abs >= 1e3 ? [1e3, 'K'] : [1, ''];
  const ratio = step / unit;
  let decimals = 0;
  while (decimals < 2 && Math.abs(ratio * 10 ** decimals - Math.round(ratio * 10 ** decimals)) > 1e-6) decimals += 1;
  const digits = (abs / unit).toFixed(decimals);
  return `${sign}₹${decimals ? digits.replace(/\.?0+$/, '') : digits}${suffix}`;
}

function trimZero(value: number): string {
  return value.toFixed(1).replace(/\.0$/, '');
}

/** Whole rupees without the symbol, e.g. 1,59,006. */
export function formatDigits(amount: number): string {
  return new Intl.NumberFormat(LOCALE, { maximumFractionDigits: 0 }).format(Math.abs(Math.round(amount)));
}

/** Two decimals without the symbol, as statements print them: 1,59,006.50. */
export function formatDecimal(amount: number): string {
  return new Intl.NumberFormat(LOCALE, { minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(amount);
}

/** A count in the en-IN grouping: 1,00,000. */
export function formatCount(n: number): string {
  return new Intl.NumberFormat(LOCALE).format(n);
}

/** Card reward points with their sign: +1,250 pts / −40 pts. */
export function formatPoints(n: number): string {
  return `${n < 0 ? '−' : '+'}${formatCount(Math.abs(n))} pts`;
}

/** 512 B, 84 KB, 2.3 MB */
export function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

/**
 * A model's confidence as a percentage. Steps report it as 0–1 and some older
 * records as 0–100, so a value up to 1 is read as a share.
 */
export function confidencePercent(value: number): number {
  return Math.max(0, Math.min(value <= 1 ? value * 100 : value, 100));
}

/** 87%, 4.5%, or — when there's no confidence. */
export function formatConfidence(value?: number | null): string {
  if (value === null || value === undefined) return '—';
  const percent = confidencePercent(value);
  return `${percent.toFixed(percent >= 10 ? 0 : 1)}%`;
}

export type ConfidenceLevel = 'high' | 'medium' | 'low' | 'unknown';

/** How sure a prediction was, in the buckets chips and meters are coloured by. */
export function confidenceLevel(value?: number | null): ConfidenceLevel {
  if (value === null || value === undefined) return 'unknown';
  const percent = confidencePercent(value);
  if (percent >= 80) return 'high';
  if (percent >= 50) return 'medium';
  return 'low';
}
