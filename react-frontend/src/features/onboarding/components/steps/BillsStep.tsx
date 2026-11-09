import { useEffect, useState } from 'react';
import { ArrowLeft, ArrowRight, Plus, Trash } from '@phosphor-icons/react';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { createCategory, fetchAllCategoryGroups, updateCategoryGoal } from '@/features/category/store/categorySlice';
import type { CategoryGroup } from '@/features/category/types/category.types';
import { apiClient, errorMessage, getCurrentMonthKey, toast } from '@/utils';
import type { StepProps } from '../OnboardingWizard';
import styles from './steps.module.css';

interface BillRow {
  key: number;
  name: string;
  amount: string;
  day: string;
}

const PRESETS = ['Rent', 'Electricity', 'Phone', 'Internet', 'Subscriptions', 'Loan EMI', 'Insurance'];
const BILLS_GROUP = 'Bills';

let nextKey = 1;
const emptyRow = (name = ''): BillRow => ({ key: nextKey++, name, amount: '', day: '' });

/**
 * Fixed monthly bills become categories with a monthly target, so the budget
 * opens with real numbers instead of empty category names.
 */
export function BillsStep({ onNext, onBack, busy }: StepProps) {
  const dispatch = useAppDispatch();
  const budgetId = useAppSelector((state) => state.budgets.selectedBudget?.id);
  const groups = useAppSelector((state) => state.categories.allCategoryGroups);
  const [rows, setRows] = useState<BillRow[]>([emptyRow()]);
  const [saving, setSaving] = useState(false);
  const month = getCurrentMonthKey();

  useEffect(() => {
    dispatch(fetchAllCategoryGroups(month));
  }, [dispatch, month]);

  const valid = rows.filter((row) => row.name.trim() && Number(row.amount) > 0);
  const invalidDay = rows.some((row) => row.day && !(Number(row.day) >= 1 && Number(row.day) <= 31));

  const update = (key: number, changes: Partial<BillRow>) =>
    setRows((current) => current.map((row) => (row.key === key ? { ...row, ...changes } : row)));

  const addPreset = (name: string) => {
    setRows((current) => {
      const blank = current.find((row) => !row.name.trim());
      if (blank) return current.map((row) => (row.key === blank.key ? { ...row, name } : row));
      return [...current, emptyRow(name)];
    });
  };

  const billsGroup = async (): Promise<string> => {
    const existing = groups.find((group) => !group.isSystem && group.name.toLowerCase() === BILLS_GROUP.toLowerCase());
    if (existing?.id) return existing.id;
    const created = await apiClient.post<CategoryGroup, { name: string; budgetId: string }>('category-groups', {
      name: BILLS_GROUP,
      budgetId: budgetId ?? '',
    });
    if (!created.id) throw new Error('The Bills group was not created');
    return created.id;
  };

  const save = async () => {
    if (!budgetId || invalidDay) return;
    if (valid.length === 0) {
      await onNext({ skipped: true });
      return;
    }
    setSaving(true);
    try {
      const groupId = await billsGroup();
      const byName = new Map(
        groups.flatMap((group) => group.categories).map((category) => [category.name.toLowerCase(), category]),
      );
      for (const row of valid) {
        const name = row.name.trim();
        // A template category with the same name gets the target instead of a duplicate.
        const categoryId =
          byName.get(name.toLowerCase())?.id ??
          (await dispatch(createCategory({ name, budgetId, categoryGroupId: groupId })).unwrap()).id;
        await dispatch(
          updateCategoryGoal({
            categoryId,
            goal: {
              type: 'NEED',
              target: Number(row.amount),
              cadence: 'monthly',
              cadenceFrequency: 1,
              day: row.day ? Number(row.day) : undefined,
              needsWholeAmount: false,
              creationMonth: month,
            },
          }),
        ).unwrap();
      }
      dispatch(fetchAllCategoryGroups(month));
      toast.success(`${valid.length} bill${valid.length === 1 ? '' : 's'} added`);
      await onNext();
    } catch (err) {
      toast.error('Could not save your bills', { description: errorMessage(err, 'Try again.') });
    } finally {
      setSaving(false);
    }
  };

  return (
    <>
      <p className={styles.eyebrow}>Step 2 · Optional</p>
      <h1 className={styles.title}>What do you pay every month?</h1>
      <p className={styles.lead}>
        Each bill becomes a category with a monthly target, so you'll know exactly what to set aside. Leave the day
        empty if it varies.
      </p>

      <div className={`${styles.section} ${styles.chips}`}>
        {PRESETS.map((preset) => (
          <button
            key={preset}
            type="button"
            className={styles.segment}
            onClick={() => addPreset(preset)}
            disabled={rows.some((row) => row.name === preset)}
          >
            <Plus size={12} /> {preset}
          </button>
        ))}
      </div>

      <div className={`${styles.section} ${styles.panel} ${styles.form}`}>
        {rows.map((row) => (
          <div key={row.key} className={styles.row}>
            <label className={styles.field}>
              <span className={styles.label}>Bill</span>
              <input
                className={styles.input}
                value={row.name}
                placeholder="e.g. Rent"
                maxLength={60}
                onChange={(event) => update(row.key, { name: event.target.value })}
              />
            </label>
            <label className={styles.field}>
              <span className={styles.label}>Monthly amount</span>
              <input
                className={styles.input}
                value={row.amount}
                placeholder="0"
                inputMode="decimal"
                onChange={(event) => update(row.key, { amount: event.target.value.replace(/[^\d.]/g, '') })}
              />
            </label>
            <label className={styles.field}>
              <span className={styles.label}>Due day</span>
              <div className={styles.segmented}>
                <input
                  className={`${styles.input} ${row.day && invalidDay ? styles.inputInvalid : ''}`}
                  value={row.day}
                  placeholder="—"
                  inputMode="numeric"
                  maxLength={2}
                  style={{ flex: 1 }}
                  onChange={(event) => update(row.key, { day: event.target.value.replace(/\D/g, '') })}
                />
                <button
                  type="button"
                  className={styles.iconButton}
                  aria-label={`Remove ${row.name || 'bill'}`}
                  onClick={() =>
                    setRows((current) => (current.length > 1 ? current.filter((r) => r.key !== row.key) : [emptyRow()]))
                  }
                >
                  <Trash size={16} />
                </button>
              </div>
            </label>
          </div>
        ))}
        <div>
          <button
            type="button"
            className={styles.secondary}
            onClick={() => setRows((current) => [...current, emptyRow()])}
          >
            <Plus size={14} /> Add another
          </button>
        </div>
      </div>

      <div className={styles.actions}>
        <button type="button" className={styles.ghost} onClick={onBack} disabled={busy || saving}>
          <ArrowLeft size={16} /> Back
        </button>
        <span className={styles.spacer} />
        <button
          type="button"
          className={styles.ghost}
          onClick={() => onNext({ skipped: true })}
          disabled={busy || saving}
        >
          Skip
        </button>
        <button type="button" className={styles.primary} onClick={save} disabled={busy || saving || invalidDay}>
          {saving ? 'Saving…' : valid.length ? `Save ${valid.length} bill${valid.length === 1 ? '' : 's'}` : 'Continue'}
          <ArrowRight size={16} />
        </button>
      </div>
    </>
  );
}
