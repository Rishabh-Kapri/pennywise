import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { fetchAllAccounts } from '@/features/accounts/store/accountSlice';
import {
  BudgetAccountNames,
  LoanAccountNames,
  TrackingAccountNames,
  type Account,
} from '@/features/accounts/types/account.types';
import { selectSelectedBudget } from '@/features/budget';
import { Transaction } from '@/features/transactions/components/Transaction/Transaction';
import { LoadingState } from '@/utils';
import { getCurrencyLocaleString } from '@/utils/date.utils';
import styles from './Accounts.module.css';

const accountTypeNames = [...BudgetAccountNames, ...TrackingAccountNames, ...LoanAccountNames];
const budgetAccountTypes = new Set<string>(BudgetAccountNames.map((type) => type.value));

function typeName(type: Account['type']) {
  return accountTypeNames.find((item) => item.value === type)?.name ?? type;
}

function formatDate(date: string) {
  const parsed = new Date(date);
  return Number.isNaN(parsed.getTime())
    ? date
    : parsed.toLocaleDateString('en-IN', { day: 'numeric', month: 'short', year: 'numeric' });
}

export default function Accounts() {
  const dispatch = useAppDispatch();
  const selectedBudget = useAppSelector(selectSelectedBudget);
  const { allAccounts, loading, error } = useAppSelector((state) => state.accounts);
  const [searchParams, setSearchParams] = useSearchParams();
  const [expandedGroups, setExpandedGroups] = useState<Record<string, boolean>>({});
  const requestedAccountId = searchParams.get('account');
  const accounts = allAccounts.filter((account) => !account.deleted && account.budgetId === selectedBudget?.id);
  const otherGroups = Array.from(new Set([
    ...accountTypeNames.map((type) => type.value),
    ...accounts.map((account) => account.type),
  ]))
    .filter((type) => !budgetAccountTypes.has(type))
    .map((type) => ({ type: String(type), name: typeName(type), accounts: accounts.filter((account) => account.type === type && !account.closed) }));
  const accountGroups = [
    { type: 'budget', name: 'Budget', accounts: accounts.filter((account) => budgetAccountTypes.has(account.type) && !account.closed) },
    ...otherGroups,
  ]
    .filter((group) => group.accounts.length > 0);
  const hiddenAccounts = accounts.filter((account) => account.closed);
  if (hiddenAccounts.length > 0) {
    accountGroups.push({ type: 'hidden', name: 'Hidden accounts', accounts: hiddenAccounts });
  }
  const selectedAccount = accounts.find((account) => account.id === requestedAccountId) ?? accountGroups[0]?.accounts[0] ?? null;

  useEffect(() => {
    if (selectedBudget?.id) dispatch(fetchAllAccounts());
  }, [dispatch, selectedBudget?.id]);

  return (
    <div className={styles.section}>
      <div className={styles.heading}>
        <h1>Accounts</h1>
        <p>View balances and transactions for the accounts in this budget.</p>
      </div>

      {loading === LoadingState.ERROR && accounts.length === 0 ? (
        <div className={styles.message} role="alert">
          <p>{error ?? 'Could not load accounts.'}</p>
          <button type="button" onClick={() => dispatch(fetchAllAccounts())}>Retry</button>
        </div>
      ) : accounts.length === 0 ? (
        <div className={styles.message}>{loading === LoadingState.PENDING || loading === LoadingState.IDLE ? 'Loading accounts…' : 'No accounts in this budget yet.'}</div>
      ) : (
        <div className={styles.layout}>
          <nav className={styles.accountList} aria-label="Accounts">
            {accountGroups.map((group) => {
              const expanded = expandedGroups[group.type] ?? group.type !== 'hidden';
              const balance = group.accounts.reduce((sum, account) => sum + (account.balance ?? 0), 0);
              return (
                <section key={group.type} className={styles.accountGroup} aria-labelledby={`account-group-${group.type}`}>
                  <h2 id={`account-group-${group.type}`} className={styles.groupHeading}>
                    <button
                      type="button"
                      className={styles.groupToggle}
                      aria-expanded={expanded}
                      aria-controls={`account-list-${group.type}`}
                      onClick={() => setExpandedGroups((previous) => ({ ...previous, [group.type]: !expanded }))}>
                      <span className={styles.groupLabel}><span aria-hidden="true">{expanded ? '▾' : '▸'}</span> {group.name} <small>{group.accounts.length}</small></span>
                      <span className={`${styles.groupBalance} ${balance < 0 ? styles.negativeBalance : styles.positiveBalance}`} title="Combined account balance">{getCurrencyLocaleString(balance)}</span>
                    </button>
                  </h2>
                  <div id={`account-list-${group.type}`} className={styles.groupAccounts} hidden={!expanded}>
                    {group.accounts.map((account) => (
                      <button
                        key={account.id}
                        type="button"
                        title={account.name}
                        className={`${styles.accountButton} ${selectedAccount?.id === account.id ? styles.selected : ''}`}
                        aria-current={selectedAccount?.id === account.id ? 'true' : undefined}
                        onClick={() => account.id && setSearchParams({ account: account.id })}>
                        <span className={styles.accountIdentity}>
                          <strong>{account.name}</strong>
                          <small>{typeName(account.type)}{account.closed ? ' · Hidden' : ''}</small>
                        </span>
                        <span className={`${styles.accountBalance} ${(account.balance ?? 0) < 0 ? styles.negativeBalance : styles.positiveBalance}`}>{getCurrencyLocaleString(account.balance ?? 0)}</span>
                      </button>
                    ))}
                  </div>
                </section>
              );
            })}
          </nav>

          {selectedAccount?.id && (
            <div className={styles.detail}>
              <div className={styles.meta}>
                <span>Type <strong>{typeName(selectedAccount.type)}</strong></span>
                <span>Status <strong>{selectedAccount.closed ? 'Closed' : 'Open'}</strong></span>
                {selectedAccount.createdAt && <span>Added <strong>{formatDate(selectedAccount.createdAt)}</strong></span>}
              </div>
              <div className={styles.transactions}>
                <Transaction key={`${selectedBudget?.id}:${selectedAccount.id}`} accountId={selectedAccount.id} embedded />
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
