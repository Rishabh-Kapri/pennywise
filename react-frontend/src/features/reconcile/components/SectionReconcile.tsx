import { ArrowsLeftRight, CheckCircle, Info, WarningCircle } from '@phosphor-icons/react';
import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { useAppSelector } from '@/app/hooks';
import {
  signedAmount,
  type StatementCreateItem,
  type StatementCreateResult,
  type StatementSectionReconciliation,
} from '../types/reconcile.types';
import styles from './StatementReconcile.module.css';

type Tab = 'missing' | 'matched' | 'ledgerOnly';

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

interface RowDraft {
  selected: boolean;
  payeeName: string;
  categoryId: string;
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
  const [message, setMessage] = useState<{ ok: boolean; text: string } | null>(null);
  const [savingAccount, setSavingAccount] = useState(false);

  const draftFor = (rowId: string, fallbackPayee: string): RowDraft =>
    drafts[rowId] ?? { selected: false, payeeName: fallbackPayee, categoryId: '' };

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
    setMessage(null);
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
      const parts = [`Created ${res.created.length}`];
      if (res.skipped.length) parts.push(`${res.skipped.length} already present`);
      if (res.failed.length) parts.push(`${res.failed.length} failed: ${res.failed[0].error}`);
      setMessage({ ok: res.failed.length === 0, text: parts.join(' · ') });
      setDrafts({});
    } catch (err) {
      setMessage({ ok: false, text: err instanceof Error ? err.message : 'Could not create transactions' });
    } finally {
      setBusy(false);
    }
  };

  const changeAccount = async (value: string) => {
    setSavingAccount(true);
    try {
      await onAccountChange(value || null);
    } finally {
      setSavingAccount(false);
    }
  };

  const title = [section.accountLabel, section.accountNumber].filter(Boolean).join(' · ') || 'Statement';

  return (
    <section className={styles.sectionCard}>
      <header className={styles.sectionHeader}>
        <div>
          <h3>{title}</h3>
          <span className={styles.importMeta}>
            {section.periodStart} → {section.periodEnd} · {section.rowCount} rows
            {section.currency ? ` · ${section.currency}` : ''}
          </span>
        </div>
        <label className={styles.field}>
          <span>Reconcile against</span>
          <select
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
        </label>
      </header>

      {section.balanceCheck && (
        <p className={section.balanceCheck.ok ? styles.balanceOk : styles.warning}>
          {section.balanceCheck.ok ? <CheckCircle size={14} /> : <WarningCircle size={14} />}
          {section.balanceCheck.ok
            ? `Rows add up to the closing balance of ${money(section.balanceCheck.reported)}.`
            : `Rows add up to ${money(section.balanceCheck.computed)} but the statement closes at ${money(
                section.balanceCheck.reported,
              )} (off by ${money(Math.abs(section.balanceCheck.difference))}). A row may have been missed or misread.`}
        </p>
      )}

      {!section.accountId ? (
        <p className={styles.hint}>
          <Info size={14} /> Choose the account this section belongs to. The {section.rowCount} rows will then be matched
          against its transactions.
        </p>
      ) : (
        <>
          <div className={styles.tabs} role="tablist">
            <button
              type="button"
              role="tab"
              aria-selected={tab === 'missing'}
              className={tab === 'missing' ? styles.tabActive : styles.tab}
              onClick={() => setTab('missing')}>
              Missing in Pennywise <span className={styles.count}>{section.missing.length}</span>
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={tab === 'matched'}
              className={tab === 'matched' ? styles.tabActive : styles.tab}
              onClick={() => setTab('matched')}>
              Matched <span className={styles.count}>{section.matched.length}</span>
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={tab === 'ledgerOnly'}
              className={tab === 'ledgerOnly' ? styles.tabActive : styles.tab}
              onClick={() => setTab('ledgerOnly')}>
              Not on statement <span className={styles.count}>{section.ledgerOnly.length}</span>
            </button>
          </div>

          {tab === 'missing' &&
            (section.missing.length === 0 ? (
              <p className={styles.balanceOk}>
                <CheckCircle size={14} /> Every statement row has a matching transaction.
              </p>
            ) : (
              <>
                <datalist id={`payees-${section.index}`}>
                  {payees.map((p) => (
                    <option key={p.id} value={p.name} />
                  ))}
                </datalist>
                <div className={styles.tableWrap}>
                  <table className={styles.table}>
                    <thead>
                      <tr>
                        <th>
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
                            <td>
                              <input
                                type="checkbox"
                                aria-label={`Select ${row.description}`}
                                checked={d.selected}
                                onChange={(e) => updateDraft(row.id, row.payee, { selected: e.target.checked })}
                              />
                            </td>
                            <td className={styles.nowrap}>{row.date}</td>
                            <td className={styles.description}>
                              {row.description}
                              {row.directionCorrected && (
                                <span
                                  className={styles.badge}
                                  title="The running balance showed the model had debit and credit reversed">
                                  direction fixed
                                </span>
                              )}
                            </td>
                            <td>
                              <input
                                className={styles.cellInput}
                                list={`payees-${section.index}`}
                                value={d.payeeName}
                                placeholder={row.description}
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
                <div className={styles.createBar}>
                  <span className={styles.hint}>
                    Created as unapproved, so they show up for review. A payee name that doesn't exist yet is
                    created; pick a transfer payee for card payments between your accounts.
                  </span>
                  <button
                    type="button"
                    className={styles.primaryButton}
                    disabled={!selectedRows.length || busy}
                    onClick={() => void createSelected()}>
                    Create {selectedRows.length || ''} transaction{selectedRows.length === 1 ? '' : 's'}
                  </button>
                </div>
              </>
            ))}

          {tab === 'matched' && (
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>Statement</th>
                    <th />
                    <th>Pennywise</th>
                    <th className={styles.num}>Amount</th>
                  </tr>
                </thead>
                <tbody>
                  {section.matched.map((m) => (
                    <tr key={m.row.id}>
                      <td className={styles.description}>
                        <span className={styles.nowrap}>{m.row.date}</span> {m.row.description}
                      </td>
                      <td>
                        <ArrowsLeftRight size={14} />
                      </td>
                      <td className={styles.description}>
                        <Link to={`/transactions/${m.transaction.id}`}>
                          <span className={styles.nowrap}>{m.transaction.date}</span>{' '}
                          {m.transaction.payeeName || m.transaction.note}
                        </Link>
                        {m.dateDiffDays > 0 && (
                          <span className={styles.badgeMuted}>
                            {m.dateDiffDays} day{m.dateDiffDays === 1 ? '' : 's'} apart
                          </span>
                        )}
                        {m.transaction.status === 'UNAPPROVED' && (
                          <span className={styles.badgeMuted}>unapproved</span>
                        )}
                      </td>
                      <td className={styles.num}>
                        <Amount value={signedAmount(m.row)} />
                        {m.kind === 'amount_differs' && (
                          <span className={styles.badge} title="Close date and matching text, different amount">
                            ledger {money(m.transaction.amount)}
                          </span>
                        )}
                      </td>
                    </tr>
                  ))}
                  {section.matched.length === 0 && (
                    <tr>
                      <td colSpan={4} className={styles.hint}>
                        Nothing matched yet.
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          )}

          {tab === 'ledgerOnly' && (
            <div className={styles.tableWrap}>
              <p className={styles.hint}>
                In {section.accountName ?? 'this account'} between {section.periodStart} and {section.periodEnd}
                , but not on the statement: possibly a duplicate, a pending charge, or a transaction filed under
                the wrong account.
              </p>
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
                      <td className={styles.nowrap}>{t.date}</td>
                      <td className={styles.description}>
                        <Link to={`/transactions/${t.id}`}>{t.payeeName || t.note || '—'}</Link>
                        {t.status === 'UNAPPROVED' && <span className={styles.badgeMuted}>unapproved</span>}
                      </td>
                      <td>{t.categoryName ?? '—'}</td>
                      <td className={styles.num}>
                        <Amount value={t.amount} />
                      </td>
                    </tr>
                  ))}
                  {section.ledgerOnly.length === 0 && (
                    <tr>
                      <td colSpan={4} className={styles.hint}>
                        Every transaction in this period appears on the statement.
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          )}

          {message && <p className={message.ok ? styles.balanceOk : styles.error}>{message.text}</p>}
        </>
      )}
    </section>
  );
}
