export interface APIKey {
  id: string;
  name: string;
  description?: string;
  keyId?: string;
  maskedKey?: string;
  scopes?: string[];
  rateLimit?: number;
  isActive?: boolean;
  expiresAt?: string;
  lastUsedAt?: string;
  createdAt?: string;
}

export type PredictionSource = 'RULE' | 'VECTOR' | 'LLM' | 'MANUAL' | 'UNCATEGORIZED';
