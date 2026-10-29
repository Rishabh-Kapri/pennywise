import type React from 'react';
import { createPortal } from 'react-dom';
import styles from './Modal.module.css';
import { useEffect } from 'react';

interface ModalProps {
  isOpen: boolean;
  onClose: () => void;
  children: React.ReactNode;
  /** Extra class for the content box, e.g. a wider max-width. */
  className?: string;
}

export function Modal({ isOpen, onClose, children, className }: ModalProps) {
  useEffect(() => {
    const handleEscape = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') {
        return;
      }
      onClose();
    };
    if (isOpen) {
      document.addEventListener('keydown', handleEscape);
    }

    return () => {
      document.removeEventListener('keydown', handleEscape);
    };
  }, [isOpen, onClose]);

  if (!isOpen) {
    return null;
  }

  return createPortal(
    <div className={styles.modalOverlay} onClick={onClose}>
      <div
        className={`${styles.modalContent} ${className ?? ''}`}
        role="dialog"
        aria-modal="true"
        onClick={(e) => e.stopPropagation()}>
        <div className={styles.modalBody}>{children}</div>
      </div>
    </div>,
    document.body,
  );
}
