import type { Middleware } from '@reduxjs/toolkit';
import { fetchAllAccounts } from '../features/accounts/store/accountSlice';
import { fetchAllBudgets, setSelectedBudget, setSelectedMonth } from '../features/budget/store/budgetSlice';
import { fetchAllCategoryGroups, fetchCategoryById, fetchInflowAmount, updateCategoryBudget } from '../features/category/store/categorySlice';
import { fetchAllLoanMetadata } from '../features/loans/store/loanSlice';
import { fetchAllPayees } from '../features/payees/store/payeeSlice';
import { fetchAllTags } from '../features/tags/store/tagSlice';
import { fetchAllTransactions } from '../features/transactions/store/transactionSlice';
import { saveSelectedBudgetId } from '../utils/storage';
import type { AppDispatch, RootState } from './store';

export const dataFetchMiddleware: Middleware = (store) => (next) => (action) => {
  const result = next(action);
  const dispatch = store.dispatch as AppDispatch;
  const state = store.getState() as RootState;
  const month = state.budgets.selectedMonth;

  if (fetchAllBudgets.fulfilled.match(action) || setSelectedBudget.match(action)) {
    dispatch(fetchAllAccounts());
    dispatch(fetchAllTransactions());
    if (month) dispatch(fetchAllCategoryGroups(month));
    dispatch(fetchInflowAmount());
    dispatch(fetchAllPayees());
    dispatch(fetchAllLoanMetadata());
    dispatch(fetchAllTags());

    const selectedBudget = (store.getState() as RootState).budgets.selectedBudget;
    if (selectedBudget?.metadata?.inflowCategoryId) {
      dispatch(fetchCategoryById(selectedBudget.metadata.inflowCategoryId));
    }
  }

  return result;
};

/**
 * Mirrors the selected budget into AsyncStorage. Background tasks (widgets, the
 * location snap) have no Redux store, so this is the only way they can know
 * which budget to send as `x-budget-id`. Fire-and-forget: a failed write must
 * not interfere with the action being dispatched.
 */
export const budgetPersistenceMiddleware: Middleware = (store) => {
  // Every dispatched action passes through here, so only touch storage when the
  // id actually changes rather than writing on each one.
  let lastWritten: string | null = null;

  return (next) => (action) => {
    const result = next(action);
    const budgetId = (store.getState() as RootState).budgets.selectedBudget?.id ?? null;
    if (budgetId && budgetId !== lastWritten) {
      lastWritten = budgetId;
      void saveSelectedBudgetId(budgetId).catch(() => undefined);
    }
    return result;
  };
};

export const dateChangeMiddleware: Middleware = (store) => (next) => (action) => {
  const result = next(action);
  if (setSelectedMonth.match(action)) {
    (store.dispatch as AppDispatch)(fetchAllCategoryGroups(action.payload));
  }
  return result;
};

export const budgetUpdateMiddleware: Middleware = (store) => (next) => (action) => {
  const result = next(action);
  if (updateCategoryBudget.fulfilled.match(action)) {
    (store.dispatch as AppDispatch)(fetchInflowAmount());
  }
  return result;
};
