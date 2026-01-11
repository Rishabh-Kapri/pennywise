import { Provider } from 'react-redux';
import { store } from './store';
import { BrowserRouter, Route, Routes } from 'react-router-dom';
import '../styles/index.css';
import { Layout } from '@/components/layout';
import { lazy, Suspense } from 'react';

const Dashboard = lazy(() => import('@/components/layout/Dashboard/Dashboard'));
const Budget = lazy(() => import('@/features/budget/components/Budget'));
const Transaction = lazy(() =>
  import('@/features/transactions/components/Transaction').then((module) => ({
    default: module.Transaction,
  })),
);
function App() {
  return (
    <Provider store={store}>
      <BrowserRouter>
        <Routes>
          <Route element={<Layout />}>
            <Route
              path="/"
              element={
                <Suspense fallback={<div>Loading...</div>}>
                  <Dashboard />
                </Suspense>
              }></Route>
            <Route
              path="/budget"
              element={
                <Suspense fallback={<div>This is loading...</div>}>
                  <Budget />
                </Suspense>
              }></Route>
            <Route
              path="/transactions/:id?"
              element={
                <Suspense fallback={<div>Something is happening...</div>}>
                  <Transaction />
                </Suspense>
              }></Route>
          </Route>
        </Routes>
      </BrowserRouter>
    </Provider>
  );
  // return (
  //   <Provider store={store}>
  //     <BrowserRouter>
  //       <Suspense fallback={<div>This is loading...</div>}>
  //         <Routes>
  //           <Route element={<Layout />}>
  //             <Route path="/" element={<Dashboard />}></Route>
  //             <Route path="/budget" element={<Budget />}></Route>
  //             <Route
  //               path="/transactions/:id?"
  //               element={<Transaction />}></Route>
  //           </Route>
  //         </Routes>
  //       </Suspense>
  //     </BrowserRouter>
  //   </Provider>
  // );
}

export default App;
