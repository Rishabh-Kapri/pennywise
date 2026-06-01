import type { BudgetTemplateGroupInput } from './types';

export const budgetTemplates: BudgetTemplateGroupInput[] = [
  {
    name: 'Monthly Bills',
    categories: [{ name: 'Rent' }, { name: 'Electricity' }, { name: 'Phone Bill' }, { name: 'Subscriptions' }]
  },
  {
    name: 'Everyday Spending',
    categories: [{ name: 'Groceries' }, { name: 'Dining Out' }, { name: 'Travel - ST' }, { name: 'Shopping' }]
  },
  {
    name: 'Savings',
    categories: [{ name: 'Emergency Fund' }, { name: 'Vacation' }, { name: 'Gifts' }]
  }
];
