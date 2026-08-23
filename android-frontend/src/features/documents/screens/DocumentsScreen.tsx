import { useEffect, useMemo, useState } from 'react';
import {
  ActivityIndicator,
  Alert,
  FlatList,
  Image,
  Modal,
  Pressable,
  RefreshControl,
  ScrollView,
  StyleSheet,
  TextInput,
  ToastAndroid,
  View
} from 'react-native';
import { useNavigation } from '@react-navigation/native';
import { ChevronLeft, Download, Eye, FileText, Files, Search, Trash2, X } from 'lucide-react-native';
import { AppText } from '../../../components/AppText';
import { Card } from '../../../components/Card';
import { EmptyState } from '../../../components/EmptyState';
import { Screen } from '../../../components/Screen';
import { formatCurrency, formatShortDate } from '../../../utils/date';
import { colors, radii, spacing } from '../../../theme';
import { useDocumentLibrary } from '../hooks/useDocumentLibrary';
import { saveDocumentToDevice } from '../documentFile';
import { DocumentViewer } from '../components/DocumentViewer';
import {
  EMPTY_DOCUMENT_FILTERS,
  type DocumentFilters,
  type DocumentKind,
  type DocumentListItem
} from '../types';

const KIND_CHIPS: { id: DocumentKind; label: string }[] = [
  { id: '', label: 'All' },
  { id: 'image', label: 'Photos' },
  { id: 'pdf', label: 'PDFs' }
];

type PeriodId = 'all' | 'month' | 'quarter' | 'year';

const PERIOD_CHIPS: { id: PeriodId; label: string }[] = [
  { id: 'all', label: 'Any time' },
  { id: 'month', label: 'This month' },
  { id: 'quarter', label: 'Last 3 months' },
  { id: 'year', label: 'This year' }
];

function isoDate(date: Date): string {
  return date.toISOString().slice(0, 10);
}

/**
 * Period chips rather than two date fields: the app has no native date picker,
 * and typing a YYYY-MM-DD into a text input to filter a gallery is worse than
 * the four ranges anyone actually wants on a phone.
 */
function periodRange(period: PeriodId): { startDate: string; endDate: string } {
  const now = new Date();
  switch (period) {
    case 'month':
      return { startDate: isoDate(new Date(now.getFullYear(), now.getMonth(), 1)), endDate: '' };
    case 'quarter':
      return { startDate: isoDate(new Date(now.getFullYear(), now.getMonth() - 2, 1)), endDate: '' };
    case 'year':
      return { startDate: isoDate(new Date(now.getFullYear(), 0, 1)), endDate: '' };
    default:
      return { startDate: '', endDate: '' };
  }
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function DocumentTile({
  doc,
  thumbnail,
  onPress
}: {
  doc: DocumentListItem;
  thumbnail?: string;
  onPress: () => void;
}) {
  return (
    <Pressable
      style={({ pressed }) => [styles.tile, pressed && styles.pressed]}
      onPress={onPress}
      android_ripple={{ color: colors.surfaceTertiary }}
    >
      {thumbnail ? (
        <Image source={{ uri: thumbnail }} style={styles.thumb} />
      ) : (
        <View style={styles.thumbFallback}>
          <FileText size={26} color={colors.muted} />
          {doc.mimeType === 'application/pdf' ? (
            <AppText variant="caption" tone="faint" style={styles.pdfBadge}>
              PDF
            </AppText>
          ) : null}
        </View>
      )}
      <View style={styles.tileMeta}>
        <AppText weight="medium" numberOfLines={1} style={styles.tileTitle}>
          {doc.payeeName || doc.fileName}
        </AppText>
        <AppText variant="caption" muted numberOfLines={1} style={styles.tileSub}>
          {formatShortDate(doc.transactionDate)} · {formatCurrency(Math.abs(doc.transactionAmount))}
        </AppText>
      </View>
    </Pressable>
  );
}

/**
 * Detail sheet for one document: the full-size image when there is one, plus
 * the transaction it belongs to and a delete action.
 */
function DocumentDetail({
  doc,
  thumbnail,
  onClose,
  onDelete,
  onView
}: {
  doc: DocumentListItem;
  thumbnail?: string;
  onClose: () => void;
  onDelete: () => void;
  onView: () => void;
}) {
  const [isSaving, setIsSaving] = useState(false);

  const handleSave = async () => {
    if (isSaving) return;
    setIsSaving(true);
    try {
      const result = await saveDocumentToDevice(doc);
      if (result.status === 'saved') {
        ToastAndroid.show(`Saved ${result.fileName}`, ToastAndroid.SHORT);
      }
    } catch (err: unknown) {
      Alert.alert('Download failed', err instanceof Error ? err.message : 'Could not save this document.');
    } finally {
      setIsSaving(false);
    }
  };

  const confirmDelete = () => {
    Alert.alert('Delete document', `Delete ${doc.fileName}?`, [
      { text: 'Cancel', style: 'cancel' },
      {
        text: 'Delete',
        style: 'destructive',
        onPress: () => {
          onClose();
          onDelete();
        }
      }
    ]);
  };

  return (
    <Modal visible transparent animationType="slide" onRequestClose={onClose}>
      <View style={styles.sheetBackdrop}>
        <View style={styles.sheet}>
          <View style={styles.sheetHeader}>
            <AppText variant="heading" numberOfLines={1} style={styles.sheetTitle}>
              {doc.payeeName || doc.fileName}
            </AppText>
            <Pressable onPress={onClose} hitSlop={10} style={styles.sheetClose}>
              <X size={18} color={colors.text} />
            </Pressable>
          </View>

          <ScrollView showsVerticalScrollIndicator={false} contentContainerStyle={styles.sheetBody}>
            {thumbnail ? (
              <Image source={{ uri: thumbnail }} style={styles.sheetImage} resizeMode="contain" />
            ) : (
              <View style={styles.sheetPlaceholder}>
                <FileText size={34} color={colors.muted} />
                <AppText variant="caption" muted style={styles.sheetPlaceholderText}>
                  {doc.mimeType === 'application/pdf'
                    ? 'PDF pages are not rendered in the app yet. Download it to open in a PDF viewer.'
                    : 'No preview available for this file type.'}
                </AppText>
              </View>
            )}

            <Card style={styles.detailCard}>
              <DetailRow label="Transaction" value={formatShortDate(doc.transactionDate)} />
              <DetailRow label="Amount" value={formatCurrency(Math.abs(doc.transactionAmount))} />
              {doc.accountName ? <DetailRow label="Account" value={doc.accountName} /> : null}
              {doc.categoryName ? <DetailRow label="Category" value={doc.categoryName} /> : null}
              <DetailRow label="File" value={doc.fileName} />
              <DetailRow label="Size" value={formatSize(doc.sizeBytes)} />
            </Card>

            <View style={styles.actionRow}>
              <Pressable
                accessibilityRole="button"
                style={({ pressed }) => [styles.actionButton, styles.viewButton, pressed && styles.pressed]}
                onPress={onView}
              >
                <Eye size={16} color={colors.primary} />
                <AppText weight="semibold" tone="primary">
                  View
                </AppText>
              </Pressable>
              <Pressable
                accessibilityRole="button"
                disabled={isSaving}
                style={({ pressed }) => [
                  styles.actionButton,
                  styles.downloadButton,
                  pressed && styles.pressed,
                  isSaving && styles.pressed
                ]}
                onPress={() => void handleSave()}
              >
                {isSaving ? (
                  <ActivityIndicator size="small" color={colors.text} />
                ) : (
                  <Download size={16} color={colors.text} />
                )}
                <AppText weight="semibold">{isSaving ? 'Saving…' : 'Download'}</AppText>
              </Pressable>
            </View>

            <Pressable
              style={({ pressed }) => [styles.deleteButton, pressed && styles.pressed]}
              onPress={confirmDelete}
            >
              <Trash2 size={16} color={colors.danger} />
              <AppText weight="semibold" tone="danger">
                Delete document
              </AppText>
            </Pressable>
          </ScrollView>
        </View>
      </View>
    </Modal>
  );
}

function DetailRow({ label, value }: { label: string; value: string }) {
  return (
    <View style={styles.detailRow}>
      <AppText variant="caption" muted>
        {label}
      </AppText>
      <AppText variant="caption" weight="medium" numberOfLines={1} style={styles.detailValue}>
        {value}
      </AppText>
    </View>
  );
}

/** Every receipt in the budget in one grid, searchable and filterable. */
export function DocumentsScreen() {
  const navigation = useNavigation();
  const [filters, setFilters] = useState<DocumentFilters>(EMPTY_DOCUMENT_FILTERS);
  const [searchInput, setSearchInput] = useState('');
  const [period, setPeriod] = useState<PeriodId>('all');
  const [selected, setSelected] = useState<DocumentListItem | null>(null);
  const [viewing, setViewing] = useState<DocumentListItem | null>(null);

  // debounce so typing doesn't fire a request per keystroke
  useEffect(() => {
    const timer = setTimeout(() => {
      setFilters((prev) => (prev.search === searchInput ? prev : { ...prev, search: searchInput }));
    }, 350);
    return () => clearTimeout(timer);
  }, [searchInput]);

  const {
    documents,
    thumbnails,
    total,
    hasMore,
    isLoading,
    isLoadingMore,
    isRefreshing,
    error,
    loadMore,
    refresh,
    remove
  } = useDocumentLibrary(filters);

  const hasActiveFilters = useMemo(
    () => Boolean(filters.search || filters.kind || filters.startDate),
    [filters]
  );

  const applyPeriod = (id: PeriodId) => {
    setPeriod(id);
    setFilters((prev) => ({ ...prev, ...periodRange(id) }));
  };

  const clearFilters = () => {
    setSearchInput('');
    setPeriod('all');
    setFilters(EMPTY_DOCUMENT_FILTERS);
  };

  return (
    <Screen scroll={false} style={styles.screen}>
      <View style={styles.headerRow}>
        <Pressable
          accessibilityRole="button"
          accessibilityLabel="Go back"
          style={({ pressed }) => [styles.backButton, pressed && styles.pressed]}
          onPress={() => navigation.goBack()}
        >
          <ChevronLeft size={20} color={colors.text} />
        </Pressable>
        <View style={styles.headerText}>
          <AppText variant="title">Documents</AppText>
          <AppText variant="caption" muted>
            {isLoading ? 'Loading receipts…' : `${total} ${total === 1 ? 'receipt' : 'receipts'}`}
          </AppText>
        </View>
      </View>

      <View style={styles.searchBox}>
        <Search size={17} color={colors.faint} />
        <TextInput
          value={searchInput}
          onChangeText={setSearchInput}
          placeholder="Search by payee or file name"
          placeholderTextColor={colors.faint}
          style={styles.searchInput}
        />
        {searchInput ? (
          <Pressable onPress={() => setSearchInput('')} hitSlop={8}>
            <X size={15} color={colors.faint} />
          </Pressable>
        ) : null}
      </View>

      <ScrollView
        horizontal
        showsHorizontalScrollIndicator={false}
        contentContainerStyle={styles.chipRow}
        style={styles.chipScroll}
      >
        {KIND_CHIPS.map((chip) => {
          const active = filters.kind === chip.id;
          return (
            <Pressable
              key={chip.id || 'all'}
              style={[styles.chip, active && styles.chipActive]}
              onPress={() => setFilters((prev) => ({ ...prev, kind: chip.id }))}
            >
              <AppText variant="caption" weight="medium" tone={active ? 'primary' : 'muted'}>
                {chip.label}
              </AppText>
            </Pressable>
          );
        })}

        <View style={styles.chipDivider} />

        {PERIOD_CHIPS.map((chip) => {
          const active = period === chip.id;
          return (
            <Pressable
              key={chip.id}
              style={[styles.chip, active && styles.chipActive]}
              onPress={() => applyPeriod(chip.id)}
            >
              <AppText variant="caption" weight="medium" tone={active ? 'primary' : 'muted'}>
                {chip.label}
              </AppText>
            </Pressable>
          );
        })}
      </ScrollView>

      {error ? (
        <AppText variant="caption" tone="danger" style={styles.error}>
          {error}
        </AppText>
      ) : null}

      {isLoading ? (
        <View style={styles.loading}>
          <ActivityIndicator color={colors.primary} />
        </View>
      ) : (
        <FlatList
          data={documents}
          numColumns={2}
          keyExtractor={(item) => item.id}
          columnWrapperStyle={styles.column}
          contentContainerStyle={styles.listContent}
          showsVerticalScrollIndicator={false}
          refreshControl={
            <RefreshControl refreshing={isRefreshing} onRefresh={refresh} tintColor={colors.primary} />
          }
          ListEmptyComponent={
            <EmptyState
              icon={<Files size={26} color={colors.primary} />}
              title={hasActiveFilters ? 'No matches' : 'No receipts yet'}
              body={
                hasActiveFilters
                  ? 'Nothing matches these filters. Try clearing them.'
                  : 'Scan a bill from any transaction and it will show up here.'
              }
            />
          }
          ListFooterComponent={
            isLoadingMore ? (
              <View style={styles.footerLoading}>
                <ActivityIndicator color={colors.primary} />
              </View>
            ) : null
          }
          onEndReachedThreshold={0.4}
          onEndReached={() => {
            if (hasMore) loadMore();
          }}
          renderItem={({ item }) => (
            <DocumentTile
              doc={item}
              thumbnail={thumbnails[item.id]}
              onPress={() => setSelected(item)}
            />
          )}
        />
      )}

      {hasActiveFilters && documents.length === 0 && !isLoading ? (
        <Pressable style={({ pressed }) => [styles.clearButton, pressed && styles.pressed]} onPress={clearFilters}>
          <AppText variant="caption" weight="semibold" tone="primary">
            Clear filters
          </AppText>
        </Pressable>
      ) : null}

      {selected ? (
        <DocumentDetail
          doc={selected}
          thumbnail={thumbnails[selected.id]}
          onClose={() => setSelected(null)}
          onDelete={() => void remove(selected.id)}
          onView={() => setViewing(selected)}
        />
      ) : null}

      {viewing ? (
        <DocumentViewer
          doc={viewing}
          title={viewing.payeeName || viewing.fileName}
          onClose={() => setViewing(null)}
        />
      ) : null}
    </Screen>
  );
}

const styles = StyleSheet.create({
  screen: {
    gap: spacing.md
  },
  headerRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md
  },
  headerText: {
    flex: 1,
    gap: 1
  },
  backButton: {
    width: 38,
    height: 38,
    borderRadius: 19,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.surfaceStrong
  },
  searchBox: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    paddingHorizontal: spacing.lg,
    minHeight: 44,
    borderRadius: radii.full,
    backgroundColor: colors.surface
  },
  searchInput: {
    flex: 1,
    color: colors.text,
    fontSize: 14,
    paddingVertical: 0
  },
  chipScroll: {
    flexGrow: 0
  },
  chipRow: {
    gap: spacing.sm,
    alignItems: 'center',
    paddingRight: spacing.lg
  },
  chip: {
    paddingHorizontal: spacing.lg,
    paddingVertical: spacing.sm,
    borderRadius: radii.full,
    backgroundColor: colors.surface
  },
  chipActive: {
    backgroundColor: colors.primaryMuted
  },
  chipDivider: {
    width: StyleSheet.hairlineWidth,
    height: 20,
    backgroundColor: colors.border,
    marginHorizontal: spacing.xs
  },
  listContent: {
    paddingBottom: spacing.xxl,
    gap: spacing.md
  },
  column: {
    gap: spacing.md
  },
  tile: {
    flex: 1,
    borderRadius: radii.md,
    backgroundColor: colors.surface,
    overflow: 'hidden'
  },
  thumb: {
    width: '100%',
    aspectRatio: 4 / 3,
    backgroundColor: colors.surfaceStrong
  },
  thumbFallback: {
    width: '100%',
    aspectRatio: 4 / 3,
    alignItems: 'center',
    justifyContent: 'center',
    gap: spacing.xs,
    backgroundColor: colors.surfaceStrong
  },
  pdfBadge: {
    fontSize: 10,
    letterSpacing: 1
  },
  tileMeta: {
    padding: spacing.sm,
    gap: 1
  },
  tileTitle: {
    fontSize: 13
  },
  tileSub: {
    fontSize: 11
  },
  loading: {
    flex: 1,
    alignItems: 'center',
    justifyContent: 'center'
  },
  footerLoading: {
    paddingVertical: spacing.lg,
    alignItems: 'center'
  },
  error: {
    paddingHorizontal: spacing.xs
  },
  clearButton: {
    alignSelf: 'center',
    paddingHorizontal: spacing.xl,
    paddingVertical: spacing.md,
    borderRadius: radii.full,
    backgroundColor: colors.primaryMuted,
    marginBottom: spacing.lg
  },
  pressed: {
    opacity: 0.7
  },

  /* detail sheet */
  sheetBackdrop: {
    flex: 1,
    justifyContent: 'flex-end',
    backgroundColor: colors.scrim
  },
  sheet: {
    maxHeight: '88%',
    backgroundColor: colors.background,
    borderTopLeftRadius: radii.xl,
    borderTopRightRadius: radii.xl,
    paddingHorizontal: spacing.xl,
    paddingTop: spacing.lg,
    paddingBottom: spacing.xl
  },
  sheetHeader: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md,
    marginBottom: spacing.lg
  },
  sheetTitle: {
    flex: 1
  },
  sheetClose: {
    width: 32,
    height: 32,
    borderRadius: 16,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.surfaceStrong
  },
  sheetBody: {
    gap: spacing.lg,
    paddingBottom: spacing.lg
  },
  sheetImage: {
    width: '100%',
    height: 280,
    borderRadius: radii.md,
    backgroundColor: colors.surface
  },
  sheetPlaceholder: {
    alignItems: 'center',
    justifyContent: 'center',
    gap: spacing.sm,
    height: 180,
    borderRadius: radii.md,
    paddingHorizontal: spacing.xl,
    backgroundColor: colors.surface
  },
  sheetPlaceholderText: {
    textAlign: 'center'
  },
  detailCard: {
    gap: spacing.sm
  },
  detailRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: spacing.lg
  },
  detailValue: {
    flexShrink: 1,
    textAlign: 'right'
  },
  actionRow: {
    flexDirection: 'row',
    gap: spacing.md
  },
  actionButton: {
    flex: 1,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    gap: spacing.sm,
    minHeight: 46,
    borderRadius: radii.full
  },
  viewButton: {
    backgroundColor: colors.primaryMuted
  },
  downloadButton: {
    backgroundColor: colors.surfaceStrong
  },
  deleteButton: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    gap: spacing.sm,
    minHeight: 46,
    borderRadius: radii.full,
    backgroundColor: colors.dangerMuted
  }
});
