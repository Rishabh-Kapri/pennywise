import { useMemo, useState } from 'react';
import {
  ActivityIndicator,
  Alert,
  FlatList,
  Pressable,
  RefreshControl,
  StyleSheet,
  TextInput,
  ToastAndroid,
  View
} from 'react-native';
import { useNavigation } from '@react-navigation/native';
import type { NativeStackNavigationProp } from '@react-navigation/native-stack';
import * as DocumentPicker from 'expo-document-picker';
import { AlertCircle, FileText, RotateCw, Upload } from 'lucide-react-native';
import { useAppSelector } from '../../../app/hooks';
import { AppText } from '../../../components/AppText';
import { Button } from '../../../components/Button';
import { Card } from '../../../components/Card';
import { EmptyState } from '../../../components/EmptyState';
import { PickerField, PickerModal, type PickerOption } from '../../../components/Picker';
import { ProgressBar } from '../../../components/ProgressBar';
import { Screen } from '../../../components/Screen';
import { Sheet } from '../../../components/Sheet';
import { StackHeader } from '../../../components/StackHeader';
import type { RootStackParamList } from '../../../navigation/types';
import { colors, radii, spacing } from '../../../theme';
import { StatementIdentityView } from '../components/StatementIdentityView';
import { statementProgressLabel, useStatementList, type PickedPdf } from '../statementApi';
import { longDay, statementIdentity } from '../statementIdentity';
import type { StatementImport } from '../types';

const MAX_BYTES = 20 * 1024 * 1024;

function percentOf(imp: StatementImport): number | null {
  if (!imp.pageCount) return null;
  return Math.min(100, Math.round((imp.pagesProcessed / imp.pageCount) * 100));
}

/**
 * Settings → Statements: upload bank and card statement PDFs, then open one to
 * compare it with the ledger (the web's Settings → Reconcile).
 */
export function ReconcileScreen() {
  const navigation = useNavigation<NativeStackNavigationProp<RootStackParamList>>();
  const accounts = useAppSelector((state) => state.accounts.allAccounts);
  const { imports, isLoading, isRefreshing, error, refresh, upload, remove, retry } = useStatementList();

  const [staged, setStaged] = useState<PickedPdf[]>([]);
  const [accountId, setAccountId] = useState('');
  const [password, setPassword] = useState('');
  const [pickingAccount, setPickingAccount] = useState(false);
  const [uploading, setUploading] = useState(false);

  const openAccounts = useMemo(() => accounts.filter((a) => a.id && !a.closed && !a.deleted), [accounts]);
  const accountOptions: PickerOption[] = useMemo(
    () => openAccounts.map((a) => ({ id: a.id as string, label: a.name, sublabel: a.suffix ? `••${a.suffix}` : undefined })),
    [openAccounts]
  );

  const pick = async () => {
    const result = await DocumentPicker.getDocumentAsync({
      // file managers label a CSV any of these; the name decides
      type: ['application/pdf', 'text/csv', 'text/comma-separated-values', 'text/plain'],
      copyToCacheDirectory: true,
      multiple: true
    });
    if (result.canceled) return;
    const all = result.assets ?? [];
    const assets = all.filter((a) => /\.(pdf|csv)$/i.test(a.name));
    if (assets.length < all.length) Alert.alert('Only PDFs and CSVs', 'Other files were left out.');
    const tooBig = assets.filter((a) => (a.size ?? 0) > MAX_BYTES);
    if (tooBig.length) Alert.alert('Too large', `${tooBig.map((a) => a.name).join(', ')} is over 20 MB.`);
    const files = assets
      .filter((a) => (a.size ?? 0) <= MAX_BYTES)
      .map((a) => ({ uri: a.uri, name: a.name, type: /\.csv$/i.test(a.name) ? 'text/csv' : 'application/pdf' }));
    if (files.length) setStaged(files);
  };

  const closeUpload = () => {
    setStaged([]);
    setPassword('');
  };

  const submit = async () => {
    setUploading(true);
    try {
      const outcomes = await upload(staged, accountId, password);
      const failed = outcomes.filter((o) => !o.ok);
      if (failed.length) {
        Alert.alert(
          failed.length === outcomes.length ? 'Upload failed' : 'Some uploads failed',
          failed.map((o) => `${o.fileName}: ${o.message ?? 'failed'}`).join('\n')
        );
      } else {
        ToastAndroid.show(
          outcomes.length === 1 ? 'Statement uploaded. Reading it now.' : `${outcomes.length} statements uploaded`,
          ToastAndroid.SHORT
        );
      }
      closeUpload();
    } finally {
      setUploading(false);
    }
  };

  const confirmRemove = (imp: StatementImport) =>
    Alert.alert('Delete statement', `Delete ${imp.fileName}? Transactions already added from it stay.`, [
      { text: 'Cancel', style: 'cancel' },
      {
        text: 'Delete',
        style: 'destructive',
        onPress: () => void remove(imp.id).catch((err: unknown) => Alert.alert('Delete failed', String(err)))
      }
    ]);

  const renderItem = ({ item: imp }: { item: StatementImport }) => {
    const percent = percentOf(imp);
    return (
      <Pressable
        style={({ pressed }) => [styles.item, pressed && styles.pressed]}
        onPress={() => navigation.navigate('Statement', { id: imp.id })}
        onLongPress={() => confirmRemove(imp)}
      >
        <Card style={styles.itemCard}>
          {imp.status === 'ready' && imp.summary?.length ? (
            <StatementIdentityView identity={statementIdentity(imp.summary[0], accounts)} more={imp.summary.length - 1}>
              {imp.rowCount} rows · {imp.fileName}
            </StatementIdentityView>
          ) : (
            <View style={styles.fileRow}>
              <View style={[styles.fileIcon, imp.status === 'failed' && styles.fileIconFailed]}>
                {imp.status === 'failed' ? (
                  <AlertCircle size={18} color={colors.danger} />
                ) : (
                  <FileText size={18} color={colors.primary} />
                )}
              </View>
              <View style={styles.fileText}>
                <AppText weight="semibold" numberOfLines={1}>
                  {imp.fileName}
                </AppText>
                {imp.status === 'processing' ? (
                  <>
                    <AppText variant="caption" muted>
                      {statementProgressLabel(imp)}
                    </AppText>
                    <View style={styles.progress}>
                      {percent === null ? (
                        <ActivityIndicator size="small" color={colors.primary} />
                      ) : (
                        <ProgressBar percent={Math.max(percent, 4)} />
                      )}
                    </View>
                  </>
                ) : imp.status === 'failed' ? (
                  <AppText variant="caption" tone="danger">
                    Couldn't be read
                  </AppText>
                ) : (
                  <AppText variant="caption" muted>
                    {imp.rowCount} rows · uploaded {longDay(imp.createdAt)}
                  </AppText>
                )}
              </View>
              {imp.status === 'failed' && imp.jobId ? (
                <Pressable
                  accessibilityLabel={`Retry reading ${imp.fileName}`}
                  hitSlop={8}
                  onPress={() => void retry(imp.id).catch((err: unknown) => Alert.alert('Retry failed', String(err)))}
                >
                  <RotateCw size={18} color={colors.muted} />
                </Pressable>
              ) : null}
            </View>
          )}
        </Card>
      </Pressable>
    );
  };

  return (
    <Screen scroll={false} style={styles.screen}>
      <StackHeader
        title="Statements"
        subtitle="Compare bank and card statements with your transactions"
        right={
          <Pressable
            accessibilityRole="button"
            accessibilityLabel="Upload a statement"
            style={({ pressed }) => [styles.uploadButton, pressed && styles.pressed]}
            onPress={() => void pick()}
          >
            <Upload size={18} color={colors.onPrimary} />
          </Pressable>
        }
      />

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
          data={imports}
          keyExtractor={(item) => item.id}
          renderItem={renderItem}
          contentContainerStyle={styles.list}
          showsVerticalScrollIndicator={false}
          refreshControl={<RefreshControl refreshing={isRefreshing} onRefresh={refresh} tintColor={colors.primary} />}
          ListEmptyComponent={
            <EmptyState
              icon={<FileText size={26} color={colors.primary} />}
              title="No statements yet"
              body="Upload a bank or card statement PDF, or a CSV exported from your bank. Pennywise reads it and shows which rows match your transactions and which are missing."
            />
          }
          ListFooterComponent={
            imports.length ? (
              <AppText variant="caption" tone="faint" style={styles.hint}>
                Long-press a statement to delete it.
              </AppText>
            ) : null
          }
        />
      )}

      <Sheet
        visible={staged.length > 0}
        title={staged.length === 1 ? 'Upload statement' : `Upload ${staged.length} statements`}
        subtitle={staged.map((f) => f.name).join(', ')}
        onClose={closeUpload}
        footer={
          <Button disabled={uploading} onPress={() => void submit()}>
            {uploading ? <ActivityIndicator color={colors.onPrimary} /> : 'Upload'}
          </Button>
        }
      >
        <View style={styles.form}>
          <PickerField
            label="Account"
            value={accountOptions.find((o) => o.id === accountId)?.label}
            placeholder="I'll choose later"
            onPress={() => setPickingAccount(true)}
          />
          <View style={styles.field}>
            <AppText variant="label" tone="faint">
              PDF password
            </AppText>
            <TextInput
              value={password}
              onChangeText={setPassword}
              secureTextEntry
              autoCapitalize="none"
              autoCorrect={false}
              placeholder="Only if it's locked"
              placeholderTextColor={colors.faint}
              style={styles.input}
            />
          </View>
        </View>
      </Sheet>

      <PickerModal
        visible={pickingAccount}
        title="Account"
        options={accountOptions}
        selectedId={accountId || null}
        searchPlaceholder="Search accounts"
        clearLabel="I'll choose later"
        onSelect={(id) => {
          setAccountId(id ?? '');
          setPickingAccount(false);
        }}
        onClose={() => setPickingAccount(false)}
      />
    </Screen>
  );
}

const styles = StyleSheet.create({
  screen: { gap: spacing.lg },
  uploadButton: {
    width: 38,
    height: 38,
    borderRadius: 19,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.primary
  },
  pressed: { opacity: 0.75 },
  error: { marginTop: -spacing.sm },
  loading: { paddingTop: spacing.xxl, alignItems: 'center' },
  list: { gap: spacing.sm, paddingBottom: spacing.xxl },
  item: {},
  itemCard: { padding: spacing.lg },
  fileRow: { flexDirection: 'row', alignItems: 'center', gap: spacing.md },
  fileIcon: {
    width: 40,
    height: 40,
    borderRadius: radii.sm,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.primaryMuted
  },
  fileIconFailed: { backgroundColor: colors.dangerMuted },
  fileText: { flex: 1, minWidth: 0, gap: 2 },
  progress: { marginTop: spacing.xs },
  hint: { textAlign: 'center', marginTop: spacing.md },
  form: { gap: spacing.lg, paddingBottom: spacing.md },
  field: { gap: spacing.xs },
  input: {
    minHeight: 48,
    paddingHorizontal: spacing.md,
    borderRadius: radii.md,
    backgroundColor: colors.surfaceStrong,
    color: colors.text,
    fontSize: 15
  }
});
