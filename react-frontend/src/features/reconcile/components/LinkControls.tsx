import {
  ArrowCounterClockwise,
  CheckSquare,
  CircleNotch,
  FloppyDisk,
  LinkSimple,
  Plus,
  Square,
} from '@phosphor-icons/react';
import { useMemo, useState } from 'react';
import { CategoryDropdown } from '@/features/category/components/CategoryDropdown';
import { errorMessage, toast } from '@/utils';
import { formatDecimal } from '@/utils/format';
import {
  signedAmount,
  type StatementDifferenceRequest,
  type StatementLedgerTxn,
  type StatementRow,
  type StatementRowEdit,
} from '../types/reconcile.types';
import styles from './StatementReconcile.module.css';

function day(date: string): string {
  const d = new Date(`${date.slice(0, 10)}T00:00:00`);
  return Number.isNaN(d.getTime()) ? date : d.toLocaleDateString(undefined, { day: '2-digit', month: 'short' });
}

const signed = (v: number) => `${v < 0 ? '−' : '+'}${formatDecimal(Math.abs(v))}`;

const dayNumber = (date: string) => Date.parse(`${date.slice(0, 10)}T00:00:00Z`) / 86_400_000;

/**
 * The account's transactions that aren't on the statement, to pair with
 * statement rows by hand: the same amount first, then the nearest dates. Several can be
 * picked: one payment made in parts, or a row that bundles several. With
 * several rows (EMI lines one payment paid) they're compared in total. What's
 * left after them can be booked as a difference after linking.
 */
export function LinkCandidates({
  rows,
  candidates,
  linked = 0,
  onLink,
  onCancel,
}: {
  rows: StatementRow[];
  candidates: StatementLedgerTxn[];
  /** what the row's linked transactions already add up to */
  linked?: number;
  onLink: (transactionIds: string[]) => Promise<void>;
  onCancel: () => void;
}) {
  const [picked, setPicked] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const [query, setQuery] = useState('');
  const cents = (v: number) => Math.round(v * 100) / 100;
  const total = rows.reduce((sum, r) => sum + signedAmount(r), 0);
  // a long list is scanned top down: what most likely pays for the row first
  const sorted = useMemo(() => {
    const want = cents(total - linked);
    const at = dayNumber(rows[0]?.date ?? '');
    const score = (t: StatementLedgerTxn) => [cents(t.amount) === want ? 0 : 1, Math.abs(dayNumber(t.date) - at) || 0];
    return [...candidates].sort((a, b) => {
      const [x, y] = [score(a), score(b)];
      return x[0] - y[0] || x[1] - y[1];
    });
  }, [candidates, rows, total, linked]);
  const needle = query.trim().toLowerCase();
  const shown = needle
    ? sorted.filter((t) =>
        [t.payeeName, t.note, t.categoryName, formatDecimal(Math.abs(t.amount)), String(Math.abs(t.amount))]
          .filter(Boolean)
          .some((v) => v!.toLowerCase().includes(needle)),
      )
    : sorted;
  const left = cents(
    total - linked - candidates.filter((t) => picked.includes(t.id)).reduce((sum, t) => sum + t.amount, 0),
  );

  const toggle = (id: string) => setPicked((p) => (p.includes(id) ? p.filter((x) => x !== id) : [...p, id]));
  const link = async () => {
    setBusy(true);
    try {
      await onLink(picked);
    } catch (err) {
      toast.error("Couldn't link them", { description: errorMessage(err, 'Try again.') });
      setBusy(false);
    }
  };

  return (
    <div className={styles.linkPanel}>
      <div className={styles.linkPanelHeader}>
        <strong>
          {rows.length > 1
            ? `Link ${rows.length} rows (${signed(total)}) to transactions`
            : `Link ${signed(total)} on ${day(rows[0].date)} to transactions that aren't on the statement`}
        </strong>
        <button type="button" className={styles.textButton} onClick={onCancel}>
          Cancel
        </button>
      </div>
      {sorted.length === 0 ? (
        <p className={styles.muted}>Every transaction in this account for the statement's period is already matched.</p>
      ) : (
        <>
          {sorted.length > 6 && (
            <input
              className={styles.linkSearch}
              type="search"
              value={query}
              placeholder="Search payee, category or amount"
              aria-label="Search transactions to link"
              onChange={(e) => setQuery(e.target.value)}
            />
          )}
          {shown.length === 0 && <p className={styles.muted}>Nothing matches “{query.trim()}”.</p>}
          <ul className={styles.linkList}>
            {shown.map((t) => {
              const on = picked.includes(t.id);
              const apart = cents(left - t.amount);
              return (
                <li key={t.id}>
                  <button
                    type="button"
                    role="checkbox"
                    aria-checked={on}
                    className={`${styles.linkOption} ${on ? styles.linkOptionOn : ''}`}
                    disabled={busy}
                    onClick={() => toggle(t.id)}
                  >
                    {on ? <CheckSquare size={18} weight="fill" /> : <Square size={18} />}
                    <span className={styles.dateCell}>{day(t.date)}</span>
                    <span className={styles.linkPayee}>
                      {t.payeeName || t.note || 'Transaction'}
                      {t.categoryName && <small>{t.categoryName}</small>}
                    </span>
                    <span className={styles.linkAmount}>
                      <span className={t.amount < 0 ? styles.outflow : styles.inflow}>{signed(t.amount)}</span>
                      {!on && apart !== 0 && <small>{formatDecimal(Math.abs(apart))} apart</small>}
                    </span>
                  </button>
                </li>
              );
            })}
          </ul>
          <div className={styles.linkFooter}>
            <span className={styles.muted}>
              {picked.length === 0
                ? 'Pick one or more.'
                : left === 0
                  ? `${picked.length} picked · adds up exactly`
                  : `${picked.length} picked · ${formatDecimal(Math.abs(left))} ${left > 0 ? 'short' : 'over'}; you can book the difference after linking`}
            </span>
            <button
              type="button"
              className={styles.secondaryButton}
              disabled={busy || picked.length === 0}
              onClick={() => void link()}
            >
              {busy ? <CircleNotch size={16} className={styles.spin} /> : <LinkSimple size={16} />}
              Link {picked.length > 1 ? picked.length : ''}
            </button>
          </div>
        </>
      )}
    </div>
  );
}

/**
 * A linked row whose transactions don't add up to it: book the difference
 * (cashback used, a fee) as its own transaction on the account, linked to
 * the row too. It's created unapproved, like rows added from the statement.
 */
export function DifferenceForm({
  difference,
  onAdd,
}: {
  /** row amount minus the linked transactions' */
  difference: number;
  onAdd: (body: StatementDifferenceRequest) => Promise<void>;
}) {
  const [payeeName, setPayeeName] = useState('');
  const [category, setCategory] = useState<{ id: string; name: string }>({ id: '', name: '' });
  const [busy, setBusy] = useState(false);

  const add = async () => {
    setBusy(true);
    try {
      await onAdd({ payeeName: payeeName.trim() || undefined, categoryId: category.id || undefined });
      toast.success('Difference added', { description: 'It is unapproved, so you can review it in Transactions.' });
    } catch (err) {
      toast.error("Couldn't add the difference", { description: errorMessage(err, 'Try again.') });
      setBusy(false);
    }
  };

  return (
    <div className={styles.differenceForm}>
      <span>
        {difference > 0 ? 'Statement has' : 'Pennywise has'} <strong>{formatDecimal(Math.abs(difference))}</strong> more. Add it
        as its own transaction:
      </span>
      <input
        className={styles.cellInput}
        value={payeeName}
        placeholder="Payee, e.g. CRED"
        aria-label="Payee for the difference"
        onChange={(e) => setPayeeName(e.target.value)}
      />
      <div className={styles.categoryCell}>
        <CategoryDropdown
          variant="form"
          value={category.name}
          placeholder="Category"
          onClick={(id, name) => setCategory({ id, name })}
        />
      </div>
      <button
        type="button"
        className={styles.secondaryButton}
        disabled={busy || !payeeName.trim()}
        title={payeeName.trim() ? undefined : 'Name a payee first'}
        onClick={() => void add()}
      >
        {busy ? <CircleNotch size={16} className={styles.spin} /> : <Plus size={16} />}
        Add {signed(difference)}
      </button>
    </div>
  );
}

/**
 * From the ledger's side: statement rows to link one transaction to. Several
 * rows can share it: EMI lines one payment paid, or charges paid together.
 */
export function RowPicker({
  txn,
  rows,
  onLink,
  onCancel,
}: {
  txn: StatementLedgerTxn;
  /** rows still without a transaction: missing ones and EMI lines set aside */
  rows: StatementRow[];
  onLink: (rowIds: string[]) => Promise<void>;
  onCancel: () => void;
}) {
  const [picked, setPicked] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const sorted = [...rows].sort((a, b) => a.date.localeCompare(b.date));
  const cents = (v: number) => Math.round(v * 100) / 100;
  const left = cents(
    txn.amount - rows.filter((r) => picked.includes(r.id)).reduce((sum, r) => sum + signedAmount(r), 0),
  );
  const toggle = (id: string) => setPicked((p) => (p.includes(id) ? p.filter((x) => x !== id) : [...p, id]));
  const link = async () => {
    setBusy(true);
    try {
      await onLink(picked);
    } catch (err) {
      toast.error("Couldn't link them", { description: errorMessage(err, 'Try again.') });
      setBusy(false);
    }
  };

  return (
    <div className={styles.linkPanel}>
      <div className={styles.linkPanelHeader}>
        <strong>Statement rows this transaction pays for</strong>
        <button type="button" className={styles.textButton} onClick={onCancel}>
          Cancel
        </button>
      </div>
      {sorted.length === 0 ? (
        <p className={styles.muted}>Every row on this statement already has a transaction.</p>
      ) : (
        <>
          <ul className={styles.linkList}>
            {sorted.map((r) => {
              const on = picked.includes(r.id);
              const amount = signedAmount(r);
              return (
                <li key={r.id}>
                  <button
                    type="button"
                    role="checkbox"
                    aria-checked={on}
                    className={`${styles.linkOption} ${on ? styles.linkOptionOn : ''}`}
                    disabled={busy}
                    onClick={() => toggle(r.id)}
                  >
                    {on ? <CheckSquare size={18} weight="fill" /> : <Square size={18} />}
                    <span className={styles.dateCell}>{day(r.date)}</span>
                    <span className={styles.linkPayee}>
                      {r.description}
                      {r.emi && <small>EMI{r.emi.number && r.emi.of ? ` ${r.emi.number} of ${r.emi.of}` : ''}</small>}
                    </span>
                    <span className={styles.linkAmount}>
                      <span className={amount < 0 ? styles.outflow : styles.inflow}>{signed(amount)}</span>
                    </span>
                  </button>
                </li>
              );
            })}
          </ul>
          <div className={styles.linkFooter}>
            <span className={styles.muted}>
              {picked.length === 0
                ? 'Pick one or more.'
                : left === 0
                  ? `${picked.length} picked · adds up exactly`
                  : `${picked.length} picked · ${formatDecimal(Math.abs(left))} ${left < 0 ? 'more on the statement' : 'more in Pennywise'}`}
            </span>
            <button
              type="button"
              className={styles.secondaryButton}
              disabled={busy || picked.length === 0}
              onClick={() => void link()}
            >
              {busy ? <CircleNotch size={16} className={styles.spin} /> : <LinkSimple size={16} />}
              Link {picked.length > 1 ? picked.length : ''}
            </button>
          </div>
        </>
      )}
    </div>
  );
}

/**
 * Correct a row read wrong: its date, text, amount and whether money went
 * out or came in. What was read is kept, so it can be put back.
 */
export function RowEditor({
  row,
  onSave,
  onRevert,
  onCancel,
}: {
  row: StatementRow;
  onSave: (edit: StatementRowEdit) => Promise<void>;
  onRevert: () => Promise<void>;
  onCancel: () => void;
}) {
  const [date, setDate] = useState(row.date);
  const [time, setTime] = useState(row.time ?? '');
  const [description, setDescription] = useState(row.description);
  const [amount, setAmount] = useState(String(row.amount));
  const [direction, setDirection] = useState(row.direction);
  const [busy, setBusy] = useState(false);

  const run = async (fn: () => Promise<void>, failure: string) => {
    setBusy(true);
    try {
      await fn();
      onCancel();
    } catch (err) {
      toast.error(failure, { description: errorMessage(err, 'Try again.') });
      setBusy(false);
    }
  };

  const save = () => {
    const value = Number(amount.replace(/,/g, ''));
    if (!Number.isFinite(value) || value <= 0) {
      toast.error('Enter an amount more than 0', { description: 'Choose Money in or Money out for its direction.' });
      return;
    }
    const edit: StatementRowEdit = {};
    if (date !== row.date) edit.date = date;
    if (time !== (row.time ?? '')) edit.time = time;
    if (description.trim() !== row.description) edit.description = description.trim();
    if (value !== row.amount) edit.amount = value;
    if (direction !== row.direction) edit.direction = direction;
    if (Object.keys(edit).length === 0) {
      onCancel();
      return;
    }
    void run(() => onSave(edit), "Couldn't save the row");
  };

  return (
    <form
      className={styles.rowEditor}
      onSubmit={(e) => {
        e.preventDefault();
        save();
      }}
    >
      <label className={styles.field}>
        <span>Date</span>
        <input type="date" required value={date} onChange={(e) => setDate(e.target.value)} />
      </label>
      <label className={styles.field}>
        <span>Time</span>
        <input type="time" value={time} onChange={(e) => setTime(e.target.value)} />
      </label>
      <label className={`${styles.field} ${styles.rowEditorText}`}>
        <span>Statement text</span>
        <input required value={description} onChange={(e) => setDescription(e.target.value)} />
      </label>
      <label className={styles.field}>
        <span>Amount</span>
        <input inputMode="decimal" required value={amount} onChange={(e) => setAmount(e.target.value)} />
      </label>
      <div className={styles.field}>
        <span>Direction</span>
        <div className={styles.segmented} role="radiogroup" aria-label="Direction">
          {(['debit', 'credit'] as const).map((d) => (
            <button
              key={d}
              type="button"
              role="radio"
              aria-checked={direction === d}
              className={direction === d ? styles.segmentOn : undefined}
              onClick={() => setDirection(d)}
            >
              {d === 'debit' ? 'Money out' : 'Money in'}
            </button>
          ))}
        </div>
      </div>
      <div className={styles.rowEditorActions}>
        {row.edited && (
          <button
            type="button"
            className={styles.textButton}
            disabled={busy}
            title={`Read as ${row.edited.description}, ${row.edited.direction === 'debit' ? '−' : '+'}${formatDecimal(row.edited.amount)} on ${day(row.edited.date)}${row.edited.time ? ` at ${row.edited.time}` : ''}`}
            onClick={() => void run(onRevert, "Couldn't restore the row")}
          >
            <ArrowCounterClockwise size={15} /> Restore what was read
          </button>
        )}
        <span className={styles.spacer} />
        <button type="button" className={styles.ghostButton} disabled={busy} onClick={onCancel}>
          Cancel
        </button>
        <button type="submit" className={styles.secondaryButton} disabled={busy}>
          {busy ? <CircleNotch size={16} className={styles.spin} /> : <FloppyDisk size={16} />}
          Save
        </button>
      </div>
    </form>
  );
}
