import { useCallback, useState } from 'react';
import { Image, Pressable, ScrollView, StyleSheet, View } from 'react-native';
import { useFocusEffect, useNavigation } from '@react-navigation/native';
import type { NativeStackNavigationProp } from '@react-navigation/native-stack';
import { ChevronRight, FileText } from 'lucide-react-native';
import * as FileSystem from 'expo-file-system/legacy';
import { AppText } from '../../../components/AppText';
import { Card } from '../../../components/Card';
import type { RootStackParamList } from '../../../navigation/types';
import { apiClient } from '../../../utils/api';
import { colors, radii, spacing } from '../../../theme';
import type { DocumentLibraryResponse, DocumentListItem } from '../types';

const PREVIEW_COUNT = 6;

/**
 * Compact "recent receipts" strip for the dashboard, and the discoverable
 * entry point into the full library — scanning happens on the phone, so the
 * newest receipts are worth surfacing where the user already lands.
 */
export function RecentDocumentsCard() {
  const navigation = useNavigation<NativeStackNavigationProp<RootStackParamList>>();
  const [documents, setDocuments] = useState<DocumentListItem[]>([]);
  const [thumbnails, setThumbnails] = useState<Record<string, string>>({});
  const [total, setTotal] = useState(0);
  const [hasLoaded, setHasLoaded] = useState(false);

  const load = useCallback(async () => {
    try {
      const res = await apiClient.get<DocumentLibraryResponse>(`documents?limit=${PREVIEW_COUNT}&offset=0`);
      const page = res?.data ?? [];
      setDocuments(page);
      setTotal(res?.total ?? 0);

      for (const doc of page) {
        if (!doc.mimeType.startsWith('image/')) continue;
        try {
          const { url, headers } = apiClient.getAuthorizedRequest(`documents/${doc.id}/content`);
          const ext = doc.fileName.includes('.') ? doc.fileName.slice(doc.fileName.lastIndexOf('.')) : '.jpg';
          const target = `${FileSystem.cacheDirectory}receipt-${doc.id}${ext}`;
          const info = await FileSystem.getInfoAsync(target);
          if (!info.exists) {
            const result = await FileSystem.downloadAsync(url, target, { headers });
            if (result.status !== 200) continue;
          }
          setThumbnails((prev) => ({ ...prev, [doc.id]: target }));
        } catch {
          // a missing thumbnail is non-fatal
        }
      }
    } catch {
      // the dashboard should still render if the library is unreachable
      setDocuments([]);
    } finally {
      setHasLoaded(true);
    }
  }, []);

  // Runs on mount and on every refocus, so a receipt scanned since the last
  // visit shows up without a manual refresh.
  useFocusEffect(
    useCallback(() => {
      void load();
    }, [load])
  );

  // Stay out of the way until there is something to show: an empty card on a
  // fresh install is noise on the most-visited screen.
  if (!hasLoaded || documents.length === 0) return null;

  return (
    <Card style={styles.card}>
      <Pressable style={styles.headerRow} onPress={() => navigation.navigate('Documents')}>
        <View style={styles.headerText}>
          <AppText weight="semibold">Recent receipts</AppText>
          <AppText variant="caption" muted>
            {total} {total === 1 ? 'document' : 'documents'} saved
          </AppText>
        </View>
        <ChevronRight size={18} color={colors.muted} />
      </Pressable>

      <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={styles.strip}>
        {documents.map((doc) => (
          <Pressable
            key={doc.id}
            style={({ pressed }) => [styles.tile, pressed && styles.pressed]}
            onPress={() => navigation.navigate('Documents')}
          >
            {thumbnails[doc.id] ? (
              <Image source={{ uri: thumbnails[doc.id] }} style={styles.thumb} />
            ) : (
              <View style={styles.thumbFallback}>
                <FileText size={20} color={colors.muted} />
              </View>
            )}
            <AppText variant="caption" muted numberOfLines={1} style={styles.tileLabel}>
              {doc.payeeName || doc.fileName}
            </AppText>
          </Pressable>
        ))}
      </ScrollView>
    </Card>
  );
}

const styles = StyleSheet.create({
  card: {
    gap: spacing.md
  },
  headerRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: spacing.md
  },
  headerText: {
    flex: 1,
    gap: 1
  },
  strip: {
    gap: spacing.sm
  },
  tile: {
    width: 78,
    gap: spacing.xs
  },
  thumb: {
    width: 78,
    height: 78,
    borderRadius: radii.md,
    backgroundColor: colors.surfaceStrong
  },
  thumbFallback: {
    width: 78,
    height: 78,
    borderRadius: radii.md,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.surfaceStrong
  },
  tileLabel: {
    fontSize: 10
  },
  pressed: {
    opacity: 0.7
  }
});
