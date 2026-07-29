import { useState } from 'react';
import { Linking, Pressable, StyleSheet, View } from 'react-native';
import { Crosshair, MapPin, Trash2 } from 'lucide-react-native';
import * as Location from 'expo-location';
import { AppText } from '../../../components/AppText';
import { colors, spacing, tints } from '../../../theme';
import { useAppDispatch } from '../../../app/hooks';
import { updateTransactionLocation } from '../store/transactionSlice';
import { LocationSource } from '../types';

export type DraftLocation = {
  lat: number | null;
  lng: number | null;
  name: string | null;
  source: LocationSource | null;
};

export async function getCurrentCoords(): Promise<{ lat: number; lng: number } | null> {
  const { status } = await Location.requestForegroundPermissionsAsync();
  if (status !== 'granted') return null;
  try {
    const position = await Location.getCurrentPositionAsync({ accuracy: Location.Accuracy.Balanced });
    return { lat: position.coords.latitude, lng: position.coords.longitude };
  } catch {
    return null;
  }
}

/**
 * Location line inside the transaction editor. For saved transactions the
 * buttons PATCH immediately; for unsaved drafts changes go through onDraftChange
 * and ride along with the create payload.
 */
export function TransactionLocationRow({
  transactionId,
  location,
  onDraftChange
}: {
  transactionId?: string;
  location: DraftLocation;
  onDraftChange: (location: DraftLocation) => void;
}) {
  const dispatch = useAppDispatch();
  const [isLocating, setIsLocating] = useState(false);

  const hasLocation = location.lat != null && location.lng != null;

  const apply = (next: DraftLocation) => {
    onDraftChange(next);
    if (transactionId) {
      void dispatch(
        updateTransactionLocation({
          id: transactionId,
          location: { lat: next.lat, lng: next.lng, source: next.source ?? LocationSource.MANUAL }
        })
      );
    }
  };

  const useCurrentLocation = async () => {
    setIsLocating(true);
    const coords = await getCurrentCoords();
    setIsLocating(false);
    if (!coords) return;
    apply({ lat: coords.lat, lng: coords.lng, name: null, source: LocationSource.MANUAL });
  };

  const openInMaps = () => {
    if (!hasLocation) return;
    const label = encodeURIComponent(location.name ?? 'Transaction');
    void Linking.openURL(`geo:${location.lat},${location.lng}?q=${location.lat},${location.lng}(${label})`);
  };

  return (
    <View style={styles.container}>
      <View style={styles.labelRow}>
        <AppText weight="semibold">Location</AppText>
        {location.source === LocationSource.AUTO && (
          <View style={styles.autoBadge}>
            <AppText style={styles.autoBadgeText}>AUTO</AppText>
          </View>
        )}
      </View>

      {hasLocation && (
        <Pressable onPress={openInMaps}>
          <AppText muted numberOfLines={2}>
            {location.name ?? `${location.lat?.toFixed(5)}, ${location.lng?.toFixed(5)}`}
            {'  '}
            <AppText style={styles.mapsLink}>Open in Maps</AppText>
          </AppText>
        </Pressable>
      )}

      <View style={styles.actionsRow}>
        <Pressable
          style={[styles.actionBtn, styles.actionBtnPrimary, isLocating && styles.actionBtnDisabled]}
          onPress={() => void useCurrentLocation()}
          disabled={isLocating}>
          <Crosshair size={14} color={colors.primaryLight} />
          <AppText style={[styles.actionLabel, styles.labelPrimary]}>
            {isLocating ? 'Locating…' : hasLocation ? 'Update' : 'Use current location'}
          </AppText>
        </Pressable>
        {hasLocation && (
          <Pressable
            style={[styles.actionBtn, styles.actionBtnDanger]}
            onPress={() => apply({ lat: null, lng: null, name: null, source: null })}>
            <Trash2 size={14} color={colors.budgetNegative} />
            <AppText style={[styles.actionLabel, styles.labelDanger]}>Remove</AppText>
          </Pressable>
        )}
        {!hasLocation && (
          <View style={styles.hintRow}>
            <MapPin size={14} color={colors.muted} />
            <AppText muted style={styles.hintText}>
              No location attached
            </AppText>
          </View>
        )}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    gap: spacing.sm
  },
  labelRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm
  },
  autoBadge: {
    paddingHorizontal: spacing.sm,
    paddingVertical: 2,
    borderRadius: 999,
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: tints.primaryBorder,
    backgroundColor: tints.primaryFill
  },
  autoBadgeText: {
    fontSize: 10,
    lineHeight: 14,
    fontWeight: '700',
    letterSpacing: 0.6,
    color: colors.primaryLight
  },
  mapsLink: {
    color: colors.primary
  },
  actionsRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm
  },
  // tinted pills, matching the web panel's status/location action styling
  actionBtn: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    gap: spacing.xs,
    paddingHorizontal: spacing.md,
    minHeight: 36,
    borderRadius: 999,
    borderWidth: StyleSheet.hairlineWidth
  },
  actionBtnPrimary: {
    borderColor: tints.primaryBorder,
    backgroundColor: tints.primaryFill
  },
  actionBtnNeutral: {
    borderColor: tints.neutralBorder,
    backgroundColor: tints.neutralFill
  },
  actionBtnDanger: {
    borderColor: tints.dangerBorder,
    backgroundColor: tints.dangerFill
  },
  actionBtnDisabled: {
    opacity: 0.55
  },
  actionLabel: {
    fontSize: 12,
    fontWeight: '700',
    letterSpacing: 0.3
  },
  labelPrimary: {
    color: colors.primaryLight
  },
  labelNeutral: {
    color: colors.muted
  },
  labelDanger: {
    color: colors.budgetNegative
  },
  hintRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.xs
  },
  hintText: {
    fontSize: 12
  }
});
