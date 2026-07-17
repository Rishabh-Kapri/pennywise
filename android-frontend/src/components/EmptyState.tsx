import type { ReactNode } from 'react';
import { StyleSheet, View } from 'react-native';
import { spacing } from '../theme';
import { AppText } from './AppText';
import { IconTile } from './IconTile';

export function EmptyState({ icon, title, body }: { icon: ReactNode; title: string; body?: string }) {
  return (
    <View style={styles.container}>
      <IconTile size={56}>{icon}</IconTile>
      <AppText variant="heading">{title}</AppText>
      {body ? (
        <AppText variant="caption" muted style={styles.body}>
          {body}
        </AppText>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    alignItems: 'center',
    gap: spacing.md,
    paddingVertical: spacing.xxl,
    paddingHorizontal: spacing.xl
  },
  body: {
    textAlign: 'center',
    maxWidth: 260
  }
});
