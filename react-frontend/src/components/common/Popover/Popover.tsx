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
  placement?: 'top' | 'bottom' | 'left' | 'right';
  alignment?: 'start' | 'center';
  onMouseEnter?: () => void;
  onMouseLeave?: () => void;
}

export function Popover({
  id,
  isOpen,
  triggerRef,
  children,
  width,
  zIndex,
  onClose,
  placement = 'bottom',
  alignment = 'start',
  onMouseEnter,
  onMouseLeave,
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
  useEffect(() => {
    if (!isOpen || !onClose) return;

    const handleOutsideClick = (e: MouseEvent) => {
      const target = e.target as Node;
      const isInsidePopover = popoverRef.current?.contains(target);
      const isInsideTrigger = triggerRef.current?.contains(target);

      if (!isInsidePopover && !isInsideTrigger) {
        onClose();
      }
    };

    document.addEventListener('mousedown', handleOutsideClick);
    return () => document.removeEventListener('mousedown', handleOutsideClick);
  }, [isOpen, onClose, triggerRef]);

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
        const popoverWidth = popover.offsetWidth;
        const viewportHeight = window.innerHeight;
        const viewportWidth = window.innerWidth;
        const gap = 8;

        let top = 0;
        let left = 0;

        switch (placement) {
          case 'right':
            left = triggerRect.right + gap;
            top = triggerRect.top;
            // Check overflow right (basic)
            if (left + popoverWidth > viewportWidth) {
              left = triggerRect.left - popoverWidth - gap; // Flip to left
            }
            break;
          case 'left':
            left = triggerRect.left - popoverWidth - gap;
            top = triggerRect.top;
            break;
          case 'top':
            top = triggerRect.top - popoverHeight - gap;
            left = triggerRect.left;
            break;
          case 'bottom':
          default:
            // Check space below
            const spaceBelow = viewportHeight - triggerRect.bottom - gap;
            const spaceAbove = triggerRect.top - gap;
            if (spaceBelow >= popoverHeight || spaceBelow >= spaceAbove) {
               top = triggerRect.bottom + gap;
            } else {
               top = triggerRect.top - popoverHeight - gap;
            }
            if (alignment === 'center') {
              left = triggerRect.left + (triggerRect.width / 2) - (popoverWidth / 2);
            } else {
              left = triggerRect.left;
            }
            break;
        }
        
        // Basic vertical overflow adjustment for side placements
        if ((placement === 'left' || placement === 'right') && top + popoverHeight > viewportHeight) {
            top = Math.max(gap, viewportHeight - popoverHeight - gap);
        }

        popover.style.transform = `translate(${left}px, ${top}px)`;

        if (event === 'resize') {
          if (width) {
            return;
          }
           // Only match width for top/bottom unless forced
          if (placement === 'top' || placement === 'bottom') {
              setElWidth(triggerRef.current.offsetWidth);
          }
        }
      }
    },
    [isOpen, triggerRef, width, placement, alignment],
  );

  const updateWidth = useCallback(() => {
    // if width is provided, don't update position
    if (width) {
      return;
    }
    if (triggerRef.current && (placement === 'top' || placement === 'bottom')) {
      setElWidth(triggerRef.current.offsetWidth);
    }
  }, [triggerRef, width, placement]);

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
      onMouseEnter={onMouseEnter}
      onMouseLeave={onMouseLeave}
      style={{
        position: 'fixed',
        top: 0,
        left: 0,
        zIndex: zIndex ?? 1000,
        width: typeof width === 'number' ? `${width}px` : undefined
      }}>
      <div className={styles.container} style={{ width: width ? `${width}px` : (elWidth ? `${elWidth}px` : 'auto') }}>
        {children}
      </div>
    </div>,
    document.body,
  );
}
