import { useMemo, useState } from 'react';
import { ActivityIndicator, Alert, Pressable, ScrollView, StyleSheet, TextInput, View } from 'react-native';
import { AlertCircle, Check, CheckCircle2, FileSearch, Link2, Square, SquareCheck, Unlink } from 'lucide-react-native';
import { AppText } from '../../../components/AppText';
import { Button } from '../../../components/Button';
import { Card } from '../../../components/Card';
import { PickerField, PickerModal, type PickerOption } from '../../../components/Picker';
import { Sheet } from '../../../components/Sheet';
import type { Account } from '../../accounts/types';
import { colors, radii, spacing } from '../../../theme';
import { accountEnding, formatPeriod, longDay, money, shortDay, signedMoney } from '../statementIdentity';
import {
  signedAmount,
  type StatementCreateItem,
  type StatementDifferenceRequest,
  type StatementLedgerTxn,
  type StatementRow,
  type StatementSectionReconciliation
} from '../types';

type Tab = 'missing' | 'matched' | 'ledgerOnly' | 'covered';

/** What the screen's shared category picker is open for. */
export interface CategoryTarget {
  current: string;
  onSelect: (categoryId: string) => void;
}

interface RowDraft {
  selected: boolean;
  payeeName: string;
  categoryId: string;
}

function Amount({ value, strong }: { value: number; strong?: boolean }) {
  return (
    <AppText tabular weight={strong ? 'semibold' : 'medium'} tone={value < 0 ? 'default' : 'success'}>
      {signedMoney(value)}
    </AppText>
  );
}

/** "p. 3": opens the PDF beside the rows at the page this row was read from. */
function PageButton({ row, onShowPage }: { row: StatementRow; onShowPage?: (page: number) => void }) {
  if (!onShowPage || !row.page) return null;
  return (
    <Pressable
      accessibilityLabel={`Show page ${row.page} of the PDF`}
      hitSlop={8}
      style={({ pressed }) => [styles.pageButton, pressed && styles.pressed]}
      onPress={() => onShowPage(row.page as number)}
    >
      <FileSearch size={12} color={colors.primary} />
      <AppText variant="caption" tone="primary" style={styles.badgeText}>
        p. {row.page}
      </AppText>
    </Pressable>
  );
}

/** "+12 pts", or "−40 pts" for points taken back (a refund, a redemption). */
function PointsBadge({ row }: { row: StatementRow }) {
  if (!row.rewardPoints) return null;
  const n = row.rewardPoints;
  return <Badge>{`${n > 0 ? '+' : '−'}${new Intl.NumberFormat('en-IN').format(Math.abs(n))} pts`}</Badge>;
}

/** "not an EMI · Undo" on a row the user moved out of Covered by EMI. */
function NotEMIUndo({ row, onSetEMI }: { row: StatementRow; onSetEMI: (rowId: string, emi: boolean | null) => Promise<void> }) {
  if (row.emiOverride !== false) return null;
  return (
    <Pressable
      accessibilityLabel="Undo not an EMI"
      hitSlop={8}
      style={({ pressed }) => [styles.pageButton, pressed && styles.pressed]}
      onPress={() => {
        onSetEMI(row.id, null).catch((err: unknown) =>
          Alert.alert("Couldn't undo", err instanceof Error ? err.message : String(err))
        );
      }}
    >
      <AppText variant="caption" tone="primary" style={styles.badgeText}>
        not an EMI · Undo
      </AppText>
    </Pressable>
  );
}

function Badge({ children, tone = 'muted' }: { children: string; tone?: 'muted' | 'warning' }) {
  return (
    <View style={[styles.badge, tone === 'warning' && styles.badgeWarning]}>
      <AppText variant="caption" tone={tone === 'warning' ? 'default' : 'muted'} style={styles.badgeText}>
        {children}
      </AppText>
    </View>
  );
}

/**
 * One account's block of a statement: what the card says is due, whether the
 * rows add up, and the rows sorted into missing (on the statement, not in
 * Pennywise), matched, and not on the statement. Missing rows can be added as
 * transactions or linked by hand to one that's already there.
 */
export function SectionView({
  section,
  accounts,
  showIdentity,
  categoryName,
  onPickCategory,
  onAccountChange,
  onCreate,
  onLink,
  onUnlink,
  onSetEMI,
  onAddDifference,
  onShowPage
}: {
  section: StatementSectionReconciliation;
  accounts: Account[];
  /** name the account and period here: the screen header does when there's one section */
  showIdentity: boolean;
  categoryName: Record<string, string>;
  onPickCategory: (target: CategoryTarget) => void;
  onAccountChange: (accountId: string | null) => Promise<void>;
  onCreate: (items: StatementCreateItem[]) => Promise<void>;
  /** add some of the account's transactions to a row's links */
  onLink: (rowId: string, transactionIds: string[]) => Promise<void>;
  onUnlink: (rowId: string) => Promise<void>;
  /** move a row in or out of Covered by EMI; null goes back to what its text says */
  onSetEMI: (rowId: string, emi: boolean | null) => Promise<void>;
  onAddDifference: (rowId: string, body: StatementDifferenceRequest) => Promise<void>;
  /** set when the statement's PDF is kept: rows can open it at their page */
  onShowPage?: (page: number) => void;
}) {
  const [tab, setTab] = useState<Tab>(section.missing.length > 0 ? 'missing' : 'matched');
  const [drafts, setDrafts] = useState<Record<string, RowDraft>>({});
  const [busy, setBusy] = useState(false);
  const [pickingAccount, setPickingAccount] = useState(false);
  // the row being linked, and what its linked transactions already add up to
  const [linking, setLinking] = useState<{ row: StatementRow; linked: number } | null>(null);

  const accountOptions: PickerOption[] = useMemo(
    () =>
      accounts
        .filter((a) => a.id && !a.closed && !a.deleted)
        .map((a) => ({
          id: a.id as string,
          label: a.name,
          sublabel: a.suffix ? `••${a.suffix}` : undefined
        })),
    [accounts]
  );
  const accountName = accountOptions.find((o) => o.id === section.accountId)?.label ?? section.accountName;

  const draftFor = (row: StatementRow): RowDraft =>
    drafts[row.id] ?? {
      selected: false,
      payeeName: row.payee,
      categoryId: row.suggestedCategoryId ?? ''
    };
  const updateDraft = (row: StatementRow, patch: Partial<RowDraft>) =>
    setDrafts((prev) => ({
      ...prev,
      [row.id]: { ...draftFor(row), ...patch }
    }));
  const selectedRows = section.missing.filter((r) => draftFor(r).selected);

  const run = async (action: () => Promise<void>, failed: string) => {
    setBusy(true);
    try {
      await action();
    } catch (err) {
      Alert.alert(failed, err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  const createSelected = () =>
    run(async () => {
      await onCreate(
        selectedRows.map((r) => {
          const d = draftFor(r);
          return {
            rowId: r.id,
            payeeName: d.payeeName.trim() || undefined,
            categoryId: d.categoryId || undefined
          };
        })
      );
      setDrafts({});
    }, "Couldn't add the transactions");

  const ending = accountEnding(section.accountNumber);
  const period = formatPeriod(section.periodStart, section.periodEnd);
  const facts = [
    showIdentity && [section.accountLabel, ending && `ending ${ending}`].filter(Boolean).join(' '),
    showIdentity && period,
    `${section.rowCount} rows`,
    section.currency
  ]
    .filter(Boolean)
    .join(' · ');

  const accountPicker = (
    <PickerModal
      visible={pickingAccount}
      title="Account"
      options={accountOptions}
      selectedId={section.accountId ?? null}
      searchPlaceholder="Search accounts"
      clearLabel={section.accountId ? 'No account' : undefined}
      onSelect={(id) => {
        setPickingAccount(false);
        if ((id ?? null) !== (section.accountId ?? null)) {
          void run(() => onAccountChange(id ?? null), "Couldn't set the account");
        }
      }}
      onClose={() => setPickingAccount(false)}
    />
  );

  if (!section.accountId) {
    return (
      <Card style={styles.state}>
        <AppText variant="heading">Which account is this statement for?</AppText>
        <AppText variant="caption" muted style={styles.center}>
          {[section.accountLabel, ending && `ending ${ending}`, period].filter(Boolean).join(' · ')}
        </AppText>
        {ending ? (
          <AppText variant="caption" tone="faint" style={styles.center}>
            No account ends in {ending}. Set an account's last digits to {ending} and future statements will be matched to it
            automatically.
          </AppText>
        ) : null}
        <View style={styles.fullWidth}>
          <PickerField label="Account" value={null} placeholder="Choose an account" onPress={() => setPickingAccount(true)} />
        </View>
        {accountPicker}
      </Card>
    );
  }

  const tabs: { id: Tab; label: string; count: number }[] = [
    { id: 'missing', label: 'Missing', count: section.missing.length },
    { id: 'matched', label: 'Matched', count: section.matched.length },
    {
      id: 'ledgerOnly',
      label: 'Not on statement',
      count: section.ledgerOnly.length
    },
    ...(section.covered.length
      ? [
          {
            id: 'covered' as Tab,
            label: 'Covered by EMI',
            count: section.covered.length
          }
        ]
      : [])
  ];

  return (
    <View style={styles.section}>
      <View style={styles.sectionHead}>
        <AppText variant="caption" muted style={styles.flex}>
          {facts}
        </AppText>
      </View>
      <PickerField label="Account" value={accountName} onPress={() => setPickingAccount(true)} />
      {accountPicker}

      <CardSummary section={section} />

      {section.balanceCheck ? (
        section.balanceCheck.ok ? (
          <View style={[styles.banner, styles.bannerOk]}>
            <CheckCircle2 size={16} color={colors.success} />
            <AppText variant="caption" style={styles.flex}>
              The rows add up to the statement's closing balance of {money(section.balanceCheck.reported)}.
            </AppText>
          </View>
        ) : (
          <View style={[styles.banner, styles.bannerWarn]}>
            <AlertCircle size={16} color={colors.warning} />
            <AppText variant="caption" style={styles.flex}>
              The rows add up to {money(section.balanceCheck.computed)}, but the statement closes at{' '}
              {money(section.balanceCheck.reported)} (off by {money(Math.abs(section.balanceCheck.difference))}). A row may have
              been missed or misread.
            </AppText>
          </View>
        )
      ) : null}

      <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={styles.tabs}>
        {tabs.map((t) => {
          const active = tab === t.id;
          return (
            <Pressable key={t.id} style={[styles.tab, active && styles.tabActive]} onPress={() => setTab(t.id)}>
              <AppText variant="caption" weight="semibold" tone={active ? 'primary' : 'muted'}>
                {t.label}
              </AppText>
              <View style={[styles.count, t.id === 'missing' && t.count > 0 && styles.countAttention]}>
                <AppText variant="caption" tabular style={styles.countText}>
                  {t.count}
                </AppText>
              </View>
            </Pressable>
          );
        })}
      </ScrollView>

      {tab === 'missing' ? (
        section.missing.length === 0 ? (
          <Empty title="Nothing missing" body="Every row on the statement is in Pennywise." />
        ) : (
          <View style={styles.list}>
            <AppText variant="caption" muted>
              On the statement but not in Pennywise. Tick the ones to add; they're created unapproved. Or link one to a
              transaction that's already there.
            </AppText>
            {selectedRows.length > 0 ? (
              <Button disabled={busy} onPress={() => void createSelected()}>
                {busy ? (
                  <ActivityIndicator color={colors.onPrimary} />
                ) : selectedRows.length > 1 ? (
                  `Add ${selectedRows.length} transactions`
                ) : (
                  'Add transaction'
                )}
              </Button>
            ) : null}
            {section.missing.map((row) => {
              const d = draftFor(row);
              return (
                <Card key={row.id} style={[styles.rowCard, d.selected && styles.rowCardSelected]}>
                  <View style={styles.rowTop}>
                    <Pressable
                      accessibilityRole="checkbox"
                      accessibilityState={{ checked: d.selected }}
                      accessibilityLabel={`Select ${row.description}`}
                      hitSlop={8}
                      style={[styles.check, d.selected && styles.checkOn]}
                      onPress={() => updateDraft(row, { selected: !d.selected })}
                    >
                      {d.selected ? <Check size={14} color={colors.onPrimary} /> : null}
                    </Pressable>
                    <View style={styles.flex}>
                      <AppText weight="medium" numberOfLines={2}>
                        {row.description}
                      </AppText>
                      <AppText variant="caption" muted>
                        {shortDay(row.date)}
                        {row.reference ? ` · Ref ${row.reference}` : ''}
                        {row.chargeKind ? ` · ${row.chargeKind}` : ''}
                        {row.directionCorrected ? ' · direction fixed' : ''}
                      </AppText>
                      <View style={styles.badges}>
                        <PageButton row={row} onShowPage={onShowPage} />
                        <PointsBadge row={row} />
                        <NotEMIUndo row={row} onSetEMI={onSetEMI} />
                      </View>
                    </View>
                    <Amount value={signedAmount(row)} strong />
                  </View>
                  <View style={styles.rowFields}>
                    <TextInput
                      value={d.payeeName}
                      onChangeText={(payeeName) => updateDraft(row, { payeeName })}
                      placeholder="Payee"
                      placeholderTextColor={colors.faint}
                      style={[styles.input, styles.flex]}
                    />
                    <Pressable
                      style={({ pressed }) => [styles.categoryButton, pressed && styles.pressed]}
                      onPress={() =>
                        onPickCategory({
                          current: d.categoryId,
                          onSelect: (categoryId) => updateDraft(row, { categoryId })
                        })
                      }
                    >
                      <AppText numberOfLines={1} tone={d.categoryId ? 'default' : 'faint'}>
                        {categoryName[d.categoryId] ?? 'Uncategorized'}
                      </AppText>
                    </Pressable>
                    <Pressable
                      accessibilityLabel={`Link ${row.description} to a transaction`}
                      style={({ pressed }) => [styles.iconButton, pressed && styles.pressed]}
                      onPress={() => setLinking({ row, linked: 0 })}
                    >
                      <Link2 size={18} color={colors.primary} />
                    </Pressable>
                  </View>
                </Card>
              );
            })}
          </View>
        )
      ) : null}

      {tab === 'matched' ? (
        section.matched.length === 0 ? (
          <Empty title="Nothing matched yet" body="No statement row lines up with a transaction in this account." />
        ) : (
          <View style={styles.list}>
            {section.matched.map((m) => (
              <Card key={m.row.id} style={styles.rowCard}>
                <View style={styles.rowTop}>
                  <View style={styles.flex}>
                    <AppText weight="medium" numberOfLines={2}>
                      {m.row.description}
                    </AppText>
                    <AppText variant="caption" muted numberOfLines={2}>
                      {shortDay(m.row.date)} · {m.transaction.payeeName || m.transaction.note || 'Transaction'}
                      {m.others?.map((o) => ` + ${o.payeeName || 'Transaction'} ${money(o.amount)}`).join('')}
                    </AppText>
                    <View style={styles.badges}>
                      <PageButton row={m.row} onShowPage={onShowPage} />
                      <PointsBadge row={m.row} />
                      <NotEMIUndo row={m.row} onSetEMI={onSetEMI} />
                      {m.linked ? <Badge>linked by you</Badge> : null}
                      {m.dateDiffDays > 0 ? (
                        <Badge>{`${m.dateDiffDays} day${m.dateDiffDays === 1 ? '' : 's'} apart`}</Badge>
                      ) : null}
                      {m.transaction.status === 'UNAPPROVED' ? <Badge>unapproved</Badge> : null}
                      {m.kind === 'amount_differs' && !m.linked ? (
                        <Badge tone="warning">{`Pennywise has ${money(m.transaction.amount)}`}</Badge>
                      ) : null}
                    </View>
                  </View>
                  <View style={styles.amountCol}>
                    <Amount value={signedAmount(m.row)} strong />
                    {m.linked && m.difference ? (
                      <Pressable
                        accessibilityLabel={`Link more transactions to ${m.row.description}`}
                        hitSlop={8}
                        disabled={busy}
                        onPress={() =>
                          setLinking({
                            row: m.row,
                            linked: [m.transaction, ...(m.others ?? [])].reduce((sum, t) => sum + t.amount, 0)
                          })
                        }
                      >
                        <Link2 size={17} color={colors.primary} />
                      </Pressable>
                    ) : null}
                    {m.linked ? (
                      <Pressable
                        accessibilityLabel={`Unlink ${m.row.description}`}
                        hitSlop={8}
                        disabled={busy}
                        onPress={() => void run(() => onUnlink(m.row.id), "Couldn't unlink")}
                      >
                        <Unlink size={17} color={colors.muted} />
                      </Pressable>
                    ) : null}
                  </View>
                </View>
                {m.linked && m.difference ? (
                  <DifferenceForm
                    difference={m.difference}
                    categoryName={categoryName}
                    onPickCategory={onPickCategory}
                    onAdd={(body) => onAddDifference(m.row.id, body)}
                  />
                ) : null}
              </Card>
            ))}
          </View>
        )
      ) : null}

      {tab === 'ledgerOnly' ? (
        section.ledgerOnly.length === 0 ? (
          <Empty title="Nothing extra" body="Every transaction in this period appears on the statement." />
        ) : (
          <View style={styles.list}>
            <AppText variant="caption" muted>
              In {accountName ?? 'this account'} for the statement's period but not on the statement. Each could be a duplicate, a
              pending charge, or filed under the wrong account.
            </AppText>
            {section.ledgerOnly.map((t) => (
              <LedgerRow key={t.id} txn={t} />
            ))}
          </View>
        )
      ) : null}

      {tab === 'covered' ? (
        <View style={styles.list}>
          <AppText variant="caption" muted>
            EMI instalments on this statement. They pay off purchases already in your budget, so they aren't missing. If one isn't
            an instalment, tap Not an EMI and it's matched like any purchase. If you track an EMI as its own transaction, link the
            row to it.
          </AppText>
          {section.covered.map((row) => (
            <Card key={row.id} style={styles.rowCard}>
              <View style={styles.rowTop}>
                <View style={styles.flex}>
                  <AppText weight="medium" numberOfLines={2}>
                    {row.description}
                  </AppText>
                  <AppText variant="caption" muted>
                    {shortDay(row.date)}
                  </AppText>
                  <View style={styles.badges}>
                    <PageButton row={row} onShowPage={onShowPage} />
                    <PointsBadge row={row} />
                  </View>
                </View>
                <View style={styles.amountCol}>
                  <Amount value={signedAmount(row)} />
                  <Button
                    size="sm"
                    variant="secondary"
                    disabled={busy}
                    onPress={() => void run(() => onSetEMI(row.id, false), "Couldn't move it")}
                  >
                    Not an EMI
                  </Button>
                  <Pressable
                    accessibilityLabel={`Link ${row.description} to a transaction`}
                    hitSlop={8}
                    disabled={busy}
                    onPress={() => setLinking({ row, linked: 0 })}
                  >
                    <Link2 size={18} color={colors.primary} />
                  </Pressable>
                </View>
              </View>
            </Card>
          ))}
        </View>
      ) : null}

      <Sheet
        visible={linking !== null}
        title="Link to transactions"
        subtitle={linking ? `${linking.row.description} · ${signedMoney(signedAmount(linking.row))}` : undefined}
        size="tall"
        onClose={() => setLinking(null)}
      >
        {linking ? (
          <LinkCandidates
            row={linking.row}
            linked={linking.linked}
            candidates={section.ledgerOnly}
            onLink={async (ids) => {
              await onLink(linking.row.id, ids);
              setLinking(null);
              setTab('matched');
            }}
          />
        ) : null}
      </Sheet>
    </View>
  );
}

function Empty({ title, body }: { title: string; body: string }) {
  return (
    <View style={styles.empty}>
      <AppText weight="semibold">{title}</AppText>
      <AppText variant="caption" muted style={styles.center}>
        {body}
      </AppText>
    </View>
  );
}

function LedgerRow({ txn }: { txn: StatementLedgerTxn }) {
  return (
    <Card style={styles.rowCard}>
      <View style={styles.rowTop}>
        <View style={styles.flex}>
          <AppText weight="medium" numberOfLines={1}>
            {txn.payeeName || txn.note || 'Transaction'}
          </AppText>
          <AppText variant="caption" muted numberOfLines={1}>
            {shortDay(txn.date)}
            {txn.categoryName ? ` · ${txn.categoryName}` : ''}
            {txn.status === 'UNAPPROVED' ? ' · unapproved' : ''}
          </AppText>
        </View>
        <Amount value={txn.amount} />
      </View>
    </Card>
  );
}

/**
 * The account's transactions not on the statement, oldest first like the
 * statement. Several can be picked: a payment made in parts, or a row that
 * bundles several.
 */
function LinkCandidates({
  row,
  linked,
  candidates,
  onLink
}: {
  row: StatementRow;
  /** what the row's linked transactions already add up to */
  linked: number;
  candidates: StatementLedgerTxn[];
  onLink: (transactionIds: string[]) => Promise<void>;
}) {
  const [picked, setPicked] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const sorted = [...candidates].sort((a, b) => a.date.slice(0, 10).localeCompare(b.date.slice(0, 10)));
  const cents = (v: number) => Math.round(v * 100) / 100;
  const left = cents(
    signedAmount(row) - linked - candidates.filter((t) => picked.includes(t.id)).reduce((sum, t) => sum + t.amount, 0)
  );

  if (sorted.length === 0) {
    return (
      <AppText muted style={styles.center}>
        Every transaction in this account for the statement's period is already matched.
      </AppText>
    );
  }
  const link = () => {
    setBusy(true);
    onLink(picked).catch((err: unknown) => {
      setBusy(false);
      Alert.alert("Couldn't link them", err instanceof Error ? err.message : String(err));
    });
  };
  return (
    <View style={styles.flex}>
      <ScrollView contentContainerStyle={styles.list} keyboardShouldPersistTaps="handled">
        {sorted.map((t) => {
          const on = picked.includes(t.id);
          const apart = cents(left - t.amount);
          return (
            <Pressable
              key={t.id}
              accessibilityRole="checkbox"
              accessibilityState={{ checked: on }}
              disabled={busy}
              style={({ pressed }) => [styles.candidate, on && styles.candidateOn, pressed && styles.pressed]}
              onPress={() => setPicked((p) => (on ? p.filter((x) => x !== t.id) : [...p, t.id]))}
            >
              {on ? <SquareCheck size={18} color={colors.primary} /> : <Square size={18} color={colors.muted} />}
              <View style={styles.flex}>
                <AppText weight="medium" numberOfLines={1}>
                  {t.payeeName || t.note || 'Transaction'}
                </AppText>
                <AppText variant="caption" muted numberOfLines={1}>
                  {shortDay(t.date)}
                  {t.categoryName ? ` · ${t.categoryName}` : ''}
                </AppText>
              </View>
              <View style={styles.amountCol}>
                <Amount value={t.amount} />
                {!on && apart !== 0 ? (
                  <AppText variant="caption" tone="faint">
                    {money(Math.abs(apart))} apart
                  </AppText>
                ) : null}
              </View>
            </Pressable>
          );
        })}
      </ScrollView>
      <View style={styles.linkFooter}>
        <AppText variant="caption" muted style={styles.flex}>
          {picked.length === 0
            ? 'Pick one or more. A different total is fine: you can book the difference after linking.'
            : left === 0
              ? `${picked.length} picked · adds up exactly`
              : `${picked.length} picked · ${money(Math.abs(left))} ${left > 0 ? 'short' : 'over'}`}
        </AppText>
        <Button size="sm" disabled={busy || picked.length === 0} onPress={link}>
          {busy ? <ActivityIndicator color={colors.text} /> : `Link${picked.length > 1 ? ` ${picked.length}` : ''}`}
        </Button>
      </View>
    </View>
  );
}

/** Books what a linked row's transactions are short of it, as its own unapproved transaction. */
function DifferenceForm({
  difference,
  categoryName,
  onPickCategory,
  onAdd
}: {
  difference: number;
  categoryName: Record<string, string>;
  onPickCategory: (target: CategoryTarget) => void;
  onAdd: (body: StatementDifferenceRequest) => Promise<void>;
}) {
  const [payeeName, setPayeeName] = useState('');
  const [categoryId, setCategoryId] = useState('');
  const [busy, setBusy] = useState(false);

  const add = async () => {
    setBusy(true);
    try {
      await onAdd({
        payeeName: payeeName.trim(),
        categoryId: categoryId || undefined
      });
    } catch (err) {
      Alert.alert("Couldn't add the difference", err instanceof Error ? err.message : String(err));
      setBusy(false);
    }
  };

  return (
    <View style={styles.difference}>
      <AppText variant="caption">
        {difference > 0 ? 'Statement has' : 'Pennywise has'}{' '}
        <AppText variant="caption" weight="semibold" tabular>
          {money(Math.abs(difference))}
        </AppText>{' '}
        more. Add it as its own transaction:
      </AppText>
      <View style={styles.rowFields}>
        <TextInput
          value={payeeName}
          onChangeText={setPayeeName}
          placeholder="Payee, e.g. CRED"
          placeholderTextColor={colors.faint}
          style={[styles.input, styles.flex]}
        />
        <Pressable
          style={({ pressed }) => [styles.categoryButton, pressed && styles.pressed]}
          onPress={() => onPickCategory({ current: categoryId, onSelect: setCategoryId })}
        >
          <AppText numberOfLines={1} tone={categoryId ? 'default' : 'faint'}>
            {categoryName[categoryId] ?? 'Category'}
          </AppText>
        </Pressable>
      </View>
      <Button size="sm" variant="secondary" disabled={busy || !payeeName.trim()} onPress={() => void add()}>
        {busy ? <ActivityIndicator color={colors.text} /> : `Add ${signedMoney(difference)}`}
      </Button>
    </View>
  );
}

/** What a card statement says beyond its rows: dues, charges, cashback, reward points. */
function CardSummary({ section }: { section: StatementSectionReconciliation }) {
  const tiles = [
    section.totalDue !== undefined && {
      label: 'Total due',
      value: money(section.totalDue)
    },
    section.minimumDue !== undefined && {
      label: 'Minimum due',
      value: money(section.minimumDue)
    },
    section.dueDate && { label: 'Pay by', value: longDay(section.dueDate) },
    section.creditLimit !== undefined && {
      label: 'Credit limit',
      value: money(section.creditLimit)
    }
  ].filter(Boolean) as { label: string; value: string }[];
  const charges = section.charges ?? [];
  const rewards = section.rewards;
  const rewardRows = rewards
    ? ([
        ['Opening', rewards.opening],
        ['Earned', rewards.earned],
        ['Redeemed', rewards.redeemed],
        ['Expired', rewards.expired],
        ['Closing', rewards.closing]
      ].filter(([, v]) => v !== undefined) as [string, number][])
    : [];

  if (!tiles.length && !charges.length && !section.cashback && !rewardRows.length && section.rewardPointsOnRows === undefined) {
    return null;
  }

  return (
    <View style={styles.summary}>
      {tiles.length ? (
        <View style={styles.tiles}>
          {tiles.map((t) => (
            <Card key={t.label} style={styles.tile}>
              <AppText variant="caption" muted>
                {t.label}
              </AppText>
              <AppText weight="semibold" tabular>
                {t.value}
              </AppText>
            </Card>
          ))}
        </View>
      ) : null}
      {charges.length || section.cashback ? (
        <Card style={styles.panel}>
          <AppText weight="semibold">
            {section.cashback && charges.length ? 'Charges and cashback' : section.cashback ? 'Cashback' : 'Charges'}
          </AppText>
          {charges.map((c) => (
            <View key={c.kind} style={styles.panelRow}>
              <AppText variant="caption" muted style={styles.flex}>
                {c.kind === 'tax' ? 'GST and taxes' : c.kind === 'interest' ? 'Interest' : c.kind === 'fee' ? 'Fees' : c.kind} ·{' '}
                {c.count} line{c.count === 1 ? '' : 's'}
              </AppText>
              <AppText variant="caption" tabular>
                {money(c.amount)}
              </AppText>
            </View>
          ))}
          {section.cashback ? (
            <View style={styles.panelRow}>
              <AppText variant="caption" muted style={styles.flex}>
                Cashback received · {section.cashback.count} line
                {section.cashback.count === 1 ? '' : 's'}
              </AppText>
              <AppText variant="caption" tone="success" tabular>
                + {money(section.cashback.amount)}
              </AppText>
            </View>
          ) : null}
        </Card>
      ) : null}
      {rewardRows.length || section.rewardPointsOnRows !== undefined ? (
        <Card style={styles.panel}>
          <AppText weight="semibold">Reward points</AppText>
          {rewardRows.map(([label, value]) => (
            <View key={label} style={styles.panelRow}>
              <AppText variant="caption" muted style={styles.flex}>
                {label}
              </AppText>
              <AppText variant="caption" tabular>
                {new Intl.NumberFormat('en-IN').format(value)}
              </AppText>
            </View>
          ))}
          {section.rewardPointsOnRows !== undefined ? (
            <View style={styles.panelRow}>
              <AppText variant="caption" muted style={styles.flex}>
                On this statement's rows
              </AppText>
              <AppText variant="caption" tabular>
                {new Intl.NumberFormat('en-IN').format(section.rewardPointsOnRows)}
              </AppText>
            </View>
          ) : null}
        </Card>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  section: { gap: spacing.md },
  sectionHead: { flexDirection: 'row', alignItems: 'center' },
  state: { alignItems: 'center', gap: spacing.md, padding: spacing.xl },
  fullWidth: { alignSelf: 'stretch' },
  center: { textAlign: 'center' },
  flex: { flex: 1, minWidth: 0 },
  pressed: { opacity: 0.75 },
  banner: {
    flexDirection: 'row',
    gap: spacing.sm,
    alignItems: 'flex-start',
    padding: spacing.md,
    borderRadius: radii.md
  },
  bannerOk: { backgroundColor: colors.successMuted },
  bannerWarn: { backgroundColor: colors.warningMuted },
  tabs: { gap: spacing.sm, paddingVertical: spacing.xs },
  tab: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.xs,
    paddingHorizontal: spacing.md,
    height: 36,
    borderRadius: radii.full,
    backgroundColor: colors.surface
  },
  tabActive: { backgroundColor: colors.primaryMuted },
  count: {
    minWidth: 22,
    paddingHorizontal: 6,
    borderRadius: radii.full,
    backgroundColor: colors.surfaceStrong,
    alignItems: 'center'
  },
  countAttention: { backgroundColor: colors.warningMuted },
  countText: { fontSize: 12 },
  list: { gap: spacing.sm },
  empty: { alignItems: 'center', gap: spacing.xs, paddingVertical: spacing.xl },
  rowCard: { padding: spacing.md, gap: spacing.md },
  rowCardSelected: { backgroundColor: colors.primaryMuted },
  rowTop: { flexDirection: 'row', alignItems: 'flex-start', gap: spacing.md },
  check: {
    width: 22,
    height: 22,
    marginTop: 2,
    borderRadius: 6,
    borderWidth: 1.5,
    borderColor: colors.faint,
    alignItems: 'center',
    justifyContent: 'center'
  },
  checkOn: { backgroundColor: colors.primary, borderColor: colors.primary },
  rowFields: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm },
  input: {
    minHeight: 42,
    paddingHorizontal: spacing.md,
    borderRadius: radii.sm,
    backgroundColor: colors.surfaceStrong,
    color: colors.text,
    fontSize: 14
  },
  categoryButton: {
    flex: 1,
    minWidth: 0,
    minHeight: 42,
    justifyContent: 'center',
    paddingHorizontal: spacing.md,
    borderRadius: radii.sm,
    backgroundColor: colors.surfaceStrong
  },
  iconButton: {
    width: 42,
    height: 42,
    borderRadius: radii.sm,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.primaryMuted
  },
  badges: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: spacing.xs,
    marginTop: spacing.xs
  },
  badge: {
    paddingHorizontal: spacing.sm,
    paddingVertical: 2,
    borderRadius: radii.full,
    backgroundColor: colors.surfaceStrong
  },
  badgeWarning: { backgroundColor: colors.warningMuted },
  badgeText: { fontSize: 12 },
  pageButton: {
    flexDirection: 'row',
    alignItems: 'center',
    alignSelf: 'flex-start',
    gap: 4,
    paddingHorizontal: spacing.sm,
    paddingVertical: 2,
    borderRadius: radii.full,
    backgroundColor: colors.primaryMuted
  },
  amountCol: { alignItems: 'flex-end', gap: spacing.sm },
  candidate: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md,
    padding: spacing.md,
    borderRadius: radii.md,
    borderWidth: 1,
    borderColor: 'transparent',
    backgroundColor: colors.surfaceStrong
  },
  candidateOn: {
    borderColor: colors.primary,
    backgroundColor: colors.primaryMuted
  },
  linkFooter: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md,
    paddingTop: spacing.md
  },
  difference: {
    gap: spacing.sm,
    paddingTop: spacing.sm,
    borderTopWidth: StyleSheet.hairlineWidth,
    borderTopColor: colors.border
  },
  summary: { gap: spacing.sm },
  tiles: { flexDirection: 'row', flexWrap: 'wrap', gap: spacing.sm },
  tile: { flexGrow: 1, flexBasis: '45%', padding: spacing.md, gap: 2 },
  panel: { padding: spacing.md, gap: spacing.sm },
  panelRow: { flexDirection: 'row', alignItems: 'center', gap: spacing.md }
});
