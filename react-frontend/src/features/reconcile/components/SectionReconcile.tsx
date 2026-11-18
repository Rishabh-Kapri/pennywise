import {
  Bank,
  CalendarX,
  CheckCircle,
  CircleNotch,
  FileMagnifyingGlass,
  LinkBreak,
  LinkSimple,
  NotePencil,
  PencilSimple,
  Plus,
  WarningCircle,
} from '@phosphor-icons/react';
import { type Dispatch, Fragment, memo, type SetStateAction, useCallback, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { useAppSelector } from '@/app/hooks';
import { errorMessage, errorText, toast } from '@/utils';
import {
  signedAmount,
  type StatementCreateItem,
  type StatementCreateResult,
  type StatementDifferenceRequest,
  type StatementRow,
  type StatementRowEdit,
  type StatementLedgerTxn,
  type StatementSectionReconciliation,
} from '../types/reconcile.types';
import { accountEnding, formatPeriod, money } from '../statementIdentity';
import type { Account } from '@/features/accounts/types/account.types';
import { CategoryDropdown } from '@/features/category/components/CategoryDropdown';
import { PayeeDropdown } from '@/features/payees/components/PayeeDropdown';
import { AccountPicker } from './AccountPicker';
import { CardSummary } from './CardSummary';
import { DifferenceForm, LinkCandidates, RowEditor, RowPicker } from './LinkControls';
import styles from './StatementReconcile.module.css';

type Tab = 'missing' | 'matched' | 'ledgerOnly' | 'covered';

// money keeps paise: a reconciliation that rounds hides the 0.76 it is about

function Amount({ value }: { value: number }) {
  return (
    <span className={value < 0 ? styles.outflow : styles.inflow}>
      {value < 0 ? '−' : '+'}
      {money(Math.abs(value))}
    </span>
  );
}

// "01 Oct" from the API's YYYY-MM-DD; anything else is shown as it came
function shortDay(date: string): string {
  const d = new Date(`${date}T00:00:00`);
  if (Number.isNaN(d.getTime())) return date;
  return d.toLocaleDateString(undefined, { day: '2-digit', month: 'short' });
}

/** The row's date, with the time under it when the statement printed one. */
function RowDate({ row }: { row: StatementRow }) {
  return (
    <>
      {shortDay(row.date)}
      {row.time && <span className={styles.rowTime}>{row.time}</span>}
    </>
  );
}

const CHARGE_BADGES: Record<string, string> = { tax: 'tax', interest: 'interest', fee: 'fee' };

/** "+12 pts", or "−40 pts" for points taken back (a refund, a redemption). */
const pointsLabel = (n: number) => `${n > 0 ? '+' : '−'}${Math.abs(n).toLocaleString('en-IN')} pts`;

function RowBadges({
  row,
  onShowPage,
  onShowLine,
  onSetEMI,
}: {
  row: StatementRow;
  onShowPage?: (page: number) => void;
  onShowLine?: (line: number) => void;
  onSetEMI?: (rowId: string, emi: boolean | null) => Promise<void>;
}) {
  return (
    <>
      {row.edited && (
        <span
          className={styles.badgeEdited}
          title={`Read as "${row.edited.description}", ${row.edited.direction === 'debit' ? '−' : '+'}${money(row.edited.amount)} on ${shortDay(row.edited.date)}`}
        >
          corrected
        </span>
      )}
      {row.emiOverride === false && onSetEMI ? (
        <button
          type="button"
          className={styles.pageButton}
          title="You said this isn't an EMI instalment. Undo puts it back under Covered by EMI."
          onClick={() => void onSetEMI(row.id, null)}
        >
          not an EMI · Undo
        </button>
      ) : null}
      {onShowPage && row.page ? (
        <button
          type="button"
          className={styles.pageButton}
          title="Show this page of the PDF"
          onClick={() => onShowPage(row.page!)}
        >
          <FileMagnifyingGlass size={13} />
          p. {row.page}
        </button>
      ) : null}
      {onShowLine && row.line ? (
        <button
          type="button"
          className={styles.pageButton}
          title="Show this line of the CSV"
          onClick={() => onShowLine(row.line!)}
        >
          <FileMagnifyingGlass size={13} />
          line {row.line}
        </button>
      ) : null}
      {row.valueDateMismatch && row.valueDate && (
        <span className={styles.badge} title="The value date printed in the narration is days away from the row's date">
          value date {shortDay(row.valueDate)}
        </span>
      )}
      {row.chargeKind && <span className={styles.badge}>{CHARGE_BADGES[row.chargeKind]}</span>}
      {row.rewardPoints ? (
        <span className={styles.badgeMuted} title="Reward points the statement printed for this row">
          {pointsLabel(row.rewardPoints)}
        </span>
      ) : null}
      {row.reference && <span className={styles.badgeMuted}>Ref {row.reference}</span>}
    </>
  );
}

interface RowDraft {
  selected: boolean;
  payeeName: string;
  categoryId: string;
  note: string;
}

/** Open one row's panel, or close it when it's the one already open; null closes any. */
const toggler = (set: Dispatch<SetStateAction<string | null>>) => (id: string | null) =>
  set((cur) => (id === null || cur === id ? null : id));

/** The same function every render, calling the latest one: lets memoised rows skip re-rendering. */
function useStable<A extends unknown[], R>(fn: (...args: A) => R): (...args: A) => R {
  const ref = useRef(fn);
  useLayoutEffect(() => {
    ref.current = fn;
  });
  return useCallback((...args: A) => ref.current(...args), []);
}

function EditButton({ row, editing, onToggle }: { row: StatementRow; editing: boolean; onToggle: (id: string) => void }) {
  return (
    <button
      type="button"
      className={`${styles.iconButton} ${editing ? styles.iconButtonActive : ''}`}
      title="Correct this row: it was read wrong"
      aria-label={`Correct ${row.description}`}
      aria-expanded={editing}
      onClick={() => onToggle(row.id)}
    >
      <PencilSimple size={17} />
    </button>
  );
}

/**
 * One missing row with its pickers and the panels it opens. Memoised: a CSV
 * can have hundreds of rows, and ticking one or typing a note shouldn't
 * re-render every row's payee and category pickers.
 */
const MissingRow = memo(function MissingRow({
  row,
  draft: d,
  categoryLabel,
  noting,
  editing,
  linking,
  candidates,
  onUpdate,
  onToggleNote,
  onToggleEdit,
  onToggleLink,
  onEditRow,
  onRevertRow,
  onLink,
  onShowPage,
  onShowLine,
  onSetEMI,
}: {
  row: StatementRow;
  draft: RowDraft;
  categoryLabel: string;
  noting: boolean;
  editing: boolean;
  linking: boolean;
  candidates: StatementLedgerTxn[];
  onUpdate: (rowId: string, patch: Partial<RowDraft>) => void;
  onToggleNote: (rowId: string | null) => void;
  onToggleEdit: (rowId: string | null) => void;
  onToggleLink: (rowId: string | null) => void;
  onEditRow: (rowId: string, edit: StatementRowEdit) => Promise<void>;
  onRevertRow: (rowId: string) => Promise<void>;
  onLink: (rowId: string, transactionIds: string[]) => Promise<void>;
  onShowPage?: (page: number) => void;
  onShowLine?: (line: number) => void;
  onSetEMI: (rowId: string, emi: boolean | null) => Promise<void>;
}) {
  return (
    <>
      <tr className={d.selected ? styles.rowSelected : undefined}>
        <td className={styles.checkCell}>
          <input
            type="checkbox"
            aria-label={`Select ${row.description}`}
            checked={d.selected}
            onChange={(e) => onUpdate(row.id, { selected: e.target.checked })}
          />
        </td>
        <td className={styles.dateCell}>
          <RowDate row={row} />
        </td>
        <td className={styles.textCell}>
          <span className={styles.rowTitle} title={row.rawDescription || row.description}>
            {row.description}
          </span>
          <div className={styles.rowMeta}>
            <RowBadges row={row} onShowPage={onShowPage} onShowLine={onShowLine} onSetEMI={onSetEMI} />
            {row.directionCorrected && (
              <span title="The running balance showed debit and credit were reversed">direction fixed</span>
            )}
            {row.suggestedPayeeName && (
              <span title="The payee and category you chose for this line on an earlier statement">
                filled from last time
              </span>
            )}
            {d.note && !noting && (
              <button type="button" className={styles.metaNote} onClick={() => onToggleNote(row.id)}>
                “{d.note}”
              </button>
            )}
          </div>
        </td>
        <td className={styles.payeeCell}>
          <PayeeDropdown
            variant="form"
            value={d.payeeName}
            placeholder="Choose a payee"
            onClick={(_, name) => onUpdate(row.id, { payeeName: name })}
            onCreate={(name) => onUpdate(row.id, { payeeName: name })}
          />
        </td>
        <td className={styles.categoryCell}>
          <CategoryDropdown
            variant="form"
            value={categoryLabel}
            placeholder="Choose a category"
            onClick={(id) => onUpdate(row.id, { categoryId: id })}
          />
        </td>
        <td className={`${styles.num} ${styles.amountCell}`}>
          <Amount value={signedAmount(row)} />
        </td>
        <td className={styles.actionCell}>
          <div className={styles.rowActionsQuiet}>
            <button
              type="button"
              className={`${styles.iconButton} ${noting || d.note ? styles.iconButtonActive : ''}`}
              title={d.note ? 'Change the note' : 'Add a note'}
              aria-label={`Note for ${row.description}`}
              aria-expanded={noting}
              onClick={() => onToggleNote(row.id)}
            >
              <NotePencil size={18} />
            </button>
            <EditButton row={row} editing={editing} onToggle={onToggleEdit} />
            <button
              type="button"
              className={`${styles.iconButton} ${linking ? styles.iconButtonActive : ''}`}
              title="Link to a transaction already in Pennywise"
              aria-label={`Link ${row.description} to a transaction`}
              aria-expanded={linking}
              onClick={() => onToggleLink(row.id)}
            >
              <LinkSimple size={18} />
            </button>
          </div>
        </td>
      </tr>
      {noting && (
        <tr className={styles.noteRow}>
          <td />
          <td />
          <td colSpan={5}>
            <input
              className={styles.noteField}
              value={d.note}
              placeholder="Add a note for this transaction, then press Enter"
              aria-label={`Note for ${row.description}`}
              autoFocus
              onChange={(e) => onUpdate(row.id, { note: e.target.value })}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === 'Escape') onToggleNote(null);
              }}
            />
          </td>
        </tr>
      )}
      {editing && (
        <tr className={styles.linkRow}>
          <td colSpan={7}>
            <RowEditor
              row={row}
              onSave={(edit) => onEditRow(row.id, edit)}
              onRevert={() => onRevertRow(row.id)}
              onCancel={() => onToggleEdit(null)}
            />
          </td>
        </tr>
      )}
      {linking && (
        <tr className={styles.linkRow}>
          <td colSpan={7}>
            <LinkCandidates
              rows={[row]}
              candidates={candidates}
              onLink={(ids) => onLink(row.id, ids)}
              onCancel={() => onToggleLink(null)}
            />
          </td>
        </tr>
      )}
    </>
  );
});

interface Props {
  section: StatementSectionReconciliation;
  accounts: Account[];
  onAccountChange: (accountId: string | null) => Promise<void>;
  onCreate: (items: StatementCreateItem[]) => Promise<StatementCreateResult>;
  /** pair a row with some of the account's transactions by hand, and undo it */
  onLink: (rowId: string, transactionIds: string[]) => Promise<void>;
  /** link several rows and transactions together */
  onLinkMany: (rowIds: string[], transactionIds: string[]) => Promise<void>;
  onUnlink: (rowId: string) => Promise<void>;
  /** correct a row read wrong, and put back what was read */
  onEditRow: (rowId: string, edit: StatementRowEdit) => Promise<void>;
  onRevertRow: (rowId: string) => Promise<void>;
  /** book what a linked row's transactions are short of it */
  onAddDifference: (rowId: string, body: StatementDifferenceRequest) => Promise<void>;
  /** move a row in or out of Covered by EMI; null goes back to what its text says */
  onSetEMI: (rowId: string, emi: boolean | null) => Promise<void>;
  /** take a transaction off Not on statement: another month's, or the wrong account's */
  onSetAside: (transactionId: string, aside: boolean) => Promise<void>;
  /** name the account and period in the section's header: the page header
   * already does when the statement has one section */
  showIdentity?: boolean;
  /** set when the PDF is kept: rows open it at the page they were read from */
  onShowPage?: (page: number) => void;
  onShowLine?: (line: number) => void;
}

export function SectionReconcile({
  section,
  accounts,
  onAccountChange,
  onCreate,
  onLink,
  onLinkMany,
  onUnlink,
  onEditRow,
  onRevertRow,
  onAddDifference,
  onSetEMI,
  onSetAside,
  showIdentity = true,
  onShowPage,
  onShowLine,
}: Props) {
  const categories = useAppSelector((state) => state.categories.allCategories);
  // the picker shows a category by name; drafts hold its id
  const categoryName = useMemo(
    () => Object.fromEntries(categories.filter((c) => c.id).map((c) => [c.id!, c.name])) as Record<string, string>,
    [categories],
  );

  const [tab, setTab] = useState<Tab>(section.missing.length > 0 ? 'missing' : 'matched');
  // where the tabs sit before they stick to the top
  const tabsAnchor = useRef<HTMLDivElement>(null);
  const tabsBar = useRef<HTMLDivElement>(null);
  const scroller = useRef<HTMLElement | null>(null);
  // Stick to the scroll panel's top edge, not below its padding, where rows
  // would show through. The page sits in more than one panel, so ask it.
  const hasAccount = !!section.accountId;
  useLayoutEffect(() => {
    const bar = tabsBar.current;
    let el = bar?.parentElement ?? null;
    while (el && !/(auto|scroll)/.test(getComputedStyle(el).overflowY)) el = el.parentElement;
    scroller.current = el;
    if (bar && el) bar.style.top = `-${getComputedStyle(el).paddingTop}`;
  }, [hasAccount]);
  // The tabs stay at the top while rows scroll. Switching from deep in a long
  // list goes to the new tab's first row, not into the middle of it.
  const showTab = (next: Tab) => {
    setTab(next);
    const anchor = tabsAnchor.current;
    const bar = tabsBar.current;
    const panel = scroller.current;
    const gap = anchor && bar ? anchor.getBoundingClientRect().top - bar.getBoundingClientRect().top : 0;
    // scroll the panel only: scrollIntoView moves the whole app too
    if (panel && gap < -1) panel.scrollTop += gap;
  };
  const [drafts, setDrafts] = useState<Record<string, RowDraft>>({});
  const [busy, setBusy] = useState(false);
  const [savingAccount, setSavingAccount] = useState(false);
  // the row whose link list is open: a missing one, or a linked one short of its amount
  const [linking, setLinking] = useState<string | null>(null);
  const [unlinking, setUnlinking] = useState<string | null>(null);
  // the transaction being set aside or put back
  const [asiding, setAsiding] = useState<string | null>(null);
  const setAside = async (t: StatementLedgerTxn, aside: boolean) => {
    setAsiding(t.id);
    try {
      await onSetAside(t.id, aside);
      if (pickingFor === t.id) setPickingFor(null);
    } catch (err) {
      toast.error(aside ? "Couldn't set it aside" : "Couldn't put it back", {
        description: errorMessage(err, 'Try again.'),
      });
    } finally {
      setAsiding(null);
    }
  };
  // the row being corrected, the ledger transaction whose row list is open,
  // and the EMI lines ticked to link together
  const [editing, setEditing] = useState<string | null>(null);
  // the missing row whose note field was just opened
  const [noting, setNoting] = useState<string | null>(null);
  const [pickingFor, setPickingFor] = useState<string | null>(null);
  const [linkingSelected, setLinkingSelected] = useState(false);
  const [coveredPicked, setCoveredPicked] = useState<string[]>([]);
  const [linkingCovered, setLinkingCovered] = useState(false);

  const linkMany = async (rowIds: string[], transactionIds: string[]) => {
    await onLinkMany(rowIds, transactionIds);
    setPickingFor(null);
    setLinkingSelected(false);
    setLinkingCovered(false);
    setCoveredPicked([]);
    // stay on this tab: the linked rows leave it, the rest keep what was typed
  };

  const editor = (row: StatementRow, colSpan: number) =>
    editing === row.id ? (
      <tr className={styles.linkRow}>
        <td colSpan={colSpan}>
          <RowEditor
            row={row}
            onSave={(edit) => onEditRow(row.id, edit)}
            onRevert={() => onRevertRow(row.id)}
            onCancel={() => setEditing(null)}
          />
        </td>
      </tr>
    ) : null;

  const toggleNote = useMemo(() => toggler(setNoting), []);
  const toggleEdit = useMemo(() => toggler(setEditing), []);
  const toggleLink = useMemo(() => toggler(setLinking), []);

  const editButton = (row: StatementRow) => (
    <EditButton row={row} editing={editing === row.id} onToggle={toggleEdit} />
  );

  // Stay on the tab: going to Matched lost your place in a long Missing list.
  // The row leaves the list, so say it worked.
  const link = useStable(async (rowId: string, transactionIds: string[]) => {
    await onLink(rowId, transactionIds);
    setLinking(null);
    toast.success(transactionIds.length > 1 ? `Linked to ${transactionIds.length} transactions` : 'Linked');
  });
  const editRow = useStable(onEditRow);
  const revertRow = useStable(onRevertRow);
  const setEMI = useStable(onSetEMI);
  const showPage = useStable((page: number) => onShowPage?.(page));
  const showLine = useStable((line: number) => onShowLine?.(line));

  const unlink = async (rowId: string) => {
    setUnlinking(rowId);
    try {
      await onUnlink(rowId);
    } catch (err) {
      toast.error("Couldn't unlink", { description: errorMessage(err, 'Try again.') });
    } finally {
      setUnlinking(null);
    }
  };

  // a charge row starts with the category the API suggested for it
  // and the payee an earlier statement's same line was given
  const suggested = useMemo(
    () =>
      Object.fromEntries(
        section.missing.map((r) => [r.id, { categoryId: r.suggestedCategoryId ?? '', payee: r.suggestedPayeeName }]),
      ),
    [section.missing],
  );
  // what a row starts with before it's touched; kept so memoised rows see the same object
  const defaults = useMemo(
    () =>
      Object.fromEntries(
        section.missing.map((r) => [
          r.id,
          { selected: false, payeeName: suggested[r.id]?.payee || r.payee, categoryId: suggested[r.id]?.categoryId ?? '', note: '' },
        ]),
      ) as Record<string, RowDraft>,
    [section.missing, suggested],
  );
  const draftFor = (rowId: string): RowDraft => drafts[rowId] ?? defaults[rowId];

  const updateDraft = useCallback(
    (rowId: string, patch: Partial<RowDraft>) =>
      setDrafts((prev) => ({ ...prev, [rowId]: { ...(prev[rowId] ?? defaults[rowId]), ...patch } })),
    [defaults],
  );

  const selectedRows = section.missing.filter((r) => draftFor(r.id).selected);
  const allSelected = section.missing.length > 0 && selectedRows.length === section.missing.length;

  // The selection bar's pickers set a payee or category on every ticked row.
  const applyToSelected = (patch: Partial<RowDraft>) =>
    setDrafts((prev) => {
      const out = { ...prev };
      for (const r of selectedRows) out[r.id] = { ...draftFor(r.id), ...patch };
      return out;
    });

  const toggleAll = () => {
    const next = !allSelected;
    setDrafts((prev) => {
      const out = { ...prev };
      for (const r of section.missing) out[r.id] = { ...draftFor(r.id), selected: next };
      return out;
    });
  };

  const createSelected = async () => {
    if (!selectedRows.length) return;
    setBusy(true);
    try {
      const res = await onCreate(
        selectedRows.map((r) => {
          const d = draftFor(r.id);
          return {
            rowId: r.id,
            payeeName: d.payeeName.trim() || undefined,
            categoryId: d.categoryId || undefined,
            note: d.note.trim() || undefined,
          };
        }),
      );
      const parts: string[] = [];
      if (res.skipped.length) parts.push(`${res.skipped.length} already there`);
      if (res.failed.length) parts.push(`${res.failed.length} failed: ${errorText(res.failed[0].error)}`);
      const title = `Created ${res.created.length} transaction${res.created.length === 1 ? '' : 's'}`;
      const description = parts.join(' · ') || 'They are unapproved, so you can review them in Transactions.';
      if (res.failed.length) toast.warning(title, { description });
      else toast.success(title, { description });
      setDrafts({});
    } catch (err) {
      toast.error("Couldn't create the transactions", { description: errorMessage(err, 'Try again.') });
    } finally {
      setBusy(false);
    }
  };

  const changeAccount = async (value: string) => {
    setSavingAccount(true);
    try {
      await onAccountChange(value || null);
    } catch (err) {
      toast.error("Couldn't set the account", { description: errorMessage(err, 'Try again.') });
    } finally {
      setSavingAccount(false);
    }
  };

  const ending = accountEnding(section.accountNumber);
  const label = [section.accountLabel, ending && `ending ${ending}`].filter(Boolean).join(' ');
  const period = formatPeriod(section.periodStart, section.periodEnd);
  const facts = [label, period, `${section.rowCount} rows`, section.currency].filter(Boolean).join(' · ');
  const headerFacts = showIdentity ? facts : [`${section.rowCount} rows`, section.currency].filter(Boolean).join(' · ');

  const accountSelect = (
    <div className={styles.accountSelect}>
      <AccountPicker
        accounts={accounts}
        value={section.accountId ?? ''}
        disabled={savingAccount}
        noneLabel={section.accountId ? 'No account' : undefined}
        onChange={(id) => void changeAccount(id)}
      />
    </div>
  );

  if (!section.accountId) {
    return (
      <section className={styles.stateCard}>
        <Bank size={32} className={styles.stateIcon} />
        <h3>Which account is this statement for?</h3>
        <p className={styles.muted}>{facts}</p>
        {ending && (
          <p className={styles.muted}>
            No account ends in {ending}. Set an account's last digits to {ending} and future statements will be matched
            to it automatically.
          </p>
        )}
        {accountSelect}
        <p className={styles.muted}>Its rows will be matched against that account's transactions.</p>
      </section>
    );
  }

  const tabs: { id: Tab; label: string; count: number }[] = [
    { id: 'missing', label: 'Missing', count: section.missing.length },
    { id: 'matched', label: 'Matched', count: section.matched.length },
    { id: 'ledgerOnly', label: 'Not on statement', count: section.ledgerOnly.length },
    ...(section.covered?.length
      ? [{ id: 'covered' as Tab, label: 'Covered by EMI', count: section.covered.length }]
      : []),
  ];

  return (
    <section className={styles.section}>
      <header className={styles.sectionHeader}>
        <p className={styles.muted}>{headerFacts}</p>
        <label className={styles.inlineField}>
          <span>Account</span>
          {accountSelect}
        </label>
      </header>

      <CardSummary section={section} />

      {section.balanceCheck &&
        (section.balanceCheck.ok ? (
          <p className={styles.okBanner}>
            <CheckCircle size={18} weight="fill" />
            <span>The rows add up to the statement's closing balance of {money(section.balanceCheck.reported)}.</span>
          </p>
        ) : (
          <p className={styles.warningBanner}>
            <WarningCircle size={18} />
            <span>
              The rows add up to {money(section.balanceCheck.computed)}, but the statement closes at{' '}
              {money(section.balanceCheck.reported)} (off by {money(Math.abs(section.balanceCheck.difference))}). A row
              may have been missed or misread.
            </span>
          </p>
        ))}

      <div ref={tabsAnchor} className={styles.tabsAnchor} aria-hidden />
      <div ref={tabsBar} className={styles.tabs} role="tablist">
        {tabs.map((t) => (
          <button
            key={t.id}
            type="button"
            role="tab"
            aria-selected={tab === t.id}
            className={`${styles.tab} ${tab === t.id ? styles.tabActive : ''}`}
            onClick={() => showTab(t.id)}
          >
            {t.label}
            <span className={`${styles.count} ${t.id === 'missing' && t.count > 0 ? styles.countAttention : ''}`}>
              {t.count}
            </span>
          </button>
        ))}
      </div>

      {tab === 'missing' &&
        (section.missing.length === 0 ? (
          <div className={styles.emptyTab}>
            <CheckCircle size={32} weight="fill" className={styles.okIcon} />
            <strong>Nothing is missing</strong>
            <span>Every row on the statement has a matching transaction.</span>
          </div>
        ) : (
          <>
            <p className={styles.muted}>
              These are on the statement but not in Pennywise. Tick the ones to add; they're created unapproved so you
              can review them.
            </p>
            <div className={styles.tableWrap}>
              <table className={`${styles.table} ${styles.missingTable}`}>
                <colgroup>
                  <col className={styles.colCheck} />
                  <col className={styles.colDate} />
                  <col />
                  <col className={styles.colPayee} />
                  <col className={styles.colCategory} />
                  <col className={styles.colAmount} />
                  <col className={styles.colActions} />
                </colgroup>
                <thead>
                  <tr>
                    <th className={styles.checkCell}>
                      <input
                        type="checkbox"
                        aria-label="Select all missing rows"
                        checked={allSelected}
                        onChange={toggleAll}
                      />
                    </th>
                    <th>Date</th>
                    <th>On the statement</th>
                    <th>Payee</th>
                    <th>Category</th>
                    <th className={styles.num}>Amount</th>
                    <th className={styles.actionCell}>
                      <span className={styles.srOnly}>Note, correct or link</span>
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {section.missing.map((row) => {
                    const d = drafts[row.id] ?? defaults[row.id];
                    return (
                      <MissingRow
                        key={row.id}
                        row={row}
                        draft={d}
                        categoryLabel={categoryName[d.categoryId] ?? ''}
                        noting={noting === row.id}
                        editing={editing === row.id}
                        linking={linking === row.id}
                        candidates={section.ledgerOnly}
                        onUpdate={updateDraft}
                        onToggleNote={toggleNote}
                        onToggleEdit={toggleEdit}
                        onToggleLink={toggleLink}
                        onEditRow={editRow}
                        onRevertRow={revertRow}
                        onLink={link}
                        onShowPage={onShowPage && showPage}
                        onShowLine={onShowLine && showLine}
                        onSetEMI={setEMI}
                      />
                    );
                  })}
                </tbody>
              </table>
            </div>
            {selectedRows.length > 0 && (
              <div className={`${styles.createBar} ${styles.selectionBar}`}>
                <div className={styles.barCount}>
                  <strong>{selectedRows.length}</strong>
                  <span>of {section.missing.length} selected</span>
                  <button
                    type="button"
                    className={styles.textButton}
                    onClick={() => applyToSelected({ selected: false })}
                  >
                    Clear
                  </button>
                </div>
                <div className={styles.bulkFields}>
                  <div className={styles.payeeCell}>
                    <PayeeDropdown
                      variant="form"
                      value=""
                      placeholder="Payee for all"
                      onClick={(_, name) => applyToSelected({ payeeName: name })}
                      onCreate={(name) => applyToSelected({ payeeName: name })}
                    />
                  </div>
                  <div className={styles.categoryCell}>
                    <CategoryDropdown
                      variant="form"
                      value=""
                      placeholder="Category for all"
                      onClick={(id) => applyToSelected({ categoryId: id })}
                    />
                  </div>
                  <input
                    className={`${styles.cellInput} ${styles.bulkNote}`}
                    placeholder="Note for all"
                    aria-label="Note for every selected row"
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') applyToSelected({ note: e.currentTarget.value });
                    }}
                    onBlur={(e) => e.currentTarget.value && applyToSelected({ note: e.currentTarget.value })}
                  />
                </div>
                <div className={styles.barActions}>
                  <button
                    type="button"
                    className={styles.secondaryButton}
                    title="They're one payment in Pennywise: link them to it"
                    onClick={() => setLinkingSelected(!linkingSelected)}
                  >
                    <LinkSimple size={17} /> Link to one
                  </button>
                  <button
                    type="button"
                    className={styles.primaryButton}
                    disabled={busy}
                    onClick={() => void createSelected()}
                  >
                    {busy ? <CircleNotch size={18} className={styles.spin} /> : <Plus size={18} />}
                    {selectedRows.length > 1 ? `Add ${selectedRows.length} transactions` : 'Add transaction'}
                  </button>
                </div>
                {linkingSelected && (
                  <div className={styles.createBarPanel}>
                    <LinkCandidates
                      rows={selectedRows}
                      candidates={section.ledgerOnly}
                      onLink={(ids) =>
                        linkMany(
                          selectedRows.map((r) => r.id),
                          ids,
                        )
                      }
                      onCancel={() => setLinkingSelected(false)}
                    />
                  </div>
                )}
              </div>
            )}
            {/* room under the bar, so it never sits on the last row */}
            {selectedRows.length > 0 && <div className={styles.createBarSpacer} aria-hidden />}
          </>
        ))}

      {tab === 'matched' &&
        (section.matched.length === 0 ? (
          <div className={styles.emptyTab}>
            <strong>Nothing matched yet</strong>
            <span>No statement row lines up with a transaction in this account.</span>
          </div>
        ) : (
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>Date</th>
                  <th>On the statement</th>
                  <th>In Pennywise</th>
                  <th className={styles.num}>Amount</th>
                  <th className={styles.actionCell}>
                    <span className={styles.srOnly}>Unlink</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {section.matched.map((m) => (
                  <Fragment key={m.row.id}>
                    <tr>
                      <td className={styles.dateCell}>
                        <RowDate row={m.row} />
                      </td>
                      <td>
                        <span className={styles.description} title={m.row.rawDescription || m.row.description}>
                          {m.row.description}
                        </span>
                        <div className={styles.rowMeta}>
                          <RowBadges row={m.row} onShowPage={onShowPage} onShowLine={onShowLine} onSetEMI={onSetEMI} />
                        </div>
                        {m.otherRows?.map((r) => (
                          <span key={r.id} className={styles.linkedOther}>
                            + {r.description} <Amount value={signedAmount(r)} />
                          </span>
                        ))}
                      </td>
                      <td>
                        <Link to={`/transactions/${m.transaction.id}`}>
                          {m.transaction.payeeName || m.transaction.note || 'Open transaction'}
                        </Link>
                        {m.dateDiffDays > 0 && (
                          <span className={styles.badgeMuted}>
                            {m.dateDiffDays} day{m.dateDiffDays === 1 ? '' : 's'} apart
                          </span>
                        )}
                        {m.transaction.status === 'UNAPPROVED' && <span className={styles.badgeMuted}>unapproved</span>}
                        {m.linked && <span className={styles.badgeMuted}>linked by you</span>}
                        {m.others?.map((o) => (
                          <span key={o.id} className={styles.linkedOther}>
                            + <Link to={`/transactions/${o.id}`}>{o.payeeName || o.note || 'Transaction'}</Link>{' '}
                            {money(o.amount)}
                          </span>
                        ))}
                      </td>
                      <td className={styles.num}>
                        <Amount value={[m.row, ...(m.otherRows ?? [])].reduce((sum, r) => sum + signedAmount(r), 0)} />
                        {m.kind === 'amount_differs' && !m.linked && (
                          <span className={styles.badge} title="Close date and matching text, different amount">
                            Pennywise has {money(m.transaction.amount)}
                          </span>
                        )}
                      </td>
                      <td className={styles.actionCell}>
                        {!m.otherRows?.length && editButton(m.row)}
                        {m.linked && m.difference ? (
                          <button
                            type="button"
                            className={`${styles.iconButton} ${linking === m.row.id ? styles.iconButtonActive : ''}`}
                            title="Link more transactions"
                            aria-label={`Link more transactions to ${m.row.description}`}
                            aria-expanded={linking === m.row.id}
                            onClick={() => setLinking(linking === m.row.id ? null : m.row.id)}
                          >
                            <LinkSimple size={18} />
                          </button>
                        ) : null}
                        {m.linked && (
                          <button
                            type="button"
                            className={styles.iconButton}
                            title="Unlink"
                            aria-label={`Unlink ${m.row.description}`}
                            disabled={unlinking === m.row.id}
                            onClick={() => void unlink(m.row.id)}
                          >
                            {unlinking === m.row.id ? (
                              <CircleNotch size={18} className={styles.spin} />
                            ) : (
                              <LinkBreak size={18} />
                            )}
                          </button>
                        )}
                      </td>
                    </tr>
                    {editor(m.row, 5)}
                    {linking === m.row.id && (
                      <tr className={styles.linkRow}>
                        <td colSpan={5}>
                          <LinkCandidates
                            rows={[m.row, ...(m.otherRows ?? [])]}
                            candidates={section.ledgerOnly}
                            linked={[m.transaction, ...(m.others ?? [])].reduce((sum, t) => sum + t.amount, 0)}
                            onLink={(ids) => link(m.row.id, ids)}
                            onCancel={() => setLinking(null)}
                          />
                        </td>
                      </tr>
                    )}
                    {m.linked && m.difference && linking !== m.row.id ? (
                      <tr className={styles.linkRow}>
                        <td colSpan={5}>
                          <DifferenceForm difference={m.difference} onAdd={(body) => onAddDifference(m.row.id, body)} />
                        </td>
                      </tr>
                    ) : null}
                  </Fragment>
                ))}
              </tbody>
            </table>
          </div>
        ))}

      {tab === 'ledgerOnly' && (
        <>
          {section.ledgerOnly.length === 0 ? (
            <div className={styles.emptyTab}>
              <CheckCircle size={32} weight="fill" className={styles.okIcon} />
              <strong>Nothing extra</strong>
              <span>Every transaction in this period appears on the statement.</span>
            </div>
          ) : (
            <>
              <p className={styles.muted}>
                These are in {section.accountName ?? 'this account'} for the statement's period but not on the
                statement. Each could be a duplicate, a pending charge, or filed under the wrong account. If one belongs
                to another month's statement, set it aside.
              </p>
              <div className={styles.tableWrap}>
                <table className={styles.table}>
                  <thead>
                    <tr>
                      <th>Date</th>
                      <th>Payee</th>
                      <th>Category</th>
                      <th className={styles.num}>Amount</th>
                      <th className={styles.actionCell}>
                        <span className={styles.srOnly}>Link or set aside</span>
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {section.ledgerOnly.map((t) => (
                      <Fragment key={t.id}>
                        <tr>
                          <td className={styles.dateCell}>{shortDay(t.date)}</td>
                          <td>
                            <Link to={`/transactions/${t.id}`}>{t.payeeName || t.note || 'Open transaction'}</Link>
                            {t.status === 'UNAPPROVED' && <span className={styles.badgeMuted}>unapproved</span>}
                          </td>
                          <td>{t.categoryName ?? '—'}</td>
                          <td className={styles.num}>
                            <Amount value={t.amount} />
                          </td>
                          <td className={styles.actionCell}>
                            <div className={styles.rowActionsQuiet}>
                              <button
                                type="button"
                                className={styles.iconButton}
                                title="Not on this statement: it's another month's, or in the wrong account. Set it aside."
                                aria-label={`Set ${t.payeeName || 'this transaction'} aside: not on this statement`}
                                disabled={asiding === t.id}
                                onClick={() => void setAside(t, true)}
                              >
                                {asiding === t.id ? (
                                  <CircleNotch size={18} className={styles.spin} />
                                ) : (
                                  <CalendarX size={18} />
                                )}
                              </button>
                              <button
                                type="button"
                                className={`${styles.iconButton} ${pickingFor === t.id ? styles.iconButtonActive : ''}`}
                                title="Link to rows on the statement"
                                aria-label={`Link ${t.payeeName || 'this transaction'} to statement rows`}
                                aria-expanded={pickingFor === t.id}
                                onClick={() => setPickingFor(pickingFor === t.id ? null : t.id)}
                              >
                                <LinkSimple size={18} />
                              </button>
                            </div>
                          </td>
                        </tr>
                        {pickingFor === t.id && (
                          <tr className={styles.linkRow}>
                            <td colSpan={5}>
                              <RowPicker
                                txn={t}
                                rows={[...section.missing, ...(section.covered ?? [])]}
                                onLink={(rowIds) => linkMany(rowIds, [t.id])}
                                onCancel={() => setPickingFor(null)}
                              />
                            </td>
                          </tr>
                        )}
                      </Fragment>
                    ))}
                  </tbody>
                </table>
              </div>
            </>
          )}
          {section.setAside && section.setAside.length > 0 && (
            <details className={styles.setAside}>
              <summary>
                Set aside <span className={styles.count}>{section.setAside.length}</span>
                <span className={styles.muted}>not on this statement: another month's, or the wrong account</span>
              </summary>
              <ul>
                {section.setAside.map((t) => (
                  <li key={t.id}>
                    <span className={styles.dateCell}>{shortDay(t.date)}</span>
                    <Link to={`/transactions/${t.id}`}>{t.payeeName || t.note || 'Open transaction'}</Link>
                    <span className={styles.spacer} />
                    <Amount value={t.amount} />
                    <button
                      type="button"
                      className={styles.textButton}
                      disabled={asiding === t.id}
                      onClick={() => void setAside(t, false)}
                    >
                      Put back
                    </button>
                  </li>
                ))}
              </ul>
            </details>
          )}
        </>
      )}
      {tab === 'covered' && (
        <>
          <p className={styles.muted}>
            EMI instalments on this statement. They pay off purchases already in your budget, so they aren't missing.
            EMI interest is listed under Missing as a charge. If a row isn't an instalment, choose Not an EMI and it
            will be matched like any purchase. If you track an EMI as its own transaction, link the row to it.
          </p>
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th className={styles.checkCell}>
                    <span className={styles.srOnly}>Select</span>
                  </th>
                  <th>Date</th>
                  <th>Statement text</th>
                  <th>Instalment</th>
                  <th className={styles.num}>Amount</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {(section.covered ?? []).map((r) => (
                  <Fragment key={r.id}>
                    <tr className={coveredPicked.includes(r.id) ? styles.rowSelected : undefined}>
                      <td className={styles.checkCell}>
                        <input
                          type="checkbox"
                          aria-label={`Select ${r.description}`}
                          checked={coveredPicked.includes(r.id)}
                          onChange={(e) =>
                            setCoveredPicked((p) => (e.target.checked ? [...p, r.id] : p.filter((x) => x !== r.id)))
                          }
                        />
                      </td>
                      <td className={styles.dateCell}>
                        <RowDate row={r} />
                      </td>
                      <td>
                        <span className={styles.description} title={r.rawDescription || r.description}>
                          {r.description}
                        </span>
                      </td>
                      <td className={styles.dateCell}>
                        {r.emi?.part === 'conversion'
                          ? 'Converted to EMI'
                          : r.emi?.number && r.emi.of
                            ? `${r.emi.number} of ${r.emi.of}`
                            : '—'}
                      </td>
                      <td className={styles.num}>
                        <Amount value={signedAmount(r)} />
                      </td>
                      <td className={styles.num}>
                        <button
                          type="button"
                          className={styles.secondaryButton}
                          title="Move it to matching: it's a purchase, not an EMI instalment"
                          onClick={() => void onSetEMI(r.id, false)}
                        >
                          Not an EMI
                        </button>
                        <button
                          type="button"
                          className={`${styles.iconButton} ${linking === r.id ? styles.iconButtonActive : ''}`}
                          title="Link to a transaction already in Pennywise"
                          aria-label={`Link ${r.description} to a transaction`}
                          aria-expanded={linking === r.id}
                          onClick={() => setLinking(linking === r.id ? null : r.id)}
                        >
                          <LinkSimple size={18} />
                        </button>
                      </td>
                    </tr>
                    {linking === r.id && (
                      <tr className={styles.linkRow}>
                        <td colSpan={6}>
                          <LinkCandidates
                            rows={[r]}
                            candidates={section.ledgerOnly}
                            onLink={(ids) => link(r.id, ids)}
                            onCancel={() => setLinking(null)}
                          />
                        </td>
                      </tr>
                    )}
                  </Fragment>
                ))}
              </tbody>
            </table>
          </div>
          {coveredPicked.length > 0 && (
            <>
              <div className={styles.createBar}>
                <span>
                  {coveredPicked.length} EMI line{coveredPicked.length === 1 ? '' : 's'} selected
                </span>
                <span className={styles.spacer} />
                <button type="button" className={styles.ghostButton} onClick={() => setCoveredPicked([])}>
                  Clear
                </button>
                <button
                  type="button"
                  className={styles.primaryButton}
                  onClick={() => setLinkingCovered(!linkingCovered)}
                >
                  <LinkSimple size={18} /> Link to one transaction
                </button>
                {linkingCovered && (
                  <div className={styles.createBarPanel}>
                    <LinkCandidates
                      rows={(section.covered ?? []).filter((r) => coveredPicked.includes(r.id))}
                      candidates={section.ledgerOnly}
                      onLink={(ids) => linkMany(coveredPicked, ids)}
                      onCancel={() => setLinkingCovered(false)}
                    />
                  </div>
                )}
              </div>
              <div className={styles.createBarSpacer} aria-hidden />
            </>
          )}
        </>
      )}
    </section>
  );
}
