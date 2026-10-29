import { useEffect, useMemo, useState } from 'react';
import { GitMerge, Warning, X } from '@phosphor-icons/react';
import { Modal } from '@/components/common/Modal';
import { SearchSelect } from '@/features/transactions/components/popovers/SearchSelect';
import type { Payee, PayeeStats } from '../types/payee.types';
import { formatLastUsed } from '../payeeUtils';
import { PayeeAvatar } from './PayeeAvatar';
import styles from './Payees.module.css';

interface MergePayeesModalProps {
  isOpen: boolean;
  /** payees to start with; one id opens it as "merge this into…" */
  initialIds: string[];
  suggestedTargetId?: string;
  payees: Payee[];
  statsById: Record<string, PayeeStats>;
  saving: boolean;
  onClose: () => void;
  onConfirm: (targetId: string, sourceIds: string[], name: string) => void;
}

export function MergePayeesModal({
  isOpen,
  initialIds,
  suggestedTargetId,
  payees,
  statsById,
  saving,
  onClose,
  onConfirm,
}: MergePayeesModalProps) {
  const [ids, setIds] = useState<string[]>(initialIds);
  const [targetId, setTargetId] = useState('');
  const [name, setName] = useState('');

  const byId = useMemo(() => new Map(payees.map((payee) => [payee.id!, payee])), [payees]);
  const usage = (id: string) => statsById[id]?.transactionCount ?? 0;

  useEffect(() => {
    if (!isOpen) return;
    const valid = initialIds.filter((id) => byId.has(id));
    // keep the most used payee unless told otherwise; ties go to the shorter name
    const target =
      suggestedTargetId && valid.includes(suggestedTargetId)
        ? suggestedTargetId
        : ([...valid].sort(
            (a, b) => usage(b) - usage(a) || (byId.get(a)?.name.length ?? 0) - (byId.get(b)?.name.length ?? 0),
          )[0] ?? '');
    setIds(valid);
    setTargetId(target);
    setName(byId.get(target)?.name ?? '');
    // reset only when the dialog opens
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOpen]);

  const chosen = ids.map((id) => byId.get(id)).filter((payee): payee is Payee => Boolean(payee));
  const sources = chosen.filter((payee) => payee.id !== targetId);
  const target = byId.get(targetId);

  const moved = sources.reduce(
    (sum, payee) => ({
      transactions: sum.transactions + usage(payee.id!),
      rules: sum.rules + (statsById[payee.id!]?.ruleCount ?? 0),
    }),
    { transactions: 0, rules: 0 },
  );

  const addable = useMemo(
    () =>
      payees
        .filter((payee) => payee.id && !payee.transferAccountId && !ids.includes(payee.id))
        .map((payee) => ({ id: payee.id!, name: payee.name }))
        .sort((a, b) => a.name.localeCompare(b.name)),
    [payees, ids],
  );

  const pickTarget = (id: string) => {
    // follow the new target's name unless the user typed their own
    if (!name.trim() || name === target?.name) setName(byId.get(id)?.name ?? '');
    setTargetId(id);
  };

  const remove = (id: string) => {
    const next = ids.filter((other) => other !== id);
    setIds(next);
    if (id === targetId && next[0]) pickTarget(next[0]);
  };

  const nameOptions = [...new Set(chosen.map((payee) => payee.name))];
  const canMerge = Boolean(target && sources.length > 0 && name.trim() && !saving);

  return (
    <Modal isOpen={isOpen} onClose={onClose} className={`${styles.dialog} ${styles.mergeModal}`}>
      <header className={styles.modalHeader}>
        <span className={styles.modalIcon}>
          <GitMerge size={20} />
        </span>
        <div>
          <h2>Merge payees</h2>
          <p>Pick the payee to keep. Everything from the others moves onto it, and they are deleted.</p>
        </div>
        <button type="button" className={styles.iconButton} aria-label="Close" onClick={onClose}>
          <X size={16} />
        </button>
      </header>

      <div className={styles.mergeList} role="radiogroup" aria-label="Payee to keep">
        {chosen.map((payee) => {
          const id = payee.id!;
          const stats = statsById[id];
          const isTarget = id === targetId;
          const lastUsed = formatLastUsed(stats?.lastUsed);
          return (
            <label key={id} className={`${styles.mergeItem} ${isTarget ? styles.mergeItemTarget : ''}`}>
              <input type="radio" name="merge-target" checked={isTarget} onChange={() => pickTarget(id)} />
              <PayeeAvatar name={payee.name} />
              <span className={styles.mergeItemText}>
                <strong>{payee.name}</strong>
                <span>
                  {usage(id)} {usage(id) === 1 ? 'transaction' : 'transactions'} · {stats?.ruleCount ?? 0}{' '}
                  {(stats?.ruleCount ?? 0) === 1 ? 'rule' : 'rules'}
                  {lastUsed ? ` · ${lastUsed}` : ''}
                </span>
              </span>
              <span className={isTarget ? styles.keepBadge : styles.mergeBadge}>{isTarget ? 'Keep' : 'Merge in'}</span>
              {chosen.length > 1 && (
                <button
                  type="button"
                  className={styles.iconButton}
                  aria-label={`Leave ${payee.name} out`}
                  onClick={(event) => {
                    event.preventDefault();
                    remove(id);
                  }}>
                  <X size={14} />
                </button>
              )}
            </label>
          );
        })}
      </div>

      <div className={styles.field}>
        <span>Add another payee</span>
        <SearchSelect
          key={ids.length}
          value=""
          items={addable}
          variant="form"
          placeholder="Search payees"
          emptyText="No other payees"
          onSelect={(item) => {
            setIds((prev) => [...prev, item.id]);
            if (!targetId) pickTarget(item.id);
          }}
        />
      </div>

      <div className={styles.field}>
        <span>Name after merge</span>
        <input className={styles.input} value={name} onChange={(event) => setName(event.target.value)} />
        {nameOptions.length > 1 && (
          <div className={styles.nameOptions}>
            {nameOptions.map((option) => (
              <button
                key={option}
                type="button"
                className={`${styles.chip} ${option === name ? styles.chipActive : ''}`}
                onClick={() => setName(option)}>
                {option}
              </button>
            ))}
          </div>
        )}
      </div>

      {target && sources.length > 0 ? (
        <div className={styles.mergeSummary}>
          <p>
            <strong>{moved.transactions}</strong> {moved.transactions === 1 ? 'transaction' : 'transactions'} and{' '}
            <strong>{moved.rules}</strong> {moved.rules === 1 ? 'rule' : 'rules'} move to <strong>{name.trim() || target.name}</strong>,
            along with recurring schedules and what the AI has learned about{' '}
            {sources.length === 1 ? 'that payee' : 'those payees'}.
          </p>
          <p className={styles.hintWarn}>
            <Warning size={14} />
            <span>
              {sources.map((payee) => payee.name).join(', ')} will be deleted. This can’t be undone.
            </span>
          </p>
        </div>
      ) : (
        <p className={styles.mergeHint}>Add at least one more payee to merge.</p>
      )}

      <footer className={styles.modalFooter}>
        <button type="button" className={styles.ghostButton} onClick={onClose}>
          Cancel
        </button>
        <button
          type="button"
          className={styles.primaryButton}
          disabled={!canMerge}
          onClick={() => onConfirm(targetId, sources.map((payee) => payee.id!), name.trim())}>
          {saving ? 'Merging…' : chosen.length > 1 ? `Merge ${chosen.length} payees` : 'Merge'}
        </button>
      </footer>
    </Modal>
  );
}
