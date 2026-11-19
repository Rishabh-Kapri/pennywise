import { StyleSheet, View } from 'react-native';
import { colors, spacing } from '../theme';
import { AppText } from './AppText';
import { PennywiseMark } from './PennywiseMark';

export function LoadingStateView({ label = 'Loading' }: { label?: string }) {
  return (
    <View style={styles.container}>
      <PennywiseMark size={56} animated />
      <AppText variant="caption" muted>{label}</AppText>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    minHeight: 240,
    alignItems: 'center',
    justifyContent: 'center',
    gap: spacing.md,
    backgroundColor: colors.background
  }
});
