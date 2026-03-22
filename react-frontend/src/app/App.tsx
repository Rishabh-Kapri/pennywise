import { Provider } from 'react-redux';
import { store } from './store';
import { BrowserRouter, Route, Routes } from 'react-router-dom';
import '../styles/index.css';
import { Layout } from '@/components/layout';
import { lazy, Suspense } from 'react';
import { Login, ProtectedRoute } from '@/features/auth';

const Dashboard = lazy(() => import('@/components/layout/Dashboard/Dashboard'));
const Budget = lazy(() => import('@/features/budget/components/Budget'));
const Transaction = lazy(() =>
  import('@/features/transactions/components/Transaction').then((module) => ({
    default: module.Transaction,
  })),
);
const LoanOverview = lazy(() =>
  import('@/features/loans/components/LoanOverview'),
);

function App() {
  return (
    <Provider store={store}>
      <BrowserRouter>
        <Routes>
          {/* Public route - Login */}
          <Route
            path="/login"
            element={
              <Suspense fallback={<div>Loading...</div>}>
                <Login />
              </Suspense>
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
              path="/"
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
          </Route>
        </Routes>
      </BrowserRouter>
    </Provider>
  );
}

export default App;
