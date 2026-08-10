import { useEffect, useMemo, useState } from 'react';
import {
  ArrowsClockwiseIcon,
  PauseIcon,
  PencilSimpleIcon,
  PlayIcon,
  PlusIcon,
  TrashIcon,
  XIcon,
} from '@phosphor-icons/react';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { LoadingState } from '@/utils';
import { getCurrencyLocaleString, getLocaleDate, getTodaysDate } from '@/utils/date.utils';
import { AccountDropdown } from '@/features/transactions/components/popovers/AccountPopover';
import { PayeeDropdown } from '@/features/transactions/components/popovers/PayeePopover';
import { CategoryDropdown } from '@/features/transactions/components/popovers/CategoryPopover';
import {
  createRecurringTransaction,
  deleteRecurringTransaction,
  fetchRecurringTransactions,
  runRecurringTransactions,
  selectRecurringError,
  selectRecurringLastRun,
  selectRecurringLoading,
  selectRecurringRules,
  selectRecurringSaving,
  updateRecurringTransaction,
} from '../store/recurringSlice';
import {
  describeSchedule,
  type RecurringFrequency,
  type RecurringTransaction,
  type RecurringTransactionDraft,
} from '../types/recurring.types';
import styles from './Recurring.module.css';

const FREQUENCIES: { value: RecurringFrequency; label: string }[] = [
  { value: 'DAILY', label: 'Daily' },
  { value: 'WEEKLY', label: 'Weekly' },
  { value: 'MONTHLY', label: 'Monthly' },
  { value: 'YEARLY', label: 'Yearly' },
];

interface FormState {
  name: string;
  accountId: string;
  accountName: string;
  payeeId: string;
  payeeName: string;
  categoryId: string;
  categoryName: string;
  amount: string;
  isInflow: boolean;
  note: string;
  frequency: RecurringFrequency;
  intervalCount: string;
  nextDate: string;
  endDate: string;
  paused: boolean;
}

const EMPTY_FORM: FormState = {
  name: '',
  accountId: '',
  accountName: '',
  payeeId: '',
  payeeName: '',
  categoryId: '',
  categoryName: '',
  amount: '',
  isInflow: false,
  note: '',
  frequency: 'MONTHLY',
  intervalCount: '1',
  nextDate: getTodaysDate(),
  endDate: '',
  paused: false,
};

function toForm(rule: RecurringTransaction): FormState {
  return {
    name: rule.name,
    accountId: rule.accountId,
    accountName: rule.accountName ?? '',
    payeeId: rule.payeeId ?? '',
    payeeName: rule.payeeName ?? '',
    categoryId: rule.categoryId ?? '',
    categoryName: rule.categoryName ?? '',
    amount: String(Math.abs(rule.amount)),
    isInflow: rule.amount > 0,
    note: rule.note ?? '',
    frequency: rule.frequency,
    intervalCount: String(rule.intervalCount),
    nextDate: rule.nextDate,
    endDate: rule.endDate ?? '',
    paused: rule.paused,
  };
}

function toDraft(form: FormState): RecurringTransactionDraft {
  const magnitude = Math.abs(Number(form.amount) || 0);
  return {
    name: form.name.trim(),
    accountId: form.accountId,
    payeeId: form.payeeId || null,
    categoryId: form.categoryId || null,
    amount: form.isInflow ? magnitude : -magnitude,
    note: form.note,
    frequency: form.frequency,
    intervalCount: Math.max(1, Number(form.intervalCount) || 1),
    nextDate: form.nextDate,
    endDate: form.endDate || null,
    paused: form.paused,
  };
}

function RuleForm({
  form,
  setForm,
  onSubmit,
  onCancel,
  saving,
  isEditing,
}: {
  form: FormState;
  setForm: (form: FormState) => void;
  onSubmit: () => void;
  onCancel: () => void;
  saving: boolean;
  isEditing: boolean;
}) {
  const update = (patch: Partial<FormState>) => setForm({ ...form, ...patch });
  const isValid =
    form.name.trim() !== '' && form.accountId !== '' && form.payeeId !== '' && Number(form.amount) > 0;

  return (
    <div className={styles.formCard}>
      <div className={styles.formHeader}>
        <h2>{isEditing ? 'Edit recurring transaction' : 'New recurring transaction'}</h2>
        <button type="button" className={styles.iconBtn} onClick={onCancel} aria-label="Close form">
          <XIcon size={18} />
        </button>
      </div>

      <div className={styles.formGrid}>
        <label className={styles.field}>
          <span className={styles.fieldLabel}>Name</span>
          <input
            className={styles.input}
            placeholder="Rent, Netflix, Salary…"
            value={form.name}
            onChange={(e) => update({ name: e.target.value })}
            aria-label="Name"
          />
        </label>

        <div className={styles.field}>
          <span className={styles.fieldLabel}>Amount</span>
          <div className={styles.amountRow}>
            <div className={styles.typeToggle}>
              <button
                type="button"
                className={!form.isInflow ? styles.typeBtnActive : styles.typeBtn}
                onClick={() => update({ isInflow: false })}>
                Outflow
              </button>
              <button
                type="button"
                className={form.isInflow ? styles.typeBtnActive : styles.typeBtn}
                onClick={() => update({ isInflow: true })}>
                Inflow
              </button>
            </div>
            <input
              className={styles.input}
              type="number"
              min="0"
              placeholder="0"
              value={form.amount}
              onChange={(e) => update({ amount: e.target.value })}
              aria-label="Amount"
            />
          </div>
        </div>

        <div className={styles.field}>
          <span className={styles.fieldLabel}>Account</span>
          <AccountDropdown
            value={form.accountName}
            variant="form"
            onClick={(id, name) => update({ accountId: id, accountName: name })}
          />
        </div>

        <div className={styles.field}>
          <span className={styles.fieldLabel}>Payee</span>
          <PayeeDropdown
            value={form.payeeName}
            variant="form"
            onClick={(id, name) => update({ payeeId: id, payeeName: name })}
          />
        </div>

        <div className={styles.field}>
          <span className={styles.fieldLabel}>Category</span>
          <CategoryDropdown
            value={form.categoryName}
            variant="form"
            onClick={(id, name) => update({ categoryId: id, categoryName: name })}
          />
        </div>

        <div className={styles.field}>
          <span className={styles.fieldLabel}>Repeats</span>
          <div className={styles.repeatRow}>
            <span className={styles.everyLabel}>Every</span>
            <input
              className={styles.intervalInput}
              type="number"
              min="1"
              value={form.intervalCount}
              onChange={(e) => update({ intervalCount: e.target.value })}
              aria-label="Interval"
            />
            <select
              className={styles.select}
              value={form.frequency}
              onChange={(e) => update({ frequency: e.target.value as RecurringFrequency })}
              aria-label="Frequency">
              {FREQUENCIES.map((f) => (
                <option key={f.value} value={f.value}>
                  {f.label}
                </option>
              ))}
            </select>
          </div>
        </div>

        <label className={styles.field}>
          <span className={styles.fieldLabel}>Next date</span>
          <input
            className={styles.input}
            type="date"
            value={form.nextDate}
            onChange={(e) => update({ nextDate: e.target.value })}
            aria-label="Next date"
          />
        </label>

        <label className={styles.field}>
          <span className={styles.fieldLabel}>End date (optional)</span>
          <input
            className={styles.input}
            type="date"
            value={form.endDate}
            min={form.nextDate}
            onChange={(e) => update({ endDate: e.target.value })}
            aria-label="End date"
          />
        </label>

        <label className={styles.fieldWide}>
          <span className={styles.fieldLabel}>Note</span>
          <input
            className={styles.input}
            placeholder="Added to each generated transaction"
            value={form.note}
            onChange={(e) => update({ note: e.target.value })}
            aria-label="Note"
          />
        </label>
      </div>

      <div className={styles.formActions}>
        <button type="button" className={styles.secondaryBtn} onClick={onCancel}>
          Cancel
        </button>
        <button
          type="button"
          className={styles.primaryBtn}
          onClick={onSubmit}
          disabled={!isValid || saving}>
          {saving ? 'Saving…' : isEditing ? 'Save changes' : 'Create'}
        </button>
      </div>
    </div>
  );
}

export default function Recurring() {
  const dispatch = useAppDispatch();
  const rules = useAppSelector(selectRecurringRules);
  const loading = useAppSelector(selectRecurringLoading);
  const saving = useAppSelector(selectRecurringSaving);
  const error = useAppSelector(selectRecurringError);
  const lastRun = useAppSelector(selectRecurringLastRun);

  const [form, setForm] = useState<FormState | null>(null);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null);

  useEffect(() => {
    if (loading === LoadingState.IDLE) {
      dispatch(fetchRecurringTransactions());
    }
  }, [dispatch, loading]);

  const today = getTodaysDate();
  const dueCount = useMemo(
    () => rules.filter((rule) => !rule.paused && rule.nextDate <= today).length,
    [rules, today],
  );

  const handleSubmit = async () => {
    if (!form) return;
    const draft = toDraft(form);
    if (editingId) {
      await dispatch(updateRecurringTransaction({ id: editingId, draft }));
    } else {
      await dispatch(createRecurringTransaction(draft));
    }
    setForm(null);
    setEditingId(null);
  };

  const startEdit = (rule: RecurringTransaction) => {
    setEditingId(rule.id);
    setForm(toForm(rule));
  };

  const togglePaused = (rule: RecurringTransaction) => {
    const draft = toDraft({ ...toForm(rule), paused: !rule.paused });
    dispatch(updateRecurringTransaction({ id: rule.id, draft }));
  };

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <div>
          <h1>Recurring</h1>
          <p className={styles.subtitle}>
            Scheduled transactions are created automatically when they come due.
          </p>
        </div>
        <div className={styles.headerActions}>
          <button
            type="button"
            className={styles.secondaryBtn}
            onClick={() => dispatch(runRecurringTransactions())}
            title="Create everything that is due right now">
            <ArrowsClockwiseIcon size={16} />
            Run due{dueCount > 0 ? ` (${dueCount})` : ''}
          </button>
          <button
            type="button"
            className={styles.primaryBtn}
            onClick={() => {
              setEditingId(null);
              setForm({ ...EMPTY_FORM });
            }}>
            <PlusIcon size={16} />
            New
          </button>
        </div>
      </div>

      {error && <div className={styles.error}>{error}</div>}
      {lastRun && (
        <div className={styles.runResult}>
          Created {lastRun.created} transaction{lastRun.created === 1 ? '' : 's'}
          {lastRun.skipped > 0 ? ` · ${lastRun.skipped} rule(s) skipped` : ''}
        </div>
      )}

      {form && (
        <RuleForm
          form={form}
          setForm={setForm}
          onSubmit={handleSubmit}
          onCancel={() => {
            setForm(null);
            setEditingId(null);
          }}
          saving={saving}
          isEditing={editingId !== null}
        />
      )}

      {loading === LoadingState.PENDING && rules.length === 0 ? (
        <div className={styles.emptyState}>Loading…</div>
      ) : rules.length === 0 ? (
        <div className={styles.emptyState}>
          No recurring transactions yet. Create one for rent, salary, or a subscription.
        </div>
      ) : (
        <ul className={styles.ruleList}>
          {rules.map((rule) => {
            const isDue = !rule.paused && rule.nextDate <= today;
            return (
              <li key={rule.id} className={rule.paused ? styles.ruleCardPaused : styles.ruleCard}>
                <div className={styles.ruleMain}>
                  <div className={styles.ruleTitleRow}>
                    <span className={styles.ruleName}>{rule.name}</span>
                    {rule.paused && <span className={styles.pausedBadge}>Paused</span>}
                    {isDue && <span className={styles.dueBadge}>Due</span>}
                  </div>
                  <div className={styles.ruleMeta}>
                    {describeSchedule(rule.frequency, rule.intervalCount)}
                    {' · next '}
                    {getLocaleDate(rule.nextDate, { month: 'short', day: 'numeric', year: 'numeric' })}
                    {rule.endDate ? ` · until ${getLocaleDate(rule.endDate, { month: 'short', day: 'numeric', year: 'numeric' })}` : ''}
                  </div>
                  <div className={styles.ruleMeta}>
                    {rule.payeeName ?? 'No payee'}
                    {' · '}
                    {rule.categoryName ?? 'Uncategorized'}
                    {' · '}
                    {rule.accountName}
                  </div>
                </div>

                <div className={styles.ruleRight}>
                  <span className={rule.amount > 0 ? styles.amountInflow : styles.amountOutflow}>
                    {rule.amount > 0 ? '+' : '−'}
                    {getCurrencyLocaleString(Math.abs(rule.amount))}
                  </span>
                  <div className={styles.ruleActions}>
                    {confirmDeleteId === rule.id ? (
                      <>
                        <button
                          type="button"
                          className={styles.deleteConfirmBtn}
                          onClick={() => {
                            dispatch(deleteRecurringTransaction(rule.id));
                            setConfirmDeleteId(null);
                          }}>
                          Delete?
                        </button>
                        <button
                          type="button"
                          className={styles.iconBtn}
                          onClick={() => setConfirmDeleteId(null)}
                          aria-label="Cancel delete">
                          <XIcon size={16} />
                        </button>
                      </>
                    ) : (
                      <>
                        <button
                          type="button"
                          className={styles.iconBtn}
                          onClick={() => togglePaused(rule)}
                          title={rule.paused ? 'Resume' : 'Pause'}
                          aria-label={rule.paused ? 'Resume rule' : 'Pause rule'}>
                          {rule.paused ? <PlayIcon size={16} /> : <PauseIcon size={16} />}
                        </button>
                        <button
                          type="button"
                          className={styles.iconBtn}
                          onClick={() => startEdit(rule)}
                          title="Edit"
                          aria-label="Edit rule">
                          <PencilSimpleIcon size={16} />
                        </button>
                        <button
                          type="button"
                          className={styles.iconBtn}
                          onClick={() => setConfirmDeleteId(rule.id)}
                          title="Delete"
                          aria-label="Delete rule">
                          <TrashIcon size={16} />
                        </button>
                      </>
                    )}
                  </div>
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
