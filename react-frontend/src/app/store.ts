import { configureStore } from '@reduxjs/toolkit';
import accounts from '../features/accounts/store/accountSlice';
import budgets from '../features/budget/store/budgetSlice';
import categorySlice from '@/features/category/store/categorySlice';
import payees from '@/features/payees/store/payeeSlice';
import transactions from '@/features/transactions/store/transactionSlice';
import authSlice, { saveSession, sessionEnded, tokenRefreshed } from '@/features/auth/store/authSlice';
import dashboardSlice from '@/features/dashboard/store/dashboardSlice';
import loans from '@/features/loans/store/loanSlice';
import tags from '@/features/tags/store/tagSlice';
import agent from '@/features/agent/store/agentSlice';
import reports, { saveTreatments } from '@/features/reports/store/reportSlice';
import jobs from '@/features/jobs/store/jobSlice';
import recurring from '@/features/recurring/store/recurringSlice';
import predictionReview from '@/features/predictionReview/store/predictionReviewSlice';
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

// What outlives a reload is saved here, so reducers stay free of storage.
let saved = store.getState();
store.subscribe(() => {
  const state = store.getState();
  if (state.auth.user !== saved.auth.user || state.auth.tokens !== saved.auth.tokens) saveSession(state.auth);
  if (state.reports.groupTreatments !== saved.reports.groupTreatments) saveTreatments(state.reports.groupTreatments);
  saved = state;
});

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;
