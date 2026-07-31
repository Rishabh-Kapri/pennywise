import type { PropsWithChildren } from 'react';
import { Pressable, StyleSheet, type PressableProps } from 'react-native';
import { colors, radii, spacing } from '../theme';
import { AppText } from './AppText';

type Props = PropsWithChildren<
  PressableProps & {
    variant?: 'primary' | 'secondary' | 'ghost' | 'danger';
    size?: 'md' | 'sm';
  }
>;

export function Button({ children, variant = 'primary', size = 'md', style, ...props }: Props) {
  return (
    <Pressable
      {...props}
      style={({ pressed }) => [
        styles.base,
        size === 'sm' && styles.small,
        variant === 'primary' && styles.primary,
        variant === 'secondary' && styles.secondary,
        variant === 'ghost' && styles.ghost,
        variant === 'danger' && styles.danger,
        pressed && styles.pressed,
        props.disabled && styles.disabled,
        typeof style === 'function' ? style({ pressed }) : style
      ]}
    >
      {typeof children === 'string' || typeof children === 'number' ? (
        <AppText
          weight="semibold"
          style={[
            styles.label,
            variant === 'primary' && styles.primaryLabel,
            variant === 'danger' && styles.dangerLabel,
            variant === 'ghost' && styles.ghostLabel
          ]}
        >
          {children}
        </AppText>
      ) : (
        children
      )}
    </Pressable>
  );
}

const styles = StyleSheet.create({
  base: {
    minHeight: 50,
    borderRadius: radii.full,
    alignItems: 'center',
    justifyContent: 'center',
    paddingHorizontal: spacing.xl
  },
  small: {
    minHeight: 38,
    paddingHorizontal: spacing.lg
  },
  primary: {
    backgroundColor: colors.primary
  },
  secondary: {
    backgroundColor: colors.surfaceStrong
  },
  ghost: {
    backgroundColor: 'transparent'
  },
  danger: {
    backgroundColor: colors.dangerMuted
  },
  label: {
    textAlign: 'center'
  },
  primaryLabel: {
    color: colors.onPrimary
  },
  dangerLabel: {
    color: colors.danger
  },
  ghostLabel: {
    color: colors.primary
  },
  pressed: {
    opacity: 0.72
  },
  disabled: {
    opacity: 0.45
  }
});
