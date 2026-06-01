import { Provider } from 'react-redux';
import { store } from './store';
import { BrowserRouter, Route, Routes } from 'react-router-dom';
import '../styles/index.css';
import { Layout } from '@/components/layout';
import { lazy, Suspense } from 'react';
import { Login, ProtectedRoute } from '@/features/auth';
import Homepage from '@/features/home/components/Homepage';
import LegalPage from '@/features/home/components/LegalPage';

const Dashboard = lazy(() => import('@/components/layout/Dashboard/Dashboard'));
const Budget = lazy(() => import('@/features/budget/components/Budget'));
const BudgetOnboarding = lazy(
  () => import('@/features/budget/components/BudgetOnboarding'),
);
const Settings = lazy(() => import('@/features/settings/components/Settings'));
const Transaction = lazy(() =>
  import('@/features/transactions/components/Transaction').then((module) => ({
    default: module.Transaction,
  })),
);
const LoanOverview = lazy(() =>
  import('@/features/loans/components/LoanOverview'),
);
const Payees = lazy(() => import('@/features/payees/components/Payees'));

function App() {
  return (
    <Provider store={store}>
      <BrowserRouter>
        <Routes>
          <Route path="/" element={<Homepage />} />
          <Route path="/terms" element={<LegalPage />} />
          <Route path="/privacy" element={<LegalPage />} />

          {/* Public route - Login */}
          <Route
            path="/login"
            element={
              <Suspense fallback={<div>Loading...</div>}>
                <Login />
              </Suspense>
            }
          />
          <Route
            path="/signup"
            element={
              <Suspense fallback={<div>Loading...</div>}>
                <Login />
              </Suspense>
            }
          />

          <Route
            path="/budget/new"
            element={
              <ProtectedRoute>
                <Suspense fallback={<div>Loading...</div>}>
                  <BudgetOnboarding />
                </Suspense>
              </ProtectedRoute>
            }
          />

          {/* Protected routes - require authentication */}
          <Route
            element={
              <ProtectedRoute>
                <Layout />
              </ProtectedRoute>
            }>
            <Route
              path="/dashboard"
              element={
                <Suspense fallback={<div>Loading...</div>}>
                  <Dashboard />
                </Suspense>
              }
            />
            <Route
              path="/budget"
              element={
                <Suspense fallback={<div>Loading...</div>}>
                  <Budget />
                </Suspense>
              }
            />
            <Route
              path="/settings"
              element={
                <Suspense fallback={<div>Loading...</div>}>
                  <Settings />
                </Suspense>
              }
            />
            <Route
              path="/transactions/:id?"
              element={
                <Suspense fallback={<div>Loading...</div>}>
                  <Transaction />
                </Suspense>
              }
            />
            <Route
              path="/loans/:id?"
              element={
                <Suspense fallback={<div>Loading...</div>}>
                  <LoanOverview />
                </Suspense>
              }
            />
            <Route
              path="/payees"
              element={
                <Suspense fallback={<div>Loading...</div>}>
                  <Payees />
                </Suspense>
              }
            />
          </Route>
        </Routes>
      </BrowserRouter>
    </Provider>
  );
}

export default App;
