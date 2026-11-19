import { useEffect, useRef, useState } from 'react';
import { Animated, Easing, StyleSheet } from 'react-native';
import { PennywiseMark } from './PennywiseMark';

// The native splash (expo-splash-screen in app.config.ts) shows the icon's
// opening frame (assets/splash-icon.png: the tile with no sun or lines yet) on
// the same background, centred, at MARK_SIZE dp, so taking over from it is
// seamless. Keep the three in step: background colour, image and imageWidth.
export const SPLASH_BACKGROUND = '#0B0B10';
const MARK_SIZE = 112;

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
 * The launch animation: PennywiseMark's loop, picking up from the native
 * splash's still tile, so the first emails fall into the water and become its
 * lines. It shows at least MIN_SHOW_MS (the first line has come up by then),
 * waits for `ready`, never longer than MAX_WAIT_MS so a slow network can't trap
 * the user here, then fades out into the app.
 */
const MIN_SHOW_MS = 1500;
const MAX_WAIT_MS = 4000;

export function BootSplash({ ready, onDone }: { ready: boolean; onDone: () => void }) {
  const exit = useRef(new Animated.Value(0)).current;
  const [settled, setSettled] = useState(false);
  const [timedOut, setTimedOut] = useState(false);
  const exiting = useRef(false);
  const started = useRef(false);

  // our copy of the tile is on screen: drop the native one and start the clock
  const start = () => {
    if (started.current) return;
    started.current = true;
    hideNativeSplash();
    setTimeout(() => setSettled(true), MIN_SHOW_MS);
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
      <Animated.View style={{ transform: [{ scale }] }}>
        <PennywiseMark size={MARK_SIZE} animated skipFadeIn />
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
  }
});
