export function getCurrentMonthKey(date = new Date()): string {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}`;
}

export function shiftMonth(month: string, delta: number): string {
  const [year, monthNumber] = month.split('-').map(Number);
  const date = new Date(year, (monthNumber || 1) - 1 + delta, 1);
  return getCurrentMonthKey(date);
}

export function getSelectedMonthInHumanFormat(month: string): string {
  if (!month) return '';
  const [year, monthNumber] = month.split('-').map(Number);
  return new Intl.DateTimeFormat('en-IN', {
    month: 'long',
    year: 'numeric'
  }).format(new Date(year, (monthNumber || 1) - 1, 1));
}

export function formatCurrency(amount: number, options?: { signed?: boolean }): string {
  const value = Number.isFinite(amount) ? amount : 0;
  const formatted = new Intl.NumberFormat('en-IN', {
    style: 'currency',
    currency: 'INR',
    minimumFractionDigits: 0,
    maximumFractionDigits: 0
  }).format(Math.abs(value));

  if (!options?.signed) {
    return value < 0 ? `-${formatted}` : formatted;
  }
  if (value > 0) return `+${formatted}`;
  if (value < 0) return `-${formatted}`;
  return formatted;
}

export function formatShortDate(dateString: string): string {
  const date = new Date(dateString);
  if (Number.isNaN(date.getTime())) return dateString;
  return new Intl.DateTimeFormat('en-IN', {
    day: '2-digit',
    month: 'short',
    year: 'numeric'
  }).format(date);
}

/** Today as a local YYYY-MM-DD (not UTC: an early-morning entry is still today). */
export function todayKey(date = new Date()): string {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
}

/** Shifts a YYYY-MM-DD date by whole days. Runs in UTC so DST never skips a day. */
export function addDays(date: string, days: number): string {
  const d = new Date(`${date}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() + days);
  return d.toISOString().slice(0, 10);
}

function parseDay(date: string): Date {
  return new Date(`${date.slice(0, 10)}T00:00:00`);
}

/** "Today", "Yesterday", else "Mon, 19 Oct" (with the year when it isn't this one). */
export function formatRelativeDay(date: string, today = todayKey()): string {
  if (date === today) return 'Today';
  if (date === addDays(today, -1)) return 'Yesterday';
  const parsed = parseDay(date);
  const sameYear = parsed.getFullYear() === new Date().getFullYear();
  return parsed.toLocaleDateString('en-IN', {
    weekday: 'short',
    day: 'numeric',
    month: 'short',
    ...(sameYear ? {} : { year: 'numeric' })
  });
}

/** "14 Oct" */
export function formatDayMonth(date: string): string {
  return parseDay(date).toLocaleDateString('en-IN', { day: 'numeric', month: 'short' });
}

/** "Oct ’25" for a YYYY-MM key; just "Oct" without the year. */
export function formatMonthShort(month: string, withYear = true): string {
  const [year, monthNumber] = month.split('-').map(Number);
  const name = new Date(year, monthNumber - 1, 1).toLocaleString('en-US', { month: 'short' });
  return withYear ? `${name} ’${String(year).slice(2)}` : name;
}

/** "October", or "October 2025" when it isn't this year. */
export function formatMonthName(month: string, style: 'long' | 'short' = 'long'): string {
  const [year, monthNumber] = month.split('-').map(Number);
  const date = new Date(year, monthNumber - 1, 1);
  const sameYear = year === new Date().getFullYear();
  return date.toLocaleDateString('en-IN', sameYear ? { month: style } : { month: style, year: 'numeric' });
}

/** Compact INR for chart axes and dense labels: ₹1.2L, ₹45K, ₹2Cr. */
export function formatCompactCurrency(amount: number): string {
  const abs = Math.abs(amount);
  const sign = amount < 0 ? '−' : '';
  const trim = (value: number) => value.toFixed(1).replace(/\.0$/, '');
  if (abs >= 1e7) return `${sign}₹${trim(abs / 1e7)}Cr`;
  if (abs >= 1e5) return `${sign}₹${trim(abs / 1e5)}L`;
  if (abs >= 1e3) return `${sign}₹${trim(abs / 1e3)}K`;
  return `${sign}₹${Math.round(abs)}`;
}

/** −₹1,191 / +₹500 (with `plus`) / ₹0, rounded to the rupee, with a true minus sign. */
export function formatSignedCurrency(amount: number, plus = false): string {
  const rounded = Math.round(amount);
  const body = formatCurrency(Math.abs(rounded));
  if (rounded < 0) return `−${body}`;
  if (rounded > 0 && plus) return `+${body}`;
  return body;
}
