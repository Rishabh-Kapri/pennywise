import { useState, type FormEvent } from 'react';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { selectIsDemoUser, selectUser } from '@/features/auth/store';
import { fetchAllBudgets, selectSelectedBudget } from '@/features/budget/store';
import { errorMessage, toast } from '@/utils';
import { leaveBudget } from '../api';
import { useBudgetSharing } from '../hooks/useBudgetSharing';
import {
  ROLE_HINTS,
  ROLE_LABELS,
  isOwner,
  type BudgetInvite,
  type BudgetMember,
  type InvitableRole,
} from '../types';
import { Avatar } from './Avatar';
import { PendingInvites } from './PendingInvites';
import styles from './Sharing.module.css';

const INVITABLE: InvitableRole[] = ['editor', 'viewer'];

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, { day: 'numeric', month: 'short' });
}

interface MemberRowProps {
  member: BudgetMember;
  isYou: boolean;
  /** The signed-in user owns the budget, so they can change this member. */
  manage: boolean;
  onChangeRole: (role: InvitableRole) => Promise<boolean>;
  onRemove: () => Promise<boolean>;
}

function MemberRow({ member, isYou, manage, onChangeRole, onRemove }: MemberRowProps) {
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const editable = manage && member.role !== 'owner';
  const displayName = member.name || member.email || 'Someone';

  const run = async (action: () => Promise<boolean>) => {
    setBusy(true);
    await action();
    setBusy(false);
  };

  return (
    <li className={styles.row}>
      <Avatar name={member.name} email={member.email} picture={member.picture} />
      <span className={styles.rowText}>
        <span className={styles.name}>
          {displayName}
          {isYou && <span className={styles.you}>(you)</span>}
        </span>
        {member.email && member.email !== displayName && <span className={styles.muted}>{member.email}</span>}
      </span>
      <span className={styles.actions}>
        {!editable && <span className={styles.roleLabel}>{ROLE_LABELS[member.role]}</span>}
        {editable && confirming && (
          <>
            <button type="button" className={styles.action} disabled={busy} onClick={() => setConfirming(false)}>
              Keep
            </button>
            <button
              type="button"
              className={`${styles.action} ${styles.danger}`}
              disabled={busy}
              onClick={() => run(onRemove)}>
              Remove {member.name ? member.name.split(' ')[0] : 'them'}
            </button>
          </>
        )}
        {editable && !confirming && (
          <>
            <select
              className={styles.select}
              aria-label={`Access for ${displayName}`}
              value={member.role}
              disabled={busy}
              onChange={(event) => run(() => onChangeRole(event.target.value as InvitableRole))}>
              {INVITABLE.map((role) => (
                <option key={role} value={role}>
                  {ROLE_LABELS[role]}
                </option>
              ))}
            </select>
            <button type="button" className={styles.action} disabled={busy} onClick={() => setConfirming(true)}>
              Remove
            </button>
          </>
        )}
      </span>
    </li>
  );
}

function InviteRow({ invite, onRevoke }: { invite: BudgetInvite; onRevoke: () => Promise<boolean> }) {
  const [busy, setBusy] = useState(false);
  return (
    <li className={styles.row}>
      <Avatar name="" email={invite.email} />
      <span className={styles.rowText}>
        <span className={styles.name}>{invite.email}</span>
        <span className={styles.muted}>
          {ROLE_LABELS[invite.role]} · waiting until {formatDate(invite.expiresAt)}
        </span>
      </span>
      <span className={styles.actions}>
        <button
          type="button"
          className={styles.action}
          disabled={busy}
          onClick={async () => {
            setBusy(true);
            await onRevoke();
            setBusy(false);
          }}>
          Cancel invite
        </button>
      </span>
    </li>
  );
}

function InviteForm({ onInvite }: { onInvite: (email: string, role: InvitableRole) => Promise<boolean> }) {
  const [email, setEmail] = useState('');
  const [role, setRole] = useState<InvitableRole>('editor');
  const [sending, setSending] = useState(false);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    const trimmed = email.trim();
    if (!trimmed) return;
    setSending(true);
    const ok = await onInvite(trimmed, role);
    setSending(false);
    if (ok) {
      setEmail('');
      toast.success('Invite sent', {
        description: `${trimmed} will see it after signing in to Pennywise with that Google account.`,
      });
    }
  };

  return (
    <div className={styles.group}>
      <h3>Invite someone</h3>
      <form className={styles.form} onSubmit={submit}>
        <input
          className={styles.input}
          type="email"
          required
          autoComplete="off"
          placeholder="Their Google email"
          aria-label="Email to invite"
          value={email}
          onChange={(event) => setEmail(event.target.value)}
        />
        <select
          className={styles.select}
          aria-label="Access"
          value={role}
          onChange={(event) => setRole(event.target.value as InvitableRole)}>
          {INVITABLE.map((r) => (
            <option key={r} value={r}>
              {ROLE_LABELS[r]}
            </option>
          ))}
        </select>
        <button type="submit" className={styles.primary} disabled={sending || !email.trim()}>
          {sending ? 'Sending…' : 'Send invite'}
        </button>
      </form>
      <p className={styles.muted}>{ROLE_HINTS[role]} No email is sent: they'll find the invite when they sign in.</p>
    </div>
  );
}

/** Settings → Sharing: who's in the selected budget, invites, and leaving. */
export function SharingSettings() {
  const dispatch = useAppDispatch();
  const budget = useAppSelector(selectSelectedBudget);
  const user = useAppSelector(selectUser);
  const isDemoUser = useAppSelector(selectIsDemoUser);
  const owner = isOwner(budget?.role);
  const { members, invites, loadError, invite, revoke, changeRole, remove } = useBudgetSharing(budget?.id, owner);
  const [confirmLeave, setConfirmLeave] = useState(false);
  const [leaving, setLeaving] = useState(false);

  const ownerMember = members?.find((m) => m.role === 'owner');

  const leave = async () => {
    if (!budget?.id) return;
    setLeaving(true);
    try {
      await leaveBudget(budget.id);
      toast.success('You left the budget', { description: budget.name });
      // The list no longer has it, so the slice opens another one.
      await dispatch(fetchAllBudgets());
    } catch (err) {
      toast.error("Couldn't leave the budget", { description: errorMessage(err, 'Try again.') });
      setLeaving(false);
      setConfirmLeave(false);
    }
  };

  return (
    <div className={styles.page}>
      <PendingInvites />

      <div className={styles.group}>
        <h3>People in {budget?.name ?? 'this budget'}</h3>
        {loadError && <p className={styles.notice}>{loadError}</p>}
        {!members && !loadError && <p className={styles.muted}>Loading…</p>}
        {members && (
          <ul className={styles.list}>
            {members.map((member) => (
              <MemberRow
                key={member.userId}
                member={member}
                isYou={member.userId === user?.id}
                manage={owner}
                onChangeRole={(role) => changeRole(member.userId, role)}
                onRemove={() => remove(member.userId)}
              />
            ))}
          </ul>
        )}
        <p className={styles.muted}>
          Everyone here sees the same accounts, transactions and budget. The AI that sorts transactions
          runs on {owner ? 'your' : "the owner's"} AI keys.
        </p>
      </div>

      {owner && invites.length > 0 && (
        <div className={styles.group}>
          <h3>Waiting to join</h3>
          <ul className={styles.list}>
            {invites.map((item) => (
              <InviteRow key={item.id} invite={item} onRevoke={() => revoke(item.id)} />
            ))}
          </ul>
        </div>
      )}

      {owner && !isDemoUser && <InviteForm onInvite={invite} />}
      {owner && isDemoUser && <p className={styles.muted}>Sharing isn't available in the demo.</p>}

      {!owner && (
        <div className={styles.group}>
          <p className={styles.muted}>
            Only {ownerMember?.name || 'the owner'} can invite people or change who can do what.
          </p>
          <div className={styles.form}>
            {confirmLeave ? (
              <>
                <button type="button" className={styles.action} disabled={leaving} onClick={() => setConfirmLeave(false)}>
                  Stay
                </button>
                <button
                  type="button"
                  className={`${styles.action} ${styles.danger}`}
                  disabled={leaving}
                  onClick={leave}>
                  {leaving ? 'Leaving…' : `Leave ${budget?.name ?? 'budget'}`}
                </button>
                <span className={styles.muted}>You'd need a new invite to come back.</span>
              </>
            ) : (
              <button type="button" className={`${styles.action} ${styles.danger}`} onClick={() => setConfirmLeave(true)}>
                Leave this budget
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
