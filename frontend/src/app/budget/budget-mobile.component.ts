import { ChangeDetectionStrategy, Component, OnInit } from '@angular/core';
import { Store } from '@ngxs/store';
import { Observable, map } from 'rxjs';
import { Category, InflowCategory } from 'src/app/models/category.model';
import { CategoryGroupData } from 'src/app/models/state.model';
import { NormalizedTransaction, Transaction } from 'src/app/models/transaction.model';
import { Account } from 'src/app/models/account.model';
import { Payee } from 'src/app/models/payee.model';
import { BudgetsState } from 'src/app/store/dashboard/states/budget/budget.state';
import { CategoryGroupsState } from 'src/app/store/dashboard/states/categoryGroups/categoryGroups.state';
import { CategoriesState } from 'src/app/store/dashboard/states/categories/categories.state';
import { TransactionsState } from 'src/app/store/dashboard/states/transactions/transactions.state';
import { AccountsState } from 'src/app/store/dashboard/states/accounts/accounts.state';
import { PayeesState } from 'src/app/store/dashboard/states/payees/payees.state';
import { HelperService } from 'src/app/services/helper.service';
import { DatabaseService } from 'src/app/services/database.service';
import { CategoryGroupsActions } from 'src/app/store/dashboard/states/categoryGroups/categoryGroups.action';

@Component({
  selector: 'app-budget-mobile',
  templateUrl: './budget-mobile.component.html',
  styleUrls: ['./budget-mobile.component.scss'],
  changeDetection: ChangeDetectionStrategy.OnPush,
  standalone: false,
})
export class BudgetMobileComponent implements OnInit {
  categoryGroupsData$ = this.ngxsStore.select(CategoryGroupsState.getCategoryGroupData);
  selectedMonth$ = this.ngxsStore.select(BudgetsState.getSelectedMonth);
  selectedHumanMonth$ = this.ngxsStore.select(BudgetsState.getSelectedHumanMonth);
  inflowCategory$ = this.ngxsStore.select(CategoriesState.getInflowWithBalance);

  // Data for modal
  accountObj$: Observable<Record<string, Account>>;
  payeeObj$: Observable<Record<string, Payee>>;
  categoryGroupData$ = this.ngxsStore.select(CategoryGroupsState.getCategoryGroupData);

  isDetailsModalOpen = false;
  isMoveModalOpen = false;
  showCategorySelector = false;
  selectedCategory: Category | InflowCategory | null = null;
  categoryTransactions: NormalizedTransaction[] = [];

  // Move money data
  moveData = {
    from: { categoryId: '', groupId: '', amount: 0 },
    to: { categoryId: '', groupId: '', name: 'Select Category' },
  };

  constructor(
    private ngxsStore: Store,
    private helperService: HelperService,
    private dbService: DatabaseService,
  ) { }

  ngOnInit(): void {
    this.accountObj$ = this.ngxsStore.select(AccountsState.getAllAccounts).pipe(
      map((accounts) => {
        return accounts.reduce((obj: Record<string, Account>, acc: Account) => {
          const data = Object.assign(obj, { [acc.id!]: acc });
          return data;
        }, {});
      }),
    );

    this.payeeObj$ = this.ngxsStore.select(PayeesState.getAllPayees).pipe(
      map((payees) => {
        return payees.reduce((obj: Record<string, Payee>, payee: Payee) => {
          const data = Object.assign(obj, { [payee.id!]: payee });
          return data;
        }, {});
      }),
    );
  }

  showCategoryDetails(category: Category | InflowCategory) {
    this.selectedCategory = category;
    this.isDetailsModalOpen = true;
    this.loadCategoryTransactions(category);
  }

  closeDetailsModal() {
    this.isDetailsModalOpen = false;
    this.selectedCategory = null;
    this.categoryTransactions = [];
  }

  loadCategoryTransactions(category: Category | InflowCategory) {
    const selectedMonth = this.ngxsStore.selectSnapshot(BudgetsState.getSelectedMonth);
    const allTransactions = this.ngxsStore.selectSnapshot(TransactionsState.getNormalizedTransaction);
    const ccAccounts = this.ngxsStore.selectSnapshot(AccountsState.getCreditCardAccounts);

    let categoryTransactions: NormalizedTransaction[] = [];

    if ('balance' in category) {
      // Regular Category
      if (this.helperService.isCategoryCreditCard(category)) {
        categoryTransactions = this.helperService.getTransactionsForAccount(allTransactions, [
          ...ccAccounts.map((acc) => acc.id!),
        ]) as NormalizedTransaction[];
      } else {
        categoryTransactions = this.helperService.getTransactionsForCategory(allTransactions, [category.id!]);
      }
    }

    this.categoryTransactions = this.helperService.filterTransactionsBasedOnMonth(categoryTransactions, selectedMonth);
  }

  getTransactionCount(category: Category): number {
    const selectedMonth = this.ngxsStore.selectSnapshot(BudgetsState.getSelectedMonth);
    const allTransactions = this.ngxsStore.selectSnapshot(TransactionsState.getNormalizedTransaction);
    const ccAccounts = this.ngxsStore.selectSnapshot(AccountsState.getCreditCardAccounts);
    if (category.id === '404f1661-caee-496a-9f53-a7981f74397d') {
      console.log(category);
      console.log(selectedMonth, allTransactions);
    }

    let categoryTransactions: NormalizedTransaction[] = [];

    if (this.helperService.isCategoryCreditCard(category)) {
      categoryTransactions = this.helperService.getTransactionsForAccount(allTransactions, [
        ...ccAccounts.map((acc) => acc.id!),
      ]) as NormalizedTransaction[];
    } else {
      if (category.id === '404f1661-caee-496a-9f53-a7981f74397d')
        console.log('fetching transactions for category');
      categoryTransactions = this.helperService.getTransactionsForCategory(allTransactions, [category.id!]);
      if (category.id === '404f1661-caee-496a-9f53-a7981f74397d')
        console.log(categoryTransactions);
    }

    const filteredTransactions = this.helperService.filterTransactionsBasedOnMonth(categoryTransactions, selectedMonth);
    if (category.id === '404f1661-caee-496a-9f53-a7981f74397d')
      console.log(filteredTransactions);

    return filteredTransactions.length;
  }

  getProgressPercentage(category: Category): number {
    // For now, return 100% (fully filled green bar)
    // This will be used for goals in the future
    return 100;
  }

  getAvailableAmountColor(category: Category | InflowCategory): string {
    const selectedMonth = this.ngxsStore.selectSnapshot(BudgetsState.getSelectedMonth);

    // Handle InflowCategory differently since it doesn't have balance
    if ('balance' in category) {
      const balance = category.balance?.[selectedMonth] ?? 0;
      if (balance > 0) return 'text-green-400';
      if (balance < 0) return 'text-red-400';
      return 'text-neutral-400';
    } else {
      // For InflowCategory, use budgeted amount
      const budgeted = category.budgeted as number;
      if (budgeted > 0) return 'text-green-400';
      if (budgeted < 0) return 'text-red-400';
      return 'text-neutral-400';
    }
  }

  getCategoryBalance(category: Category | InflowCategory, selectedMonth: string | null): number {
    if (!selectedMonth) return 0;

    if ('balance' in category) {
      return category.balance?.[selectedMonth] ?? 0;
    } else {
      // For InflowCategory, return budgeted amount
      return category.budgeted as number;
    }
  }

  getCategoryBudgeted(category: Category | InflowCategory, selectedMonth: string | null): number {
    if (!selectedMonth) return 0;

    if ('budgeted' in category && typeof category.budgeted === 'object') {
      return (category.budgeted as Record<string, number>)[selectedMonth] ?? 0;
    } else {
      // For InflowCategory, return budgeted amount directly
      return category.budgeted as number;
    }
  }

  getTransactionAmount(transaction: Transaction): { amount: number; isInflow: boolean } {
    if (transaction.amount < 0) {
      return { amount: transaction.amount, isInflow: false };
    } else {
      return { amount: transaction.amount, isInflow: true };
    }
  }

  // Move money functionality
  showMoveModal(category: Category) {
    this.selectedCategory = category;
    this.moveData = {
      from: {
        categoryId: category.id!,
        groupId: category.categoryGroupId,
        amount: category.balance?.[this.ngxsStore.selectSnapshot(BudgetsState.getSelectedMonth)] ?? 0,
      },
      to: { categoryId: '', groupId: '', name: 'Select Category' },
    };
    this.isMoveModalOpen = true;
  }

  closeMoveModal() {
    this.isMoveModalOpen = false;
    this.selectedCategory = null;
    this.showCategorySelector = false;
    this.moveData = {
      from: { categoryId: '', groupId: '', amount: 0 },
      to: { categoryId: '', groupId: '', name: 'Select Category' },
    };
  }

  selectMoveCategory(category: Category) {
    this.moveData.to.name = category.name;
    this.moveData.to.categoryId = category.id!;
    this.moveData.to.groupId = category.categoryGroupId;
  }

  changeMoveAmount(value: number) {
    if (value === null || value < 0) {
      this.moveData.from.amount = 0;
    } else {
      const maxAmount =
        this.selectedCategory && 'balance' in this.selectedCategory
          ? (this.selectedCategory.balance?.[this.ngxsStore.selectSnapshot(BudgetsState.getSelectedMonth)] ?? 0)
          : 0;

      if (value > maxAmount) {
        this.moveData.from.amount = maxAmount;
      } else {
        this.moveData.from.amount = value;
      }
    }
  }

  async moveBalance() {
    if (this.moveData?.from?.amount === null || !this.moveData?.to?.categoryId) {
      return;
    }

    const budgetKey = this.ngxsStore.selectSnapshot(BudgetsState.getSelectedMonth);
    const moveTo = this.ngxsStore.selectSnapshot(
      CategoryGroupsState.getCategory(this.moveData.to.categoryId, this.moveData.to.groupId),
    );
    const moveFrom = this.ngxsStore.selectSnapshot(
      CategoryGroupsState.getCategory(this.moveData.from.categoryId, this.moveData.from.groupId),
    );

    if (moveTo && moveFrom) {
      // Update moveTo category
      moveTo.budgeted[budgetKey] += this.moveData.from.amount;
      if (moveTo.balance) {
        moveTo.balance[budgetKey] += this.moveData.from.amount;
      } else {
        moveTo.balance = { [budgetKey]: this.moveData.from.amount };
      }
      await this.dbService.editCategory(moveTo);

      // Update moveFrom category
      moveFrom.budgeted[budgetKey] -= this.moveData.from.amount;
      if (moveFrom.balance) {
        moveFrom.balance[budgetKey] -= this.moveData.from.amount;
      }
      await this.dbService.editCategory(moveFrom);

      this.closeMoveModal();
    }
  }

  // TrackBy functions for performance
  trackByGroup(index: number, group: CategoryGroupData): string {
    return group.id || index.toString();
  }

  trackByCategory(index: number, category: Category): string {
    return category.id || index.toString();
  }

  trackByTransaction(index: number, transaction: NormalizedTransaction): string {
    return transaction.id || index.toString();
  }

  // Additional methods for enhanced functionality
  getTotalBudgetedForGroup(categoryGroup: CategoryGroupData, selectedMonth: string | null): number {
    if (!selectedMonth) return 0;
    return categoryGroup.categories.reduce((total, category) => {
      return total + this.getCategoryBudgeted(category, selectedMonth);
    }, 0);
  }

  getCategoryActivity(category: Category | InflowCategory, selectedMonth: string | null): number {
    if (!selectedMonth) return 0;
    
    // For now, return 0 as activity calculation needs to be implemented
    // This would typically calculate the sum of all transactions in this category for the month
    return 0;
  }

  getProgressBarClass(category: Category | InflowCategory, selectedMonth: string | null): string {
    const balance = this.getCategoryBalance(category, selectedMonth);
    const budgeted = this.getCategoryBudgeted(category, selectedMonth);

    if (budgeted === 0) return 'bg-zinc-500';
    if (balance < 0) return 'bg-red-500';
    if (balance >= budgeted) return 'bg-emerald-500';
    return 'bg-yellow-500';
  }

  getActivityColor(category: Category | InflowCategory, selectedMonth: string | null): string {
    const activity = this.getCategoryActivity(category, selectedMonth);
    if (activity < 0) return 'text-red-400';
    if (activity > 0) return 'text-emerald-400';
    return 'text-zinc-400';
  }

  getCategoryGroupName(category: Category | InflowCategory | null): string {
    if (!category) return '';
    
    // Find the category group name by looking through all category groups
    const categoryGroups = this.ngxsStore.selectSnapshot(CategoryGroupsState.getCategoryGroupData);
    for (const group of categoryGroups) {
      const foundCategory = group.categories.find(cat => cat.id === category.id);
      if (foundCategory) {
        return group.name;
      }
    }
    
    return 'Unknown Group';
  }

  openAddTransactionModal() {
    // TODO: Implement add transaction modal
    console.log('Add transaction modal not implemented yet');
  }
}
