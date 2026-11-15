export const TAG_COLORS = [
  '#6366f1', '#ec4899', '#f59e0b', '#10b981', '#3b82f6',
  '#8b5cf6', '#ef4444', '#14b8a6', '#f97316', '#06b6d4',
];

export function getAutoColor(name: string): string {
  let hash = 0;
  for (let i = 0; i < name.length; i++) {
    hash = name.charCodeAt(i) + ((hash << 5) - hash);
  }
  return TAG_COLORS[Math.abs(hash) % TAG_COLORS.length];
}

const FULL_MONTHS = [
  'january', 'february', 'march', 'april', 'may', 'june',
  'july', 'august', 'september', 'october', 'november', 'december',
];
const MONTH_LABELS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

export interface DatedTag {
  /** Place or label part, title-cased: "new-delhi-mar-2026" → "New Delhi". */
  place: string;
  /** 0-based month. */
  month: number;
  year: number;
  monthLabel: string;
}

function parseMonth(token: string): number {
  if (/^\d{1,2}$/.test(token)) {
    const n = Number(token);
    return n >= 1 && n <= 12 ? n - 1 : -1;
  }
  const t = token.toLowerCase();
  // "sep", "sept" and "september" all match; anything under three letters doesn't.
  if (t.length < 3) return -1;
  return FULL_MONTHS.findIndex((m) => m.startsWith(t));
}

/**
 * Reads tags named `<place>-<month>-<year>` (e.g. "goa-dec-2025", "manali_jan_26").
 * Returns null for names that don't follow it.
 */
export function parseDatedTag(name: string): DatedTag | null {
  const parts = name.trim().split(/[-_\s/]+/).filter(Boolean);
  if (parts.length < 3) return null;
  const yearToken = parts[parts.length - 1];
  if (!/^(\d{2}|\d{4})$/.test(yearToken)) return null;
  const month = parseMonth(parts[parts.length - 2]);
  if (month < 0) return null;
  const year = yearToken.length === 2 ? 2000 + Number(yearToken) : Number(yearToken);
  const place = parts
    .slice(0, -2)
    .map((p) => p.charAt(0).toUpperCase() + p.slice(1))
    .join(' ');
  return { place, month, year, monthLabel: MONTH_LABELS[month] };
}

/** What to show for a tag: "goa-dec-2025" → "Goa · Dec 2025"; other names unchanged. */
export function tagLabel(name: string): string {
  const dated = parseDatedTag(name);
  return dated?.place ? `${dated.place} · ${dated.monthLabel} ${dated.year}` : name;
}

/** Search matches either the stored name or the label shown for it. */
export function tagMatches(name: string, query: string): boolean {
  const q = query.trim().toLowerCase();
  if (!q) return true;
  return name.toLowerCase().includes(q) || tagLabel(name).toLowerCase().includes(q);
}
