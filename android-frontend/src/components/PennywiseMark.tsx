import type { ReactNode } from 'react';
import { StyleSheet, View } from 'react-native';
import Animated, {
  Easing,
  useAnimatedProps,
  useAnimatedStyle,
  useFrameCallback,
  useReducedMotion,
  useSharedValue,
  type SharedValue
} from 'react-native-reanimated';
import Svg, { ClipPath, Defs, G, Path, Rect } from 'react-native-svg';

// The Pennywise icon (docs/logo/pennywise-icon.svg) and its loop
// (docs/logo/pennywise-icon-animated.svg): emails fall into the water and come
// up as its lines, then the sun rises. Same keyframes and easings as the web's
// PennywiseMark.module.css; change both together. Still, it is the loop's
// last frame, the same picture as the app icon.

const SKY = '#3176AA';
const WATER = '#0D1A26';
const LINE = '#ADD2EB';
const HORIZON = 282;
const LOOP_MS = 8000;
const STAGGER_MS = 700;

const LINES = [
  { x: 160, y: 300, width: 192 },
  { x: 188, y: 326, width: 136 },
  { x: 216, y: 352, width: 80 },
  { x: 240, y: 378, width: 32 }
] as const;

const sunPath = (dy: number) => {
  'worklet';
  return `M144 ${HORIZON + dy}A112 112 0 0 1 368 ${HORIZON + dy}Z`;
};

// Keyframe tracks: [offset 0..1, value]. Like CSS, the easing runs per segment.
type Track = readonly (readonly [number, number])[];
const tileEase = Easing.bezierFn(0.2, 0.8, 0.2, 1);
const fallEase = Easing.bezierFn(0.55, 0, 0.75, 0.2);
const settleEase = Easing.bezierFn(0.22, 1, 0.36, 1);

const T = {
  tileOpacity: [[0, 0], [0.04, 1], [0.92, 1], [1, 0]],
  tileScale: [[0, 0.94], [0.04, 1], [0.92, 1], [1, 0.97]],
  envOpacity: [[0, 0], [0.02, 1], [0.11, 1], [0.14, 0], [1, 0]],
  envY: [[0, -280], [0.11, 0], [0.14, 10], [1, 10]],
  envScale: [[0, 1], [0.11, 1], [0.14, 0.15], [1, 0.15]],
  lineOpacity: [[0, 0], [0.13, 0], [0.18, 1], [0.88, 1], [0.96, 0], [1, 0]],
  lineScale: [[0, 0], [0.13, 0], [0.18, 1.08], [0.22, 1], [1, 1]],
  sunY: [[0, 124], [0.44, 124], [0.6, 0], [1, 0]]
} satisfies Record<string, Track>;

function at(track: Track, p: number, ease: (t: number) => number) {
  'worklet';
  if (p <= track[0][0]) return track[0][1];
  for (let i = 1; i < track.length; i++) {
    const [b, vb] = track[i];
    if (p <= b) {
      const [a, va] = track[i - 1];
      return va + (vb - va) * ease((p - a) / (b - a));
    }
  }
  return track[track.length - 1][1];
}

// Where in the loop an element is, after its delay (held at 0% until then).
function phase(elapsed: number, delay: number) {
  'worklet';
  return elapsed < delay ? 0 : ((elapsed - delay) % LOOP_MS) / LOOP_MS;
}

const AnimatedG = Animated.createAnimatedComponent(G);
const AnimatedRect = Animated.createAnimatedComponent(Rect);
const AnimatedPath = Animated.createAnimatedComponent(Path);

function Envelope({ elapsed, index }: { elapsed: SharedValue<number>; index: number }) {
  // falls from the top of the sky, then squashes into the horizon from its bottom edge
  const group = useAnimatedProps(() => ({
    opacity: at(T.envOpacity, phase(elapsed.value, index * STAGGER_MS), fallEase)
  }));
  const body = useAnimatedProps(() => {
    const p = phase(elapsed.value, index * STAGGER_MS);
    const s = at(T.envScale, p, fallEase);
    const bottom = HORIZON + at(T.envY, p, fallEase);
    return { y: bottom - 42 * s, height: 42 * s };
  });
  const flap = useAnimatedProps(() => {
    const p = phase(elapsed.value, index * STAGGER_MS);
    const s = at(T.envScale, p, fallEase);
    const bottom = HORIZON + at(T.envY, p, fallEase);
    const y = (v: number) => bottom - (HORIZON - v) * s;
    return { d: `M235 ${y(251)}L256 ${y(266)}L277 ${y(251)}` };
  });
  return (
    <AnimatedG animatedProps={group}>
      <AnimatedRect animatedProps={body} x={226} width={60} rx={8} fill="#FFFFFF" />
      <AnimatedPath
        animatedProps={flap}
        fill="none"
        stroke={SKY}
        strokeWidth={5}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </AnimatedG>
  );
}

function Line({ elapsed, index }: { elapsed: SharedValue<number>; index: number }) {
  const { x, y, width } = LINES[index];
  // grows from its centre with a small overshoot
  const props = useAnimatedProps(() => {
    const p = phase(elapsed.value, index * STAGGER_MS);
    const w = width * at(T.lineScale, p, settleEase);
    return { x: x + (width - w) / 2, width: w, opacity: at(T.lineOpacity, p, settleEase) };
  });
  return <AnimatedRect animatedProps={props} y={y} height={12} rx={6} fill={LINE} />;
}

function Tile({ size, children, sun }: { size: number; children?: ReactNode; sun: ReactNode }) {
  return (
    <Svg width={size} height={size} viewBox="0 0 512 512">
      <Defs>
        <ClipPath id="pwTile">
          <Rect width={512} height={512} rx={116} />
        </ClipPath>
        <ClipPath id="pwSky">
          <Rect width={512} height={HORIZON} />
        </ClipPath>
      </Defs>
      <G clipPath="url(#pwTile)">
        <Rect width={512} height={290} fill={SKY} />
        <Rect y={HORIZON} width={512} height={230} fill={WATER} />
        <G clipPath="url(#pwSky)">{sun}</G>
        {children}
      </G>
    </Svg>
  );
}

function AnimatedMark({ size, skipFadeIn }: { size: number; skipFadeIn: boolean }) {
  const elapsed = useSharedValue(0);
  useFrameCallback((frame) => {
    elapsed.value = frame.timeSinceFirstFrame;
  });

  const tile = useAnimatedStyle(() => {
    const p = phase(elapsed.value, 0);
    // taking over from a still copy (the native splash): no fade-in on the first pass
    const held = skipFadeIn && elapsed.value < LOOP_MS * 0.04;
    return {
      opacity: held ? 1 : at(T.tileOpacity, p, tileEase),
      transform: [{ scale: held ? 1 : at(T.tileScale, p, tileEase) }]
    };
  });
  const sun = useAnimatedProps(() => ({ d: sunPath(at(T.sunY, phase(elapsed.value, 0), settleEase)) }));

  return (
    <Animated.View style={[{ width: size, height: size }, tile]}>
      <Tile
        size={size}
        sun={
          <>
            <AnimatedPath animatedProps={sun} fill="#FFFFFF" />
            {LINES.map((_, i) => (
              <Envelope key={i} elapsed={elapsed} index={i} />
            ))}
          </>
        }
      >
        {LINES.map((_, i) => (
          <Line key={i} elapsed={elapsed} index={i} />
        ))}
      </Tile>
    </Animated.View>
  );
}

export function PennywiseMark({
  size = 56,
  animated = false,
  skipFadeIn = false
}: {
  size?: number;
  /** Play the 8 s loop. With reduced motion on, the still mark shows instead. */
  animated?: boolean;
  /** Start with the tile already on screen, for taking over from a still copy of it. */
  skipFadeIn?: boolean;
}) {
  const reduceMotion = useReducedMotion();
  if (animated && !reduceMotion) return <AnimatedMark size={size} skipFadeIn={skipFadeIn} />;
  return (
    <View style={[styles.still, { width: size, height: size }]}>
      <Tile size={size} sun={<Path d={sunPath(0)} fill="#FFFFFF" />}>
        {LINES.map((line, i) => (
          <Rect key={i} x={line.x} y={line.y} width={line.width} height={12} rx={6} fill={LINE} />
        ))}
      </Tile>
    </View>
  );
}

const styles = StyleSheet.create({
  still: { overflow: 'hidden' }
});
