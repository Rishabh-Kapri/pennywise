// Design tokens for the Pennywise Android app.
// Minimal dark palette: near-black canvas, borderless raised surfaces,
// a single soft-indigo accent, and tonal (translucent) fills for states.

export const colors = {
  background: '#0B0B0F',
  surface: '#15151A',
  surfaceStrong: '#1E1E25',
  surfaceTertiary: '#2A2A33',
  text: '#F4F4F6',
  muted: '#9C9CA6',
  faint: '#5F5F6B',
  textTertiary: '#8B93FF',
  textQuaternary: '#2A2A33',
  border: 'rgba(255, 255, 255, 0.08)',
  borderLight: 'rgba(255, 255, 255, 0.14)',
  borderMuted: 'rgba(255, 255, 255, 0.05)',
  primary: '#8B93FF',
  primaryMuted: 'rgba(139, 147, 255, 0.14)',
  primaryLight: 'rgba(139, 147, 255, 0.14)',
  primaryDark: '#6E77E8',
  onPrimary: '#0B0B0F',
  secondary: '#F4F4F6',
  accent: '#8B93FF',
  danger: '#FF7A7A',
  dangerMuted: 'rgba(255, 122, 122, 0.14)',
  success: '#5BD99B',
  successMuted: 'rgba(91, 217, 155, 0.14)',
  warning: '#FFC663',
  warningMuted: 'rgba(255, 198, 99, 0.14)',
  info: '#6FB7FF',
  budgetPositive: '#5BD99B',
  budgetNegative: '#FF7A7A',
  budgetPositiveBg: 'rgba(91, 217, 155, 0.14)',
  budgetPositiveFg: '#5BD99B',
  budgetNegativeBg: 'rgba(255, 122, 122, 0.14)',
  budgetNegativeFg: '#FF7A7A',
  budgetUpcomingBg: 'rgba(255, 198, 99, 0.14)',
  budgetUpcomingFg: '#FFC663',
  slate: '#2A2A33',
  /** overlay behind controls floating on top of imagery (receipt thumbnails) */
  scrim: 'rgba(0, 0, 0, 0.55)'
};

export const spacing = {
  xs: 4,
  sm: 8,
  md: 12,
  lg: 16,
  xl: 24,
  xxl: 32
};

export const radii = {
  sm: 10,
  md: 14,
  lg: 20,
  xl: 28,
  full: 999
};

// Clearance so scrollable content is never hidden behind the floating tab bar.
export const tabBarClearance = 118;

export const typography = {
  display: {
    fontSize: 36,
    lineHeight: 42,
    fontWeight: '700' as const,
    letterSpacing: -1
  },
  title: {
    fontSize: 24,
    lineHeight: 30,
    fontWeight: '700' as const,
    letterSpacing: -0.5
  },
  heading: {
    fontSize: 17,
    lineHeight: 22,
    fontWeight: '600' as const,
    letterSpacing: -0.2
  },
  body: {
    fontSize: 15,
    lineHeight: 21,
    letterSpacing: 0
  },
  caption: {
    fontSize: 13,
    lineHeight: 18,
    letterSpacing: 0
  },
  label: {
    fontSize: 11,
    lineHeight: 14,
    fontWeight: '600' as const,
    letterSpacing: 1.1,
    textTransform: 'uppercase' as const
  }
};
