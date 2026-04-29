import type { BudgetTemplateGroupInput } from '../store';

export const STARTER_BUDGET_TEMPLATE_GROUPS: BudgetTemplateGroupInput[] = [
  {
    name: 'Monthly Bills',
    categories: [
      { name: 'Rent / Mortgage' },
      { name: 'Electricity' },
      { name: 'Water' },
      { name: 'Internet' },
      { name: 'Phone' },
    ],
  },
  {
    name: 'Everyday Spending',
    categories: [
      { name: 'Groceries' },
      { name: 'Dining Out' },
      { name: 'Fuel' },
      { name: 'Shopping' },
      { name: 'Personal Care' },
    ],
  },
  {
    name: 'Savings Goals',
    categories: [
      { name: 'Emergency Fund' },
      { name: 'Vacation' },
      { name: 'Investments' },
      { name: 'Big Purchases' },
    ],
  },
  {
    name: 'Debt Payments',
    categories: [
      { name: 'Credit Card' },
      { name: 'Student Loan' },
      { name: 'Auto Loan' },
      { name: 'Personal Loan' },
    ],
  },
  {
    name: 'Quality of Life',
    categories: [
      { name: 'Entertainment' },
      { name: 'Fitness' },
      { name: 'Subscriptions' },
      { name: 'Gifts' },
      { name: 'Travel' },
    ],
  },
];

export const DEFAULT_SELECTED_TEMPLATE_GROUPS = STARTER_BUDGET_TEMPLATE_GROUPS.map(
  (group) => group.name,
);
