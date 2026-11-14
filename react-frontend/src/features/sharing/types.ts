/** What a member may do in a budget. The API enforces it; the UI only hides what it would refuse. */
export type BudgetRole = 'owner' | 'editor' | 'viewer';

/** A role you can give someone. There is one owner: whoever created the budget. */
export type InvitableRole = Exclude<BudgetRole, 'owner'>;

export interface BudgetMember {
  userId: string;
  role: BudgetRole;
  name: string;
  email: string;
  picture?: string;
  createdAt: string;
}

export interface BudgetInvite {
  id: string;
  budgetId: string;
  email: string;
  role: InvitableRole;
  invitedBy: string;
  createdAt: string;
  expiresAt: string;
  /** Set on invites to you, so you can tell what you're joining. */
  budgetName?: string;
  inviterName?: string;
}

export const ROLE_LABELS: Record<BudgetRole, string> = {
  owner: 'Owner',
  editor: 'Can edit',
  viewer: 'View only',
};

export const ROLE_HINTS: Record<InvitableRole, string> = {
  editor: 'Add and change transactions, assign money, manage accounts.',
  viewer: 'See everything, change nothing.',
};

/** Read-only budgets can't be changed by this user (the API answers 403). */
export function canWrite(role: BudgetRole | undefined): boolean {
  // Budgets loaded before roles existed have no role: they're the user's own.
  return role === undefined || role === 'owner' || role === 'editor';
}

export function isOwner(role: BudgetRole | undefined): boolean {
  return role === undefined || role === 'owner';
}
