import { configureStore } from '@reduxjs/toolkit';
import accounts from '../features/accounts/store/accountSlice';
import agent from '../features/agent/store/agentSlice';
import auth from '../features/auth/store/authSlice';
import budgets from '../features/budget/store/budgetSlice';
import categories from '../features/category/store/categorySlice';
import loans from '../features/loans/store/loanSlice';
import payees from '../features/payees/store/payeeSlice';
import pipeline from '../features/pipeline/store/pipelineSlice';
import tags from '../features/tags/store/tagSlice';
import transactions from '../features/transactions/store/transactionSlice';
import { apiClient } from '../utils/api';
import {
  budgetPersistenceMiddleware,
  budgetUpdateMiddleware,
  dataFetchMiddleware,
  dateChangeMiddleware,
  widgetRefreshMiddleware
} from './middlewares';

export const store = configureStore({
  reducer: {
    accounts,
    agent,
    auth,
    budgets,
    categories,
    loans,
    payees,
    pipeline,
    tags,
    transactions
  },
  middleware: (getDefaultMiddleware) =>
    getDefaultMiddleware({
      serializableCheck: false
    }).concat([
      dataFetchMiddleware,
      dateChangeMiddleware,
      budgetUpdateMiddleware,
      budgetPersistenceMiddleware,
      widgetRefreshMiddleware
    ]),
  devTools: true
});

apiClient.setGetState(store.getState);
apiClient.setDispatch(store.dispatch);

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;
