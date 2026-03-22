import { useState, useEffect, type FormEvent } from 'react';
import { X } from 'lucide-react';
import { useAppDispatch } from '@/app/hooks';
import { fetchAllAccounts } from '@/features/accounts/store/accountSlice';
import { createLoanMetadata, updateLoanMetadata } from '../store/loanSlice';
import { LoanAccountNames, type LoanAccountType } from '@/features/accounts/types/account.types';
import { apiClient, toast } from '@/utils';
import { fetchAllCategoryGroups } from '@/features/category/store/categorySlice';
import type { LoanMetadata } from '../types/loan.types';
import styles from './LoanAccountForm.module.css';

interface LoanAccountFormProps {
  isOpen: boolean;
  onClose: () => void;
  /** When provided, the form operates in edit mode for loan metadata */
  editData?: {
    accountId: string;
    accountName: string;
    metadata: LoanMetadata;
  };
}

export default function LoanAccountForm({ isOpen, onClose, editData }: LoanAccountFormProps) {
  const dispatch = useAppDispatch();
  const isEditMode = !!editData;

  const [name, setName] = useState('');
  const [loanType, setLoanType] = useState<LoanAccountType>(LoanAccountNames[0].value);
  const [balance, setBalance] = useState('');
  const [interestRate, setInterestRate] = useState('');
  const [monthlyPayment, setMonthlyPayment] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Pre-populate fields in edit mode
  useEffect(() => {
    if (editData) {
      setName(editData.accountName);
      setInterestRate(editData.metadata.interestRate.toString());
      setMonthlyPayment(editData.metadata.monthlyPayment.toString());
      setBalance(editData.metadata.originalBalance.toString());
    } else {
      setName('');
      setBalance('');
      setInterestRate('');
      setMonthlyPayment('');
    }
  }, [editData]);

  if (!isOpen) return null;

  const isValid =
    name.trim() !== '' &&
    parseFloat(balance) > 0 &&
    parseFloat(interestRate) >= 0 &&
    parseFloat(monthlyPayment) > 0;

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!isValid) return;

    setSubmitting(true);
    setError(null);

    try {
      if (isEditMode && editData) {
        // Edit mode — update loan metadata via API
        await dispatch(
          updateLoanMetadata({
            accountId: editData.accountId,
            updates: {
              interestRate: parseFloat(interestRate),
              originalBalance: Math.abs(parseFloat(balance)),
              monthlyPayment: parseFloat(monthlyPayment),
            },
          }),
        ).unwrap();
        toast.success('Loan details updated');
        onClose();
      } else {
        // Create mode — create account + category group + category + loan metadata
        const absBalance = Math.abs(parseFloat(balance));

        // 1. Create the account via existing API — balance is negative for loans
        const account = await apiClient.post<Record<string, unknown>>('accounts', {
          name: name.trim(),
          type: loanType,
          balance: -absBalance,
        });

        // 2. Create a paired budget category group for loan payments
        let categoryId: string | undefined;
        try {
          const group = await apiClient.post<Record<string, unknown>>('category-groups', {
            name: `${name.trim()}: Loan`,
          });

          // 3. Create a "Loan Payment" category in the group
          const category = await apiClient.post<Record<string, unknown>>('categories', {
            name: 'Loan Payment',
            categoryGroupId: group.id,
          });
          categoryId = category.id as string;
        } catch (catErr) {
          console.error('Failed to create loan category group:', catErr);
          // Non-fatal — loan account was still created
        }

        // 4. Store loan metadata via API
        await dispatch(
          createLoanMetadata({
            accountId: account.id as string,
            interestRate: parseFloat(interestRate),
            originalBalance: absBalance,
            monthlyPayment: parseFloat(monthlyPayment),
            loanStartDate: new Date().toISOString().split('T')[0],
            categoryId,
          }),
        ).unwrap();

        // 5. Refresh accounts and categories list
        dispatch(fetchAllAccounts());
        const month = new Date().toISOString().slice(0, 7); // YYYY-MM
        dispatch(fetchAllCategoryGroups(month));

        toast.success('Loan account created');

        // Reset form
        setName('');
        setBalance('');
        setInterestRate('');
        setMonthlyPayment('');
        onClose();
      }
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to save loan account';
      setError(msg);
      toast.error(msg);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className={styles.overlay} onClick={onClose}>
      <div className={styles.modal} onClick={(e) => e.stopPropagation()}>
        <div className={styles.header}>
          <h2>{isEditMode ? 'Edit Loan Details' : 'Add Loan Account'}</h2>
          <button className={styles.closeBtn} onClick={onClose}>
            <X size={20} />
          </button>
        </div>

        <form className={styles.form} onSubmit={handleSubmit}>
          {!isEditMode && (
            <>
              <div className={styles.field}>
                <label htmlFor="loan-name">Account Name</label>
                <input
                  id="loan-name"
                  type="text"
                  placeholder="e.g. Home Mortgage, Car Loan"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  autoFocus
                />
              </div>

              <div className={styles.field}>
                <label htmlFor="loan-type">Loan Type</label>
                <select
                  id="loan-type"
                  value={loanType}
                  onChange={(e) => setLoanType(e.target.value as LoanAccountType)}
                >
                  {LoanAccountNames.map((lt) => (
                    <option key={lt.value} value={lt.value}>
                      {lt.name}
                    </option>
                  ))}
                </select>
              </div>
            </>
          )}

          {isEditMode && (
            <div className={styles.field}>
              <label>Account</label>
              <input type="text" value={name} disabled />
            </div>
          )}

          <div className={styles.field}>
            <label htmlFor="loan-balance">
              {isEditMode ? 'Original Balance (₹)' : 'Current Balance (₹)'}
            </label>
            <input
              id="loan-balance"
              type="number"
              placeholder="0.00"
              min="0"
              step="0.01"
              value={balance}
              onChange={(e) => setBalance(e.target.value)}
            />
          </div>

          <div className={styles.row}>
            <div className={styles.field}>
              <label htmlFor="loan-rate">Interest Rate (%)</label>
              <input
                id="loan-rate"
                type="number"
                placeholder="6.0"
                min="0"
                step="0.01"
                value={interestRate}
                onChange={(e) => setInterestRate(e.target.value)}
              />
            </div>

            <div className={styles.field}>
              <label htmlFor="loan-payment">Monthly Payment (₹)</label>
              <input
                id="loan-payment"
                type="number"
                placeholder="0.00"
                min="0"
                step="0.01"
                value={monthlyPayment}
                onChange={(e) => setMonthlyPayment(e.target.value)}
              />
            </div>
          </div>

          {error && <div className={styles.error}>{error}</div>}

          <div className={styles.actions}>
            <button type="button" className={styles.cancelBtn} onClick={onClose}>
              Cancel
            </button>
            <button
              type="submit"
              className={styles.submitBtn}
              disabled={!isValid || submitting}
            >
              {submitting
                ? isEditMode
                  ? 'Saving...'
                  : 'Creating...'
                : isEditMode
                  ? 'Save Changes'
                  : 'Create Loan'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
