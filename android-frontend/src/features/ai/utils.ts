import type { CipherPrediction } from '../transactions/types';
import type { PredictionSource } from './types';

export function formatConfidence(value?: number | null): string {
  if (value === null || value === undefined) return '—';
  const percent = value <= 1 ? value * 100 : value;
  return `${percent.toFixed(percent >= 10 ? 0 : 1)}%`;
}

export function formatDate(dateStr?: string): string {
  if (!dateStr) return '—';
  const date = new Date(dateStr);
  if (Number.isNaN(date.getTime())) return '—';
  return date.toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric'
  });
}

export function computeSourceCounts(
  predictions: CipherPrediction[]
): Record<PredictionSource, number> {
  const counts: Record<string, number> = {
    RULE: 0,
    VECTOR: 0,
    LLM: 0,
    MANUAL: 0,
    UNCATEGORIZED: 0
  };
  for (const prediction of predictions) {
    const source = (prediction.source ?? 'UNCATEGORIZED').toUpperCase();
    if (source in counts) {
      counts[source]++;
    } else {
      counts.UNCATEGORIZED++;
    }
  }
  return counts as Record<PredictionSource, number>;
}

/** Mean of every payee/category confidence recorded on the given predictions. */
function meanConfidence(predictions: CipherPrediction[]): number | null {
  const values: number[] = [];
  for (const prediction of predictions) {
    if (prediction.payeeConfidence != null) values.push(prediction.payeeConfidence);
    if (prediction.categoryConfidence != null) values.push(prediction.categoryConfidence);
  }
  if (values.length === 0) return null;
  return values.reduce((sum, value) => sum + value, 0) / values.length;
}

export function computeAvgConfidence(predictions: CipherPrediction[]): number | null {
  return meanConfidence(predictions);
}

export function computeAvgConfidenceBySource(
  predictions: CipherPrediction[],
  source: string
): number | null {
  return meanConfidence(predictions.filter((prediction) => prediction.source === source));
}

const THIRTY_DAYS_MS = 30 * 24 * 60 * 60 * 1000;

export function countLast30Days(predictions: CipherPrediction[]): number {
  return predictions.filter((prediction) => {
    if (!prediction.createdAt) return false;
    return Date.now() - new Date(prediction.createdAt).getTime() <= THIRTY_DAYS_MS;
  }).length;
}
