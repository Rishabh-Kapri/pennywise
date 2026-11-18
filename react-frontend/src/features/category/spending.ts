import type { Category } from './types/category.types';

/** What a category spent in a month: its outflow, never below zero. */
export function spentIn(category: Category, month: string): number {
  return Math.max(-(category.activity?.[month] ?? 0), 0);
}

/** A category that has spent this share of its assigned amount is almost spent. */
export const ALMOST_SPENT_SHARE = 0.8;
