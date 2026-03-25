import { configureStore } from '@reduxjs/toolkit';
import accounts from '../features/accounts/store/accountSlice';
import budgets from '../features/budget/store/budgetSlice';
import { categorySlice } from '@/features/category/store';
import payees from '@/features/payees/store';
import transactions from '@/features/transactions/store';
import authSlice from '@/features/auth/store';
import { dashboardSlice } from '@/features/dashboard/store';
import loans from '@/features/loans/store/loanSlice';
import tags from '@/features/tags/store';
import { apiClient } from '@/utils';
import {
  budgetUpdateMiddleware,
  dataFetchMiddleware,
  dateChangeMiddleware,
} from './middlewares';

export const store = configureStore({
  reducer: {
    auth: authSlice,
    accounts: accounts,
    budgets: budgets,
    payees: payees,
    dashboard: dashboardSlice,
    categories: categorySlice,
    transactions: transactions,
    loans: loans,
    tags: tags,
  },
  middleware: (getDefaultMiddleWare) =>
    getDefaultMiddleWare({
      // serializableCheck: {
      //   ignoredActions: [''],
      // },
    }).concat([
      dataFetchMiddleware,
      dateChangeMiddleware,
      budgetUpdateMiddleware,
    ]),
  devTools: true,
});

apiClient.setGetState(store.getState);
apiClient.setDispatch(store.dispatch);

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;
