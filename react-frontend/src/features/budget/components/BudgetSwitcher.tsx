import {
  Check,
  CaretDown as ChevronDown,
  EnvelopeOpen,
  Plus,
  UsersThree,
  Wallet as WalletCards,
} from '@phosphor-icons/react';
import { useEffect, useRef, useState } from 'react';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { selectIsDemoUser } from '@/features/auth/store';
import {
  selectAllBudgets,
  selectSelectedBudget,
  setSelectedBudget,
  updateBudgetSelection,
} from '../store';
import { useNavigate } from 'react-router-dom';
import { useMyInvites } from '@/features/sharing/hooks/useMyInvites';
import { ROLE_LABELS } from '@/features/sharing/types';
import type { Budget } from '../types/budget.types';
import styles from './BudgetSwitcher.module.css';

/** "Shared" on a budget you share, your role on one shared with you. */
function sharingTag(budget: Budget): string | null {
  if (budget.role && budget.role !== 'owner') return ROLE_LABELS[budget.role];
  if ((budget.memberCount ?? 1) > 1) return 'Shared';
  return null;
}

export default function BudgetSwitcher() {
  const dispatch = useAppDispatch();
  const navigate = useNavigate();
  const budgets = useAppSelector(selectAllBudgets);
  const selectedBudget = useAppSelector(selectSelectedBudget);
  const isDemoUser = useAppSelector(selectIsDemoUser);
  const [isOpen, setIsOpen] = useState(false);
  const { invites, reload: reloadInvites } = useMyInvites();
  const inviteCount = invites?.length ?? 0;
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!isOpen) {
      return;
    }
    void reloadInvites();

    const handlePointerDown = (event: PointerEvent) => {
      if (!containerRef.current?.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };

    document.addEventListener('pointerdown', handlePointerDown);
    return () => document.removeEventListener('pointerdown', handlePointerDown);
  }, [isOpen, reloadInvites]);

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
                <span className={styles.optionName}>
                  <span>{budget.name}</span>
                  {sharingTag(budget) && <span className={styles.tag}>{sharingTag(budget)}</span>}
                </span>
                {isSelected && <Check size={15} strokeWidth={2.2} weight="fill" />}
              </button>
            );
          })}

          <div className={styles.divider} />

          {inviteCount > 0 && (
            <button
              type="button"
              className={styles.createOption}
              onClick={() => {
                setIsOpen(false);
                navigate('/settings?section=sharing');
              }}>
              <EnvelopeOpen size={15} strokeWidth={2.2} />
              <span>
                {inviteCount === 1 ? '1 invitation' : `${inviteCount} invitations`} to join
              </span>
            </button>
          )}

          <button
            type="button"
            className={styles.createOption}
            onClick={() => {
              setIsOpen(false);
              navigate('/settings?section=sharing');
            }}>
            <UsersThree size={15} strokeWidth={2.2} />
            <span>Share this budget</span>
          </button>

          <button
            type="button"
            className={styles.createOption}
            disabled={isDemoUser}
            title={isDemoUser ? 'Creating budgets is disabled for the demo account' : undefined}
            style={isDemoUser ? { opacity: 0.5, cursor: 'not-allowed' } : undefined}
            onClick={() => {
              if (isDemoUser) return;
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
