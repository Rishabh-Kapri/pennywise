import { useEffect, useState } from 'react';
import { ActivityIndicator, Image, Modal, Pressable, ScrollView, StyleSheet, View } from 'react-native';
import Pdf from 'react-native-pdf';
import { X } from 'lucide-react-native';
import { AppText } from '../../../components/AppText';
import { colors, radii, spacing } from '../../../theme';
import { ensureDocumentCached, type DocumentFileRef } from '../documentFile';

/**
 * Full-screen, in-app view of one document. The body is read from the app's
 * private cache — nothing is written to the gallery or Downloads by opening
 * this, which is what separates viewing from the explicit save action.
 *
 * Images render through <Image>; everything else goes to <Pdf>, which
 * rasterises natively from the same cached file. Passing it the local path
 * rather than the API URL keeps the request out of the native layer, so the
 * body is fetched once, with our auth headers, by ensureDocumentCached.
 */
export function DocumentViewer({
  doc,
  title,
  onClose
}: {
  doc: DocumentFileRef;
  title: string;
  onClose: () => void;
}) {
  const [localUri, setLocalUri] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [pageCount, setPageCount] = useState(0);
  const [page, setPage] = useState(1);

  const isImage = doc.mimeType.startsWith('image/');

  useEffect(() => {
    let cancelled = false;
    ensureDocumentCached(doc)
      .then((uri) => {
        if (!cancelled) setLocalUri(uri);
      })
      .catch((err: unknown) => {
        if (!cancelled) setError(err instanceof Error ? err.message : 'Could not open this document');
      });
    return () => {
      cancelled = true;
    };
  }, [doc]);

  return (
    <Modal visible transparent={false} animationType="fade" onRequestClose={onClose} statusBarTranslucent>
      <View style={styles.container}>
        <View style={styles.header}>
          <AppText variant="heading" numberOfLines={1} style={styles.title}>
            {title}
          </AppText>
          <Pressable
            accessibilityRole="button"
            accessibilityLabel="Close viewer"
            onPress={onClose}
            hitSlop={10}
            style={({ pressed }) => [styles.close, pressed && styles.pressed]}
          >
            <X size={18} color={colors.text} />
          </Pressable>
        </View>

        {error ? (
          <View style={styles.centered}>
            <AppText variant="caption" tone="danger" style={styles.centeredText}>
              {error}
            </AppText>
          </View>
        ) : !localUri ? (
          <View style={styles.centered}>
            <ActivityIndicator color={colors.primary} />
            <AppText variant="caption" muted>Opening…</AppText>
          </View>
        ) : isImage ? (
          <ScrollView
            style={styles.imageScroll}
            contentContainerStyle={styles.imageContent}
            maximumZoomScale={4}
            minimumZoomScale={1}
            centerContent
          >
            <Image source={{ uri: localUri }} style={styles.image} resizeMode="contain" />
          </ScrollView>
        ) : (
          <>
            <Pdf
              source={{ uri: localUri }}
              style={styles.pdf}
              trustAllCerts={false}
              onLoadComplete={(pages) => setPageCount(pages)}
              onPageChanged={(current) => setPage(current)}
              onError={(err: unknown) =>
                setError(err instanceof Error ? err.message : 'This document could not be opened')
              }
            />
            {pageCount > 1 ? (
              <View style={styles.pageBadge}>
                <AppText variant="caption" muted tabular>
                  {page} / {pageCount}
                </AppText>
              </View>
            ) : null}
          </>
        )}
      </View>
    </Modal>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: colors.background
  },
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md,
    paddingHorizontal: spacing.xl,
    paddingTop: spacing.xxl,
    paddingBottom: spacing.md
  },
  title: {
    flex: 1
  },
  close: {
    width: 34,
    height: 34,
    borderRadius: 17,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.surfaceStrong
  },
  centered: {
    flex: 1,
    alignItems: 'center',
    justifyContent: 'center',
    gap: spacing.md,
    paddingHorizontal: spacing.xxl
  },
  centeredText: {
    textAlign: 'center'
  },
  pdf: {
    flex: 1,
    width: '100%',
    backgroundColor: colors.background
  },
  pageBadge: {
    position: 'absolute',
    alignSelf: 'center',
    bottom: spacing.xl,
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.xs,
    borderRadius: radii.full,
    backgroundColor: colors.scrim
  },
  imageScroll: {
    flex: 1
  },
  imageContent: {
    flexGrow: 1,
    justifyContent: 'center'
  },
  image: {
    width: '100%',
    height: '100%'
  },
  pressed: {
    opacity: 0.7
  }
});
