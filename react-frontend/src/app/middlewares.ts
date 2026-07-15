import { fetchAllAccounts } from '@/features/accounts/store/accountSlice';
import { fetchAllBudgets, setSelectedBudget, setSelectedMonth } from '@/features/budget';
import type { Middleware } from '@reduxjs/toolkit';
import { fetchAllCategoryGroups, fetchInflowAmount } from '@/features';
import type { AppDispatch, RootState } from '.';
import { fetchAllPayees } from '@/features/payees/store/payeeSlice';
import {
    fetchAllCategories,
  fetchCategoryById,
  updateCategoryBudget,
} from '@/features/category/store/categorySlice';
import { fetchAllLoanMetadata } from '@/features/loans/store/loanSlice';
import { fetchAllTransaction } from '@/features/transactions/store/transactionSlice';
import { fetchAllTags } from '@/features/tags/store/tagSlice';
import { fetchPipelineRuns } from '@/features/pipeline/store/pipelineSlice';

/*
 * Fetch all data on app start
 */
export const dataFetchMiddleware: Middleware =
  (store) => (next) => (action) => {
    const result = next(action);
    // const dispatch = useAppDispatch();

    const dispatch = store.dispatch as AppDispatch;

    const month = (store.getState() as RootState).budgets.selectedMonth;
    if (fetchAllBudgets.fulfilled.match(action)) {
      dispatch(fetchAllAccounts());
      dispatch(fetchAllTransaction());
      dispatch(fetchAllCategoryGroups(month));
      dispatch(fetchAllCategories());
      dispatch(fetchInflowAmount());
      dispatch(fetchAllPayees());
      dispatch(fetchAllLoanMetadata());
      dispatch(fetchAllTags());
      dispatch(fetchPipelineRuns());
      // fetch inflow category from selected budget metadata
      const selectedBudget = (store.getState() as RootState).budgets
        .selectedBudget;
      if (selectedBudget?.metadata?.inflowCategoryId) {
        dispatch(fetchCategoryById(selectedBudget.metadata.inflowCategoryId));
      }
    }

    if (setSelectedBudget.match(action)) {
      dispatch(fetchAllAccounts());
      dispatch(fetchAllTransaction());
      dispatch(fetchAllCategoryGroups(month));
      dispatch(fetchAllCategories());
      dispatch(fetchInflowAmount());
      dispatch(fetchAllPayees());
      dispatch(fetchAllLoanMetadata());
      dispatch(fetchAllTags());
      dispatch(fetchPipelineRuns());
      if (action.payload.metadata?.inflowCategoryId) {
        dispatch(fetchCategoryById(action.payload.metadata.inflowCategoryId));
      }
    }
    return result;
  };

/**
 * Listen to the date change and fetch the category budget data
 */
export const dateChangeMiddleware: Middleware =
  (store) => (next) => (action) => {
    const result = next(action);

    const dispatch = store.dispatch as AppDispatch;

    if (setSelectedMonth.match(action)) {
      dispatch(fetchAllCategoryGroups(action.payload));
    }

    return result;
  };

/**
 * Listen to the budget change and fetch the inflow amount
 */
export const budgetUpdateMiddleware: Middleware =
  (store) => (next) => (action) => {
    const result = next(action);
    const dispatch = store.dispatch as AppDispatch;

    if (updateCategoryBudget.fulfilled.match(action)) {
      dispatch(fetchInflowAmount());
    }

    return result;
  };
