import { configureStore } from '@reduxjs/toolkit';
import accounts from '../features/accounts/store/accountSlice';
import auth from '../features/auth/store/authSlice';
import budgets from '../features/budget/store/budgetSlice';
import categories from '../features/category/store/categorySlice';
import loans from '../features/loans/store/loanSlice';
import payees from '../features/payees/store/payeeSlice';
import tags from '../features/tags/store/tagSlice';
import transactions from '../features/transactions/store/transactionSlice';
import { apiClient } from '../utils/api';
import { budgetUpdateMiddleware, dataFetchMiddleware, dateChangeMiddleware } from './middlewares';

export const store = configureStore({
  reducer: {
    accounts,
    auth,
    budgets,
    categories,
    loans,
    payees,
    tags,
    transactions
  },
  middleware: (getDefaultMiddleware) =>
    getDefaultMiddleware({
      serializableCheck: false
    }).concat([dataFetchMiddleware, dateChangeMiddleware, budgetUpdateMiddleware]),
  devTools: true
});

apiClient.setGetState(store.getState);
apiClient.setDispatch(store.dispatch);

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;
