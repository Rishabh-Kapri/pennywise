import type { Payee, PayeeRule, PayeeStats } from './types/payee.types';

// ── Rule matching ─────────────────────────────────────────────────────────────
// Mirrors payeeRuleRepo.FindByMatchString: an EXACT rule wins when it equals
// the bank string (case-sensitive), otherwise a PATTERN rule wins when the
// string is ILIKE it. Cipher then skips a winner without a category.

const likeCache = new Map<string, RegExp>();

/** SQL LIKE (as ILIKE) to an anchored, case-insensitive RegExp. `\` escapes. */
export function likeToRegExp(pattern: string): RegExp {
  const cached = likeCache.get(pattern);
  if (cached) return cached;
  let source = '';
  for (let i = 0; i < pattern.length; i++) {
    const char = pattern[i];
    if (char === '\\' && i + 1 < pattern.length) {
      source += pattern[++i].replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    } else if (char === '%') {
      source += '[\\s\\S]*';
    } else if (char === '_') {
      source += '[\\s\\S]';
    } else {
      source += char.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    }
  }
  const regex = new RegExp(`^${source}$`, 'i');
  likeCache.set(pattern, regex);
  return regex;
}

export function ruleMatches(rule: PayeeRule, text: string): boolean {
  if (!text) return false;
  return rule.matchType === 'PATTERN' ? likeToRegExp(rule.matchString).test(text) : rule.matchString === text;
}

export interface RuleTestResult {
  /** the rule the pipeline would pick */
  winner: PayeeRule | null;
  /** every rule that matches, winner first */
  matches: PayeeRule[];
}

export function testRules(rules: PayeeRule[], text: string): RuleTestResult {
  const matches = rules
    .filter((rule) => ruleMatches(rule, text))
    .sort((a, b) => (a.matchType === b.matchType ? 0 : a.matchType === 'EXACT' ? -1 : 1));
  return { winner: matches[0] ?? null, matches };
}

/** A pattern with no wildcard is just a case-insensitive exact match. */
export function hasWildcard(pattern: string): boolean {
  return /(^|[^\\])[%_]/.test(pattern);
}

// ── Duplicate detection ───────────────────────────────────────────────────────

const NOISE_WORDS = new Set([
  'pvt', 'private', 'ltd', 'limited', 'llp', 'inc', 'co', 'corp', 'company', 'india', 'in', 'com', 'www',
  'the', 'upi', 'pos', 'payment', 'payments', 'online', 'technologies', 'technology', 'tech', 'services',
  'retail', 'store', 'stores', 'and',
]);

/**
 * Collapses the ways one merchant shows up across bank strings and AI
 * extraction ("SWIGGY*BLR 1234", "Swiggy", "swiggy.in") to one key.
 */
export function normalizePayeeName(name: string): string {
  // card/aggregator strings join merchant and city or gateway with "*"
  // ("SWIGGY*BLR", "RAZ*SWIGGY"); the longest part is the merchant
  const base = name.includes('*')
    ? name.split('*').reduce((longest, part) => (part.trim().length > longest.length ? part.trim() : longest), '')
    : name;
  return base
    .toLowerCase()
    .replace(/&/g, ' and ')
    .split(/[^a-z0-9]+/)
    .filter((word) => word && !NOISE_WORDS.has(word) && !/^\d+$/.test(word))
    .join('');
}

export interface DuplicateGroup {
  key: string;
  payees: Payee[];
  /** the payee to keep: the most used one */
  suggestedTargetId: string;
}

export function findDuplicateGroups(payees: Payee[], statsById: Record<string, PayeeStats>): DuplicateGroup[] {
  const groups = new Map<string, Payee[]>();
  for (const payee of payees) {
    if (!payee.id || payee.transferAccountId) continue;
    const key = normalizePayeeName(payee.name);
    if (key.length < 3) continue;
    const group = groups.get(key);
    if (group) group.push(payee);
    else groups.set(key, [payee]);
  }

  return [...groups.entries()]
    .filter(([, group]) => group.length > 1)
    .map(([key, group]) => {
      const ranked = [...group].sort(
        (a, b) =>
          (statsById[b.id!]?.transactionCount ?? 0) - (statsById[a.id!]?.transactionCount ?? 0) ||
          a.name.length - b.name.length,
      );
      return { key, payees: ranked, suggestedTargetId: ranked[0].id! };
    })
    .sort((a, b) => b.payees.length - a.payees.length || a.key.localeCompare(b.key));
}

// ── Display ───────────────────────────────────────────────────────────────────

export function payeeInitials(name: string): string {
  const words = name.replace(/[^\p{L}\p{N}\s]/gu, ' ').trim().split(/\s+/).filter(Boolean);
  if (words.length === 0) return '?';
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase();
  return (words[0][0] + words[1][0]).toUpperCase();
}

/** Stable hue per name, so a payee keeps its colour across sessions. */
export function payeeHue(name: string): number {
  let hash = 0;
  for (let i = 0; i < name.length; i++) hash = (hash * 31 + name.charCodeAt(i)) | 0;
  return Math.abs(hash) % 360;
}

const DAY_MS = 24 * 60 * 60 * 1000;

/** "today", "3 days ago", "5 weeks ago", "Mar 2025". */
export function formatLastUsed(date?: string): string | null {
  if (!date) return null;
  const then = new Date(`${date.slice(0, 10)}T00:00:00`);
  if (Number.isNaN(then.getTime())) return null;
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const days = Math.round((today.getTime() - then.getTime()) / DAY_MS);
  if (days <= 0) return days === 0 ? 'today' : then.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
  if (days === 1) return 'yesterday';
  if (days < 14) return `${days} days ago`;
  if (days < 60) return `${Math.round(days / 7)} weeks ago`;
  return then.toLocaleDateString('en-US', { month: 'short', year: 'numeric' });
}
