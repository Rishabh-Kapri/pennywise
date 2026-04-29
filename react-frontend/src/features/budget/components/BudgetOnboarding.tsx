import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { Check, ChevronRight } from 'lucide-react';
import {
  createBudget,
  fetchAllBudgets,
  selectAllBudgets,
  selectBudgetError,
  selectBudgetLoading,
} from '../store';
import {
  DEFAULT_SELECTED_TEMPLATE_GROUPS,
  STARTER_BUDGET_TEMPLATE_GROUPS,
} from '../constants/budgetTemplates';
import { LoadingState, toast } from '@/utils';
import styles from './BudgetOnboarding.module.css';

export default function BudgetOnboarding() {
  const dispatch = useAppDispatch();
  const navigate = useNavigate();
  const loading = useAppSelector(selectBudgetLoading);
  const error = useAppSelector(selectBudgetError);
  const budgets = useAppSelector(selectAllBudgets);
  const [budgetName, setBudgetName] = useState('My Budget');
  const [selectedGroups, setSelectedGroups] = useState<string[]>(
    DEFAULT_SELECTED_TEMPLATE_GROUPS,
  );

  const selectedTemplateGroups = useMemo(
    () =>
      STARTER_BUDGET_TEMPLATE_GROUPS.filter((group) =>
        selectedGroups.includes(group.name),
      ),
    [selectedGroups],
  );

  const isCreating = loading === LoadingState.PENDING;

  const toggleGroup = (groupName: string) => {
    setSelectedGroups((current) =>
      current.includes(groupName)
        ? current.filter((name) => name !== groupName)
        : [...current, groupName],
    );
  };

  const handleCreateBudget = () => {
    const name = budgetName.trim();
    if (!name) {
      toast.error('Budget name is required');
      return;
    }

    dispatch(
      createBudget({
        name,
        templateGroups: selectedTemplateGroups,
      }),
    )
      .unwrap()
      .then(() => {
        toast.success('Budget created');
        dispatch(fetchAllBudgets());
        navigate('/budget', { replace: true });
      })
      .catch((err: unknown) => {
        const message =
          err instanceof Error
            ? err.message
            : typeof err === 'string'
              ? err
              : 'Failed to create budget';
        toast.error(message);
      });
  };

  return (
    <main className={styles.page}>
      <section className={styles.hero}>
        <div className={styles.eyebrow}>
          {budgets.length === 0 ? 'First budget setup' : 'New budget setup'}
        </div>
        <h1>Create your budget</h1>
        <p>
          Start with a practical category template, then customize it once your
          budget opens.
        </p>
      </section>

      <section className={styles.card}>
        <label className={styles.label} htmlFor="budget-name">
          Budget name
        </label>
        <input
          id="budget-name"
          className={styles.input}
          value={budgetName}
          onChange={(event) => setBudgetName(event.target.value)}
          disabled={isCreating}
        />

        <div className={styles.templateHeader}>
          <div>
            <h2>Choose starter groups</h2>
            <p>{selectedTemplateGroups.length} groups selected</p>
          </div>
          <button
            type="button"
            className={styles.secondaryButton}
            onClick={() => setSelectedGroups(DEFAULT_SELECTED_TEMPLATE_GROUPS)}
            disabled={isCreating}>
            Select all
          </button>
        </div>

        <div className={styles.groupGrid}>
          {STARTER_BUDGET_TEMPLATE_GROUPS.map((group) => {
            const selected = selectedGroups.includes(group.name);
            return (
              <button
                type="button"
                key={group.name}
                className={`${styles.groupCard} ${selected ? styles.selected : ''}`}
                onClick={() => toggleGroup(group.name)}
                disabled={isCreating}>
                <span className={styles.checkCircle}>
                  {selected && <Check size={16} />}
                </span>
                <span className={styles.groupName}>{group.name}</span>
                <span className={styles.categoryPreview}>
                  {group.categories.map((category) => category.name).join(', ')}
                </span>
              </button>
            );
          })}
        </div>

        {error && <div className={styles.error}>{error}</div>}

        <div className={styles.actions}>
          {budgets.length > 0 && (
            <button
              type="button"
              className={styles.cancelButton}
              onClick={() => navigate(-1)}
              disabled={isCreating}>
              Cancel
            </button>
          )}
          <button
            type="button"
            className={styles.createButton}
            onClick={handleCreateBudget}
            disabled={isCreating}>
            {isCreating ? 'Creating budget...' : 'Create budget'}
            {!isCreating && <ChevronRight size={18} />}
          </button>
        </div>
      </section>
    </main>
  );
}
