import { Image, StyleSheet, View } from 'react-native';
import { Mail } from 'lucide-react-native';
import { AppText } from '../../../components/AppText';
import { Card } from '../../../components/Card';
import { IconTile } from '../../../components/IconTile';
import { colors, spacing } from '../../../theme';
import type { ConnectedProvider } from '../types';

function formatSync(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: 'numeric',
    minute: '2-digit'
  });
}

function ProviderRow({ provider, divider }: { provider: ConnectedProvider; divider: boolean }) {
  return (
    <View style={[styles.row, divider && styles.rowDivider]}>
      {provider.picture ? (
        <Image source={{ uri: provider.picture }} style={styles.avatar} />
      ) : (
        <IconTile size={38}>
          <Mail color={colors.primary} size={18} />
        </IconTile>
      )}
      <View style={styles.main}>
        <AppText weight="medium" style={styles.providerType}>
          {provider.providerType}
        </AppText>
        <AppText variant="caption" muted numberOfLines={1}>
          {provider.email || provider.name || provider.providerId}
        </AppText>
        {provider.lastGmailSync ? (
          <AppText variant="caption" tone="faint">
            Last Gmail sync: {formatSync(provider.lastGmailSync)}
          </AppText>
        ) : null}
      </View>
    </View>
  );
}

export function ConnectedProviders({ providers }: { providers: ConnectedProvider[] }) {
  return (
    <View style={styles.section}>
      <View style={styles.sectionHead}>
        <AppText variant="label" tone="faint">Connected providers</AppText>
        <AppText variant="caption" muted>{providers.length} connected</AppText>
      </View>
      <Card style={styles.card}>
        {providers.length === 0 ? (
          <AppText variant="caption" muted>No connected providers found.</AppText>
        ) : (
          providers.map((provider, index) => (
            <ProviderRow
              key={`${provider.providerType}-${provider.providerId}`}
              provider={provider}
              divider={index > 0}
            />
          ))
        )}
      </Card>
    </View>
  );
}

const styles = StyleSheet.create({
  section: {
    gap: spacing.sm
  },
  sectionHead: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginLeft: spacing.xs
  },
  card: {
    paddingVertical: spacing.xs
  },
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md,
    paddingVertical: spacing.md
  },
  rowDivider: {
    borderTopWidth: StyleSheet.hairlineWidth,
    borderTopColor: colors.border
  },
  avatar: {
    width: 38,
    height: 38,
    borderRadius: 19
  },
  main: {
    flex: 1,
    gap: 1
  },
  providerType: {
    textTransform: 'capitalize'
  }
});
