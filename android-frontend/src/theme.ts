// Design tokens for the Pennywise Android app.
// Two palettes, picked once at startup from the system setting (see
// `scheme` below): a pale grey canvas with white cards for light, near-black
// with charcoal cards for dark. Screens build their StyleSheets at module
// scope, so a theme switch takes a JS reload (`useReloadOnSchemeChange`).

import { Appearance } from 'react-native';

export type Scheme = 'light' | 'dark';

/** The scheme this JS bundle was started with. */
export const scheme: Scheme = Appearance.getColorScheme() === 'light' ? 'light' : 'dark';

const dark = {
  background: '#0E0E0F',
  surface: '#1A1A1B',
  surfaceStrong: '#242426',
  surfaceTertiary: '#303033',
  text: '#F4F4F4',
  muted: '#A3A3A3',
  faint: '#77777A',
  textTertiary: '#7BB8E0',
  textQuaternary: '#333333',
  border: 'rgba(255, 255, 255, 0.08)',
  borderLight: 'rgba(255, 255, 255, 0.14)',
  borderMuted: 'rgba(255, 255, 255, 0.05)',
  // the web's blues (--brand-*), lightened for the dark canvas; dark text on it
  primary: '#6AAFDC',
  primaryMuted: 'rgba(106, 175, 220, 0.16)',
  primaryLight: '#ADD2EB',
  primaryDark: '#3176AA',
  onPrimary: '#0D1A26',
  secondary: '#F4F4F4',
  accent: '#6AAFDC',
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
  slate: '#303033',
  /** overlay behind controls floating on top of imagery (receipt thumbnails) */
  scrim: 'rgba(0, 0, 0, 0.55)',
  /** the dark textured hero card (balance); stays dark in both schemes */
  hero: '#1E1E20',
  heroBorder: 'rgba(255, 255, 255, 0.08)',
  onHero: '#FFFFFF',
  onHeroMuted: 'rgba(255, 255, 255, 0.6)',
  heroControl: 'rgba(255, 255, 255, 0.14)',
  /** money up / down on the dark hero card, in both schemes */
  heroPositive: '#5FD6A4',
  heroNegative: '#FF8A8A',
  /** solid chart bars (hatched); dark on light, light on dark */
  barStrong: '#D4D4D7',
  barHatch: 'rgba(0, 0, 0, 0.22)',
  /** floating tab bar: a charcoal pill that sits above the near-black canvas */
  tabBar: '#2A2A2D',
  tabBarBorder: 'rgba(255, 255, 255, 0.08)',
  tabBarActive: '#4A4A4E',
  tabBarActiveText: '#FFFFFF',
  tabBarIcon: '#8E8E93',
  shadow: '#000000'
};

const light: typeof dark = {
  background: '#EBEBEB',
  surface: '#FFFFFF',
  surfaceStrong: '#F4F4F4',
  surfaceTertiary: '#E3E3E3',
  text: '#141414',
  muted: '#6B6B6B',
  faint: '#9A9A9A',
  textTertiary: '#25699D',
  textQuaternary: '#DADADA',
  border: 'rgba(0, 0, 0, 0.08)',
  borderLight: 'rgba(0, 0, 0, 0.12)',
  borderMuted: 'rgba(0, 0, 0, 0.05)',
  // the web's --brand-deep: AA for text on white
  primary: '#25699D',
  primaryMuted: 'rgba(37, 105, 157, 0.12)',
  primaryLight: '#3176AA',
  primaryDark: '#1E5680',
  onPrimary: '#FFFFFF',
  secondary: '#141414',
  accent: '#25699D',
  danger: '#D64545',
  dangerMuted: 'rgba(214, 69, 69, 0.1)',
  success: '#17905F',
  successMuted: 'rgba(23, 144, 95, 0.1)',
  warning: '#B26B00',
  warningMuted: 'rgba(242, 169, 59, 0.16)',
  info: '#2563EB',
  budgetPositive: '#17905F',
  budgetNegative: '#D64545',
  budgetPositiveBg: 'rgba(23, 144, 95, 0.12)',
  budgetPositiveFg: '#137A51',
  budgetNegativeBg: 'rgba(214, 69, 69, 0.12)',
  budgetNegativeFg: '#C03A3A',
  budgetUpcomingBg: 'rgba(242, 169, 59, 0.18)',
  budgetUpcomingFg: '#8A5A00',
  slate: '#E3E3E3',
  scrim: 'rgba(0, 0, 0, 0.45)',
  hero: '#161616',
  heroBorder: 'rgba(0, 0, 0, 0)',
  onHero: '#FFFFFF',
  onHeroMuted: 'rgba(255, 255, 255, 0.6)',
  heroControl: 'rgba(255, 255, 255, 0.14)',
  /** money up / down on the dark hero card, in both schemes */
  heroPositive: '#5FD6A4',
  heroNegative: '#FF8A8A',
  barStrong: '#1E1E1E',
  barHatch: 'rgba(255, 255, 255, 0.22)',
  tabBar: '#161616',
  tabBarBorder: 'rgba(255, 255, 255, 0.06)',
  tabBarActive: '#3A3A3A',
  tabBarActiveText: '#FFFFFF',
  tabBarIcon: '#7C7C7C',
  shadow: '#000000'
};

export const colors = scheme === 'light' ? light : dark;

/**
 * Report roles, the web's `--rp-*` steps for the matching theme. A role keeps
 * its color on every chart (spending is always blue, investments always violet).
 */
export const chartColors =
  scheme === 'light'
    ? {
        income: '#1BAF7A',
        spending: '#2A78D6',
        oneOff: '#EB6834',
        invested: '#6250D6',
        debt: '#E87BA4',
        cash: '#9AA1AB',
        other: '#9AA1AB',
        /** loan payment split: principal / interest */
        principal: '#2A78D6',
        interest: '#EB6834',
        grid: 'rgba(0, 0, 0, 0.07)',
        track: 'rgba(0, 0, 0, 0.04)'
      }
    : {
        income: '#199E70',
        spending: '#3987E5',
        oneOff: '#D95926',
        invested: '#9085E9',
        debt: '#D55181',
        cash: '#7A7A7A',
        other: '#7A7A7A',
        principal: '#3987E5',
        interest: '#D95926',
        grid: 'rgba(255, 255, 255, 0.08)',
        track: 'rgba(255, 255, 255, 0.05)'
      };

/** Fixed categorical order for per-group / per-source series; never cycled. */
export const chartSeries =
  scheme === 'light'
    ? ['#2A78D6', '#EB6834', '#1BAF7A', '#EDA100', '#E87BA4', '#008300', '#6250D6']
    : ['#3987E5', '#D95926', '#199E70', '#C98500', '#D55181', '#008300', '#9085E9'];

/** Horizontal padding of every screen's content. */
export const screenPadding = 16;

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
