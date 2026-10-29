import { useMemo, useState, type FormEvent } from 'react';
import { CreditCardIcon } from '@phosphor-icons/react';
import { Modal } from '@/components/common/Modal/Modal';
import { useAppDispatch } from '@/app/hooks';
import { SearchSelect } from '@/features/transactions/components/popovers/SearchSelect';
import { toast } from '@/utils';
import { createAccount, updateAccount } from '../store/accountSlice';
import {
  BudgetAccountNames,
  LoanAccountNames,
  TrackingAccountNames,
  type Account,
} from '../types/account.types';
import { BANK_BRANDS, bankLogoUrl, getBankBrand } from '../bankLogos';
import styles from './AccountFormModal.module.css';

const NEW_TYPE_OPTIONS = [...BudgetAccountNames, ...TrackingAccountNames];
const ALL_TYPE_NAMES = [...NEW_TYPE_OPTIONS, ...LoanAccountNames];
const NO_BANK = 'none';

interface AccountFormModalProps {
  isOpen: boolean;
  onClose: () => void;
  /** Edit this account; leave out to open a new one. */
  account?: Account;
  /** Called with the saved account. */
  onSaved?: (account: Account) => void;
}

/** Open a new account, or (with `account`) edit one: name, bank, last digits, closed. */
export function AccountFormModal({ isOpen, onClose, account, onSaved }: AccountFormModalProps) {
  const dispatch = useAppDispatch();
  const isEdit = Boolean(account);
  const [type, setType] = useState<Account['type']>(account?.type ?? 'checking');
  const [bankId, setBankId] = useState(account?.bank || (account ? getBankBrand(account.name)?.id ?? '' : ''));
  const [suffix, setSuffix] = useState(account?.suffix ?? '');
  const [name, setName] = useState(account?.name ?? '');
  const [nameEdited, setNameEdited] = useState(isEdit);
  const [closed, setClosed] = useState(account?.closed ?? false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const bank = getBankBrand(null, bankId);
  const typeLabel = ALL_TYPE_NAMES.find((option) => option.value === type)?.name ?? '';
  // "HDFC Credit Card" until the user types their own name
  const suggestedName = bank ? `${bank.short} ${typeLabel}` : '';
  const shownName = nameEdited ? name : suggestedName;
  const suffixValid = suffix === '' || /^\d{2,6}$/.test(suffix);
  const canSubmit = shownName.trim() !== '' && suffixValid && !submitting;

  const bankItems = useMemo(
    () => [
      ...BANK_BRANDS.map((brand) => ({
        id: brand.id,
        name: brand.name,
        leading: <img className={styles.bankOptionLogo} src={bankLogoUrl(brand)} alt="" draggable={false} />,
      })),
      { id: NO_BANK, name: 'No bank / other' },
    ],
    [],
  );
  const bankValue = bank?.name ?? (bankId === NO_BANK ? 'No bank / other' : '');

  const reset = () => {
    setType(account?.type ?? 'checking');
    setBankId(account?.bank || (account ? getBankBrand(account.name)?.id ?? '' : ''));
    setSuffix(account?.suffix ?? '');
    setName(account?.name ?? '');
    setNameEdited(isEdit);
    setClosed(account?.closed ?? false);
    setError(null);
  };

  const close = () => {
    if (submitting) return;
    reset();
    onClose();
  };

  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault();
    if (!canSubmit) return;
    setSubmitting(true);
    setError(null);
    try {
      const fields = { name: shownName.trim(), type, bank: bank?.id ?? '', suffix };
      const saved = account?.id
        ? await dispatch(updateAccount({ id: account.id, edits: { ...fields, closed } })).unwrap()
        : await dispatch(createAccount(fields)).unwrap();
      toast.success(isEdit ? 'Account updated' : 'Account created');
      onClose();
      if (!isEdit) reset();
      onSaved?.(saved);
    } catch (err) {
      setError((err as { message?: string }).message ?? 'Could not save the account.');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal isOpen={isOpen} onClose={close}>
      <form className={styles.form} onSubmit={handleSubmit}>
        <div className={styles.header}>
          <h2>{isEdit ? 'Edit account' : 'New account'}</h2>
          <CreditCardIcon className={styles.headerIcon} size={32} weight="duotone" aria-hidden="true" />
        </div>

        <div className={styles.field}>
          <span className={styles.label} id="account-form-type">Type</span>
          {isEdit ? (
            <span className={styles.fixedType}>{typeLabel}</span>
          ) : (
            <div className={styles.types} role="radiogroup" aria-labelledby="account-form-type">
              {NEW_TYPE_OPTIONS.map((option) => (
                <button
                  key={option.value}
                  type="button"
                  role="radio"
                  aria-checked={type === option.value}
                  className={`${styles.typeOption} ${type === option.value ? styles.typeSelected : ''}`}
                  onClick={() => setType(option.value)}>
                  {option.name}
                </button>
              ))}
            </div>
          )}
        </div>

        <div className={styles.field}>
          <span className={styles.label}>Bank</span>
          <SearchSelect
            value={bankValue}
            items={bankItems}
            onSelect={(item) => setBankId(item.id)}
            placeholder="Select bank"
            emptyText="No banks found"
            variant="form"
          />
        </div>

        <div className={styles.row}>
          <div className={`${styles.field} ${styles.nameField}`}>
            <label className={styles.label} htmlFor="account-form-name">Name</label>
            <input
              id="account-form-name"
              className={styles.input}
              value={shownName}
              placeholder="e.g. HDFC Checking"
              maxLength={80}
              autoFocus
              onChange={(event) => {
                setName(event.target.value);
                setNameEdited(true);
              }}
            />
          </div>
          <div className={`${styles.field} ${styles.suffixField}`}>
            <label className={styles.label} htmlFor="account-form-suffix">Last digits</label>
            <input
              id="account-form-suffix"
              className={`${styles.input} ${suffixValid ? '' : styles.inputInvalid}`}
              value={suffix}
              placeholder="1234"
              inputMode="numeric"
              maxLength={6}
              aria-invalid={!suffixValid}
              onChange={(event) => setSuffix(event.target.value.replace(/\D/g, ''))}
            />
          </div>
        </div>

        {isEdit && (
          <label className={styles.checkbox}>
            <input type="checkbox" checked={closed} onChange={(event) => setClosed(event.target.checked)} />
            Closed — hide it from the account list and pickers
          </label>
        )}

        {error && <div className={styles.error} role="alert">{error}</div>}

        <div className={styles.actions}>
          <button type="button" className={styles.cancel} onClick={close}>Cancel</button>
          <button type="submit" className={styles.submit} disabled={!canSubmit}>
            {submitting ? 'Saving…' : isEdit ? 'Save changes' : 'Create account'}
          </button>
        </div>
      </form>
    </Modal>
  );
}
