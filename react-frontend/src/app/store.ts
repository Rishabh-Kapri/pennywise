import { configureStore } from '@reduxjs/toolkit';
import accounts from '../features/accounts/store/accountSlice';
import budgets from '../features/budget/store/budgetSlice';
import categoryGroups from '@/features/category/store';
import payees from '@/features/payees/store';
import transactions from '@/features/transactions/store';
import { apiClient } from '@/utils';
import { dataFetchMiddleware, dateChangeMiddleware } from './middlewares';

export const store = configureStore({
  reducer: {
    accounts: accounts,
    budgets: budgets,
    payees: payees,
    categoryGroups: categoryGroups,
    transactions: transactions,
  },
  middleware: (getDefaultMiddleWare) =>
    getDefaultMiddleWare({
      // serializableCheck: {
      //   ignoredActions: [''],
      // },
    }).concat([dataFetchMiddleware, dateChangeMiddleware]),
  devTools: true,
});

apiClient.setGetState(store.getState);

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;
