import { useCallback, useRef, useState } from 'react';
import type React from 'react';
import { Parser } from 'expr-eval';
import { useAppDispatch } from '@/app/hooks';
import { toast } from '@/utils';
import { updateCategoryBudget } from '../store/categorySlice';
import type { Category } from '../types/category.types';

const parser = new Parser();

/**
 * Inline editing for a category's assigned amount. Accepts arithmetic
 * ("1200+300") so a budget can be topped up without mental math. Enter
 * commits, Escape cancels; the table row and the inspector share it so the
 * two can never disagree on how an edit is saved.
 */
export function useAssignedEditor(category: Category | null, month: string) {
  const dispatch = useAppDispatch();
  const assigned = category?.budgeted?.[month] ?? 0;
  const [draft, setDraft] = useState('');
  const [isEditing, setIsEditing] = useState(false);
  // a ref, not state: blur can fire after Escape has already ended the edit
  const editingRef = useRef(false);

  const start = useCallback(() => {
    if (editingRef.current) return;
    editingRef.current = true;
    setDraft(String(assigned));
    setIsEditing(true);
  }, [assigned]);

  const stop = useCallback(() => {
    editingRef.current = false;
    setIsEditing(false);
  }, []);

  const save = useCallback(
    (next: number) => {
      if (!category?.id || next === assigned) return;
      dispatch(updateCategoryBudget({ budgeted: next, categoryId: category.id, month }))
        .unwrap()
        .then(() => toast.success('Budget updated'))
        .catch(() => toast.error("Couldn't update budget", { description: 'Please try again.' }));
    },
    [assigned, category?.id, dispatch, month],
  );

  const commit = useCallback(() => {
    if (!editingRef.current) return;
    stop();
    if (!category?.id) return;

    let next: number;
    try {
      next = Number(parser.evaluate(draft.trim() || '0'));
    } catch {
      toast.error('Enter an amount, or a sum like 1200+300');
      return;
    }
    if (!Number.isFinite(next)) {
      toast.error('Enter an amount, or a sum like 1200+300');
      return;
    }
    save(next);
  }, [category?.id, draft, save, stop]);

  const onChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    setDraft(e.target.value);
  }, []);

  const onKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLInputElement>) => {
      if (e.key === 'Enter') {
        e.currentTarget.blur();
      } else if (e.key === 'Escape') {
        // keep the page-level Escape (deselect category) from also firing
        e.nativeEvent.stopPropagation();
        stop();
      }
    },
    [stop],
  );

  return { assigned, draft, isEditing, start, commit, save, onChange, onKeyDown };
}
