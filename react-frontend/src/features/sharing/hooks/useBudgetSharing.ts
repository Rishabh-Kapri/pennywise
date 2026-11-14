import { useCallback, useEffect, useState } from 'react';
import { errorMessage, toast } from '@/utils';
import {
  changeMemberRole,
  createInvite,
  listBudgetInvites,
  listMembers,
  removeMember,
  revokeInvite,
} from '../api';
import type { BudgetInvite, BudgetMember, InvitableRole } from '../types';

/**
 * The selected budget's members, and for the owner its open invites and the
 * actions on both. Reloads when budgetId changes.
 */
export function useBudgetSharing(budgetId: string | undefined, owner: boolean) {
  const [members, setMembers] = useState<BudgetMember[] | null>(null);
  const [invites, setInvites] = useState<BudgetInvite[]>([]);
  const [loadError, setLoadError] = useState<string | null>(null);

  const reload = useCallback(async () => {
    if (!budgetId) return;
    try {
      const [loadedMembers, loadedInvites] = await Promise.all([
        listMembers(),
        owner ? listBudgetInvites() : Promise.resolve([]),
      ]);
      setMembers(loadedMembers);
      setInvites(loadedInvites);
      setLoadError(null);
    } catch (err) {
      setLoadError(errorMessage(err, "Couldn't load who's in this budget."));
    }
  }, [budgetId, owner]);

  useEffect(() => {
    setMembers(null);
    void reload();
  }, [reload]);

  /** Runs an action, toasts a failure, and reloads either way. Resolves true on success. */
  const run = async (action: () => Promise<unknown>, failure: string): Promise<boolean> => {
    try {
      await action();
      return true;
    } catch (err) {
      toast.error(failure, { description: errorMessage(err, 'Try again.') });
      return false;
    } finally {
      void reload();
    }
  };

  return {
    members,
    invites,
    loadError,
    invite: (email: string, role: InvitableRole) => run(() => createInvite(email, role), "Couldn't send the invite"),
    revoke: (id: string) => run(() => revokeInvite(id), "Couldn't cancel the invite"),
    changeRole: (userId: string, role: InvitableRole) =>
      run(() => changeMemberRole(userId, role), "Couldn't change their access"),
    remove: (userId: string) => run(() => removeMember(userId), "Couldn't remove them"),
  };
}
