// Design tokens for the Pennywise Android app.
// Mirrors react-frontend's dark theme (src/styles/theme.css): warm charcoal
// canvas, borderless raised surfaces, the steel-blue accent, and tonal
// (translucent) fills for states.

export const colors = {
  background: '#1F1F1E',
  surface: '#2A2A2A',
  surfaceStrong: '#343434',
  surfaceTertiary: '#424242',
  text: '#F6F6F6',
  muted: '#A9A9A9',
  faint: '#7A7A7A',
  textTertiary: '#5D9CBB',
  textQuaternary: '#333333',
  border: 'rgba(255, 255, 255, 0.08)',
  borderLight: 'rgba(255, 255, 255, 0.14)',
  borderMuted: 'rgba(255, 255, 255, 0.05)',
  primary: '#5D9CBB',
  primaryMuted: 'rgba(93, 156, 187, 0.16)',
  primaryLight: '#AEC2DD',
  primaryDark: '#4483A2',
  onPrimary: '#0E1B22',
  secondary: '#F6F6F6',
  accent: '#5D9CBB',
  danger: '#EF6B6B',
  dangerMuted: 'rgba(239, 107, 107, 0.15)',
  success: '#34C38F',
  successMuted: 'rgba(52, 195, 143, 0.15)',
  warning: '#F2A93B',
  warningMuted: 'rgba(242, 169, 59, 0.15)',
  info: '#3B82F6',
  budgetPositive: '#73EB95',
  budgetNegative: '#FF6363',
  budgetPositiveBg: 'rgba(115, 235, 149, 0.15)',
  budgetPositiveFg: '#73EB95',
  budgetNegativeBg: 'rgba(255, 99, 99, 0.16)',
  budgetNegativeFg: '#FF7B7B',
  budgetUpcomingBg: 'rgba(249, 225, 169, 0.15)',
  budgetUpcomingFg: '#F9E1A9',
  slate: '#424242',
  /** overlay behind controls floating on top of imagery (receipt thumbnails) */
  scrim: 'rgba(0, 0, 0, 0.55)'
};

/**
 * Report roles, the web's dark-theme `--rp-*` steps. A role keeps its color on
 * every chart (spending is always blue, investments always violet).
 */
export const chartColors = {
  income: '#199E70',
  spending: '#3987E5',
  oneOff: '#D95926',
  invested: '#9085E9',
  debt: '#D55181',
  cash: '#7A7A7A',
  other: '#7A7A7A',
  /** loan payment split: principal / interest */
  principal: '#3987E5',
  interest: '#D95926',
  grid: 'rgba(255, 255, 255, 0.08)'
};

/** Fixed categorical order for per-group / per-source series; never cycled. */
export const chartSeries = ['#3987E5', '#D95926', '#199E70', '#C98500', '#D55181', '#008300', '#9085E9'];

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
