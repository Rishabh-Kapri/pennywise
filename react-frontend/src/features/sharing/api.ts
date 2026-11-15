import { apiClient } from '@/utils';
import type { BudgetInvite, BudgetMember, InvitableRole } from './types';

// Budget-scoped: apiClient sends the selected budget's x-budget-id.
export const listMembers = () => apiClient.get<BudgetMember[]>('budget-members');
export const changeMemberRole = (userId: string, role: InvitableRole) =>
  apiClient.put<void, { role: InvitableRole }>(`budget-members/${userId}`, { role });
export const removeMember = (userId: string) => apiClient.delete<void>(`budget-members/${userId}`);

export const listBudgetInvites = () => apiClient.get<BudgetInvite[]>('budget-invites');
export const createInvite = (email: string, role: InvitableRole) =>
  apiClient.post<BudgetInvite, { email: string; role: InvitableRole }>('budget-invites', { email, role });
export const revokeInvite = (id: string) => apiClient.delete<void>(`budget-invites/${id}`);

// The signed-in user's side. Not budget-scoped: they aren't a member yet.
export const listMyInvites = () => apiClient.get<BudgetInvite[]>('invites');
export const acceptInvite = (id: string) =>
  apiClient.post<{ budgetId: string }, Record<string, never>>(`invites/${id}/accept`, {});
export const declineInvite = (id: string) =>
  apiClient.post<void, Record<string, never>>(`invites/${id}/decline`, {});
// apiClient leaves x-budget-id off endpoints containing "budgets".
export const leaveBudget = (budgetId: string) =>
  apiClient.post<void, Record<string, never>>(`budgets/${budgetId}/leave`, {});
