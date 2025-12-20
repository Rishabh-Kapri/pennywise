import type { Category } from '@/features/category/types/category.types';

interface BudgetActivityProps {
  selectedCategory: Category;
  month: string;
}

// I need to show following information:
// case 1. when category is not selected
// - show budget information like
//   - left over from last month
//   - assigned in current month
//   - activity
//   - overspent amount (hovering over will show overspent categories)
// case 2. when category is selected
// - category information section
//   - cash left from last month
//   - budgeted this month
//   - credit card amount for this category
//   - edit category
//   - Available 
// - goals information section
// - notes section
// - auto assign section (in the future)
// export function BudgetActivity({
// }) {
//   return (
//   )
// };
