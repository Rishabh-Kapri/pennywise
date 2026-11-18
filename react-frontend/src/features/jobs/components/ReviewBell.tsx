import { useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Bell, ChatCenteredText, EnvelopeSimple, Code } from '@phosphor-icons/react';
import { Popover } from '@/components/common/Popover/Popover';
import { formatCurrency } from '@/utils/format';
import { formatAgo } from '../jobTime';
import { useReviewInbox } from '../hooks/useReviewInbox';
import type { InboundMessage } from '../types/job.types';
import styles from './ReviewBell.module.css';

const SOURCE_ICON = { email: EnvelopeSimple, sms: ChatCenteredText, api: Code } as const;

/** "Swiggy · ₹420", else the message's first line. */
function title(message: InboundMessage): string {
  const e = message.review?.entities;
  if (e?.merchant && e.amount) return `${e.merchant} · ${formatCurrency(e.amount)}`;
  if (e?.merchant) return e.merchant;
  const firstLine = message.review?.text?.split('\n').find((line) => line.trim() !== '');
  return firstLine?.trim() || 'A bank message';
}

/**
 * The navbar's notifications: bank messages the pipeline parked for a person
 * (`needs_review`). Each opens its row in Settings → Jobs, where it can be
 * continued, edited and created, or dismissed.
 */
export function ReviewBell() {
  const navigate = useNavigate();
  const { items, count } = useReviewInbox();
  const [open, setOpen] = useState(false);
  const trigger = useRef<HTMLButtonElement>(null);

  const go = (to: string) => {
    setOpen(false);
    navigate(to);
  };

  const label = count === 0 ? 'Notifications' : `Notifications: ${count} to review`;

  return (
    <>
      <button
        ref={trigger}
        type="button"
        className={styles.bell}
        aria-label={label}
        title={label}
        aria-haspopup="dialog"
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
      >
        <Bell size={18} weight={open ? 'fill' : 'regular'} />
        {count > 0 && <span className={styles.badge}>{count > 99 ? '99+' : count}</span>}
      </button>
      <Popover
        id="review-bell"
        isOpen={open}
        triggerRef={trigger}
        onClose={() => setOpen(false)}
        width={340}
        alignment="end"
        className={styles.panel}
      >
        <div className={styles.header}>
          <strong>To review</strong>
          {count > 0 && <span className={styles.count}>{count}</span>}
        </div>
        {items.length === 0 ? (
          <p className={styles.empty}>Nothing waiting on you. Messages the pipeline isn't sure about show up here.</p>
        ) : (
          <ul className={styles.list}>
            {items.map((message) => {
              const Icon = SOURCE_ICON[message.source as keyof typeof SOURCE_ICON] ?? EnvelopeSimple;
              return (
                <li key={message.id}>
                  <button
                    type="button"
                    className={styles.item}
                    onClick={() => go(`/settings?section=jobs&job=${message.reportJobId}`)}
                  >
                    <Icon size={16} className={styles.itemIcon} aria-hidden="true" />
                    <span className={styles.itemText}>
                      <span className={styles.itemTitle}>{title(message)}</span>
                      {message.reason && <span className={styles.itemReason}>{message.reason}</span>}
                    </span>
                    <span className={styles.itemTime}>{formatAgo(message.reportedAt)}</span>
                  </button>
                </li>
              );
            })}
          </ul>
        )}
        <button type="button" className={styles.footer} onClick={() => go('/settings?section=jobs')}>
          Open Jobs
        </button>
      </Popover>
    </>
  );
}

export default ReviewBell;
