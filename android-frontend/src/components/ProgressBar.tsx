import { StyleSheet, View } from 'react-native';
import { colors, radii } from '../theme';

type Props = {
  /** 0–100 */
  percent: number;
  color?: string;
  height?: number;
};

export function ProgressBar({ percent, color = colors.primary, height = 6 }: Props) {
  const clamped = Math.max(0, Math.min(100, percent));

  return (
    <View style={[styles.track, { height, borderRadius: height / 2 }]}>
      <View style={[styles.fill, { width: `${clamped}%`, backgroundColor: color, borderRadius: height / 2 }]} />
    </View>
  );
}

const styles = StyleSheet.create({
  track: {
    backgroundColor: colors.surfaceTertiary,
    overflow: 'hidden',
    borderRadius: radii.full
  },
  fill: {
    height: '100%'
  }
});
