import { useMemo, useState, type FormEvent } from 'react';
import { ArrowLeft, ArrowRight, Info } from '@phosphor-icons/react';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { createAccount, fetchAllAccounts } from '@/features/accounts/store/accountSlice';
import { BANK_BRANDS, bankLogoUrl, getBankBrand } from '@/features/accounts/bankLogos';
import { BudgetAccountNames, type BudgetAccountType } from '@/features/accounts/types/account.types';
import { fetchInflowAmount } from '@/features/category/store/categorySlice';
import { createTransaction } from '@/features/transactions/store/transactionSlice';
import { SearchSelect } from '@/features/transactions/components/popovers/SearchSelect';
import { formatCurrency } from '@/features/dashboard/utils';
import { errorMessage, toast } from '@/utils';
import type { StepProps } from '../OnboardingWizard';
import styles from './steps.module.css';

const NO_BANK = 'none';

function today(): string {
  const now = new Date();
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`;
}

/**
 * Accounts with their last digits (how bank emails are matched to them) and
 * today's balance, booked as a Starting Balance transaction so it lands in
 * Ready to Assign. The one required step.
 */
export function AccountsStep({ onNext, onBack, busy }: StepProps) {
  const dispatch = useAppDispatch();
  const budget = useAppSelector((state) => state.budgets.selectedBudget);
  const accounts = useAppSelector((state) => state.accounts.budgetAccounts);
  const readyToAssign = useAppSelector((state) => state.categories.inflowAmount);

  const [type, setType] = useState<BudgetAccountType>('checking');
  const [bankId, setBankId] = useState('');
  const [name, setName] = useState('');
  const [nameEdited, setNameEdited] = useState(false);
  const [suffix, setSuffix] = useState('');
  const [balance, setBalance] = useState('');
  const [saving, setSaving] = useState(false);

  const bank = getBankBrand(null, bankId);
  const typeLabel = BudgetAccountNames.find((option) => option.value === type)?.name ?? '';
  const shownName = nameEdited ? name : bank ? `${bank.short} ${typeLabel}` : '';
  const isCard = type === 'creditCard';
  const amount = balance.trim() === '' ? 0 : Number(balance);
  const suffixValid = /^\d{2,6}$/.test(suffix);
  const amountValid = Number.isFinite(amount) && amount >= 0;
  const canAdd = shownName.trim() !== '' && suffixValid && amountValid && !saving;

  const bankItems = useMemo(
    () => [
      ...BANK_BRANDS.map((brand) => ({
        id: brand.id,
        name: brand.name,
        leading: <img className={styles.bankLogo} src={bankLogoUrl(brand)} alt="" draggable={false} />,
      })),
      { id: NO_BANK, name: 'No bank / other' },
    ],
    [],
  );

  const reset = () => {
    setBankId('');
    setName('');
    setNameEdited(false);
    setSuffix('');
    setBalance('');
  };

  const addAccount = async (event: FormEvent) => {
    event.preventDefault();
    if (!canAdd || !budget?.id) return;
    setSaving(true);
    try {
      const account = await dispatch(
        createAccount({ name: shownName.trim(), type, bank: bank?.id ?? '', suffix }),
      ).unwrap();
      const payeeId = budget.metadata?.startingBalPayeeId;
      if (amount > 0 && account.id && payeeId) {
        // Cash counts toward Ready to Assign; card debt has no category,
        // matching how the demo budget books an opening card balance.
        await dispatch(
          createTransaction({
            budgetId: budget.id,
            accountId: account.id,
            payeeId,
            categoryId: isCard ? null : (budget.metadata?.inflowCategoryId ?? null),
            date: today(),
            amount: isCard ? -amount : amount,
          }),
        ).unwrap();
        dispatch(fetchAllAccounts());
        dispatch(fetchInflowAmount());
      }
      toast.success(`${account.name} added`);
      reset();
    } catch (err) {
      toast.error('Could not add the account', { description: errorMessage(err, 'Try again.') });
    } finally {
      setSaving(false);
    }
  };

  return (
    <>
      <p className={styles.eyebrow}>Step 1 · Required</p>
      <h1 className={styles.title}>Add your accounts</h1>
      <p className={styles.lead}>
        Add each bank account and credit card you spend from. Loans and investments can wait; add them later under
        Settings → Loans.
      </p>

      {accounts.length > 0 && (
        <div className={styles.section}>
          <ul className={styles.list}>
            {accounts.map((account) => {
              const brand = getBankBrand(account.name, account.bank);
              return (
                <li key={account.id} className={styles.listItem}>
                  {brand && <img className={styles.bankLogo} src={bankLogoUrl(brand)} alt="" />}
                  <div className={styles.grow}>
                    {account.name}
                    <small>
                      {BudgetAccountNames.find((option) => option.value === account.type)?.name}
                      {account.suffix ? ` · ending ${account.suffix}` : ''}
                    </small>
                  </div>
                  <span className={`${styles.amount} ${(account.balance ?? 0) < 0 ? styles.negative : ''}`}>
                    {(account.balance ?? 0) < 0 ? '−' : ''}
                    {formatCurrency(account.balance ?? 0)}
                  </span>
                </li>
              );
            })}
          </ul>
          <div className={styles.summary}>
            <span>Ready to assign</span>
            <strong>{formatCurrency(readyToAssign)}</strong>
          </div>
        </div>
      )}

      <form className={`${styles.section} ${styles.panel} ${styles.form}`} onSubmit={addAccount}>
        <div className={styles.segmented} role="radiogroup" aria-label="Account type">
          {BudgetAccountNames.map((option) => (
            <button
              key={option.value}
              type="button"
              role="radio"
              aria-checked={type === option.value}
              className={`${styles.segment} ${type === option.value ? styles.segmentSelected : ''}`}
              onClick={() => setType(option.value)}
            >
              {option.name}
            </button>
          ))}
        </div>

        <div className={styles.field}>
          <span className={styles.label}>Bank</span>
          <SearchSelect
            value={bank?.name ?? (bankId === NO_BANK ? 'No bank / other' : '')}
            items={bankItems}
            onSelect={(item) => setBankId(item.id)}
            placeholder="Select bank"
            emptyText="No banks found"
            variant="form"
          />
        </div>

        <div className={styles.row}>
          <label className={styles.field}>
            <span className={styles.label}>Name</span>
            <input
              className={styles.input}
              value={shownName}
              placeholder="e.g. HDFC Savings"
              maxLength={80}
              onChange={(event) => {
                setName(event.target.value);
                setNameEdited(true);
              }}
            />
          </label>
          <label className={styles.field}>
            <span className={styles.label}>Last digits</span>
            <input
              className={`${styles.input} ${suffix && !suffixValid ? styles.inputInvalid : ''}`}
              value={suffix}
              placeholder="1234"
              inputMode="numeric"
              maxLength={6}
              onChange={(event) => setSuffix(event.target.value.replace(/\D/g, ''))}
            />
          </label>
          <label className={styles.field}>
            <span className={styles.label}>{isCard ? 'Amount owed today' : 'Balance today'}</span>
            <input
              className={`${styles.input} ${amountValid ? '' : styles.inputInvalid}`}
              value={balance}
              placeholder="0"
              inputMode="decimal"
              onChange={(event) => setBalance(event.target.value.replace(/[^\d.]/g, ''))}
            />
          </label>
        </div>

        <p className={styles.hint}>
          <Info size={14} weight="bold" aria-hidden="true" /> Bank alerts name the account by its last digits ("A/c
          XX1234", "card ending 1234"). Without them an email can't be matched to this account, so it is skipped.
        </p>

        <div>
          <button type="submit" className={styles.secondary} disabled={!canAdd}>
            {saving ? 'Adding…' : 'Add account'}
          </button>
        </div>
      </form>

      <div className={styles.actions}>
        <button type="button" className={styles.ghost} onClick={onBack} disabled={busy}>
          <ArrowLeft size={16} /> Back
        </button>
        <span className={styles.spacer} />
        <button
          type="button"
          className={styles.primary}
          onClick={() => onNext()}
          disabled={busy || accounts.length === 0}
          title={accounts.length === 0 ? 'Add at least one account to continue' : undefined}
        >
          Continue <ArrowRight size={16} />
        </button>
      </div>
    </>
  );
}
