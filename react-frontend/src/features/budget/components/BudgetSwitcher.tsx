import { Check, CaretDown as ChevronDown, Plus, Wallet as WalletCards } from '@phosphor-icons/react';
import { useEffect, useRef, useState } from 'react';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import {
  selectAllBudgets,
  selectSelectedBudget,
  setSelectedBudget,
  updateBudgetSelection,
} from '../store';
import { useNavigate } from 'react-router-dom';
import styles from './BudgetSwitcher.module.css';

export default function BudgetSwitcher() {
  const dispatch = useAppDispatch();
  const navigate = useNavigate();
  const budgets = useAppSelector(selectAllBudgets);
  const selectedBudget = useAppSelector(selectSelectedBudget);
  const [isOpen, setIsOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!isOpen) {
      return;
    }

    const handlePointerDown = (event: PointerEvent) => {
      if (!containerRef.current?.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };

    document.addEventListener('pointerdown', handlePointerDown);
    return () => document.removeEventListener('pointerdown', handlePointerDown);
  }, [isOpen]);

  const handleSelectBudget = (budgetId?: string) => {
    const budget = budgets.find((item) => item.id === budgetId);
    if (!budget) {
      return;
    }

    if (selectedBudget?.id && selectedBudget.id !== budget.id) {
      dispatch(
        updateBudgetSelection({
          budget: selectedBudget,
          isSelected: false,
        }),
      );
    }

    dispatch(updateBudgetSelection({ budget, isSelected: true }));
    dispatch(setSelectedBudget(budget));
    setIsOpen(false);
  };

  if (budgets.length === 0 && !selectedBudget) {
    return null;
  }

  return (
    <div className={styles.switcher} ref={containerRef}>
      <WalletCards
        size={16}
        strokeWidth={1.8}
        weight={selectedBudget ? 'fill' : 'regular'}
        className={styles.icon}
      />
      <button
        type="button"
        className={styles.trigger}
        onClick={() => setIsOpen((current) => !current)}
        aria-label="Select active budget"
        aria-haspopup="listbox"
        aria-expanded={isOpen}>
        <span className={styles.selectedName}>
          {selectedBudget?.name ?? budgets[0]?.name ?? 'No budget'}
        </span>
        <ChevronDown
          size={16}
          strokeWidth={2}
          className={`${styles.chevron} ${isOpen ? styles.chevronOpen : ''}`}
        />
      </button>

      {isOpen && (
        <div className={styles.menu} role="listbox">
          {budgets.map((budget) => {
            const isSelected = budget.id === selectedBudget?.id;
            return (
              <button
                key={budget.id}
                type="button"
                role="option"
                aria-selected={isSelected}
                className={`${styles.option} ${isSelected ? styles.optionSelected : ''}`}
                onClick={() => handleSelectBudget(budget.id)}>
                <span>{budget.name}</span>
                {isSelected && <Check size={15} strokeWidth={2.2} weight="fill" />}
              </button>
            );
          })}

          <div className={styles.divider} />

          <button
            type="button"
            className={styles.createOption}
            onClick={() => {
              setIsOpen(false);
              navigate('/budget/new');
            }}>
            <Plus size={15} strokeWidth={2.2} />
            <span>Create budget</span>
          </button>
        </div>
      )}
    </div>
  );
}
