import { ArrowsClockwise, CircleNotch, Sparkle, X } from '@phosphor-icons/react';
import type { Category } from '../types/category.types';
import styles from './CategoryInfo.module.css';
import { ActivityPopover } from './ActivityModal';
import { AmountCell } from './AmountCell';
import type React from 'react';
import { useEffect, useRef, useState } from 'react';
import { useDebounce } from '@/hooks/useDebounce';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { toast } from '@/utils';
import { suggestCategoryDescription, updateCategoryDetails } from '../store/categorySlice';
import { selectSelectedMonth } from '@/features/budget';
import { getLocaleDate, getPreviousMonthKey } from '@/utils/date.utils';
import { useAssignedEditor } from '../hooks/useAssignedEditor';
import { formatBudgetAmount } from './budgetFormat';
import { GoalSection } from './GoalSection';
import { CategoryActions } from './CategoryActions';

interface CategoryInfoProps {
  category: Category;
  groupName?: string;
  onClose: () => void;
}

function statusOf(available: number, assigned: number, spent: number) {
  if (available < 0) return { label: 'Overspent', className: styles.statusOver };
  if (assigned > 0 && spent >= assigned * 0.85) return { label: 'Almost spent', className: styles.statusWarn };
  if (assigned > 0 || available > 0) return { label: 'On track', className: styles.statusOk };
  return { label: 'Not funded', className: styles.statusIdle };
}

function toneClass(value: number) {
  if (value > 0) return styles.positive;
  if (value < 0) return styles.negative;
  return '';
}

export function CategoryInfo({ category, groupName, onClose }: CategoryInfoProps) {
  const month = useAppSelector(selectSelectedMonth);
  const previousMonth = getPreviousMonthKey(month);
  const [note, setNote] = useState<string>(category.note ?? '');
  const [showActivity, setShowActivity] = useState(false);
  const activityRef = useRef<HTMLButtonElement | null>(null);
  const editor = useAssignedEditor(category, month);
  const [debouncedNote] = useDebounce(note, 700);
  const dispatch = useAppDispatch();
  const categoryId = category.id ?? '';

  const [description, setDescription] = useState<string>(category.description ?? '');
  const [suggestion, setSuggestion] = useState<string | null>(null);
  const [suggesting, setSuggesting] = useState(false);

  useEffect(() => {
    if (!categoryId || debouncedNote.trim() === (category.note ?? '').trim()) return;
    dispatch(updateCategoryDetails({ categoryId, note: debouncedNote }))
      .unwrap()
      .catch(() => toast.error("Couldn't save note", { description: 'Please try again.' }));
  }, [debouncedNote, category.note, categoryId, dispatch]);

  const saveDescription = (next: string) => {
    if (!categoryId || next.trim() === (category.description ?? '').trim()) return;
    dispatch(updateCategoryDetails({ categoryId, description: next }))
      .unwrap()
      .catch(() => toast.error("Couldn't save description", { description: 'Please try again.' }));
  };

  const suggestDescription = async () => {
    if (!categoryId || suggesting) return;
    setSuggesting(true);
    try {
      setSuggestion(await suggestCategoryDescription(categoryId));
    } catch {
      toast.error("Couldn't get a suggestion right now");
    } finally {
      setSuggesting(false);
    }
  };

  const acceptSuggestion = () => {
    if (!suggestion) return;
    setDescription(suggestion);
    saveDescription(suggestion);
    setSuggestion(null);
  };

  const available = category.balance?.[month] ?? 0;
  const leftOver = category.balance?.[previousMonth] ?? 0;
  const assigned = editor.assigned;
  const activity = category.activity?.[month] ?? 0;
  const spent = Math.max(-activity, 0);
  const percentUsed = assigned > 0 ? Math.min((spent / assigned) * 100, 100) : spent > 0 ? 100 : 0;

  const status = statusOf(available, assigned, spent);
  const monthName = getLocaleDate(month, { month: 'long' });

  return (
    <aside className={styles.inspector} aria-label={`${category.name} details`}>
      <header className={styles.header}>
        <div className={styles.heading}>
          {groupName && <span className={styles.eyebrow}>{groupName}</span>}
          <h2 className={styles.title}>{category.name}</h2>
        </div>
        <div className={styles.headerActions}>
          <CategoryActions category={category} />
          <button type="button" className={styles.closeButton} aria-label="Close" onClick={onClose}>
            <X size={18} />
          </button>
        </div>
      </header>

      <section className={styles.hero}>
        <div className={styles.heroTop}>
          <span className={styles.label}>Available in {monthName}</span>
          <span className={`${styles.status} ${status.className}`}>{status.label}</span>
        </div>
        <strong className={`${styles.heroAmount} ${toneClass(available)}`}>
          {formatBudgetAmount(available)}
        </strong>
        {(assigned > 0 || spent > 0) && (
          <div className={styles.progress}>
            <div className={styles.meter} aria-hidden="true">
              <div
                className={`${styles.meterFill} ${available < 0 ? styles.meterOver : ''}`}
                style={{ width: `${percentUsed}%` }}
              />
            </div>
            <span className={styles.progressText}>
              {formatBudgetAmount(spent)} spent of {formatBudgetAmount(assigned)}
            </span>
          </div>
        )}
        {available < 0 && (
          <button
            type="button"
            className={styles.coverButton}
            onClick={() => editor.save(assigned - available)}>
            <ArrowsClockwise size={16} weight="bold" />
            Cover overspending · assign {formatBudgetAmount(-available)}
          </button>
        )}
      </section>

      <dl className={styles.tiles}>
        <div className={styles.tile}>
          <dt>Assigned</dt>
          <dd>
            {editor.isEditing ? (
              <AmountCell
                value={editor.draft}
                isEditing
                onChange={editor.onChange}
                onBlur={editor.commit}
                onKeyDown={editor.onKeyDown}
              />
            ) : (
              <button
                type="button"
                className={styles.tileButton}
                aria-label={`Edit amount assigned to ${category.name}`}
                onClick={editor.start}>
                {formatBudgetAmount(assigned)}
              </button>
            )}
          </dd>
        </div>
        <div className={styles.tile}>
          <dt>Activity</dt>
          <dd>
            <button
              ref={activityRef}
              type="button"
              className={styles.tileButton}
              aria-label={`Show ${category.name} activity`}
              onClick={() => setShowActivity(true)}>
              {formatBudgetAmount(activity)}
            </button>
          </dd>
        </div>
        <div className={`${styles.tile} ${styles.tileWide}`}>
          <dt>Left over from {getLocaleDate(previousMonth, { month: 'long' })}</dt>
          <dd className={toneClass(leftOver)}>{formatBudgetAmount(leftOver)}</dd>
        </div>
      </dl>

      <GoalSection category={category} month={month} assigned={assigned} onAssign={editor.save} />

      <section className={styles.notes}>
        <div className={styles.fieldHeader}>
          <label htmlFor={`category-description-${category.id}`} className={styles.label}>
            Description
          </label>
          <button
            type="button"
            className={styles.aiButton}
            onClick={suggestDescription}
            disabled={suggesting}>
            {suggesting ? (
              <CircleNotch size={14} weight="bold" className={styles.spin} />
            ) : (
              <Sparkle size={14} weight="fill" />
            )}
            {suggesting ? 'Thinking…' : description ? 'Rewrite with AI' : 'Suggest with AI'}
          </button>
        </div>

        {suggestion && (
          <div className={styles.suggestion} aria-live="polite">
            <span className={styles.suggestionLabel}>
              <Sparkle size={12} weight="fill" /> Suggestion
            </span>
            <p className={styles.suggestionText}>{suggestion}</p>
            <div className={styles.suggestionActions}>
              <button type="button" className={styles.useButton} onClick={acceptSuggestion}>
                Use this
              </button>
              <button
                type="button"
                className={styles.ghostButton}
                onClick={suggestDescription}
                disabled={suggesting}>
                Try again
              </button>
              <button type="button" className={styles.ghostButton} onClick={() => setSuggestion(null)}>
                Dismiss
              </button>
            </div>
          </div>
        )}

        <textarea
          id={`category-description-${category.id}`}
          className={styles.notesInput}
          placeholder="What belongs in this category?"
          rows={2}
          maxLength={2000}
          value={description}
          onChange={(event: React.ChangeEvent<HTMLTextAreaElement>) => setDescription(event.target.value)}
          onBlur={() => saveDescription(description)}
        />
      </section>

      <section className={styles.notes}>
        <label htmlFor={`category-note-${category.id}`} className={styles.label}>
          Notes
        </label>
        <textarea
          id={`category-note-${category.id}`}
          className={styles.notesInput}
          placeholder="Add a note…"
          rows={3}
          maxLength={2000}
          value={note}
          onChange={(event: React.ChangeEvent<HTMLTextAreaElement>) => setNote(event.target.value)}
        />
      </section>

      <ActivityPopover
        isOpen={showActivity}
        onClose={() => setShowActivity(false)}
        triggerRef={activityRef}
        categoryId={category.id ?? ''}
        categoryName={category.name}
        month={month}
        activityAmount={activity}
      />
    </aside>
  );
}
