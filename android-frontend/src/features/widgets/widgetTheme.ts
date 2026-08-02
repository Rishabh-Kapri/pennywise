/**
 * Widget palette, mirroring `src/theme.ts`.
 *
 * Duplicated rather than imported because the widget style props are typed as
 * `#${string}` / `rgba(...)` template literals, while `theme.ts` exposes its
 * values as plain `string`. Declaring them here with `as const` keeps the
 * literal types the widget components need. Keep the hexes in sync with
 * `src/theme.ts`.
 */
export const widgetColors = {
  background: '#0B0B0F',
  surface: '#15151A',
  surfaceStrong: '#1E1E25',
  text: '#F4F4F6',
  muted: '#9C9CA6',
  faint: '#5F5F6B',
  primary: '#8B93FF',
  primaryMuted: 'rgba(139, 147, 255, 0.14)',
  success: '#5BD99B',
  successMuted: 'rgba(91, 217, 155, 0.14)',
  warning: '#FFC663',
  warningMuted: 'rgba(255, 198, 99, 0.14)',
  danger: '#FF7A7A'
} as const;

/** Compact relative time ("12m", "3h", "2d") for the tight widget layout. */
export function shortRelativeTime(iso: string | undefined | null): string {
  if (!iso) return '—';
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return '—';

  const seconds = Math.max(0, Math.floor((Date.now() - then) / 1000));
  if (seconds < 60) return 'just now';
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}
