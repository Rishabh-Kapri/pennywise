import { useState, useEffect, useMemo } from 'react';
import { useSearchParams } from 'react-router-dom';
import { BankIcon as Landmark, PencilIcon, PlusIcon } from '@phosphor-icons/react';
import { useAppSelector, useAppDispatch } from '@/app/hooks';
import { selectLoanAccounts } from '@/features/accounts/store/accountSlice';
import { selectAllLoanMetadata } from '../store/loanSlice';
import { fetchAllTransaction } from '@/features/transactions/store/transactionSlice';
import type { Transaction } from '@/features/transactions/types/transaction.types';
import { apiClient } from '@/utils';
import { calculateAmortizationSchedule, calculateTotalInterest, formatPayoffDuration } from '../utils/payoffCalculator';
import { useLoanHistory } from '../hooks/useLoanHistory';
import LoanPaymentSplit from './LoanPaymentSplit';
import PayoffSimulator from './PayoffSimulator';
import LoanAccountForm from './LoanAccountForm';
import styles from './LoanOverview.module.css';

function formatCurrency(value: number): string {
  return new Intl.NumberFormat('en-IN', {
    style: 'currency',
    currency: 'INR',
    maximumFractionDigits: 0,
  }).format(value);
}

export default function LoanOverview() {
  // rendered as a Settings section, so the selected loan rides on ?account=
  const [searchParams, setSearchParams] = useSearchParams();
  const id = searchParams.get('account') ?? undefined;
  const dispatch = useAppDispatch();
  const loanAccounts = useAppSelector(selectLoanAccounts);
  const allMetadata = useAppSelector(selectAllLoanMetadata);
  const { transactions } = useAppSelector((state) => state.transactions);

  const [activeTab, setActiveTab] = useState<'overview' | 'activity'>('overview');
  const [showSimulator, setShowSimulator] = useState(false);
  const [showAddForm, setShowAddForm] = useState(false);
  const [showEditForm, setShowEditForm] = useState(false);

  // Find the selected loan account
  const selectedAccount = useMemo(
    () => loanAccounts.find((acc) => acc.id === id) ?? loanAccounts[0] ?? null,
    [loanAccounts, id],
  );

  const loanMeta = selectedAccount?.id ? (allMetadata[selectedAccount.id] ?? null) : null;
  // refetch when a payment lands (balance) or the terms are edited
  const { history, error: historyError } = useLoanHistory(
    selectedAccount?.id,
    `${selectedAccount?.balance ?? 0}:${JSON.stringify(loanMeta)}`,
  );

  // Fetch transactions when switching to activity tab or when account changes
  useEffect(() => {
    if (activeTab === 'activity' && selectedAccount?.id) {
      dispatch(fetchAllTransaction({ accountIds: [selectedAccount.id] }));
    }
  }, [activeTab, selectedAccount?.id, dispatch]);

  // Build category lookup from paired transfer transactions
  const [categoryMap, setCategoryMap] = useState<Record<string, string>>({});
  useEffect(() => {
    if (activeTab !== 'activity' || transactions.length === 0) return;

    const idsNeedingCategory = transactions
      .filter((t) => !t.categoryName && t.transferTransactionId)
      .map((t) => t.transferTransactionId!);

    if (idsNeedingCategory.length === 0) return;

    // Fetch all transactions to find the paired ones with categories
    apiClient
      .get<Transaction[]>('transactions/normalized')
      .then((allTxns) => {
        const map: Record<string, string> = {};
        for (const txn of allTxns) {
          if (txn.id && idsNeedingCategory.includes(txn.id) && txn.categoryName) {
            map[txn.id] = txn.categoryName;
          }
        }
        setCategoryMap(map);
      })
      .catch(() => {
        /* silent */
      });
  }, [activeTab, transactions]);

  // Computed payoff data. The balance comes from the API's amortized history,
  // where each recorded payment first clears accrued interest; only the rest
  // reduces principal. Until it loads (or if it fails), fall back to treating
  // every payment as principal, which overstates progress.
  const payoffData = useMemo(() => {
    if (!loanMeta || !selectedAccount) return null;

    const totalPayments = Math.abs(selectedAccount.balance ?? 0);
    const currentBalance = history ? history.balance : Math.max(loanMeta.originalBalance - totalPayments, 0);
    const principalPaid = history ? history.principalPaid : totalPayments;
    const interestPaid = history ? history.interestPaid : 0;
    const baseInput = {
      currentBalance,
      interestRate: loanMeta.interestRate,
      monthlyPayment: loanMeta.monthlyPayment,
    };

    const schedule = calculateAmortizationSchedule(baseInput);
    const remainingInterest = calculateTotalInterest(baseInput);
    const payoffMonths = schedule.length;
    const percentPaid =
      loanMeta.originalBalance > 0 ? Math.min(Math.round((principalPaid / loanMeta.originalBalance) * 100), 100) : 0;

    // what the next payment does: interest on what's left, the rest to principal
    const nextInterest = schedule[0]?.interestPayment ?? 0;

    // Projected payoff date
    const payoffDate = new Date();
    payoffDate.setMonth(payoffDate.getMonth() + payoffMonths);

    return {
      currentBalance,
      principalPaid,
      interestPaid,
      remainingInterest,
      lifetimeInterest: interestPaid + remainingInterest,
      payoffMonths,
      percentPaid,
      nextInterest,
      payoffDate,
      exact: history !== null,
    };
  }, [selectedAccount, loanMeta, history]);

  const selectAccount = (accountId: string) => {
    setSearchParams({ section: 'loans', account: accountId }, { replace: true });
  };

  // Empty state — no loan accounts
  if (loanAccounts.length === 0) {
    return (
      <div className={styles.container}>
        <div className={styles.emptyState}>
          <Landmark size={48} />
          <h2>No Loan Accounts Yet</h2>
          <p>
            Track your mortgages, auto loans, student loans, and more. Add a loan to start visualizing your path to
            being debt-free.
          </p>
          <button className={styles.addBtn} onClick={() => setShowAddForm(true)}>
            <PlusIcon size={16} /> Add Loan Account
          </button>
        </div>
        <LoanAccountForm isOpen={showAddForm} onClose={() => setShowAddForm(false)} />
      </div>
    );
  }

  // No metadata for this account
  if (!loanMeta || !payoffData || !selectedAccount) {
    return (
      <div className={styles.container}>
        <div className={styles.emptyState}>
          <h2>Loan Details Missing</h2>
          <p>
            This loan account doesn&apos;t have interest rate or payment information yet. Please re-create it with the
            loan form.
          </p>
        </div>
      </div>
    );
  }

  const circumference = 2 * Math.PI * 56; // radius = 56

  return (
    <div className={styles.container}>
      {/* Account switcher — the sidebar deep-links here, but the section has
          to stand on its own when opened from Settings */}
      {loanAccounts.length > 1 && (
        <div className={styles.accountSwitcher}>
          {loanAccounts.map((account) => (
            <button
              key={account.id}
              type="button"
              className={account.id === selectedAccount.id ? styles.accountPillActive : styles.accountPill}
              onClick={() => account.id && selectAccount(account.id)}
            >
              {account.name}
            </button>
          ))}
        </div>
      )}

      {/* Header */}
      <div className={styles.header}>
        <h2>{selectedAccount.name}</h2>
        <div className={styles.headerRight}>
          <button className={styles.editBtn} onClick={() => setShowEditForm(true)}>
            <PencilIcon size={16} /> Edit
          </button>
          <button className={styles.simulatorBtn} onClick={() => setShowSimulator(true)}>
            Payoff Simulator
          </button>
        </div>
      </div>

      {/* Tabs */}
      <div className={styles.tabs}>
        <button
          className={activeTab === 'overview' ? styles.tabActive : styles.tab}
          onClick={() => setActiveTab('overview')}
        >
          Overview
        </button>
        <button
          className={activeTab === 'activity' ? styles.tabActive : styles.tab}
          onClick={() => setActiveTab('activity')}
        >
          Activity
        </button>
      </div>

      {activeTab === 'overview' && (
        <>
          <div className={styles.overviewGrid}>
            {/* Progress Ring */}
            <div className={styles.progressCard}>
              <div className={styles.progressRing}>
                <svg viewBox="0 0 128 128">
                  <circle className={styles.progressBg} cx="64" cy="64" r="56" />
                  <circle
                    className={styles.progressFill}
                    cx="64"
                    cy="64"
                    r="56"
                    strokeDasharray={circumference}
                    strokeDashoffset={circumference - (circumference * payoffData.percentPaid) / 100}
                  />
                </svg>
                <span className={styles.progressPercent}>{payoffData.percentPaid}%</span>
              </div>
              <span className={styles.progressLabel}>Principal repaid</span>
            </div>

            {/* Balance Card */}
            <div className={styles.balanceCard}>
              <div className={styles.balanceRow}>
                <span className={styles.balanceLabel}>Principal Outstanding</span>
                <span className={`${styles.balanceValue} ${styles.balanceNegative}`}>
                  {formatCurrency(payoffData.currentBalance)}
                </span>
              </div>
              <div className={styles.balanceRow}>
                <span className={styles.balanceLabel}>Paid So Far</span>
                <span className={styles.balanceValue}>
                  {formatCurrency(payoffData.principalPaid + payoffData.interestPaid)}
                </span>
              </div>
              <div className={styles.balanceSubRow}>
                <span>Principal</span>
                <span className={styles.balancePositive}>{formatCurrency(payoffData.principalPaid)}</span>
              </div>
              <div className={styles.balanceSubRow}>
                <span>Interest</span>
                <span>{formatCurrency(payoffData.interestPaid)}</span>
              </div>
              <div className={styles.balanceRow}>
                <span className={styles.balanceLabel}>Interest Still to Pay (projected)</span>
                <span className={styles.balanceValue}>{formatCurrency(payoffData.remainingInterest)}</span>
              </div>
              <div className={styles.balanceRow}>
                <span className={styles.balanceLabel}>Total Interest Over the Loan</span>
                <span className={styles.balanceValue}>{formatCurrency(payoffData.lifetimeInterest)}</span>
              </div>
              {!payoffData.exact && (
                <span className={styles.balanceNote}>
                  {historyError
                    ? "Couldn't load the payment history, so every payment is counted as principal for now."
                    : 'Loading the payment history…'}
                </span>
              )}
            </div>

            {/* Insight Card */}
            <div className={styles.insightCard}>
              <span className={styles.insightTitle}>Payoff Insight</span>
              {payoffData.exact && payoffData.nextInterest > 0 && loanMeta.monthlyPayment > 0 && (
                <p className={styles.insightBody}>
                  Right now <span className={styles.insightHighlight}>{formatCurrency(payoffData.nextInterest)}</span>{' '}
                  of each {formatCurrency(loanMeta.monthlyPayment)} payment (
                  {Math.round(Math.min(payoffData.nextInterest / loanMeta.monthlyPayment, 1) * 100)}%) goes to interest;
                  only the rest brings the balance down. That share shrinks as the principal does, which is why extra
                  payments early on save the most.
                </p>
              )}
              <p className={styles.insightBody}>
                At your current monthly payment of{' '}
                <span className={styles.insightHighlight}>{formatCurrency(loanMeta.monthlyPayment)}</span>, you&apos;ll
                pay off this loan in{' '}
                <span className={styles.insightHighlight}>{formatPayoffDuration(payoffData.payoffMonths)}</span>, with{' '}
                <span className={styles.insightHighlight}>{formatCurrency(payoffData.remainingInterest)}</span> more in
                interest.
              </p>
              <div className={styles.insightActions}>
                <button className={styles.simulatorBtn} onClick={() => setShowSimulator(true)}>
                  Open Payoff Simulator
                </button>
              </div>
            </div>

            {history && history.months.some((m) => m.payment > 0) && <LoanPaymentSplit months={history.months} />}

            {/* Details Row */}
            <div className={styles.detailsCard}>
              <div className={styles.detailItem}>
                <span className={styles.detailLabel}>Interest Rate</span>
                <span className={styles.detailValue}>{loanMeta.interestRate}%</span>
              </div>
              <div className={styles.detailItem}>
                <span className={styles.detailLabel}>Monthly Payment</span>
                <span className={styles.detailValue}>{formatCurrency(loanMeta.monthlyPayment)}</span>
              </div>
              <div className={styles.detailItem}>
                <span className={styles.detailLabel}>Original Balance</span>
                <span className={styles.detailValue}>{formatCurrency(loanMeta.originalBalance)}</span>
              </div>
              <div className={styles.detailItem}>
                <span className={styles.detailLabel}>Debt Free Date</span>
                <span className={styles.detailValue}>
                  {payoffData.payoffDate.toLocaleDateString('en-IN', {
                    month: 'short',
                    year: 'numeric',
                  })}
                </span>
              </div>
            </div>
          </div>
        </>
      )}

      {activeTab === 'activity' && (
        <div className={styles.activitySection}>
          {transactions.length === 0 ? (
            <div className={styles.emptyState}>
              <p>No transactions recorded for this loan account yet.</p>
            </div>
          ) : (
            <table className={styles.txnTable}>
              <thead>
                <tr>
                  <th>Date</th>
                  <th>Payee</th>
                  <th>Category</th>
                  <th className={styles.txnAmountCol}>Amount</th>
                </tr>
              </thead>
              <tbody>
                {transactions.map((txn: Transaction) => (
                  <tr key={txn.id}>
                    <td>
                      {new Date(txn.date).toLocaleDateString('en-IN', {
                        day: 'numeric',
                        month: 'short',
                        year: 'numeric',
                      })}
                    </td>
                    <td>{txn.payeeName || '—'}</td>
                    <td>{txn.categoryName || categoryMap[txn.transferTransactionId ?? ''] || '—'}</td>
                    <td
                      className={`${styles.txnAmountCol} ${(txn.inflow ?? 0) > 0 ? styles.balancePositive : styles.balanceNegative}`}
                    >
                      {formatCurrency(txn.inflow ?? txn.outflow ?? 0)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}

      {/* Payoff Simulator Modal */}
      <PayoffSimulator
        isOpen={showSimulator}
        onClose={() => setShowSimulator(false)}
        currentBalance={payoffData.currentBalance}
        interestRate={loanMeta.interestRate}
        minimumPayment={loanMeta.monthlyPayment}
      />

      {/* Edit Loan Modal */}
      <LoanAccountForm
        isOpen={showEditForm}
        onClose={() => setShowEditForm(false)}
        editData={{
          accountId: selectedAccount.id!,
          accountName: selectedAccount.name,
          metadata: loanMeta,
        }}
      />
    </div>
  );
}
