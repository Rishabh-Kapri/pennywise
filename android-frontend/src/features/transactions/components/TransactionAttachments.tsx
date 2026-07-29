import { useCallback, useEffect, useState } from 'react';
import { Alert, Image, Pressable, ScrollView, StyleSheet, View } from 'react-native';
import { Camera, FileText, Images, Trash2 } from 'lucide-react-native';
import * as ImagePicker from 'expo-image-picker';
import * as DocumentPicker from 'expo-document-picker';
import * as FileSystem from 'expo-file-system/legacy';
import { AppText } from '../../../components/AppText';
import { colors, radii, spacing } from '../../../theme';
import { apiClient } from '../../../utils/api';
import type { TransactionDocument } from '../types';

type PickedFile = { uri: string; name: string; type: string };

function formatSize(bytes: number) {
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

/** Alert-based yes/no, so the capture loop can await the user's choice. */
function confirmAsync(title: string, message: string, confirmLabel: string): Promise<boolean> {
  return new Promise((resolve) => {
    Alert.alert(
      title,
      message,
      [
        { text: 'Done', style: 'cancel', onPress: () => resolve(false) },
        { text: confirmLabel, onPress: () => resolve(true) }
      ],
      { cancelable: true, onDismiss: () => resolve(false) }
    );
  });
}

function assetToFile(
  asset: { uri: string; fileName?: string | null; mimeType?: string | null },
  index: number
): PickedFile {
  return {
    uri: asset.uri,
    name: asset.fileName ?? `receipt-${Date.now()}-${index + 1}.jpg`,
    type: asset.mimeType ?? 'image/jpeg'
  };
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
  const [progress, setProgress] = useState<{ done: number; total: number } | null>(null);
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

  /**
   * Uploads a batch one file at a time — the API takes a single `file` part per
   * request, which keeps the per-file size cap and mime sniffing meaningful.
   * A failure part-way keeps the already-uploaded pages.
   */
  const uploadMany = async (files: PickedFile[]) => {
    if (files.length === 0) return;
    setIsBusy(true);
    setError(null);
    setProgress({ done: 0, total: files.length });
    const failures: string[] = [];

    for (const [index, file] of files.entries()) {
      try {
        const form = new FormData();
        // React Native FormData file part
        form.append('file', { uri: file.uri, name: file.name, type: file.type } as unknown as Blob);
        const doc = await apiClient.postForm<TransactionDocument>(`transactions/${transactionId}/documents`, form);
        setDocuments((docs) => [...docs, doc]);
        void loadThumb(doc);
      } catch (err) {
        failures.push(file.name);
        console.log('[receipts] upload failed', file.name, err);
      }
      setProgress({ done: index + 1, total: files.length });
    }

    if (failures.length > 0) {
      setError(
        failures.length === files.length
          ? 'Failed to upload'
          : `Uploaded ${files.length - failures.length} of ${files.length}; ${failures.length} failed`
      );
    }
    setProgress(null);
    setIsBusy(false);
  };

  /**
   * Multi-shot capture: the camera reopens after each frame so a stack of bills
   * can be scanned in one go, then the whole batch uploads together.
   */
  const pickFromCamera = async () => {
    const { status } = await ImagePicker.requestCameraPermissionsAsync();
    if (status !== 'granted') return;

    const captured: PickedFile[] = [];
    for (;;) {
      const result = await ImagePicker.launchCameraAsync({ quality: 0.7 });
      const asset = result.assets?.[0];
      if (result.canceled || !asset) break;

      captured.push(assetToFile(asset, captured.length));
      const more = await confirmAsync(
        'Scan another?',
        `${captured.length} ${captured.length === 1 ? 'bill' : 'bills'} captured.`,
        'Scan another'
      );
      if (!more) break;
    }

    await uploadMany(captured);
  };

  const pickFromGallery = async () => {
    const result = await ImagePicker.launchImageLibraryAsync({
      quality: 0.7,
      allowsMultipleSelection: true,
      selectionLimit: 10
    });
    if (result.canceled) return;
    await uploadMany((result.assets ?? []).map(assetToFile));
  };

  const pickDocument = async () => {
    const result = await DocumentPicker.getDocumentAsync({
      type: ['application/pdf', 'image/*'],
      copyToCacheDirectory: true,
      multiple: true
    });
    if (result.canceled) return;
    await uploadMany(
      (result.assets ?? []).map((asset) => ({
        uri: asset.uri,
        name: asset.name,
        type: asset.mimeType ?? 'application/pdf'
      }))
    );
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
                <Trash2 size={14} color={colors.danger} />
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
          <Camera size={16} color={colors.text} />
          <AppText weight="medium">Camera</AppText>
        </Pressable>
        <Pressable
          style={[styles.actionBtn, isBusy && styles.actionBtnDisabled]}
          onPress={() => void pickFromGallery()}
          disabled={isBusy}>
          <Images size={16} color={colors.text} />
          <AppText weight="medium">Gallery</AppText>
        </Pressable>
        <Pressable
          style={[styles.actionBtn, isBusy && styles.actionBtnDisabled]}
          onPress={() => void pickDocument()}
          disabled={isBusy}>
          <FileText size={16} color={colors.text} />
          <AppText weight="medium">File</AppText>
        </Pressable>
      </View>

      {isBusy && (
        <AppText muted>{progress ? `Uploading ${progress.done} of ${progress.total}…` : 'Uploading…'}</AppText>
      )}
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
    borderColor: colors.border,
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
    backgroundColor: colors.scrim
  },
  actionsRow: {
    flexDirection: 'row',
    gap: spacing.sm
  },
  // same bordered-rect treatment as the location actions / Button "secondary"
  actionBtn: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    gap: spacing.xs,
    paddingHorizontal: spacing.md,
    minHeight: 42,
    borderRadius: radii.sm,
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: colors.border,
    backgroundColor: colors.surface
  },
  actionBtnDisabled: {
    opacity: 0.6
  },
  errorText: {
    color: colors.danger
  }
});
