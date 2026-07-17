import type { PropsWithChildren } from 'react';
import { StyleSheet, Text, type TextProps } from 'react-native';
import { colors, typography } from '../theme';

type Variant = 'display' | 'title' | 'heading' | 'body' | 'caption' | 'label';
type Tone = 'default' | 'muted' | 'faint' | 'primary' | 'success' | 'danger' | 'onPrimary';

type Props = PropsWithChildren<
  TextProps & {
    variant?: Variant;
    tone?: Tone;
    muted?: boolean;
    tabular?: boolean;
    weight?: 'regular' | 'medium' | 'semibold' | 'bold';
  }
>;

export function AppText({ children, variant = 'body', tone, muted, tabular, weight, style, ...props }: Props) {
  return (
    <Text
      {...props}
      style={[
        styles.base,
        typography[variant],
        tone && toneStyles[tone],
        !tone && muted && toneStyles.muted,
        tabular && styles.tabular,
        weight === 'medium' && styles.medium,
        weight === 'semibold' && styles.semibold,
        weight === 'bold' && styles.bold,
        style
      ]}
    >
      {children}
    </Text>
  );
}

const styles = StyleSheet.create({
  base: {
    color: colors.text
  },
  tabular: {
    fontVariant: ['tabular-nums']
  },
  medium: {
    fontWeight: '500'
  },
  semibold: {
    fontWeight: '600'
  },
  bold: {
    fontWeight: '700'
  }
});

const toneStyles = StyleSheet.create({
  default: {
    color: colors.text
  },
  muted: {
    color: colors.muted
  },
  faint: {
    color: colors.faint
  },
  primary: {
    color: colors.primary
  },
  success: {
    color: colors.success
  },
  danger: {
    color: colors.danger
  },
  onPrimary: {
    color: colors.onPrimary
  }
});
