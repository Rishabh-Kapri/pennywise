import { useCallback, useEffect, useState } from 'react';
import { Alert, Image, Pressable, ScrollView, StyleSheet, View } from 'react-native';
import { Camera, FileText, Images, Trash2 } from 'lucide-react-native';
import * as ImagePicker from 'expo-image-picker';
import * as DocumentPicker from 'expo-document-picker';
import * as FileSystem from 'expo-file-system/legacy';
import { AppText } from '../../../components/AppText';
import { colors, radii, spacing, tints } from '../../../theme';
import { apiClient } from '../../../utils/api';
import type { TransactionDocument } from '../types';

type PickedFile = { uri: string; name: string; type: string };

function formatSize(bytes: number) {
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

/**
 * Receipts/documents list + capture for a transaction. Image previews are
 * downloaded with auth headers into the cache directory (plain <Image src>
 * can't send Authorization).
 */
export function TransactionAttachments({ transactionId }: { transactionId: string }) {
  const [documents, setDocuments] = useState<TransactionDocument[]>([]);
  const [thumbs, setThumbs] = useState<Record<string, string>>({});
  const [isBusy, setIsBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadThumb = useCallback(async (doc: TransactionDocument) => {
    if (!doc.mimeType.startsWith('image/')) return;
    try {
      const { url, headers } = apiClient.getAuthorizedRequest(`documents/${doc.id}/content`);
      const ext = doc.fileName.includes('.') ? doc.fileName.slice(doc.fileName.lastIndexOf('.')) : '.jpg';
      const target = `${FileSystem.cacheDirectory}receipt-${doc.id}${ext}`;
      const info = await FileSystem.getInfoAsync(target);
      if (!info.exists) {
        const result = await FileSystem.downloadAsync(url, target, { headers });
        if (result.status !== 200) return;
      }
      setThumbs((prev) => ({ ...prev, [doc.id]: target }));
    } catch {
      // preview failures are non-fatal
    }
  }, []);

  const refresh = useCallback(async () => {
    try {
      setError(null);
      const docs = await apiClient.get<TransactionDocument[]>(`transactions/${transactionId}/documents`);
      setDocuments(docs ?? []);
      for (const doc of docs ?? []) void loadThumb(doc);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load documents');
    }
  }, [transactionId, loadThumb]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const upload = async (file: PickedFile) => {
    setIsBusy(true);
    setError(null);
    try {
      const form = new FormData();
      // React Native FormData file part
      form.append('file', { uri: file.uri, name: file.name, type: file.type } as unknown as Blob);
      const doc = await apiClient.postForm<TransactionDocument>(`transactions/${transactionId}/documents`, form);
      setDocuments((docs) => [...docs, doc]);
      void loadThumb(doc);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to upload');
    } finally {
      setIsBusy(false);
    }
  };

  const pickFromCamera = async () => {
    const { status } = await ImagePicker.requestCameraPermissionsAsync();
    if (status !== 'granted') return;
    const result = await ImagePicker.launchCameraAsync({ quality: 0.7 });
    const asset = result.assets?.[0];
    if (!result.canceled && asset) {
      await upload({ uri: asset.uri, name: asset.fileName ?? 'receipt.jpg', type: asset.mimeType ?? 'image/jpeg' });
    }
  };

  const pickFromGallery = async () => {
    const result = await ImagePicker.launchImageLibraryAsync({ quality: 0.7 });
    const asset = result.assets?.[0];
    if (!result.canceled && asset) {
      await upload({ uri: asset.uri, name: asset.fileName ?? 'receipt.jpg', type: asset.mimeType ?? 'image/jpeg' });
    }
  };

  const pickDocument = async () => {
    const result = await DocumentPicker.getDocumentAsync({
      type: ['application/pdf', 'image/*'],
      copyToCacheDirectory: true
    });
    const asset = result.assets?.[0];
    if (!result.canceled && asset) {
      await upload({ uri: asset.uri, name: asset.name, type: asset.mimeType ?? 'application/pdf' });
    }
  };

  const remove = (doc: TransactionDocument) => {
    Alert.alert('Delete document', `Delete ${doc.fileName}?`, [
      { text: 'Cancel', style: 'cancel' },
      {
        text: 'Delete',
        style: 'destructive',
        onPress: () => {
          void (async () => {
            try {
              await apiClient.delete(`documents/${doc.id}`);
              setDocuments((docs) => docs.filter((item) => item.id !== doc.id));
            } catch (err) {
              setError(err instanceof Error ? err.message : 'Failed to delete');
            }
          })();
        }
      }
    ]);
  };

  return (
    <View style={styles.container}>
      <AppText weight="semibold">Receipts</AppText>

      {documents.length > 0 && (
        <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={styles.docRow}>
          {documents.map((doc) => (
            <View key={doc.id} style={styles.docTile}>
              {thumbs[doc.id] ? (
                <Image source={{ uri: thumbs[doc.id] }} style={styles.docThumb} />
              ) : (
                <View style={styles.docIconBox}>
                  <FileText size={26} color={colors.muted} />
                </View>
              )}
              <AppText muted numberOfLines={1} style={styles.docName}>
                {doc.fileName}
              </AppText>
              <AppText muted style={styles.docSize}>
                {formatSize(doc.sizeBytes)}
              </AppText>
              <Pressable style={styles.docDelete} onPress={() => remove(doc)} hitSlop={8}>
                <Trash2 size={14} color={colors.budgetNegative} />
              </Pressable>
            </View>
          ))}
        </ScrollView>
      )}

      <View style={styles.actionsRow}>
        <Pressable
          style={[styles.actionBtn, isBusy && styles.actionBtnDisabled]}
          onPress={() => void pickFromCamera()}
          disabled={isBusy}>
          <Camera size={14} color={colors.muted} />
          <AppText style={styles.actionLabel}>Camera</AppText>
        </Pressable>
        <Pressable
          style={[styles.actionBtn, isBusy && styles.actionBtnDisabled]}
          onPress={() => void pickFromGallery()}
          disabled={isBusy}>
          <Images size={14} color={colors.muted} />
          <AppText style={styles.actionLabel}>Gallery</AppText>
        </Pressable>
        <Pressable
          style={[styles.actionBtn, isBusy && styles.actionBtnDisabled]}
          onPress={() => void pickDocument()}
          disabled={isBusy}>
          <FileText size={14} color={colors.muted} />
          <AppText style={styles.actionLabel}>File</AppText>
        </Pressable>
      </View>

      {isBusy && <AppText muted>Uploading…</AppText>}
      {error && <AppText style={styles.errorText}>{error}</AppText>}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    gap: spacing.sm
  },
  docRow: {
    gap: spacing.sm
  },
  docTile: {
    width: 92,
    gap: 2
  },
  docThumb: {
    width: 92,
    height: 92,
    borderRadius: radii.sm,
    backgroundColor: colors.surface
  },
  docIconBox: {
    width: 92,
    height: 92,
    borderRadius: radii.sm,
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: colors.borderMuted,
    backgroundColor: colors.surface,
    alignItems: 'center',
    justifyContent: 'center'
  },
  docName: {
    fontSize: 11
  },
  docSize: {
    fontSize: 10
  },
  docDelete: {
    position: 'absolute',
    top: 4,
    right: 4,
    width: 24,
    height: 24,
    borderRadius: 12,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: tints.scrim
  },
  actionsRow: {
    flexDirection: 'row',
    gap: spacing.sm
  },
  // neutral tinted pills, matching the location actions
  actionBtn: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    gap: spacing.xs,
    paddingHorizontal: spacing.md,
    minHeight: 36,
    borderRadius: 999,
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: tints.neutralBorder,
    backgroundColor: tints.neutralFill
  },
  actionBtnDisabled: {
    opacity: 0.55
  },
  actionLabel: {
    fontSize: 12,
    fontWeight: '700',
    letterSpacing: 0.3,
    color: colors.muted
  },
  errorText: {
    color: colors.budgetNegative
  }
});
