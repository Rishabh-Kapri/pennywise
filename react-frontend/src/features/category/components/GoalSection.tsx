import { useState } from 'react';
import type React from 'react';
import { BellSimpleSlash, PencilSimple, Plus, Target, Trash } from '@phosphor-icons/react';
import { useAppDispatch } from '@/app/hooks';
import { errorMessage, toast } from '@/utils';
import type { Category, CategoryGoal, GoalCadence, GoalType } from '../types/category.types';
import { deleteCategoryGoal, updateCategoryGoal } from '../store/categorySlice';
import { getGoalProgress } from '../goals/goalProgress';
import { describeGoal, describeProgress, describeRefill, GOAL_TYPE_LABELS, WEEKDAYS } from '../goals/goalText';
import { formatBudgetAmount } from './budgetFormat';
import { getLocaleDate } from '@/utils/date.utils';
import styles from './GoalSection.module.css';

interface GoalSectionProps {
  category: Category;
  month: string;
  /** current assigned amount; "Assign" tops it up by the underfunded amount */
  assigned: number;
  onAssign: (budgeted: number) => void;
}

const STATUS_CLASS = {
  funded: styles.funded,
  underfunded: styles.underfunded,
  overspent: styles.overspent,
  snoozed: styles.snoozed,
} as const;

/** The category's target: what it asks for this month and the actions on it. */
export function GoalSection({ category, month, assigned, onAssign }: GoalSectionProps) {
  const dispatch = useAppDispatch();
  const [editing, setEditing] = useState(false);
  const goal = category.goal;
  const categoryId = category.id ?? '';

  if (editing) {
    return (
      <GoalEditor
        initial={goal ?? null}
        month={month}
        onCancel={() => setEditing(false)}
        onSave={async (next) => {
          try {
            await dispatch(updateCategoryGoal({ categoryId, goal: next })).unwrap();
            setEditing(false);
            toast.success('Target saved');
          } catch (err) {
            toast.error("Couldn't save target", { description: errorMessage(err, 'Please try again.') });
          }
        }}
      />
    );
  }

  if (!goal) {
    return (
      <section className={styles.empty}>
        <Target size={20} className={styles.emptyIcon} />
        <div className={styles.emptyText}>
          <strong>No target</strong>
          <span>Set how much this category needs and when, and see what to assign each month.</span>
        </div>
        <button type="button" className={styles.secondaryButton} onClick={() => setEditing(true)}>
          <Plus size={14} weight="bold" /> Create target
        </button>
      </section>
    );
  }

  const progress = getGoalProgress(category, month);
  const refill = describeRefill(goal);
  const snoozed = goal.snoozedMonth === month;

  const save = (next: CategoryGoal, success: string) =>
    dispatch(updateCategoryGoal({ categoryId, goal: next }))
      .unwrap()
      .then(() => toast.success(success))
      .catch(() => toast.error("Couldn't update target", { description: 'Please try again.' }));

  const remove = () =>
    dispatch(deleteCategoryGoal(categoryId))
      .unwrap()
      .then(() => toast.success('Target removed'))
      .catch(() => toast.error("Couldn't remove target", { description: 'Please try again.' }));

  return (
    <section className={styles.card} aria-label="Target">
      <div className={styles.cardHeader}>
        <span className={styles.label}>Target</span>
        <span className={styles.typeChip}>{GOAL_TYPE_LABELS[goal.type]}</span>
      </div>
      <p className={styles.summary}>{describeGoal(goal)}</p>
      {refill && <p className={styles.subtle}>{refill}</p>}

      {progress ? (
        <div className={styles.progress}>
          <div className={`${styles.meter} ${STATUS_CLASS[progress.status]}`} aria-hidden="true">
            <div className={styles.meterFill} style={{ width: `${progress.percent}%` }} />
          </div>
          <div className={styles.progressRow}>
            <span className={styles.progressStatus}>{describeProgress(progress)}</span>
            {progress.needed > 0 && (
              <span className={styles.subtle}>
                {formatBudgetAmount(Math.max(progress.funded, 0))} of {formatBudgetAmount(progress.needed)}
              </span>
            )}
          </div>
          {progress.overall && (
            <span className={styles.subtle}>
              {formatBudgetAmount(Math.max(progress.overall.funded, 0))} of{' '}
              {formatBudgetAmount(progress.overall.target)} saved overall
            </span>
          )}
          {progress.dueMonth && (
            <span className={styles.subtle}>
              Due {getLocaleDate(progress.dueMonth, { month: 'short', year: 'numeric' })}
              {progress.monthsLeft && progress.monthsLeft > 1 ? ` · ${progress.monthsLeft} months left` : ''}
            </span>
          )}
        </div>
      ) : (
        <p className={styles.subtle}>
          {month < goal.creationMonth ? 'This target starts in a later month.' : 'Nothing due this month.'}
        </p>
      )}

      {progress && progress.underfunded > 0 && !snoozed && (
        <button
          type="button"
          className={styles.primaryButton}
          onClick={() => onAssign(assigned + progress.underfunded)}>
          Assign {formatBudgetAmount(progress.underfunded)}
        </button>
      )}

      <div className={styles.actions}>
        <button type="button" className={styles.ghostButton} onClick={() => setEditing(true)}>
          <PencilSimple size={14} /> Edit
        </button>
        <button
          type="button"
          className={styles.ghostButton}
          onClick={() =>
            save(
              { ...goal, snoozedMonth: snoozed ? undefined : month },
              snoozed ? 'Target resumed' : 'Target snoozed for this month',
            )
          }>
          <BellSimpleSlash size={14} /> {snoozed ? 'Unsnooze' : 'Snooze'}
        </button>
        <button type="button" className={`${styles.ghostButton} ${styles.danger}`} onClick={remove}>
          <Trash size={14} /> Remove
        </button>
      </div>
    </section>
  );
}

// ── Editor ──────────────────────────────────────────────────────────────────

/** The NEED schedules the editor offers; "every N months" is a monthly cadence. */
type Schedule = 'weekly' | 'monthly' | 'yearly' | 'everyN' | 'once';

const SCHEDULES: { id: Schedule; label: string }[] = [
  { id: 'weekly', label: 'Weekly' },
  { id: 'monthly', label: 'Monthly' },
  { id: 'yearly', label: 'Yearly' },
  { id: 'everyN', label: 'Every N months' },
  { id: 'once', label: 'Once' },
];

const TYPE_HINTS: Record<GoalType, string> = {
  NEED: 'For bills and spending that come due on a schedule.',
  TB: 'Build up and keep a balance, with no deadline.',
  TBD: 'Reach a balance by a month, spread evenly over the months left.',
  MF: 'Assign the same amount every month, whatever the balance.',
  DEBT: 'Put a fixed payment toward a debt every month.',
};

interface Draft {
  type: GoalType;
  schedule: Schedule;
  target: string;
  day: string; // '' = last day of month
  weekday: string;
  every: string;
  targetMonth: string;
  needsWholeAmount: boolean;
}

function scheduleOf(goal: CategoryGoal): Schedule {
  if (goal.cadence === 'weekly' || goal.cadence === 'yearly') return goal.cadence;
  if (goal.cadence === 'none') return 'once';
  return (goal.cadenceFrequency ?? 1) > 1 ? 'everyN' : 'monthly';
}

function draftFrom(goal: CategoryGoal | null, month: string): Draft {
  return {
    type: goal?.type ?? 'NEED',
    schedule: goal ? scheduleOf(goal) : 'monthly',
    target: goal ? String(goal.target) : '',
    day: goal?.cadence !== 'weekly' && goal?.day ? String(goal.day) : '',
    weekday: goal?.cadence === 'weekly' ? String(goal.day ?? 0) : '1',
    every: String(goal?.cadenceFrequency && goal.cadenceFrequency > 1 ? goal.cadenceFrequency : 3),
    targetMonth: goal?.targetMonth ?? month,
    needsWholeAmount: goal?.needsWholeAmount ?? true,
  };
}

const CADENCE: Record<Schedule, GoalCadence> = {
  weekly: 'weekly',
  monthly: 'monthly',
  yearly: 'yearly',
  everyN: 'monthly',
  once: 'none',
};

function goalFrom(draft: Draft, base: CategoryGoal | null, month: string): CategoryGoal | string {
  const target = Number(draft.target);
  if (!Number.isFinite(target) || target <= 0) return 'Enter an amount greater than zero';

  const goal: CategoryGoal = {
    type: draft.type,
    target,
    needsWholeAmount: false,
    // a target applies from the month it was created in; edits keep that
    creationMonth: base?.creationMonth ?? month,
    snoozedMonth: base?.snoozedMonth,
  };
  if (draft.type === 'TBD') {
    if (!draft.targetMonth) return 'Pick the month to reach it by';
    goal.targetMonth = draft.targetMonth;
  }
  if (draft.type !== 'NEED') return goal;

  goal.cadence = CADENCE[draft.schedule];
  switch (draft.schedule) {
    case 'weekly':
      goal.day = Number(draft.weekday);
      goal.needsWholeAmount = draft.needsWholeAmount;
      break;
    case 'monthly':
      goal.needsWholeAmount = draft.needsWholeAmount;
      break;
    case 'everyN': {
      const every = Number(draft.every);
      if (!Number.isInteger(every) || every < 2 || every > 120) return 'Repeat every 2 to 120 months';
      goal.cadenceFrequency = every;
      break;
    }
  }
  if (draft.schedule !== 'weekly' && draft.day) goal.day = Number(draft.day);
  if (draft.schedule === 'yearly' || draft.schedule === 'everyN' || draft.schedule === 'once') {
    if (!draft.targetMonth) return 'Pick the month it is due';
    goal.targetMonth = draft.targetMonth;
  }
  return goal;
}

interface GoalEditorProps {
  initial: CategoryGoal | null;
  month: string;
  onCancel: () => void;
  onSave: (goal: CategoryGoal) => Promise<void>;
}

function GoalEditor({ initial, month, onCancel, onSave }: GoalEditorProps) {
  const [draft, setDraft] = useState<Draft>(() => draftFrom(initial, month));
  const [saving, setSaving] = useState(false);
  const update = (patch: Partial<Draft>) => setDraft((d) => ({ ...d, ...patch }));

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    const goal = goalFrom(draft, initial, month);
    if (typeof goal === 'string') {
      toast.error(goal);
      return;
    }
    setSaving(true);
    await onSave(goal);
    setSaving(false);
  };

  const isNeed = draft.type === 'NEED';
  const showRefill = isNeed && (draft.schedule === 'weekly' || draft.schedule === 'monthly');
  const showDay = isNeed && draft.schedule !== 'weekly';
  const needsMonth = draft.type === 'TBD' || (isNeed && ['yearly', 'everyN', 'once'].includes(draft.schedule));
  const amountLabel = (() => {
    if (draft.type === 'TB' || draft.type === 'TBD') return 'Balance to reach';
    if (draft.type === 'MF') return 'Assign each month';
    if (draft.type === 'DEBT') return 'Payment each month';
    return draft.schedule === 'weekly' ? 'I need each week' : 'I need';
  })();
  const monthLabel =
    draft.type === 'TBD' ? 'By' : draft.schedule === 'once' ? 'Due' : draft.schedule === 'yearly' ? 'Next due' : 'Starting';

  return (
    <form className={styles.card} onSubmit={submit} aria-label={initial ? 'Edit target' : 'Create target'}>
      <div className={styles.cardHeader}>
        <span className={styles.label}>{initial ? 'Edit target' : 'New target'}</span>
      </div>

      <label className={styles.field}>
        <span className={styles.fieldLabel}>Type</span>
        <select
          className={styles.input}
          value={draft.type}
          onChange={(e) => update({ type: e.target.value as GoalType })}>
          {(Object.keys(GOAL_TYPE_LABELS) as GoalType[]).map((type) => (
            <option key={type} value={type}>
              {GOAL_TYPE_LABELS[type]}
            </option>
          ))}
        </select>
        <span className={styles.subtle}>{TYPE_HINTS[draft.type]}</span>
      </label>

      {isNeed && (
        <div className={styles.segmented} role="radiogroup" aria-label="Schedule">
          {SCHEDULES.map(({ id, label }) => (
            <button
              key={id}
              type="button"
              role="radio"
              aria-checked={draft.schedule === id}
              className={draft.schedule === id ? styles.segmentActive : styles.segment}
              onClick={() => update({ schedule: id })}>
              {label}
            </button>
          ))}
        </div>
      )}

      <div className={styles.fieldRow}>
        <label className={styles.field}>
          <span className={styles.fieldLabel}>{amountLabel}</span>
          <input
            className={styles.input}
            type="number"
            inputMode="decimal"
            min="0"
            step="0.01"
            placeholder="0"
            value={draft.target}
            onChange={(e) => update({ target: e.target.value })}
            autoFocus
            required
          />
        </label>

        {isNeed && draft.schedule === 'everyN' && (
          <label className={styles.field}>
            <span className={styles.fieldLabel}>Every (months)</span>
            <input
              className={styles.input}
              type="number"
              min="2"
              max="120"
              step="1"
              value={draft.every}
              onChange={(e) => update({ every: e.target.value })}
            />
          </label>
        )}

        {isNeed && draft.schedule === 'weekly' && (
          <label className={styles.field}>
            <span className={styles.fieldLabel}>Every</span>
            <select
              className={styles.input}
              value={draft.weekday}
              onChange={(e) => update({ weekday: e.target.value })}>
              {WEEKDAYS.map((name, index) => (
                <option key={name} value={index}>
                  {name}
                </option>
              ))}
            </select>
          </label>
        )}
      </div>

      {(needsMonth || showDay) && (
        <div className={styles.fieldRow}>
          {needsMonth && (
            <label className={styles.field}>
              <span className={styles.fieldLabel}>{monthLabel}</span>
              <input
                className={styles.input}
                type="month"
                value={draft.targetMonth}
                onChange={(e) => update({ targetMonth: e.target.value })}
                required
              />
            </label>
          )}
          {showDay && (
            <label className={styles.field}>
              <span className={styles.fieldLabel}>By day</span>
              <select className={styles.input} value={draft.day} onChange={(e) => update({ day: e.target.value })}>
                <option value="">Last day of month</option>
                {Array.from({ length: 31 }, (_, i) => i + 1).map((day) => (
                  <option key={day} value={day}>
                    {day}
                  </option>
                ))}
              </select>
            </label>
          )}
        </div>
      )}

      {showRefill && (
        <fieldset className={styles.radioGroup}>
          <legend className={styles.fieldLabel}>Next month I want to</legend>
          <label className={styles.radio}>
            <input
              type="radio"
              name="refill"
              checked={!draft.needsWholeAmount}
              onChange={() => update({ needsWholeAmount: false })}
            />
            <span>
              Set aside another
              <small>Assign the full amount again, keeping what is left over</small>
            </span>
          </label>
          <label className={styles.radio}>
            <input
              type="radio"
              name="refill"
              checked={draft.needsWholeAmount}
              onChange={() => update({ needsWholeAmount: true })}
            />
            <span>
              Refill up to
              <small>Count what is left over and only top up the difference</small>
            </span>
          </label>
        </fieldset>
      )}

      <div className={styles.actions}>
        <button type="submit" className={styles.primaryButton} disabled={saving}>
          {saving ? 'Saving…' : 'Save target'}
        </button>
        <button type="button" className={styles.ghostButton} onClick={onCancel} disabled={saving}>
          Cancel
        </button>
      </div>
    </form>
  );
}
