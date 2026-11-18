import { useId } from 'react';
import { StyleSheet, View, type StyleProp, type ViewStyle } from 'react-native';
import Svg, { Defs, Line, Pattern, Rect } from 'react-native-svg';
import { colors } from '../theme';

/** Diagonal hatching that fills its parent; put it first inside a View with `overflow: 'hidden'`. */
export function Hatch({ color, gap = 6, width = 1, opacity = 1 }: { color: string; gap?: number; width?: number; opacity?: number }) {
  const id = `hatch${useId().replace(/[^a-zA-Z0-9]/g, '')}`;
  return (
    <Svg style={StyleSheet.absoluteFill} pointerEvents="none">
      <Defs>
        <Pattern id={id} patternUnits="userSpaceOnUse" width={gap} height={gap} patternTransform="rotate(45)">
          <Line x1={0} y1={0} x2={0} y2={gap} stroke={color} strokeWidth={width} strokeOpacity={opacity} />
        </Pattern>
      </Defs>
      <Rect x={0} y={0} width="100%" height="100%" fill={`url(#${id})`} />
    </Svg>
  );
}

/**
 * A rounded meter whose unused part is hatched, with a knob where the fill
 * ends. `percent` is 0–100; past 100 the whole track turns danger.
 */
export function HatchMeter({ percent, color = colors.primary, height = 14, style }: { percent: number; color?: string; height?: number; style?: StyleProp<ViewStyle> }) {
  const over = percent > 100;
  const clamped = Math.max(0, Math.min(100, percent));
  const fill = over ? colors.danger : color;
  const knob = height - 4;

  return (
    <View style={[styles.track, { height, borderRadius: height / 2 }, style]}>
      <Hatch color={fill} gap={5} width={1.4} opacity={0.45} />
      {clamped > 0 ? (
        <View style={[styles.fill, { width: `${clamped}%`, minWidth: height, borderRadius: height / 2, backgroundColor: fill }]}>
          <View style={[styles.knob, { width: knob, height: knob, borderRadius: knob / 2 }]} />
        </View>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  track: {
    overflow: 'hidden',
    backgroundColor: colors.surfaceStrong
  },
  fill: {
    height: '100%',
    alignItems: 'flex-end',
    justifyContent: 'center',
    paddingRight: 2
  },
  knob: {
    backgroundColor: colors.surface,
    borderWidth: 2,
    borderColor: 'rgba(0, 0, 0, 0.15)'
  }
});
