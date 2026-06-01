import type { PropsWithChildren } from 'react';
import { StyleSheet, Text, type TextProps } from 'react-native';
import { colors } from '../theme';

type Props = PropsWithChildren<TextProps & { muted?: boolean; weight?: 'regular' | 'medium' | 'semibold' | 'bold' }>;

export function AppText({ children, muted, weight = 'regular', style, ...props }: Props) {
  return (
    <Text
      {...props}
      style={[
        styles.text,
        muted && styles.muted,
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
  text: {
    color: colors.text,
    fontSize: 15,
    lineHeight: 21
  },
  muted: {
    color: colors.muted
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
