import { useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { CaretRight, PencilSimple, Plus } from '@phosphor-icons/react';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { fetchAllAccounts } from '@/features/accounts/store/accountSlice';
import {
  BudgetAccountNames,
  LoanAccountNames,
  TrackingAccountNames,
  type Account,
} from '@/features/accounts/types/account.types';
import { selectSelectedBudget } from '@/features/budget';
import { formatBudgetAmount } from '@/features/category/components/budgetFormat';
import { Transaction } from '@/features/transactions/components/Transaction/Transaction';
import Statements from '@/features/statements/components/Statements';
import { LoadingState } from '@/utils';
import styles from './Accounts.module.css';
import { BankLogo } from './BankLogo/BankLogo';
import { AccountFormModal } from './AccountFormModal';

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
  const [showNewAccount, setShowNewAccount] = useState(false);
  const [editingAccount, setEditingAccount] = useState<Account | null>(null);
  const [expandedGroups, setExpandedGroups] = useState<Record<string, boolean>>({});
  const requestedAccountId = searchParams.get('account');
  const showStatements = searchParams.get('view') === 'statements';
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

  const totals = useMemo(
    () =>
      accounts.reduce(
        (sum, account) => {
          const balance = account.balance ?? 0;
          return {
            assets: sum.assets + Math.max(balance, 0),
            debts: sum.debts + Math.min(balance, 0),
            netWorth: sum.netWorth + balance,
          };
        },
        { assets: 0, debts: 0, netWorth: 0 },
      ),
    [accounts],
  );

  useEffect(() => {
    if (selectedBudget?.id) dispatch(fetchAllAccounts());
  }, [dispatch, selectedBudget?.id]);

  // On phones the list is a horizontal strip; keep the selected card on screen.
  const accountListRef = useRef<HTMLElement>(null);
  const selectedAccountId = selectedAccount?.id;
  useEffect(() => {
    accountListRef.current
      ?.querySelector('[aria-current="true"]')
      ?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
  }, [selectedAccountId]);

  const selectedBalance = selectedAccount?.balance ?? 0;

  return (
    <section className={styles.page}>
      <header className={styles.top}>
        <h1>Accounts</h1>
        <div className={styles.topActions}>
          {accounts.length > 0 && (
            <dl className={styles.stats}>
              <div>
                <dt>Assets</dt>
                <dd>{formatBudgetAmount(totals.assets)}</dd>
              </div>
              <div>
                <dt>Debts</dt>
                <dd className={totals.debts < 0 ? styles.negative : undefined}>{formatBudgetAmount(totals.debts)}</dd>
              </div>
              <div>
                <dt>Net worth</dt>
                <dd className={totals.netWorth < 0 ? styles.negative : undefined}>{formatBudgetAmount(totals.netWorth)}</dd>
              </div>
            </dl>
          )}
          <button type="button" className={styles.addAccount} onClick={() => setShowNewAccount(true)}>
            <Plus size={14} weight="bold" />
            New account
          </button>
        </div>
      </header>

      {loading === LoadingState.ERROR && accounts.length === 0 ? (
        <div className={styles.message} role="alert">
          <p>{error ?? 'Could not load accounts.'}</p>
          <button type="button" onClick={() => dispatch(fetchAllAccounts())}>Retry</button>
        </div>
      ) : accounts.length === 0 ? (
        <div className={styles.message}>{loading === LoadingState.PENDING || loading === LoadingState.IDLE ? 'Loading accounts…' : 'No accounts in this budget yet. Use New account to add one.'}</div>
      ) : (
        <div className={styles.layout}>
          <nav ref={accountListRef} className={styles.accountList} aria-label="Accounts">
            {accountGroups.map((group) => {
              const expanded = expandedGroups[group.type] ?? group.type !== 'hidden';
              const balance = group.accounts.reduce((sum, account) => sum + (account.balance ?? 0), 0);
              return (
                <section
                  key={group.type}
                  className={`${styles.group} ${group.type === 'hidden' ? styles.hiddenGroup : ''}`}
                  aria-labelledby={`account-group-${group.type}`}>
                  <h2 id={`account-group-${group.type}`} className={styles.groupHeading}>
                    <button
                      type="button"
                      className={styles.groupToggle}
                      aria-expanded={expanded}
                      aria-controls={`account-list-${group.type}`}
                      onClick={() => setExpandedGroups((previous) => ({ ...previous, [group.type]: !expanded }))}>
                      <CaretRight size={13} weight="bold" className={`${styles.caret} ${expanded ? styles.caretOpen : ''}`} />
                      <span className={styles.groupLabel}>{group.name}</span>
                      <span className={styles.groupCount}>{group.accounts.length}</span>
                      <span
                        className={`${styles.groupBalance} ${balance < 0 ? styles.negative : ''}`}
                        title="Combined account balance">
                        {formatBudgetAmount(balance)}
                      </span>
                    </button>
                  </h2>
                  <div id={`account-list-${group.type}`} className={styles.groupAccounts} hidden={!expanded}>
                    {group.accounts.map((account) => {
                      const isSelected = selectedAccount?.id === account.id;
                      return (
                        <button
                          key={account.id}
                          type="button"
                          title={account.name}
                          className={`${styles.accountRow} ${isSelected ? styles.selected : ''}`}
                          aria-current={isSelected ? 'true' : undefined}
                          onClick={() => account.id && setSearchParams({ account: account.id })}>
                          <BankLogo name={account.name} bank={account.bank} type={account.type} />
                          <span className={styles.accountIdentity}>
                            <span className={styles.accountName}>{account.name}</span>
                            <small>{typeName(account.type)}{account.suffix ? ` · ••${account.suffix}` : ''}{account.closed ? ' · Hidden' : ''}</small>
                          </span>
                          <span className={`${styles.accountBalance} ${(account.balance ?? 0) < 0 ? styles.negative : ''}`}>
                            {formatBudgetAmount(account.balance ?? 0)}
                          </span>
                        </button>
                      );
                    })}
                  </div>
                </section>
              );
            })}
          </nav>

          {selectedAccount?.id && (
            <div className={styles.detail}>
              <header className={styles.accountHeader}>
                <div className={styles.headerIdentity}>
                  <BankLogo name={selectedAccount.name} bank={selectedAccount.bank} type={selectedAccount.type} size="lg" />
                  <div className={styles.headerText}>
                    <span className={styles.eyebrow}>
                      {typeName(selectedAccount.type)}
                      {selectedAccount.suffix && <> · ••{selectedAccount.suffix}</>}
                      {selectedAccount.closed && <span className={styles.closedBadge}>Closed</span>}
                      {selectedAccount.createdAt && <> · Added {formatDate(selectedAccount.createdAt)}</>}
                    </span>
                    <h2 className={styles.accountTitle}>{selectedAccount.name}</h2>
                  </div>
                  <button
                    type="button"
                    className={styles.editAccount}
                    onClick={() => setEditingAccount(selectedAccount)}
                    aria-label={`Edit ${selectedAccount.name}`}
                    title="Edit account">
                    <PencilSimple size={16} weight="bold" />
                  </button>
                </div>
                <div className={styles.headerBalance}>
                  <span className={styles.eyebrow}>Balance</span>
                  <strong className={`${styles.heroAmount} ${selectedBalance < 0 ? styles.negative : ''}`}>
                    {formatBudgetAmount(selectedBalance)}
                  </strong>
                </div>
              </header>

              {selectedAccount.type === 'creditCard' && (
                <nav className={styles.detailTabs} aria-label="Credit card account views">
                  {['transactions', 'statements'].map((view) => (
                    <button
                      key={view}
                      type="button"
                      aria-current={(view === 'statements' ? showStatements : !showStatements) ? 'page' : undefined}
                      onClick={() => setSearchParams({ account: selectedAccount.id!, ...(view === 'statements' ? { view } : {}) })}>
                      {view === 'transactions' ? 'Transactions' : 'Statements'}
                    </button>
                  ))}
                </nav>
              )}

              <div className={styles.transactions}>
                {selectedAccount.type === 'creditCard' && showStatements
                  ? <Statements key={`${selectedBudget?.id}:${selectedAccount.id}`} accountName={selectedAccount.name} />
                  : <Transaction key={`${selectedBudget?.id}:${selectedAccount.id}`} accountId={selectedAccount.id} embedded />}
              </div>
            </div>
          )}
        </div>
      )}

      {showNewAccount && (
        <AccountFormModal
          isOpen
          onClose={() => setShowNewAccount(false)}
          onSaved={(account) => account.id && setSearchParams({ account: account.id })}
        />
      )}
      {editingAccount && (
        <AccountFormModal key={editingAccount.id} isOpen account={editingAccount} onClose={() => setEditingAccount(null)} />
      )}
    </section>
  );
}
