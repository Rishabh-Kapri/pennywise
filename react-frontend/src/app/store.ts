import { configureStore } from '@reduxjs/toolkit';
import accounts from '../features/accounts/store/accountSlice';
import budgets from '../features/budget/store/budgetSlice';
import { categorySlice } from '@/features/category/store';
import payees from '@/features/payees/store';
import transactions from '@/features/transactions/store';
import authSlice, { saveSession, sessionEnded, tokenRefreshed } from '@/features/auth/store';
import { dashboardSlice } from '@/features/dashboard/store';
import loans from '@/features/loans/store/loanSlice';
import tags from '@/features/tags/store';
import agent from '@/features/agent/store';
import reports from '@/features/reports/store';
import jobs from '@/features/jobs/store';
import recurring from '@/features/recurring/store';
import predictionReview from '@/features/predictionReview/store';
import { apiClient } from '@/utils';
import { appDataListener } from './middlewares';

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
    agent: agent,
    reports: reports,
    jobs: jobs,
    recurring: recurring,
    predictionReview: predictionReview,
  },
  middleware: (getDefaultMiddleware) => getDefaultMiddleware().prepend(appDataListener.middleware),
});

apiClient.connect({
  accessToken: () => store.getState().auth.tokens?.accessToken,
  refreshToken: () => store.getState().auth.tokens?.refreshToken,
  budgetId: () => store.getState().budgets.selectedBudget?.id,
  tokenRefreshed: (token) => store.dispatch(tokenRefreshed(token)),
  expired: () => {
    store.dispatch(sessionEnded());
    window.location.href = '/login';
  },
});

let savedAuth = store.getState().auth;
store.subscribe(() => {
  const { auth } = store.getState();
  if (auth.user === savedAuth.user && auth.tokens === savedAuth.tokens) return;
  savedAuth = auth;
  saveSession(auth);
});

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;
