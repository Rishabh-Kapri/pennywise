import { Bank, CheckCircle, CircleNotch, Plus, WarningCircle } from '@phosphor-icons/react';
import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { useAppSelector } from '@/app/hooks';
import { errorMessage, toast } from '@/utils';
import {
  signedAmount,
  type StatementCreateItem,
  type StatementCreateResult,
  type StatementRow,
  type StatementSectionReconciliation,
} from '../types/reconcile.types';
import { CardSummary } from './CardSummary';
import styles from './StatementReconcile.module.css';

type Tab = 'missing' | 'matched' | 'ledgerOnly' | 'covered';

// paise matter here: a reconciliation that rounds hides the 0.76 it is about
const money = (value: number) =>
  new Intl.NumberFormat('en-IN', { minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(value);

function Amount({ value }: { value: number }) {
  return (
    <span className={value < 0 ? styles.outflow : styles.inflow}>
      {value < 0 ? '−' : '+'}
      {money(Math.abs(value))}
    </span>
  );
}

// last four digits of a printed, usually masked, account number
function accountEnding(number: string): string {
  const digits = number.replace(/\D/g, '');
  return digits.length >= 4 ? digits.slice(-4) : '';
}

// "01 Oct" from the API's YYYY-MM-DD; anything else is shown as it came
function shortDay(date: string): string {
  const d = new Date(`${date}T00:00:00`);
  if (Number.isNaN(d.getTime())) return date;
  return d.toLocaleDateString(undefined, { day: '2-digit', month: 'short' });
}

const CHARGE_BADGES: Record<string, string> = { tax: 'tax', interest: 'interest', fee: 'fee' };

function RowBadges({ row }: { row: StatementRow }) {
  return (
    <>
      {row.valueDateMismatch && row.valueDate && (
        <span className={styles.badge} title="The value date printed in the narration is days away from the row's date">
          value date {shortDay(row.valueDate)}
        </span>
      )}
      {row.chargeKind && <span className={styles.badge}>{CHARGE_BADGES[row.chargeKind]}</span>}
      {row.reference && <span className={styles.badgeMuted}>Ref {row.reference}</span>}
    </>
  );
}

interface RowDraft {
  selected: boolean;
  payeeName: string;
  categoryId: string;
}

// "1 Oct – 31 Oct 2026"; the API sends YYYY-MM-DD, or nothing when the
// statement didn't say
function formatPeriod(start: string, end: string): string {
  const parse = (d: string) => (d ? new Date(`${d}T00:00:00`) : null);
  const from = parse(start);
  const to = parse(end);
  if (!from || !to || Number.isNaN(from.getTime()) || Number.isNaN(to.getTime())) return '';
  const dayMonth = { day: 'numeric', month: 'short' } as const;
  const sameYear = from.getFullYear() === to.getFullYear();
  const left = from.toLocaleDateString(undefined, sameYear ? dayMonth : { ...dayMonth, year: 'numeric' });
  const right = to.toLocaleDateString(undefined, { ...dayMonth, year: 'numeric' });
  return `${left} – ${right}`;
}

interface Props {
  section: StatementSectionReconciliation;
  accounts: { id?: string; name: string }[];
  onAccountChange: (accountId: string | null) => Promise<void>;
  onCreate: (items: StatementCreateItem[]) => Promise<StatementCreateResult>;
}

export function SectionReconcile({ section, accounts, onAccountChange, onCreate }: Props) {
  const payees = useAppSelector((state) => state.payees.allPayees);
  const categories = useAppSelector((state) => state.categories.allCategories);
  const visibleCategories = useMemo(
    () => categories.filter((c) => !c.deleted && !c.hidden && c.id).sort((a, b) => a.name.localeCompare(b.name)),
    [categories],
  );

  const [tab, setTab] = useState<Tab>(section.missing.length > 0 ? 'missing' : 'matched');
  const [drafts, setDrafts] = useState<Record<string, RowDraft>>({});
  const [busy, setBusy] = useState(false);
  const [savingAccount, setSavingAccount] = useState(false);

  // a charge row starts with the category the API suggested for it
  const suggested = useMemo(
    () => Object.fromEntries(section.missing.map((r) => [r.id, r.suggestedCategoryId ?? ''])),
    [section.missing],
  );
  const draftFor = (rowId: string, fallbackPayee: string): RowDraft =>
    drafts[rowId] ?? { selected: false, payeeName: fallbackPayee, categoryId: suggested[rowId] ?? '' };

  const updateDraft = (rowId: string, fallbackPayee: string, patch: Partial<RowDraft>) =>
    setDrafts((prev) => ({ ...prev, [rowId]: { ...draftFor(rowId, fallbackPayee), ...patch } }));

  const selectedRows = section.missing.filter((r) => draftFor(r.id, r.payee).selected);
  const allSelected = section.missing.length > 0 && selectedRows.length === section.missing.length;

  const toggleAll = () => {
    const next = !allSelected;
    setDrafts((prev) => {
      const out = { ...prev };
      for (const r of section.missing) out[r.id] = { ...draftFor(r.id, r.payee), selected: next };
      return out;
    });
  };

  const createSelected = async () => {
    if (!selectedRows.length) return;
    setBusy(true);
    try {
      const res = await onCreate(
        selectedRows.map((r) => {
          const d = draftFor(r.id, r.payee);
          return {
            rowId: r.id,
            payeeName: d.payeeName.trim() || undefined,
            categoryId: d.categoryId || undefined,
          };
        }),
      );
      const parts: string[] = [];
      if (res.skipped.length) parts.push(`${res.skipped.length} already there`);
      if (res.failed.length) parts.push(`${res.failed.length} failed: ${res.failed[0].error}`);
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

  const accountSelect = (
    <select
      className={styles.accountSelect}
      aria-label="Account"
      value={section.accountId ?? ''}
      disabled={savingAccount}
      onChange={(e) => void changeAccount(e.target.value)}>
      <option value="">Choose an account…</option>
      {accounts.map((acc) => (
        <option key={acc.id} value={acc.id}>
          {acc.name}
        </option>
      ))}
    </select>
  );

  if (!section.accountId) {
    return (
      <section className={styles.stateCard}>
        <Bank size={32} className={styles.stateIcon} />
        <h3>Which account is this statement for?</h3>
        <p className={styles.muted}>{facts}</p>
        {ending && (
          <p className={styles.muted}>
            No account ends in {ending}. Set an account's last digits to {ending} and future statements will be
            matched to it automatically.
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
        <p className={styles.muted}>{facts}</p>
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
              {money(section.balanceCheck.reported)} (off by {money(Math.abs(section.balanceCheck.difference))}). A
              row may have been missed or misread.
            </span>
          </p>
        ))}

      <div className={styles.tabs} role="tablist">
        {tabs.map((t) => (
          <button
            key={t.id}
            type="button"
            role="tab"
            aria-selected={tab === t.id}
            className={`${styles.tab} ${tab === t.id ? styles.tabActive : ''}`}
            onClick={() => setTab(t.id)}>
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
            <datalist id={`payees-${section.index}`}>
              {payees.map((p) => (
                <option key={p.id} value={p.name} />
              ))}
            </datalist>
            <div className={styles.tableWrap}>
              <table className={styles.table}>
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
                    <th>Statement text</th>
                    <th>Payee</th>
                    <th>Category</th>
                    <th className={styles.num}>Amount</th>
                  </tr>
                </thead>
                <tbody>
                  {section.missing.map((row) => {
                    const d = draftFor(row.id, row.payee);
                    return (
                      <tr key={row.id} className={d.selected ? styles.rowSelected : undefined}>
                        <td className={styles.checkCell}>
                          <input
                            type="checkbox"
                            aria-label={`Select ${row.description}`}
                            checked={d.selected}
                            onChange={(e) => updateDraft(row.id, row.payee, { selected: e.target.checked })}
                          />
                        </td>
                        <td className={styles.dateCell}>{shortDay(row.date)}</td>
                        <td>
                          <span className={styles.description} title={row.rawDescription || row.description}>
                            {row.description}
                          </span>
                          <RowBadges row={row} />
                          {row.directionCorrected && (
                            <span
                              className={styles.badge}
                              title="The running balance showed debit and credit were reversed">
                              direction fixed
                            </span>
                          )}
                        </td>
                        <td>
                          <input
                            className={styles.cellInput}
                            list={`payees-${section.index}`}
                            value={d.payeeName}
                            placeholder="Payee"
                            onChange={(e) => updateDraft(row.id, row.payee, { payeeName: e.target.value })}
                          />
                        </td>
                        <td>
                          <select
                            className={styles.cellInput}
                            value={d.categoryId}
                            onChange={(e) => updateDraft(row.id, row.payee, { categoryId: e.target.value })}>
                            <option value="">Uncategorized</option>
                            {visibleCategories.map((c) => (
                              <option key={c.id} value={c.id}>
                                {c.name}
                              </option>
                            ))}
                          </select>
                        </td>
                        <td className={styles.num}>
                          <Amount value={signedAmount(row)} />
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
            {selectedRows.length > 0 && (
              <div className={styles.createBar}>
                <span>
                  {selectedRows.length} of {section.missing.length} selected
                </span>
                <button
                  type="button"
                  className={styles.primaryButton}
                  disabled={busy}
                  onClick={() => void createSelected()}>
                  {busy ? <CircleNotch size={18} className={styles.spin} /> : <Plus size={18} />}
                  {selectedRows.length > 1 ? `Add ${selectedRows.length} transactions` : 'Add transaction'}
                </button>
              </div>
            )}
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
                </tr>
              </thead>
              <tbody>
                {section.matched.map((m) => (
                  <tr key={m.row.id}>
                    <td className={styles.dateCell}>{shortDay(m.row.date)}</td>
                    <td>
                      <span className={styles.description} title={m.row.rawDescription || m.row.description}>
                        {m.row.description}
                      </span>
                      <RowBadges row={m.row} />
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
                    </td>
                    <td className={styles.num}>
                      <Amount value={signedAmount(m.row)} />
                      {m.kind === 'amount_differs' && (
                        <span className={styles.badge} title="Close date and matching text, different amount">
                          Pennywise has {money(m.transaction.amount)}
                        </span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ))}

      {tab === 'ledgerOnly' &&
        (section.ledgerOnly.length === 0 ? (
          <div className={styles.emptyTab}>
            <CheckCircle size={32} weight="fill" className={styles.okIcon} />
            <strong>Nothing extra</strong>
            <span>Every transaction in this period appears on the statement.</span>
          </div>
        ) : (
          <>
            <p className={styles.muted}>
              These are in {section.accountName ?? 'this account'} for the statement's period but not on the statement.
              Each could be a duplicate, a pending charge, or filed under the wrong account.
            </p>
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>Date</th>
                    <th>Payee</th>
                    <th>Category</th>
                    <th className={styles.num}>Amount</th>
                  </tr>
                </thead>
                <tbody>
                  {section.ledgerOnly.map((t) => (
                    <tr key={t.id}>
                      <td className={styles.dateCell}>{shortDay(t.date)}</td>
                      <td>
                        <Link to={`/transactions/${t.id}`}>{t.payeeName || t.note || 'Open transaction'}</Link>
                        {t.status === 'UNAPPROVED' && <span className={styles.badgeMuted}>unapproved</span>}
                      </td>
                      <td>{t.categoryName ?? '—'}</td>
                      <td className={styles.num}>
                        <Amount value={t.amount} />
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </>
        ))}
      {tab === 'covered' && (
        <>
          <p className={styles.muted}>
            EMI instalments on this statement. They pay off purchases already in your budget, so they aren't missing.
            EMI interest is listed under Missing as a charge.
          </p>
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>Date</th>
                  <th>Statement text</th>
                  <th>Instalment</th>
                  <th className={styles.num}>Amount</th>
                </tr>
              </thead>
              <tbody>
                {(section.covered ?? []).map((r) => (
                  <tr key={r.id}>
                    <td className={styles.dateCell}>{shortDay(r.date)}</td>
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
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}
    </section>
  );
}
