import type React from 'react';
import { useEffect, useRef } from 'react';
import styles from './Dropdown.module.css';
import { createPortal } from 'react-dom';

interface DropdownProps {
  isOpen: boolean;
  onClose: () => void;
  children: React.ReactNode;
  triggerRef?: React.RefObject<HTMLElement | null>;
  position?: 'bottom' | 'top' | 'left' | 'right';
  width?: number;
}

export function Dropdown({
  isOpen,
  onClose,
  children,
  triggerRef,
  position,
  width,
}: DropdownProps) {
  const dropdownRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      console.log('mouse click:', event, dropdownRef, event.target);
      if (
        dropdownRef.current &&
        !dropdownRef.current.contains(event.target as Node) &&
        triggerRef?.current &&
        !triggerRef.current.contains(event.target as Node)
      ) {
        onClose();
      }
    };

    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        onClose();
      }
    };

    if (isOpen) {
      document.addEventListener('mousedown', handleClickOutside);
      document.addEventListener('keydown', handleEscape);
      // document.addEventListener('mousemove', handleMouseLeave);
    }

    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
      document.removeEventListener('keydown', handleEscape);
      // document.removeEventListener('mousemove', handleMouseLeave);
    };
  }, [isOpen, onClose, triggerRef]);

  useEffect(() => {
    if (isOpen && triggerRef?.current && dropdownRef.current) {
      const triggerRect = triggerRef.current.getBoundingClientRect();
      const dropdown = dropdownRef.current;

      let top = 0;
      let left = 0;

      switch (position) {
        case 'left':
          top = triggerRect.bottom + 8;
          left = triggerRect.left;
          break;
        case 'right':
          top = triggerRect.bottom + 8;
          left = triggerRect.right;
          break;
        case 'top':
          top = triggerRect.top - dropdown.offsetWidth - 8;
          left = triggerRect.left;
          break;
        case 'bottom':
          top = triggerRect.top - dropdown.offsetHeight - 8;
          left = triggerRect.left;
          break;
        default:
          break;
      }
      dropdown.style.top = `${top}px`;
      dropdown.style.left = `${left}px`;
    }
  }, [isOpen, triggerRef, position, width]);

  return (
    <>
      {isOpen &&
        createPortal(
          <div className={styles.dropdownBackdrop}>
            <div
              ref={dropdownRef}
              className={styles.dropdown}
              style={{ width: `${width}px` }}>
              {children}
            </div>
          </div>,
          document.body,
        )}
    </>
  );
}
