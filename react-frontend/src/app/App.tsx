import { Provider } from 'react-redux';
import { store } from './store';
import { BrowserRouter, Navigate, Route, Routes, useParams } from 'react-router-dom';
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
            <Route path="/loans/:id?" element={<LoansRedirect />} />
            <Route
              path="/payees"
              element={
                <Suspense fallback={<div>Loading...</div>}>
                  <Payees />
                </Suspense>
              }
            />
            <Route
              path="/reports"
              element={
                <Suspense fallback={<div>Loading...</div>}>
                  <Reports />
                </Suspense>
              }
            />
            {/* These all live in Settings now; keep the old paths working */}
            <Route path="/activity" element={<Navigate to="/settings?section=activity" replace />} />
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
