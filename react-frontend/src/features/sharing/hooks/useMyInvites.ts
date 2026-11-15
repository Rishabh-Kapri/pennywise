import { useCallback, useEffect, useState } from 'react';
import { useAppDispatch } from '@/app/hooks';
import { openBudget } from '@/features/budget/store';
import { errorMessage, toast } from '@/utils';
import { acceptInvite, declineInvite, listMyInvites } from '../api';
import type { BudgetInvite } from '../types';

/** Open invites to the signed-in user, with accept (which opens the budget) and decline. */
export function useMyInvites() {
  const dispatch = useAppDispatch();
  const [invites, setInvites] = useState<BudgetInvite[] | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);

  const reload = useCallback(async () => {
    try {
      setInvites(await listMyInvites());
    } catch {
      // Invites are a side panel; a failed load shouldn't block the screen.
      setInvites([]);
    }
  }, []);

  useEffect(() => {
    void reload();
  }, [reload]);

  const accept = async (invite: BudgetInvite) => {
    setBusyId(invite.id);
    try {
      const { budgetId } = await acceptInvite(invite.id);
      setInvites((current) => current?.filter((i) => i.id !== invite.id) ?? null);
      await dispatch(openBudget(budgetId));
      toast.success('Joined the budget', { description: invite.budgetName });
    } catch (err) {
      toast.error("Couldn't join the budget", { description: errorMessage(err, 'Try again.') });
      void reload();
    } finally {
      setBusyId(null);
    }
  };

  const decline = async (invite: BudgetInvite) => {
    setBusyId(invite.id);
    try {
      await declineInvite(invite.id);
      setInvites((current) => current?.filter((i) => i.id !== invite.id) ?? null);
    } catch (err) {
      toast.error("Couldn't decline the invite", { description: errorMessage(err, 'Try again.') });
      void reload();
    } finally {
      setBusyId(null);
    }
  };

  return { invites, busyId, accept, decline, reload };
}
