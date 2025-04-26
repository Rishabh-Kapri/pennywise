import { AfterViewInit, ChangeDetectionStrategy, Component, EventEmitter, Input, Output } from '@angular/core';
import { Store } from '@ngxs/store';
import { Parser } from 'expr-eval';
import { Dropdown, DropdownOptions } from 'flowbite';
import { Observable, map, take } from 'rxjs';
import { Account, BudgetAccountType } from 'src/app/models/account.model';
import { Category, InflowCategory } from 'src/app/models/category.model';
import { Payee } from 'src/app/models/payee.model';
import { Transaction } from 'src/app/models/transaction.model';
import { HelperService } from 'src/app/services/helper.service';
import { StoreService } from 'src/app/services/store.service';
import { AccountsState } from 'src/app/store/dashboard/states/accounts/accounts.state';
import { BudgetsState } from 'src/app/store/dashboard/states/budget/budget.state';
import { CategoriesActions } from 'src/app/store/dashboard/states/categories/categories.action';
import { CategoriesState } from 'src/app/store/dashboard/states/categories/categories.state';
import { CategoryGroupsActions } from 'src/app/store/dashboard/states/categoryGroups/categoryGroups.action';
import { CategoryGroupsState } from 'src/app/store/dashboard/states/categoryGroups/categoryGroups.state';
import { PayeesState } from 'src/app/store/dashboard/states/payees/payees.state';
import { TransactionsState } from 'src/app/store/dashboard/states/transactions/transactions.state';
/**
 * Get category data
 */
@Component({
  selector: 'app-category-item',
  templateUrl: './category-item.component.html',
  styleUrls: ['./category-item.component.scss'],
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class CategoryItemComponent implements AfterViewInit {
  @Input() categories: Category[];
  @Input() budgetKey: string;
  @Output() editCategoryEvent = new EventEmitter<Category | InflowCategory>();
  @Output() deleteCategoryEvent = new EventEmitter<Category>();
  @Output() hideUnhideCategoryEvent = new EventEmitter<Category>();

  accountObj$: Observable<Record<string, Account>>;
  categoryObj$: Observable<Record<string, Category>>;
  payeeObj$: Observable<Record<string, Payee>>;
  categoryGroupData$ = this.ngxsStore.select(CategoryGroupsState.getCategoryGroupData);
  categoryActivity: Transaction[];

  menuDropdown: Dropdown;
  moveDropdown: Dropdown;
  categoryDropdown: Dropdown;
  parser = new Parser();
  moveData = {
    from: { categoryId: '', groupId: '', amount: 0 },
    to: { categoryId: '', groupId: '', name: '' },
  };
  defaultOptions: DropdownOptions = {
    placement: 'bottom',
    triggerType: 'click',
    offsetSkidding: 0,
    offsetDistance: 10,
    delay: 300,
    ignoreClickOutsideClass: false,
  };
  constructor(
    private ngxsStore: Store,
    public store: StoreService,
    private helperService: HelperService,
  ) {}

  ngOnInit(): void {
    this.accountObj$ = this.ngxsStore.select(AccountsState.getAllAccounts).pipe(
      map((accounts) => {
        return accounts.reduce((obj: Record<string, Account>, acc: Account) => {
          const data = Object.assign(obj, { [acc.id!]: acc });
          return data;
        }, {});
      }),
    );
    this.categoryObj$ = this.ngxsStore.select(CategoriesState.getAllCategories).pipe(
      map((categories) => {
        return categories.reduce((obj: Record<string, Category>, category: Category) => {
          const data = Object.assign(obj, { [category.id!]: category });
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

  ngAfterViewInit(): void {
    // add info for assignment to categories
    // 1. category 1 - assignments - '2023-11': 2000
    //                               '2023-10': 1000
  }

  showEditMenu(category: Category) {
    this.menuDropdown = this.helperService.getDropdownInstance(
      category.id!,
      'menuDropdown',
      'menuBtn',
      this.defaultOptions,
    );
    this.menuDropdown.toggle();
  }

  showBudgetInput(category: Category) {
    this.ngxsStore.dispatch(
      new CategoryGroupsActions.UpdateCategoryInGroup(category.categoryGroupId, category.id!, {
        showBudgetInput: true,
      }),
    );
  }

  /**
   * Budget to the category
   */
  hideBudgetInput(category: Category, event: any) {
    // @TODO
    // 1. improve the logic here to only call dispatch when event isn't zero
    // 2. handle catch error for ngxs
    // 3. handle editing of category in a more efficient way
    const categoryCopy = <Category>JSON.parse(JSON.stringify(category));
    const currentBudget = categoryCopy.budgeted[this.budgetKey];
    try {
      this.ngxsStore.dispatch(
        new CategoryGroupsActions.UpdateCategoryInGroup(category.categoryGroupId, category.id!, {
          showBudgetInput: false,
        }),
      );
      const expr = this.parser.parse(event.target.value);
      categoryCopy.budgeted[this.budgetKey] = expr.evaluate();
    } catch (err) {
      // do nothing
      categoryCopy.budgeted[this.budgetKey] = categoryCopy.budgeted[this.budgetKey];
    }
    if (categoryCopy.budgeted[this.budgetKey] !== currentBudget) {
      // check if inflow has the amount that is being budgeted
      const budgeted = categoryCopy.budgeted[this.budgetKey];
      const inflowCategory = this.ngxsStore.selectSnapshot(CategoriesState.getInflowWithBalance)!;
      const balance = inflowCategory.budgeted;
      const diff = Number(Number(budgeted - currentBudget).toFixed(2));
      if (diff <= balance) {
        // subtract from inflow
        inflowCategory.budgeted -= diff;
      } else {
        console.log('show alert for unavailable money');
        // unassign the budgeted
        categoryCopy.budgeted[this.budgetKey] = currentBudget;
      }
      categoryCopy.budgeted[this.budgetKey] = Number(Number(categoryCopy.budgeted[this.budgetKey]).toFixed(2));
      inflowCategory.budgeted = Number(Number(inflowCategory.budgeted).toFixed(2));
      this.editCategoryEvent.emit(categoryCopy);
      this.editCategoryEvent.emit(inflowCategory);
      // check all other categories and assign zero to them if not assigned
      this.store.assignZeroToUnassignedCategories();
    }
  }

  editCategory(category: Category) {
    this.editCategoryEvent.emit(category);
    this.menuDropdown.hide();
  }

  deleteCategory(category: Category) {
    this.deleteCategoryEvent.emit(category);
    this.menuDropdown.hide();
  }

  hideUnhideCategory(category: Category) {
    this.hideUnhideCategoryEvent.emit(category);
    this.menuDropdown.hide();
  }

  showMoveMenu(category: Category) {
    const options = { ...this.defaultOptions };
    options.placement = 'left';
    this.moveDropdown = this.helperService.getDropdownInstance(category.id!, 'moveDropdown', 'moveBtn', options);
    this.moveData = {
      from: {
        categoryId: category.id!,
        groupId: category.categoryGroupId,
        amount: category.balance?.[this.budgetKey]!,
      },
      to: {
        categoryId: '',
        groupId: '',
        name: '',
      },
    };
    this.moveDropdown.toggle();
  }

  showCategoryMenu(category: Category) {
    this.categoryDropdown = this.helperService.getDropdownInstance(
      category.id!,
      'moveCategoriesDropdown',
      'moveCategoriesBtn',
      this.defaultOptions,
    );
    this.categoryDropdown.toggle();
  }

  showActivityMenu(category: Category) {
    // filter category activity transactions
    const allTransactions = this.ngxsStore.selectSnapshot(TransactionsState.getAllTransactions);
    const ccAccounts = this.ngxsStore.selectSnapshot(AccountsState.getCreditCardAccounts);
    let categoryTransactions: Transaction[] = [];
    if (this.helperService.isCategoryCreditCard(category)) {
      categoryTransactions = this.helperService.getTransactionsForAccount(allTransactions, [
        ...ccAccounts.map((acc) => acc.id!),
      ]);
    } else {
      categoryTransactions = this.helperService.getTransactionsForCategory(allTransactions, [category.id!]);
    }
    this.categoryActivity = this.helperService.filterTransactionsBasedOnMonth(
      categoryTransactions,
      this.ngxsStore.selectSnapshot(BudgetsState.getSelectedMonth),
    );
    if (this.categoryActivity.length) {
      const activityMenuDropdown = this.helperService.getDropdownInstance(
        category.id!,
        'activityMenu',
        'activityMenuBtn',
        this.defaultOptions,
      );
      activityMenuDropdown.toggle();
    }
  }

  selectMoveCategory(category: Category) {
    this.moveData.to.name = category.name;
    this.moveData.to.categoryId = category.id!;
    this.moveData.to.groupId = category.categoryGroupId;
    this.categoryDropdown.toggle();
  }

  changeMoveData(value: number, category: Category) {
    if (value === null || value < 0) {
      this.moveData.from.amount = 0;
    } else {
      if (value > category.balance?.[this.budgetKey]!) {
        console.error("Can't assign more than category's balance");
        this.moveData.from.amount = category.balance?.[this.budgetKey]!;
      } else {
        this.moveData.from.amount = value;
      }
    }
  }

  moveBalance() {
    if (this.moveData?.from?.amount === null || !this.moveData?.to?.categoryId) {
      return;
    }
    const moveTo = this.ngxsStore.selectSnapshot(
      CategoryGroupsState.getCategory(this.moveData.to.categoryId, this.moveData.to.groupId),
    );

    const moveFrom = this.ngxsStore.selectSnapshot(
      CategoryGroupsState.getCategory(this.moveData.from.categoryId, this.moveData.from.groupId),
    );
    console.log('moveBalance:::', this.moveData);
    console.log('moveTo', moveTo);
    console.log('moveFrom', moveFrom);
    if (moveTo) {
      moveTo.budgeted[this.budgetKey] += this.moveData.from.amount;
      if (moveTo.balance) {
        moveTo.balance[this.budgetKey] += this.moveData.from.amount;
      } else {
        moveTo.balance = {
          [this.budgetKey]: this.moveData.from.amount,
        };
      }
      this.editCategoryEvent.emit(moveTo);
    }
    if (moveFrom) {
      moveFrom.budgeted[this.budgetKey] -= this.moveData.from.amount;
      if (moveFrom.balance) {
        moveFrom.balance[this.budgetKey] -= this.moveData.from.amount;
      }
      this.editCategoryEvent.emit(moveFrom);
    }
  }
}
