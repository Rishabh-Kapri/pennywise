import type React from 'react';
import { useCallback, useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import styles from './Popover.module.css';

interface PopoverProps {
  id: string;
  isOpen: boolean;
  children: React.ReactNode;
  triggerRef: React.RefObject<HTMLElement | null>;
  width?: number;
  zIndex?: number;
  onClose?: () => void;
}

export function Popover({
  id,
  isOpen,
  triggerRef,
  children,
  width,
  zIndex,
  onClose,
}: PopoverProps) {
  const popoverRef = useRef<HTMLDivElement | null>(null); // Reference to the popover element
  const [elWidth, setElWidth] = useState(width ?? 0);
  const rafRef = useRef<number>(null);

  // Handle Escape key to close popover first (before parent handlers)
  useEffect(() => {
    if (!isOpen || !onClose) return;

    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.stopPropagation(); // Prevent parent escape handlers from firing
        onClose();
      }
    };

    document.addEventListener('keydown', handleEscape, true); // capture phase
    return () => document.removeEventListener('keydown', handleEscape, true);
  }, [isOpen, onClose]);

  // Handle outside click to close popover
  // useEffect(() => {
  //   if (!isOpen || !onClose) return;

  //   const handleOutsideClick = (e: MouseEvent) => {
  //     const target = e.target as Node;
  //     const isInsidePopover = popoverRef.current?.contains(target);
  //     const isInsideTrigger = triggerRef.current?.contains(target);

  //     if (!isInsidePopover && !isInsideTrigger) {
  //       onClose();
  //     }
  //   };

  //   document.addEventListener('mousedown', handleOutsideClick);
  //   return () => document.removeEventListener('mousedown', handleOutsideClick);
  // }, [isOpen, onClose, triggerRef]);

  /*
   * Handle rendering the popover relative to the triggerRef element
   * Flips to open above if not enough space below
   */
  const updatePosition = useCallback(
    (event: string = 'scroll') => {
      if (isOpen && triggerRef.current && popoverRef.current) {
        const triggerRect = triggerRef.current.getBoundingClientRect();
        const popover = popoverRef.current;
        const popoverHeight = popover.offsetHeight;
        const viewportHeight = window.innerHeight;
        const gap = 4;

        // Check if there's enough space below the trigger
        const spaceBelow = viewportHeight - triggerRect.bottom - gap;
        const spaceAbove = triggerRect.top - gap;

        let top: number;
        if (spaceBelow >= popoverHeight || spaceBelow >= spaceAbove) {
          // Open below (default)
          top = triggerRect.bottom + gap;
        } else {
          // Open above (flip)
          top = triggerRect.top - popoverHeight - gap;
        }

        const left = triggerRect.left;
        popover.style.transform = `translate(${left}px, ${top}px)`;

        if (event === 'resize') {
          if (width) {
            return;
          }
          setElWidth(triggerRef.current.offsetWidth);
        }
      }
    },
    [isOpen, triggerRef, width],
  );

  const updateWidth = useCallback(() => {
    // if width is provided, don't update position
    if (width) {
      return;
    }
    if (triggerRef.current) {
      setElWidth(triggerRef.current.offsetWidth);
    }
  }, [triggerRef, width]);

  useEffect(() => {
    updatePosition('scroll');

    const handleScroll = () => {
      // cancel previous animation frame
      if (rafRef.current) {
        cancelAnimationFrame(rafRef.current);
      }

      rafRef.current = requestAnimationFrame(() => updatePosition('scroll'));
    };

    window.addEventListener('scroll', handleScroll, true);
    window.addEventListener('resize', () => updatePosition('resize'));

    return () => {
      window.removeEventListener('scroll', handleScroll, true);
      window.removeEventListener('resize', () => updatePosition('resize'));
      if (rafRef.current) {
        cancelAnimationFrame(rafRef.current);
      }
    };
  }, [updatePosition]);

  useEffect(() => {
    updateWidth();

    window.removeEventListener('resize', updateWidth);

    return () => {
      window.removeEventListener('resize', updateWidth);
    };
  }, [updateWidth]);

  if (!isOpen) {
    return null;
  }

  return createPortal(
    <div
      ref={popoverRef}
      id={id}
      role="dialog"
      aria-modal="true"
      className={`${styles.overlay} ${isOpen ? styles.open : styles.closed}`}
      style={{
        position: 'fixed',
        top: 0,
        left: 0,
        zIndex: zIndex ?? 1000,
      }}>
      <div className={styles.container} style={{ width: `${elWidth}px` }}>
        {children}
      </div>
    </div>,
    document.body,
  );
}
