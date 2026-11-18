import { useEffect, useState, type ReactNode } from 'react';
import { Keyboard, Pressable, StyleSheet, View } from 'react-native';
import type { BottomTabBarProps } from '@react-navigation/bottom-tabs';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import Animated, { Easing, interpolate, useAnimatedStyle, useDerivedValue, useSharedValue, withTiming, type SharedValue } from 'react-native-reanimated';
import { colors, spacing } from '../theme';
import { AppText } from './AppText';

type TabMeta = { label: string; icon: (color: string) => ReactNode };

const ITEM = 46;
const LABEL_GAP = 6;
const PILL_PAD = 14;
const TIMING = { duration: 280, easing: Easing.bezier(0.2, 0.8, 0.2, 1) };

/**
 * The floating pill tab bar. Inactive tabs are bare icons; the active one
 * grows into a pill with its label.
 *
 * The pill follows `active`, a shared value set the moment a tab is tapped,
 * and the navigation runs a frame later. Driving it from `state.index`
 * instead made the pill wait until React had rendered the next screen, which
 * on a first visit is long enough to look like a stutter.
 */
export function FloatingTabBar({ state, navigation, tabs }: BottomTabBarProps & { tabs: Record<string, TabMeta> }) {
  const insets = useSafeAreaInsets();
  const keyboardOpen = useKeyboardOpen();
  const active = useSharedValue(state.index);

  // follow navigation that didn't come from a tap here (links, back)
  useEffect(() => {
    active.value = state.index;
  }, [active, state.index]);

  if (keyboardOpen) return null;

  return (
    <View pointerEvents="box-none" style={[styles.wrap, { bottom: Math.max(insets.bottom, spacing.md) }]}>
      <View style={styles.bar}>
        {state.routes.map((route, index) => {
          const meta = tabs[route.name];
          if (!meta) return null;
          const focused = state.index === index;
          return (
            <TabItem
              key={route.key}
              meta={meta}
              index={index}
              active={active}
              focused={focused}
              onPress={() => {
                const event = navigation.emit({ type: 'tabPress', target: route.key, canPreventDefault: true });
                if (focused || event.defaultPrevented) return;
                active.value = index;
                // let the pill's first frames go out before the next screen renders
                requestAnimationFrame(() => navigation.navigate(route.name, route.params));
              }}
              onLongPress={() => navigation.emit({ type: 'tabLongPress', target: route.key })}
            />
          );
        })}
      </View>
    </View>
  );
}

function TabItem({
  meta,
  index,
  active,
  focused,
  onPress,
  onLongPress
}: {
  meta: TabMeta;
  index: number;
  active: SharedValue<number>;
  focused: boolean;
  onPress: () => void;
  onLongPress: () => void;
}) {
  const progress = useDerivedValue(() => withTiming(active.value === index ? 1 : 0, TIMING));
  const labelWidth = useSharedValue(0);
  const [measured, setMeasured] = useState(false);

  const itemStyle = useAnimatedStyle(() => ({
    width: ITEM + progress.value * (labelWidth.value + LABEL_GAP + PILL_PAD * 2 - (ITEM - 20))
  }));
  const pillStyle = useAnimatedStyle(() => ({
    opacity: progress.value,
    transform: [{ scale: interpolate(progress.value, [0, 1], [0.85, 1]) }]
  }));
  const labelStyle = useAnimatedStyle(() => ({
    width: progress.value * labelWidth.value,
    marginLeft: progress.value * LABEL_GAP,
    opacity: interpolate(progress.value, [0.4, 1], [0, 1], 'clamp')
  }));

  return (
    <Pressable onPress={onPress} onLongPress={onLongPress} accessibilityRole="tab" accessibilityState={{ selected: focused }} accessibilityLabel={meta.label}>
      <Animated.View style={[styles.item, itemStyle]}>
        <Animated.View style={[styles.pill, pillStyle]} />
        {/* both icon colors stacked, crossfaded with the pill */}
        <View>
          {meta.icon(colors.tabBarIcon)}
          <Animated.View style={[StyleSheet.absoluteFill, pillStyle]}>{meta.icon(colors.tabBarActiveText)}</Animated.View>
        </View>
        <Animated.View style={[styles.labelClip, labelStyle]}>
          {/* wider than any label, so the clip reveals it instead of truncating it */}
          <View style={styles.labelTrack}>
            <AppText variant="caption" weight="semibold" style={styles.label}>
              {meta.label}
            </AppText>
          </View>
        </Animated.View>
      </Animated.View>
      {/* measures the label once, off screen */}
      {measured ? null : (
        <View style={styles.measure} pointerEvents="none">
          <AppText
            variant="caption"
            weight="semibold"
            style={styles.label}
            onLayout={(event) => {
              labelWidth.value = Math.ceil(event.nativeEvent.layout.width);
              setMeasured(true);
            }}
          >
            {meta.label}
          </AppText>
        </View>
      )}
    </Pressable>
  );
}

function useKeyboardOpen() {
  const [open, setOpen] = useState(false);
  useEffect(() => {
    const show = Keyboard.addListener('keyboardDidShow', () => setOpen(true));
    const hide = Keyboard.addListener('keyboardDidHide', () => setOpen(false));
    return () => {
      show.remove();
      hide.remove();
    };
  }, []);
  return open;
}

const styles = StyleSheet.create({
  wrap: {
    position: 'absolute',
    left: 0,
    right: 0,
    alignItems: 'center'
  },
  bar: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 4,
    padding: 6,
    borderRadius: 32,
    backgroundColor: colors.tabBar,
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: colors.tabBarBorder,
    elevation: 16,
    shadowColor: colors.shadow,
    shadowOffset: { width: 0, height: 8 },
    shadowOpacity: 0.3,
    shadowRadius: 20
  },
  item: {
    height: ITEM,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    overflow: 'hidden'
  },
  pill: {
    ...StyleSheet.absoluteFillObject,
    borderRadius: ITEM / 2,
    backgroundColor: colors.tabBarActive
  },
  labelClip: {
    overflow: 'hidden'
  },
  labelTrack: {
    width: 160
  },
  label: {
    color: colors.tabBarActiveText,
    fontSize: 14
  },
  measure: {
    position: 'absolute',
    opacity: 0,
    left: 0,
    top: 0,
    width: 400,
    flexDirection: 'row'
  }
});
