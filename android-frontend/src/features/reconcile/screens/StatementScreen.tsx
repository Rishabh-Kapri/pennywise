import { useMemo, useState } from 'react';
import { ActivityIndicator, Alert, Pressable, ScrollView, StyleSheet, ToastAndroid, View } from 'react-native';
import { useNavigation, useRoute, type RouteProp } from '@react-navigation/native';
import { AlertCircle, Eye, RotateCw, Trash2 } from 'lucide-react-native';
import { useAppSelector } from '../../../app/hooks';
import { AppText } from '../../../components/AppText';
import { Button } from '../../../components/Button';
import { Card } from '../../../components/Card';
import { PickerModal, type PickerOption } from '../../../components/Picker';
import { ProgressBar } from '../../../components/ProgressBar';
import { Screen } from '../../../components/Screen';
import { StackHeader } from '../../../components/StackHeader';
import { apiClient } from '../../../utils/api';
import type { RootStackParamList } from '../../../navigation/types';
import { colors, radii, spacing } from '../../../theme';
import { SectionView, type CategoryTarget } from '../components/SectionView';
import { StatementPdfPane, type PageRequest } from '../components/StatementPdfPane';
import { StatementIdentityView } from '../components/StatementIdentityView';
import { statementProgressLabel, useStatementReconciliation } from '../statementApi';
import { longDay, statementIdentity } from '../statementIdentity';

/** One statement compared with the ledger, section by section (the web's StatementReconcile detail). */
export function StatementScreen() {
  const navigation = useNavigation();
  const { params } = useRoute<RouteProp<RootStackParamList, 'Statement'>>();
  const accounts = useAppSelector((state) => state.accounts.allAccounts);
  const groups = useAppSelector((state) => state.categories.allCategoryGroups);
  const { data, isLoading, error, reload, setSectionAccount, createMissing, linkRow, unlinkRow, setRowEMI, addDifference } =
    useStatementReconciliation(params.id);

  // the PDF pinned above the rows, to check what was read against it
  const [comparing, setComparing] = useState(false);
  const [pageRequest, setPageRequest] = useState<PageRequest | null>(null);
  // One category picker for the whole screen; the section says what it's for.
  const [categoryTarget, setCategoryTarget] = useState<CategoryTarget | null>(null);

  const categoryOptions: PickerOption[] = useMemo(() => {
    const ordered = [...groups.filter((g) => g.isSystem), ...groups.filter((g) => !g.isSystem)];
    return ordered
      .filter((group) => group.name !== 'Hidden')
      .flatMap((group) =>
        group.categories
          .filter((c) => c.id && !c.hidden && !c.deleted)
          .map((c) => ({ id: c.id as string, label: c.name, group: group.isSystem ? 'Inflow' : group.name }))
      );
  }, [groups]);
  const categoryName = useMemo(
    () => Object.fromEntries(categoryOptions.map((o) => [o.id, o.label])) as Record<string, string>,
    [categoryOptions]
  );

  const imp = data?.import;
  // a bank's CSV export has no pages to show
  const hasPdf = !!imp?.hasFile && !/\.csv$/i.test(imp.fileName);
  const sections = data?.sections ?? [];

  const remove = () =>
    Alert.alert('Delete statement', `Delete ${imp?.fileName}? Transactions already added from it stay.`, [
      { text: 'Cancel', style: 'cancel' },
      {
        text: 'Delete',
        style: 'destructive',
        onPress: () => {
          void (async () => {
            try {
              await apiClient.delete(`statements/${params.id}`);
              navigation.goBack();
            } catch (err) {
              Alert.alert('Delete failed', err instanceof Error ? err.message : String(err));
            }
          })();
        }
      }
    ]);

  const retry = async () => {
    try {
      await apiClient.post(`statements/${params.id}/retry`, {});
      await reload();
    } catch (err) {
      Alert.alert('Retry failed', err instanceof Error ? err.message : String(err));
    }
  };

  const percent = imp?.pageCount ? Math.min(100, Math.round((imp.pagesProcessed / imp.pageCount) * 100)) : null;

  const body = (
    <>
      {isLoading && !data ? <ActivityIndicator color={colors.primary} style={styles.loading} /> : null}
      {error ? (
        <AppText variant="caption" tone="danger">
          {error}
        </AppText>
      ) : null}

      {imp?.status === 'processing' ? (
        <Card style={styles.state}>
          <ActivityIndicator color={colors.primary} />
          <AppText variant="heading">Reading your statement{percent !== null ? ` · ${percent}%` : ''}</AppText>
          {percent !== null ? <ProgressBar percent={Math.max(percent, 4)} /> : null}
          <AppText variant="caption" muted style={styles.center}>
            {statementProgressLabel(imp)}
          </AppText>
          <AppText variant="caption" tone="faint" style={styles.center}>
            This can take a few minutes. You can leave this screen; reading carries on.
          </AppText>
        </Card>
      ) : null}

      {imp?.status === 'failed' ? (
        <Card style={styles.state}>
          <AlertCircle size={28} color={colors.danger} />
          <AppText variant="heading">This statement couldn't be read</AppText>
          <AppText variant="caption" muted style={styles.center}>
            {imp.error || 'If the PDF is locked with a password, delete it and upload it again with the password.'}
          </AppText>
          {imp.jobId ? (
            <Button size="sm" onPress={() => void retry()}>
              <View style={styles.inline}>
                <RotateCw size={16} color={colors.onPrimary} />
                <AppText weight="semibold" tone="onPrimary">
                  Try again
                </AppText>
              </View>
            </Button>
          ) : null}
        </Card>
      ) : null}

      {imp?.status === 'ready' ? (
        <>
          {sections[0] ? (
            <StatementIdentityView
              large
              identity={statementIdentity(sections[0], accounts)}
              more={sections.length - 1}
            />
          ) : null}
          {imp.droppedRows > 0 ? (
            <View style={styles.warning}>
              <AlertCircle size={16} color={colors.warning} />
              <AppText variant="caption" style={styles.flex}>
                {imp.droppedRows} row{imp.droppedRows === 1 ? '' : 's'} had an unreadable date or amount and{' '}
                {imp.droppedRows === 1 ? 'was' : 'were'} left out.
              </AppText>
            </View>
          ) : null}
          {sections.length === 0 ? (
            <AppText muted>No transactions were found in this statement.</AppText>
          ) : null}
          {sections.map((section) => (
            <SectionView
              key={section.index}
              section={section}
              accounts={accounts}
              showIdentity={sections.length > 1}
              categoryName={categoryName}
              onPickCategory={setCategoryTarget}
              onAccountChange={(accountId) => setSectionAccount(section.index, accountId)}
              onCreate={async (items) => {
                const res = await createMissing(items);
                const parts: string[] = [];
                if (res.skipped.length) parts.push(`${res.skipped.length} already there`);
                if (res.failed.length) parts.push(`${res.failed.length} failed`);
                ToastAndroid.show(
                  `Added ${res.created.length} transaction${res.created.length === 1 ? '' : 's'}${parts.length ? ` (${parts.join(', ')})` : ''}`,
                  ToastAndroid.LONG
                );
              }}
              onLink={linkRow}
              onUnlink={unlinkRow}
              onSetEMI={setRowEMI}
              onAddDifference={addDifference}
              onShowPage={
                hasPdf
                  ? (page) => {
                      setComparing(true);
                      setPageRequest({ page, at: Date.now() });
                    }
                  : undefined
              }
            />
          ))}
        </>
      ) : null}
    </>
  );

  return (
    <Screen scroll={!comparing} style={styles.screen}>
      <StackHeader
        title="Statement"
        subtitle={imp ? `${imp.fileName} · uploaded ${longDay(imp.createdAt)}` : undefined}
        right={
          <View style={styles.headerActions}>
            {hasPdf ? (
              <Pressable
                accessibilityLabel={comparing ? 'Hide the PDF' : 'Compare with the PDF'}
                hitSlop={8}
                style={[styles.headerButton, comparing && styles.headerButtonOn]}
                onPress={() => setComparing(!comparing)}
              >
                <Eye size={18} color={comparing ? colors.primary : colors.text} />
              </Pressable>
            ) : null}
            <Pressable accessibilityLabel="Delete statement" hitSlop={8} style={styles.headerButton} onPress={remove}>
              <Trash2 size={18} color={colors.danger} />
            </Pressable>
          </View>
        }
      />

      {comparing && imp ? (
        <StatementPdfPane
          statementId={imp.id}
          fileName={imp.fileName}
          request={pageRequest}
          onClose={() => setComparing(false)}
        />
      ) : null}
      {comparing ? (
        <ScrollView style={styles.flex} contentContainerStyle={styles.compareBody} showsVerticalScrollIndicator={false}>
          {body}
        </ScrollView>
      ) : (
        body
      )}

      <PickerModal
        visible={categoryTarget !== null}
        title="Category"
        options={categoryOptions}
        selectedId={categoryTarget?.current || null}
        searchPlaceholder="Search categories"
        clearLabel="Uncategorized"
        onSelect={(id) => {
          categoryTarget?.onSelect(id ?? '');
          setCategoryTarget(null);
        }}
        onClose={() => setCategoryTarget(null)}
      />

    </Screen>
  );
}

const styles = StyleSheet.create({
  screen: { gap: spacing.lg },
  headerActions: { flexDirection: 'row', gap: spacing.sm },
  headerButtonOn: { backgroundColor: colors.primaryMuted },
  compareBody: { gap: spacing.lg, paddingBottom: spacing.xxl },
  headerButton: {
    width: 38,
    height: 38,
    borderRadius: 19,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.surfaceStrong
  },
  loading: { marginTop: spacing.xxl },
  state: { alignItems: 'center', gap: spacing.md, padding: spacing.xl },
  center: { textAlign: 'center' },
  inline: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm },
  warning: {
    flexDirection: 'row',
    gap: spacing.sm,
    alignItems: 'flex-start',
    padding: spacing.md,
    borderRadius: radii.md,
    backgroundColor: colors.warningMuted
  },
  flex: { flex: 1 }
});
