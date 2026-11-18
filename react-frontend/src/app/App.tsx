import { Provider } from 'react-redux';
import { store } from './store';
import { BrowserRouter, Navigate, Route, Routes, useParams } from 'react-router-dom';
import Layout from '@/components/layout/Layout/Layout';
import { lazy, Suspense } from 'react';
import { Login, ProtectedRoute } from '@/features/auth';
import Homepage from '@/features/home/components/Homepage';
import LegalPage from '@/features/home/components/LegalPage';

const Dashboard = lazy(() => import('@/features/dashboard/components/Dashboard/Dashboard'));
const Budget = lazy(() => import('@/features/budget/components/Budget'));
const BudgetOnboarding = lazy(
  () => import('@/features/budget/components/BudgetOnboarding'),
);
const Settings = lazy(() => import('@/features/settings/components/Settings'));
const Accounts = lazy(() => import('@/features/accounts/components/Accounts'));
const Transaction = lazy(() =>
  import('@/features/transactions/components/Transaction').then((module) => ({
    default: module.Transaction,
  })),
);
const Payees = lazy(() => import('@/features/payees/components/Payees'));
const Reports = lazy(() => import('@/features/reports/components/Reports'));

/** /loans/:id kept working after loans moved into Settings. */
function LoansRedirect() {
  const { id } = useParams<{ id: string }>();
  const target = id ? `/settings?section=loans&account=${id}` : '/settings?section=loans';
  return <Navigate to={target} replace />;
}

function App() {
  return (
    <Provider store={store}>
      <BrowserRouter>
        <Routes>
          <Route path="/" element={<Homepage />} />
          <Route path="/terms" element={<LegalPage />} />
          <Route path="/privacy" element={<LegalPage />} />

          <Route path="/login" element={<Login />} />
          <Route path="/signup" element={<Login />} />

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
            <Route path="/dashboard" element={<Dashboard />} />
            <Route path="/budget" element={<Budget />} />
            <Route path="/settings" element={<Settings />} />
            <Route path="/transactions/:id?" element={<Transaction />} />
            <Route path="/accounts" element={<Accounts />} />
            <Route path="/loans/:id?" element={<LoansRedirect />} />
            <Route path="/payees" element={<Payees />} />
            <Route path="/reports" element={<Reports />} />
            {/* These all live in Settings now; keep the old paths working */}
            <Route path="/activity" element={<Navigate to="/settings?section=jobs" replace />} />
            <Route
              path="/recurring"
              element={<Navigate to="/settings?section=recurring" replace />}
            />
            <Route path="/review" element={<Navigate to="/settings?section=review" replace />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </Provider>
  );
}

export default App;
