import { useEffect, useRef, useState } from 'react';
import { Animated, Easing, StyleSheet } from 'react-native';
import Svg, { Path } from 'react-native-svg';

// The native splash (expo-splash-screen in app.config.ts) shows this same mark
// on the same background, centred, at MARK_SIZE dp, so taking over from it is
// seamless. Keep the three in step: background colour, image and imageWidth.
export const SPLASH_BACKGROUND = '#0B0B10';
const MARK_SIZE = 112;

// The icon's three layers (docs/logo/pennywise-icon.svg), cropped to the mark.
const VIEW_BOX = '112 112 288 288';
const LAYERS = [
  { fill: '#5B64DA', d: 'M164 154V142A30 30 0 0 1 194 112H318A30 30 0 0 1 348 142V154Z' },
  { fill: '#8A93FA', d: 'M136 218V202A34 34 0 0 1 170 168H342A34 34 0 0 1 376 202V218Z' },
  {
    fill: '#C9CDFF',
    d: 'M152 232H360A40 40 0 0 1 400 272V360A40 40 0 0 1 360 400H152A40 40 0 0 1 112 360V272A40 40 0 0 1 152 232ZM218 304A12 12 0 0 0 218 328H294A12 12 0 0 0 294 304Z',
    evenOdd: true
  }
] as const;

// Loaded defensively: a JS update reaching an APK built without the module
// must not stop the app from starting.
let NativeSplash: typeof import('expo-splash-screen') | null = null;
try {
  NativeSplash = require('expo-splash-screen');
} catch {
  NativeSplash = null;
}

/** Keeps the native splash up until BootSplash has drawn its copy. Call at module scope. */
export function holdNativeSplash() {
  try {
    NativeSplash?.setOptions({ duration: 0 });
    void NativeSplash?.preventAutoHideAsync().catch(() => undefined);
  } catch {
    // no native splash module: nothing to hold
  }
}

function hideNativeSplash() {
  try {
    NativeSplash?.hide();
  } catch {
    // already gone
  }
}

/**
 * The launch animation, about 0.7 s: the mark the native splash showed has its
 * two upper drawers lift and drop back into the stack, then the screen fades
 * away into the app. It waits (holding the mark) until `ready`, and never
 * longer than MAX_WAIT_MS, so a slow network can't trap the user here.
 */
const MAX_WAIT_MS = 4000;

export function BootSplash({ ready, onDone }: { ready: boolean; onDone: () => void }) {
  const lift = useRef(LAYERS.map(() => new Animated.Value(0))).current;
  const squash = useRef(new Animated.Value(1)).current;
  const exit = useRef(new Animated.Value(0)).current;
  const [settled, setSettled] = useState(false);
  const [timedOut, setTimedOut] = useState(false);
  const exiting = useRef(false);
  const started = useRef(false);

  // the intro, once our copy of the mark is on screen
  const start = () => {
    if (started.current) return;
    started.current = true;
    hideNativeSplash();
    const hop = (value: Animated.Value, height: number) =>
      Animated.sequence([
        Animated.timing(value, {
          toValue: -height,
          duration: 170,
          easing: Easing.out(Easing.quad),
          useNativeDriver: true
        }),
        Animated.timing(value, {
          toValue: 0,
          duration: 230,
          easing: Easing.out(Easing.back(2.2)),
          useNativeDriver: true
        })
      ]);
    Animated.sequence([
      Animated.stagger(70, [hop(lift[0], 16), hop(lift[1], 9)]),
      // the box takes the weight as the drawers land
      Animated.sequence([
        Animated.timing(squash, { toValue: 0.95, duration: 70, useNativeDriver: true }),
        Animated.timing(squash, { toValue: 1, duration: 120, easing: Easing.out(Easing.quad), useNativeDriver: true })
      ])
    ]).start(() => setSettled(true));
  };

  useEffect(() => {
    const timer = setTimeout(() => setTimedOut(true), MAX_WAIT_MS);
    return () => clearTimeout(timer);
  }, []);

  useEffect(() => {
    if (!settled || (!ready && !timedOut) || exiting.current) return;
    exiting.current = true;
    Animated.timing(exit, {
      toValue: 1,
      duration: 240,
      easing: Easing.in(Easing.quad),
      useNativeDriver: true
    }).start(onDone);
  }, [settled, ready, timedOut, exit, onDone]);

  const scale = exit.interpolate({ inputRange: [0, 1], outputRange: [1, 1.18] });
  const opacity = exit.interpolate({ inputRange: [0, 1], outputRange: [1, 0] });

  return (
    <Animated.View pointerEvents="none" style={[styles.overlay, { opacity }]} onLayout={start}>
      <Animated.View style={[styles.mark, { transform: [{ scale }] }]}>
        {LAYERS.map((layer, i) => (
          <Animated.View
            key={layer.fill}
            style={[
              StyleSheet.absoluteFill,
              i === 2
                ? // squash from the box's bottom edge, which is the mark's
                  { transform: [{ translateY: MARK_SIZE / 2 }, { scaleY: squash }, { translateY: -MARK_SIZE / 2 }] }
                : { transform: [{ translateY: lift[i] }] }
            ]}
          >
            <Svg width={MARK_SIZE} height={MARK_SIZE} viewBox={VIEW_BOX}>
              <Path d={layer.d} fill={layer.fill} fillRule={'evenOdd' in layer ? 'evenodd' : 'nonzero'} />
            </Svg>
          </Animated.View>
        ))}
      </Animated.View>
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  overlay: {
    ...StyleSheet.absoluteFillObject,
    zIndex: 1000,
    elevation: 1000,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: SPLASH_BACKGROUND
  },
  mark: {
    width: MARK_SIZE,
    height: MARK_SIZE
  }
});
