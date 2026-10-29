import { type ReactNode, useEffect, useMemo, useState } from 'react';
import { Plus, Trash, Warning, X } from '@phosphor-icons/react';
import { Modal } from '@/components/common/Modal';
import type { Payee } from '../types/payee.types';
import { normalizePayeeName } from '../payeeUtils';
import styles from './Payees.module.css';

interface ConfirmDeleteProps {
  isOpen: boolean;
  title: string;
  body: ReactNode;
  confirmLabel: string;
  saving: boolean;
  /** offered instead of deleting, e.g. merging a payee that still has transactions */
  alternative?: { label: string; onClick: () => void };
  onClose: () => void;
  onConfirm: () => void;
}

export function ConfirmDeleteDialog({
  isOpen,
  title,
  body,
  confirmLabel,
  saving,
  alternative,
  onClose,
  onConfirm,
}: ConfirmDeleteProps) {
  return (
    <Modal isOpen={isOpen} onClose={onClose} className={styles.dialog}>
      <header className={styles.modalHeader}>
        <span className={`${styles.modalIcon} ${styles.modalIconDanger}`}>
          <Trash size={20} />
        </span>
        <div>
          <h2>{title}</h2>
          <div className={styles.modalBodyText}>{body}</div>
        </div>
      </header>
      <footer className={styles.modalFooter}>
        <button type="button" className={styles.ghostButton} onClick={onClose}>
          Cancel
        </button>
        {alternative && (
          <button type="button" className={styles.secondaryButton} onClick={alternative.onClick}>
            {alternative.label}
          </button>
        )}
        <button type="button" className={styles.dangerButton} disabled={saving} autoFocus onClick={onConfirm}>
          {saving ? 'Deleting…' : confirmLabel}
        </button>
      </footer>
    </Modal>
  );
}

interface NewPayeeDialogProps {
  isOpen: boolean;
  payees: Payee[];
  saving: boolean;
  onClose: () => void;
  onCreate: (name: string) => void;
  onOpenExisting: (id: string) => void;
}

export function NewPayeeDialog({ isOpen, payees, saving, onClose, onCreate, onOpenExisting }: NewPayeeDialogProps) {
  const [name, setName] = useState('');

  useEffect(() => {
    if (isOpen) setName('');
  }, [isOpen]);

  // catch "Swiggy" when "SWIGGY*BLR" already exists, before it becomes a duplicate
  const lookalike = useMemo(() => {
    const key = normalizePayeeName(name);
    if (key.length < 3) return undefined;
    return payees.find((payee) => payee.id && !payee.transferAccountId && normalizePayeeName(payee.name) === key);
  }, [name, payees]);
  const exact = lookalike && lookalike.name.trim().toLowerCase() === name.trim().toLowerCase();

  return (
    <Modal isOpen={isOpen} onClose={onClose} className={styles.dialog}>
      <form
        className={styles.newPayeeForm}
        onSubmit={(event) => {
          event.preventDefault();
          if (name.trim() && !exact && !saving) onCreate(name.trim());
        }}>
        <header className={styles.modalHeader}>
          <span className={styles.modalIcon}>
            <Plus size={20} />
          </span>
          <div>
            <h2>New payee</h2>
            <p>Add rules afterwards so bank emails find it automatically.</p>
          </div>
          <button type="button" className={styles.iconButton} aria-label="Close" onClick={onClose}>
            <X size={16} />
          </button>
        </header>
        <label className={styles.field}>
          <span>Name</span>
          <input
            className={styles.input}
            value={name}
            autoFocus
            placeholder="e.g. Swiggy"
            onChange={(event) => setName(event.target.value)}
          />
        </label>
        {lookalike && (
          <p className={exact ? styles.hintError : styles.hintWarn}>
            <Warning size={14} />
            <span>
              {exact ? 'A payee with this name already exists: ' : 'Looks like an existing payee: '}
              <button type="button" className={styles.linkButton} onClick={() => onOpenExisting(lookalike.id!)}>
                {lookalike.name}
              </button>
            </span>
          </p>
        )}
        <footer className={styles.modalFooter}>
          <button type="button" className={styles.ghostButton} onClick={onClose}>
            Cancel
          </button>
          <button type="submit" className={styles.primaryButton} disabled={!name.trim() || exact || saving}>
            {saving ? 'Adding…' : 'Add payee'}
          </button>
        </footer>
      </form>
    </Modal>
  );
}
