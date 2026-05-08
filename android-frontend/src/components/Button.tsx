import type { PropsWithChildren } from 'react';
import { Pressable, StyleSheet, type PressableProps } from 'react-native';
import { colors, radii, spacing } from '../theme';
import { AppText } from './AppText';

type Props = PropsWithChildren<
  PressableProps & {
    variant?: 'primary' | 'secondary' | 'ghost' | 'danger';
  }
>;

export function Button({ children, variant = 'primary', style, ...props }: Props) {
  return (
    <Pressable
      {...props}
      style={({ pressed }) => [
        styles.base,
        variant === 'primary' && styles.primary,
        variant === 'secondary' && styles.secondary,
        variant === 'ghost' && styles.ghost,
        variant === 'danger' && styles.danger,
        pressed && styles.pressed,
        typeof style === 'function' ? style({ pressed }) : style
      ]}
    >
      {typeof children === 'string' || typeof children === 'number' ? (
        <AppText
          weight="semibold"
          style={[
            styles.label,
            variant === 'primary' && styles.primaryLabel,
            variant === 'danger' && styles.primaryLabel,
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
    minHeight: 44,
    borderRadius: radii.sm,
    alignItems: 'center',
    justifyContent: 'center',
    paddingHorizontal: spacing.lg,
    borderWidth: StyleSheet.hairlineWidth
  },
  primary: {
    backgroundColor: colors.primary,
    borderColor: colors.primary
  },
  secondary: {
    backgroundColor: colors.surface,
    borderColor: colors.border
  },
  ghost: {
    backgroundColor: 'transparent',
    borderColor: 'transparent'
  },
  danger: {
    backgroundColor: colors.danger,
    borderColor: colors.danger
  },
  label: {
    textAlign: 'center'
  },
  primaryLabel: {
    color: '#fff'
  },
  ghostLabel: {
    color: colors.primary
  },
  pressed: {
    opacity: 0.78
  }
});
