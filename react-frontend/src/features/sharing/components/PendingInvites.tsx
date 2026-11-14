import { EnvelopeOpen } from '@phosphor-icons/react';
import { useMyInvites } from '../hooks/useMyInvites';
import { ROLE_LABELS } from '../types';
import styles from './Sharing.module.css';

interface PendingInvitesProps {
  /** Shown above the list; leave out for no heading. */
  title?: string;
  className?: string;
}

/**
 * Invites to the signed-in user, with Join and Decline. Renders nothing when
 * there are none, so it can sit anywhere.
 */
export function PendingInvites({ title = 'Invitations to you', className }: PendingInvitesProps) {
  const { invites, busyId, accept, decline } = useMyInvites();
  if (!invites || invites.length === 0) return null;

  return (
    <section className={`${styles.page} ${className ?? ''}`} aria-label={title}>
      <div className={styles.group}>
        {title && <h3>{title}</h3>}
        <ul className={styles.list}>
          {invites.map((invite) => {
            const busy = busyId === invite.id;
            return (
              <li key={invite.id} className={styles.row}>
                <span className={styles.avatar} aria-hidden="true">
                  <EnvelopeOpen size={18} />
                </span>
                <span className={styles.rowText}>
                  <span className={styles.name}>{invite.budgetName || 'A budget'}</span>
                  <span className={styles.muted}>
                    {invite.inviterName ? `From ${invite.inviterName}` : 'You were invited'} ·{' '}
                    {ROLE_LABELS[invite.role]}
                  </span>
                </span>
                <span className={styles.actions}>
                  <button type="button" className={styles.action} disabled={busy} onClick={() => decline(invite)}>
                    Decline
                  </button>
                  <button
                    type="button"
                    className={`${styles.action} ${styles.emphasis}`}
                    disabled={busy}
                    onClick={() => accept(invite)}>
                    {busy ? 'Joining…' : 'Join'}
                  </button>
                </span>
              </li>
            );
          })}
        </ul>
      </div>
    </section>
  );
}
