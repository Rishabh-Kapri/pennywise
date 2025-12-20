import { fetchAllAccounts } from '@/features/accounts/store/accountSlice';
import { fetchAllBudgets, setSelectedMonth } from '@/features/budget';
import type { Middleware } from '@reduxjs/toolkit';
import { fetchAllCategoryGroups, fetchInflowAmount } from '@/features';
import type { AppDispatch, RootState } from '.';
import { fetchAllPayees } from '@/features/payees/store/payeeSlice';

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
      dispatch(fetchAllCategoryGroups(month));
      dispatch(fetchInflowAmount());
      dispatch(fetchAllPayees());
      // dispatch(fetchAllAccounts());
    }
    return result;
  };

/**
 * Listen toe the date change and fetch the category budget data
 */
export const dateChangeMiddleware: Middleware =
  (store) => (next) => (action) => {
    const result = next(action);

    const dispatch = store.dispatch as AppDispatch;

    if (setSelectedMonth.match(action)) {
      console.log('Received state:', action);
      dispatch(fetchAllCategoryGroups(action.payload));
    }

    return result;
  };
