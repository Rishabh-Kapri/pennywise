import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { ArrowLeft, Check } from '@phosphor-icons/react';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import {
  fetchAllCategoryGroups,
  fetchInflowAmount,
  updateCategoryBudget,
} from '@/features/category/store/categorySlice';
import { formatCurrency } from '@/features/dashboard/utils';
import { errorMessage, getCurrentMonthKey, toast } from '@/utils';
import type { StepProps } from '../OnboardingWizard';
import styles from './steps.module.css';

/** First pass at Ready to Assign: fund the monthly bills, then hand over to the budget. */
export function AssignStep({ onNext, onBack, busy }: StepProps) {
  const dispatch = useAppDispatch();
  const navigate = useNavigate();
  const month = getCurrentMonthKey();
  const readyToAssign = useAppSelector((state) => state.categories.inflowAmount);
  const groups = useAppSelector((state) => state.categories.allCategoryGroups);
  const [funding, setFunding] = useState(false);

  useEffect(() => {
    dispatch(fetchInflowAmount());
    dispatch(fetchAllCategoryGroups(month));
  }, [dispatch, month]);

  // Monthly targets still short of their amount this month.
  const bills = useMemo(
    () =>
      groups
        .filter((group) => !group.isSystem)
        .flatMap((group) => group.categories)
        .filter((category) => category.id && category.goal?.type === 'NEED' && category.goal.cadence === 'monthly')
        .map((category) => ({
          id: category.id as string,
          name: category.name,
          target: category.goal?.target ?? 0,
          budgeted: category.budgeted?.[month] ?? 0,
        }))
        .filter((bill) => bill.budgeted < bill.target),
    [groups, month],
  );
  const needed = bills.reduce((sum, bill) => sum + bill.target - bill.budgeted, 0);

  const fundBills = async () => {
    setFunding(true);
    let left = readyToAssign;
    try {
      for (const bill of bills) {
        if (left <= 0) break;
        const add = Math.min(bill.target - bill.budgeted, left);
        await dispatch(updateCategoryBudget({ categoryId: bill.id, month, budgeted: bill.budgeted + add })).unwrap();
        left -= add;
      }
      dispatch(fetchInflowAmount());
      dispatch(fetchAllCategoryGroups(month));
      toast.success('Bills funded');
    } catch (err) {
      toast.error('Could not assign the money', { description: errorMessage(err, 'Try again.') });
    } finally {
      setFunding(false);
    }
  };

  const finishTo = async (path: string) => {
    await onNext();
    navigate(path);
  };

  return (
    <>
      <p className={styles.eyebrow}>Last step</p>
      <h1 className={styles.title}>Give your money a job</h1>
      <p className={styles.lead}>
        Ready to Assign is money you have that isn't in a category yet. Start with the bills that are due, then spread
        the rest across what matters to you.
      </p>

      <div className={styles.summary} style={{ marginTop: '1.75rem' }}>
        <span>Ready to assign</span>
        <strong>{formatCurrency(readyToAssign)}</strong>
      </div>

      {bills.length > 0 && (
        <div className={`${styles.section} ${styles.panel}`}>
          <ul className={styles.list}>
            {bills.map((bill) => (
              <li key={bill.id} className={styles.listItem}>
                <span className={styles.grow}>{bill.name}</span>
                <span className={styles.amount}>{formatCurrency(bill.target - bill.budgeted)}</span>
              </li>
            ))}
          </ul>
          <div className={styles.actions} style={{ marginTop: '1rem' }}>
            <button
              type="button"
              className={styles.secondary}
              onClick={fundBills}
              disabled={funding || readyToAssign <= 0}
            >
              {funding ? 'Assigning…' : `Fund bills (${formatCurrency(Math.min(needed, Math.max(readyToAssign, 0)))})`}
            </button>
            {needed > readyToAssign && readyToAssign > 0 && (
              <span className={styles.hint}>Not enough to cover all of them; the first ones are funded first.</span>
            )}
          </div>
        </div>
      )}

      {readyToAssign <= 0 && bills.length === 0 && (
        <div className={`${styles.callout} ${styles.calloutSuccess}`}>
          <Check size={18} weight="bold" aria-hidden="true" /> Every rupee has a job.
        </div>
      )}

      <div className={styles.actions}>
        <button type="button" className={styles.ghost} onClick={onBack} disabled={busy}>
          <ArrowLeft size={16} /> Back
        </button>
        <span className={styles.spacer} />
        <button
          type="button"
          className={styles.ghost}
          onClick={() => finishTo('/dashboard')}
          disabled={busy || funding}
        >
          Finish
        </button>
        <button type="button" className={styles.primary} onClick={() => finishTo('/budget')} disabled={busy || funding}>
          Assign the rest in my budget
        </button>
      </div>
    </>
  );
}
