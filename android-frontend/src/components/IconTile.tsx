import type { ReactNode } from 'react';
import { StyleSheet, View, type StyleProp, type ViewStyle } from 'react-native';
import { colors, radii } from '../theme';

type Props = {
  children: ReactNode;
  tone?: 'primary' | 'success' | 'danger' | 'warning' | 'neutral';
  size?: number;
  style?: StyleProp<ViewStyle>;
};

const toneBackground = {
  primary: colors.primaryMuted,
  success: colors.successMuted,
  danger: colors.dangerMuted,
  warning: colors.warningMuted,
  neutral: colors.surfaceStrong
};

export function IconTile({ children, tone = 'primary', size = 40, style }: Props) {
  return (
    <View
      style={[
        styles.tile,
        { width: size, height: size, borderRadius: size >= 44 ? radii.md : radii.sm, backgroundColor: toneBackground[tone] },
        style
      ]}
    >
      {children}
    </View>
  );
}

const styles = StyleSheet.create({
  tile: {
    alignItems: 'center',
    justifyContent: 'center'
  }
});
