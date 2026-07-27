/**
 * Format an amount as whole-rupee INR, e.g. ₹1,23,456.
 * Pass the raw value; the sign is dropped so callers control +/− display.
 */
export function formatCurrency(amount: number): string {
  return new Intl.NumberFormat('en-IN', {
    style: 'currency',
    currency: 'INR',
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(Math.abs(amount));
}

/**
 * Compact INR for axis ticks and dense labels, e.g. ₹1.2L, ₹45K.
 */
export function formatCompactCurrency(amount: number): string {
  const abs = Math.abs(amount);
  const sign = amount < 0 ? '-' : '';
  if (abs >= 10000000) return `${sign}₹${trimZero(abs / 10000000)}Cr`;
  if (abs >= 100000) return `${sign}₹${trimZero(abs / 100000)}L`;
  if (abs >= 1000) return `${sign}₹${trimZero(abs / 1000)}K`;
  return `${sign}₹${Math.round(abs)}`;
}

function trimZero(value: number): string {
  return value.toFixed(1).replace(/\.0$/, '');
}
