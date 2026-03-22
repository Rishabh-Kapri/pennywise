import { useState, useEffect, useMemo } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Landmark, Pencil, Plus } from 'lucide-react';
import { useAppSelector, useAppDispatch } from '@/app/hooks';
import { selectLoanAccounts } from '@/features/accounts/store/accountSlice';
import { selectAllLoanMetadata } from '../store/loanSlice';
import { fetchAllTransaction } from '@/features/transactions/store/transactionSlice';
import type { Transaction } from '@/features/transactions/types/transaction.types';
import { apiClient } from '@/utils';
import {
  calculateAmortizationSchedule,
  calculateTotalInterest,
  formatPayoffDuration,
} from '../utils/payoffCalculator';
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
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
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

  const loanMeta = selectedAccount?.id
    ? allMetadata[selectedAccount.id] ?? null
    : null;

  // Fetch transactions when switching to activity tab or when account changes
  useEffect(() => {
    if (activeTab === 'activity' && selectedAccount?.id) {
      dispatch(fetchAllTransaction(selectedAccount.id));
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
    apiClient.get<Transaction[]>('transactions/normalized').then((allTxns) => {
      const map: Record<string, string> = {};
      for (const txn of allTxns) {
        if (txn.id && idsNeedingCategory.includes(txn.id) && txn.categoryName) {
          map[txn.id] = txn.categoryName;
        }
      }
      setCategoryMap(map);
    }).catch(() => { /* silent */ });
  }, [activeTab, transactions]);

  // Computed payoff data
  const payoffData = useMemo(() => {
    if (!loanMeta || !selectedAccount) return null;

    // account.balance = sum of payments (transactions) TO the loan account
    // remainingBalance = originalBalance - totalPayments
    const totalPayments = Math.abs(selectedAccount.balance ?? 0);
    const currentBalance = Math.max(loanMeta.originalBalance - totalPayments, 0);
    const baseInput = {
      currentBalance,
      interestRate: loanMeta.interestRate,
      monthlyPayment: loanMeta.monthlyPayment,
    };

    const schedule = calculateAmortizationSchedule(baseInput);
    const totalInterest = calculateTotalInterest(baseInput);
    const payoffMonths = schedule.length;
    const totalPaid = currentBalance + totalInterest;
    const paidSoFar = totalPayments;
    const percentPaid =
      loanMeta.originalBalance > 0
        ? Math.min(Math.round((paidSoFar / loanMeta.originalBalance) * 100), 100)
        : 0;

    // Projected payoff date
    const payoffDate = new Date();
    payoffDate.setMonth(payoffDate.getMonth() + payoffMonths);

    return {
      currentBalance,
      totalInterest,
      payoffMonths,
      totalPaid,
      paidSoFar,
      percentPaid,
      payoffDate,
    };
  }, [selectedAccount, loanMeta]);

  // Navigate to first loan if none selected
  if (!id && loanAccounts.length > 0 && loanAccounts[0]?.id) {
    navigate(`/loans/${loanAccounts[0].id}`, { replace: true });
    return null;
  }

  // Empty state — no loan accounts
  if (loanAccounts.length === 0) {
    return (
      <div className={styles.container}>
        <div className={styles.emptyState}>
          <Landmark size={48} />
          <h2>No Loan Accounts Yet</h2>
          <p>
            Track your mortgages, auto loans, student loans, and more. Add a loan
            to start visualizing your path to being debt-free.
          </p>
          <button className={styles.addBtn} onClick={() => setShowAddForm(true)}>
            <Plus size={16} /> Add Loan Account
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
            This loan account doesn&apos;t have interest rate or payment information
            yet. Please re-create it with the loan form.
          </p>
        </div>
      </div>
    );
  }

  const circumference = 2 * Math.PI * 56; // radius = 56

  return (
    <div className={styles.container}>
      {/* Header */}
      <div className={styles.header}>
        <h1>{selectedAccount.name}</h1>
        <div className={styles.headerRight}>
          <button className={styles.editBtn} onClick={() => setShowEditForm(true)}>
            <Pencil size={16} /> Edit
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
                    strokeDashoffset={
                      circumference - (circumference * payoffData.percentPaid) / 100
                    }
                  />
                </svg>
                <span className={styles.progressPercent}>{payoffData.percentPaid}%</span>
              </div>
              <span className={styles.progressLabel}>Paid Off</span>
            </div>

            {/* Balance Card */}
            <div className={styles.balanceCard}>
              <div className={styles.balanceRow}>
                <span className={styles.balanceLabel}>Remaining Balance</span>
                <span className={`${styles.balanceValue} ${styles.balanceNegative}`}>
                  {formatCurrency(payoffData.currentBalance)}
                </span>
              </div>
              <div className={styles.balanceRow}>
                <span className={styles.balanceLabel}>Principal Paid</span>
                <span className={`${styles.balanceValue} ${styles.balancePositive}`}>
                  {formatCurrency(payoffData.paidSoFar)}
                </span>
              </div>
              <div className={styles.balanceRow}>
                <span className={styles.balanceLabel}>Total Interest (projected)</span>
                <span className={styles.balanceValue}>
                  {formatCurrency(payoffData.totalInterest)}
                </span>
              </div>
              <div className={styles.balanceRow}>
                <span className={styles.balanceLabel}>Total Cost</span>
                <span className={styles.balanceValue}>
                  {formatCurrency(payoffData.totalPaid)}
                </span>
              </div>
            </div>

            {/* Insight Card */}
            <div className={styles.insightCard}>
              <span className={styles.insightTitle}>Payoff Insight</span>
              <p className={styles.insightBody}>
                At your current monthly payment of{' '}
                <span className={styles.insightHighlight}>
                  {formatCurrency(loanMeta.monthlyPayment)}
                </span>
                , you&apos;ll pay off this loan in{' '}
                <span className={styles.insightHighlight}>
                  {formatPayoffDuration(payoffData.payoffMonths)}
                </span>
                , with{' '}
                <span className={styles.insightHighlight}>
                  {formatCurrency(payoffData.totalInterest)}
                </span>{' '}
                in total interest.
              </p>
              <div className={styles.insightActions}>
                <button
                  className={styles.simulatorBtn}
                  onClick={() => setShowSimulator(true)}
                >
                  Open Payoff Simulator
                </button>
              </div>
            </div>

            {/* Details Row */}
            <div className={styles.detailsCard}>
              <div className={styles.detailItem}>
                <span className={styles.detailLabel}>Interest Rate</span>
                <span className={styles.detailValue}>{loanMeta.interestRate}%</span>
              </div>
              <div className={styles.detailItem}>
                <span className={styles.detailLabel}>Monthly Payment</span>
                <span className={styles.detailValue}>
                  {formatCurrency(loanMeta.monthlyPayment)}
                </span>
              </div>
              <div className={styles.detailItem}>
                <span className={styles.detailLabel}>Original Balance</span>
                <span className={styles.detailValue}>
                  {formatCurrency(loanMeta.originalBalance)}
                </span>
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
                    <td>{new Date(txn.date).toLocaleDateString('en-IN', { day: 'numeric', month: 'short', year: 'numeric' })}</td>
                    <td>{txn.payeeName || '—'}</td>
                    <td>{txn.categoryName || categoryMap[txn.transferTransactionId ?? ''] || '—'}</td>
                    <td className={`${styles.txnAmountCol} ${(txn.inflow ?? 0) > 0 ? styles.balancePositive : styles.balanceNegative}`}>
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
