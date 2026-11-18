const pad = (value: string) => value.padStart(2, '0');

/**
 * Returns the month key in format yyyy-mm
 * JS months start from 0, incrementing by 1 to match human language
 */
export function getCurrentMonthKey(): string {
  const date = new Date();
  const month = pad((date.getMonth() + 1).toString());
  return `${date.getFullYear()}-${month}`;
}

export function getPreviousMonthKey(monthKey: string): string {
  return addMonths(monthKey, -1);
}

/**
 * Returns the date formatted in locale format (e.g. en-US)
 */
export function getLocaleDate(
  dateStr: string,
  options: Intl.DateTimeFormatOptions = { month: 'short', day: 'numeric' },
  locales: string[] = ['en-US'],
): string {
  // A bare YYYY-MM or YYYY-MM-DD is a local day; new Date() would read it as UTC midnight.
  const date = /^\d{4}-\d{2}(-\d{2})?$/.test(dateStr)
    ? new Date(`${dateStr.length === 7 ? `${dateStr}-01` : dateStr}T00:00:00`)
    : new Date(dateStr);

  return date.toLocaleDateString(locales, options);
}

/**
 * Returns the month key in format yyyy-mm
 * JS months start from 0, incrementing by 1 to match human language
 */
export function getMonthKey(year: number, month: number): string {
  const monthStr = pad((month + 1).toString());
  return `${year}-${monthStr}`;
}

/**
 * Returns the today's date in format yyyy-mm-dd
 */
export function getTodaysDate(): string {
  const date = new Date();
  const month = pad((date.getMonth() + 1).toString());
  const day = pad(date.getDate().toString());
  return `${date.getFullYear()}-${month}-${day}`;
}

/**
 * Shifts a month key (yyyy-mm) by delta months, handling year rollover
 */
export function addMonths(monthKey: string, delta: number): string {
  const [year, month] = monthKey.split('-');
  const date = new Date(parseInt(year, 10), parseInt(month, 10) - 1 + delta);
  return getMonthKey(date.getFullYear(), date.getMonth());
}

export function getSelectedMonthInHumanFormat(key: string): string {
  const [year, month] = key.split('-');

  const yearInt = parseInt(year, 10);
  const monthInt = parseInt(month, 10);
  const date = new Date(yearInt, monthInt - 1, 1);

  return `${date.toLocaleString('en-us', { month: 'short' })}, ${year}`;
}
