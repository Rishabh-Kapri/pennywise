import { createListenerMiddleware, isAnyOf } from '@reduxjs/toolkit';
import { fetchAllAccounts } from '@/features/accounts/store/accountSlice';
import { fetchAllBudgets, setSelectedBudget, setSelectedMonth } from '@/features/budget/store/budgetSlice';
import {
  fetchAllCategories,
  fetchAllCategoryGroups,
  fetchCategoryById,
  fetchInflowAmount,
  updateCategoryBudget,
} from '@/features/category/store/categorySlice';
import { fetchAllLoanMetadata } from '@/features/loans/store/loanSlice';
import { fetchAllPayees } from '@/features/payees/store/payeeSlice';
import { fetchAllTags } from '@/features/tags/store/tagSlice';
import { fetchAllTransaction } from '@/features/transactions/store/transactionSlice';
import type { AppDispatch, RootState } from './store';

/**
 * Loads what every screen reads from the store: whenever the selected budget
 * is known or changes, the month changes, or a budgeted amount changes Ready
 * to Assign.
 */
export const appDataListener = createListenerMiddleware();
const startListening = appDataListener.startListening.withTypes<RootState, AppDispatch>();

startListening({
  matcher: isAnyOf(fetchAllBudgets.fulfilled, setSelectedBudget),
  effect: (_action, { dispatch, getState }) => {
    const { selectedBudget, selectedMonth } = getState().budgets;
    // A new user has no budget yet, and these all need one. Creating it
    // fetches the budgets again, which lands back here.
    if (!selectedBudget) return;
    dispatch(fetchAllAccounts());
    dispatch(fetchAllTransaction());
    dispatch(fetchAllCategoryGroups(selectedMonth));
    dispatch(fetchAllCategories());
    dispatch(fetchInflowAmount());
    dispatch(fetchAllPayees());
    dispatch(fetchAllLoanMetadata());
    dispatch(fetchAllTags());
    const inflowCategoryId = selectedBudget.metadata?.inflowCategoryId;
    if (inflowCategoryId) dispatch(fetchCategoryById(inflowCategoryId));
  },
});

startListening({
  actionCreator: setSelectedMonth,
  effect: (action, { dispatch }) => {
    dispatch(fetchAllCategoryGroups(action.payload));
  },
});

startListening({
  actionCreator: updateCategoryBudget.fulfilled,
  effect: (_action, { dispatch }) => {
    dispatch(fetchInflowAmount());
  },
});
