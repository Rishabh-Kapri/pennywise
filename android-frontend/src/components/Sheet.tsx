import { useEffect, useState, type ReactNode } from 'react';
import { Keyboard, Modal, Pressable, StyleSheet, View, useWindowDimensions } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { X } from 'lucide-react-native';
import { AppText } from './AppText';
import { colors, radii, spacing } from '../theme';

/**
 * Height of the on-screen keyboard, 0 while it's hidden. Expo 54 runs Android
 * edge-to-edge, so the window no longer shrinks for the keyboard
 * (adjustResize is a no-op) and sheets have to lift their own content.
 */
export function useKeyboardHeight(): number {
  const [height, setHeight] = useState(0);

  useEffect(() => {
    const show = Keyboard.addListener('keyboardDidShow', (event) => setHeight(event.endCoordinates.height));
    const hide = Keyboard.addListener('keyboardDidHide', () => setHeight(0));
    return () => {
      show.remove();
      hide.remove();
    };
  }, []);

  return height;
}

type SheetProps = {
  title?: string;
  subtitle?: string;
  onClose: () => void;
  children: ReactNode;
  /** pinned under the scrolling body, above the keyboard */
  footer?: ReactNode;
  /** next to the close button */
  headerRight?: ReactNode;
  /**
   * `tall` holds a fixed height so a list that filters down to a few rows
   * doesn't collapse the sheet under the keyboard; `auto` hugs its content.
   */
  size?: 'auto' | 'tall';
};

/**
 * Bottom sheet drawn as an absolute overlay. Use it inside an existing Modal
 * (nested Modals misbehave on Android); use `Sheet` when there is none.
 */
export function SheetOverlay({ title, subtitle, onClose, children, footer, headerRight, size = 'auto' }: SheetProps) {
  const insets = useSafeAreaInsets();
  const keyboard = useKeyboardHeight();
  const { height: windowHeight } = useWindowDimensions();
  const maxHeight = windowHeight - insets.top - spacing.xl;

  return (
    <View style={StyleSheet.absoluteFill}>
      <Pressable style={styles.backdrop} onPress={onClose} accessibilityLabel="Close" />
      <View
        style={[
          styles.sheet,
          size === 'tall' ? { height: maxHeight * 0.94 } : { maxHeight },
          { paddingBottom: (keyboard > 0 ? keyboard : insets.bottom) + spacing.md }
        ]}
      >
        <View style={styles.handle} />
        {title || headerRight ? (
          <View style={styles.header}>
            <View style={styles.headerText}>
              {title ? (
                <AppText variant="heading" numberOfLines={1}>
                  {title}
                </AppText>
              ) : null}
              {subtitle ? (
                <AppText variant="caption" muted numberOfLines={1}>
                  {subtitle}
                </AppText>
              ) : null}
            </View>
            {headerRight}
            <Pressable
              accessibilityRole="button"
              accessibilityLabel="Close"
              hitSlop={8}
              style={({ pressed }) => [styles.closeButton, pressed && styles.pressed]}
              onPress={onClose}
            >
              <X size={18} color={colors.muted} />
            </Pressable>
          </View>
        ) : null}
        <View style={size === 'tall' ? styles.bodyTall : styles.bodyAuto}>{children}</View>
        {footer ? <View style={styles.footer}>{footer}</View> : null}
      </View>
    </View>
  );
}

/** Standalone sheet for screens that aren't already inside a Modal. */
export function Sheet({ visible, ...props }: SheetProps & { visible: boolean }) {
  return (
    <Modal
      visible={visible}
      transparent
      statusBarTranslucent
      navigationBarTranslucent
      animationType="slide"
      onRequestClose={props.onClose}
    >
      <SheetOverlay {...props} />
    </Modal>
  );
}

const styles = StyleSheet.create({
  backdrop: {
    ...StyleSheet.absoluteFillObject,
    backgroundColor: 'rgba(0,0,0,0.55)'
  },
  sheet: {
    position: 'absolute',
    left: 0,
    right: 0,
    bottom: 0,
    backgroundColor: colors.surface,
    borderTopLeftRadius: radii.xl,
    borderTopRightRadius: radii.xl,
    paddingHorizontal: spacing.lg,
    paddingTop: spacing.sm,
    gap: spacing.md
  },
  handle: {
    alignSelf: 'center',
    width: 36,
    height: 4,
    borderRadius: 2,
    backgroundColor: colors.surfaceTertiary
  },
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md,
    paddingHorizontal: spacing.xs
  },
  headerText: {
    flex: 1,
    minWidth: 0
  },
  closeButton: {
    width: 34,
    height: 34,
    borderRadius: 17,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.surfaceStrong
  },
  pressed: {
    opacity: 0.7
  },
  bodyTall: {
    flex: 1,
    minHeight: 0
  },
  // without an explicit shrink a ScrollView child sizes to its content and the
  // sheet's maxHeight clips it instead of scrolling
  bodyAuto: {
    flexShrink: 1,
    minHeight: 0
  },
  footer: {
    paddingTop: spacing.xs
  }
});
