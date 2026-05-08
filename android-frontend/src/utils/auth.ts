export type ParsedJWT = {
  exp?: number;
  sub?: string;
  [key: string]: unknown;
};

export function parseJWT(token: string): ParsedJWT | null {
  try {
    const [, payload] = token.split('.');
    if (!payload) return null;
    const normalized = payload.replace(/-/g, '+').replace(/_/g, '/');
    const padded = normalized.padEnd(normalized.length + ((4 - (normalized.length % 4)) % 4), '=');
    if (!globalThis.atob) return null;
    const decoded = globalThis.atob(padded);
    return JSON.parse(decoded) as ParsedJWT;
  } catch {
    return null;
  }
}
