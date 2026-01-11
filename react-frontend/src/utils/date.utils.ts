function pad(
  value: string,
  padLength: number,
  padValue: string,
  isStart: boolean = true,
) {
  if (isStart) {
    return value.padStart(padLength, padValue);
  } else {
    return value.padEnd(padLength, padValue);
  }
}

/**
 * Returns the month key in format yyyy-mm
 * JS months start from 0, incrementing by 1 to match human language
 */
export function getCurrentMonthKey(): string {
  const date = new Date();
  const month = pad((date.getMonth() + 1).toString(), 2, '0');
  return `${date.getFullYear()}-${month}`;
}

export function getPreviousMonthKey(monthKey: string): string {
  const [year, currentMonth] = monthKey.split('-');
  const date = new Date(parseInt(year, 10), parseInt(currentMonth, 10) - 2);
  const month = pad((date.getMonth() + 1).toString(), 2, '0');
  return `${date.getFullYear()}-${month}`;
}

/**
 * Returns the date formatted in locale format (e.g. en-US)
 */
export function getLocaleDate(
  dateStr: string,
  options: Intl.DateTimeFormatOptions = { month: 'short', day: 'numeric' },
  locales: string[] = ['en-US'],
): string {
  const date = new Date(dateStr);

  return date.toLocaleDateString(locales, options);
}

/**
 * Returns the month key in format yyyy-mm
 * JS months start from 0, incrementing by 1 to match human language
 */
export function getMonthKey(year: number, month: number): string {
  const monthStr = pad((month + 1).toString(), 2, '0');
  return `${year}-${monthStr}`;
}

/**
 * Returns the today's date in format yyyy-mm-dd
 */
export function getTodaysDate(): string {
  const date = new Date();
  const month = pad((date.getMonth() + 1).toString(), 2, '0');
  const day = pad(date.getDate().toString(), 2, '0');
  return `${date.getFullYear()}-${month}-${day}`;
}

export function getSelectedMonthInHumanFormat(key: string): string {
  const [year, month] = key.split('-');

  const yearInt = parseInt(year, 10);
  const monthInt = parseInt(month, 10);
  const date = new Date(yearInt, monthInt - 1, 1);

  return `${date.toLocaleString('en-us', { month: 'short' })}, ${year}`;
}

export function getCurrencyLocaleString(
  value: number,
  currency: string = 'INR',
  locale = 'en-IN',
) {
  return value.toLocaleString(locale, { style: 'currency', currency });
}
