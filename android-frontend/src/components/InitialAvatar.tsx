import { StyleSheet, View } from 'react-native';
import { colors } from '../theme';
import { AppText } from './AppText';

const palette = [
  { bg: colors.primaryMuted, fg: colors.primary },
  { bg: colors.successMuted, fg: colors.success },
  { bg: colors.warningMuted, fg: colors.warning },
  { bg: 'rgba(111, 183, 255, 0.14)', fg: colors.info },
  { bg: colors.dangerMuted, fg: colors.danger }
];

export function InitialAvatar({ name, size = 40 }: { name: string; size?: number }) {
  const initial = (name.trim().charAt(0) || '?').toUpperCase();
  const tone = palette[Math.abs([...name].reduce((hash, char) => hash * 31 + char.charCodeAt(0), 7)) % palette.length];

  return (
    <View style={[styles.avatar, { width: size, height: size, borderRadius: size / 2, backgroundColor: tone.bg }]}>
      <AppText weight="semibold" style={{ color: tone.fg, fontSize: size * 0.4 }}>
        {initial}
      </AppText>
    </View>
  );
}

const styles = StyleSheet.create({
  avatar: {
    alignItems: 'center',
    justifyContent: 'center'
  }
});
