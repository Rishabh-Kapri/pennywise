import { AfterViewInit, ChangeDetectionStrategy, Component, EventEmitter, Input, Output, TemplateRef, ViewContainerRef } from '@angular/core';
import { Store } from '@ngxs/store';
import { Parser } from 'expr-eval';
import { Dropdown, DropdownOptions } from 'flowbite';
import { BehaviorSubject, Observable, map, take } from 'rxjs';
import { Account, BudgetAccountType } from 'src/app/models/account.model';
import { Category, InflowCategory } from 'src/app/models/category.model';
import { Payee } from 'src/app/models/payee.model';
import { NormalizedTransaction, Transaction } from 'src/app/models/transaction.model';
import { HelperService } from 'src/app/services/helper.service';
import { PopoverRef } from 'src/app/services/popover-ref';
import { PopoverService } from 'src/app/services/popover.service';
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
interface MoveData {
  category?: Category | null;
  amount?: number;
}
@Component({
  selector: 'app-category-item',
  templateUrl: './category-item.component.html',
  styleUrls: ['./category-item.component.scss'],
  changeDetection: ChangeDetectionStrategy.OnPush,
  standalone: false,
})
export class CategoryItemComponent implements AfterViewInit {
  @Input() categories: Category[];
  @Input() budgetKey: string;
  @Output() editCategoryEvent = new EventEmitter<Category | InflowCategory>();
  @Output() deleteCategoryEvent = new EventEmitter<Category>();
  @Output() hideUnhideCategoryEvent = new EventEmitter<Category>();
  @Output() showDetailsEvent = new EventEmitter<Category>();

  private activityMenuOverlayRef: PopoverRef;
  private moveMenuOverlayRef: PopoverRef;
  private categoryDropdownOverlayRef: PopoverRef;

  accountObj$: Observable<Record<string, Account>>;
  categoryObj$: Observable<Record<string, Category>>;
  payeeObj$: Observable<Record<string, Payee>>;
  categoryGroupData$ = this.ngxsStore.select(CategoryGroupsState.getCategoryGroupData);
  categoryActivity$ = new BehaviorSubject<{ name: string; transactions: NormalizedTransaction[]} | null>(null);
  moveData$ = new BehaviorSubject<{
    from: MoveData;
    to: MoveData;
  }>({
     from: { category: null, amount: 0 },
     to: { category: null },
  });

  menuDropdown: Dropdown;
  moveDropdown: Dropdown;
  categoryDropdown: Dropdown;
  parser = new Parser();
  moveData: {
    from: MoveData;
    to: MoveData;
  } = {
    from: { category: null, amount: 0 },
    to: { category: null },
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
    private popper: PopoverService,
    private viewContainerRef: ViewContainerRef,
    public store: StoreService,
    public helperService: HelperService,
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
    let budgeted = 0;
    try {
      this.ngxsStore.dispatch(
        new CategoryGroupsActions.UpdateCategoryInGroup(category.categoryGroupId, category.id!, {
          showBudgetInput: false,
        }),
      );
      const expr = this.parser.parse(event.target.value);
      budgeted = expr.evaluate();
      categoryCopy.budgeted[this.budgetKey] = budgeted;
    } catch (err) {
      // do nothing
      budgeted = categoryCopy.budgeted[this.budgetKey];
      categoryCopy.budgeted[this.budgetKey] = budgeted;
    }
    if (budgeted !== currentBudget) {
      // check if inflow has the amount that is being budgeted
      const inflowCategory = JSON.parse(
        JSON.stringify(this.ngxsStore.selectSnapshot(CategoriesState.getInflowWithBalance)!),
      );
      const balance = inflowCategory.budgeted;
      const diff = Number(Number(budgeted - currentBudget).toFixed(2));
      console.log(diff);
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

      console.log(categoryCopy.budgeted[this.budgetKey], inflowCategory.budgeted);
      console.log(
        {
          categoryId: categoryCopy.id!,
          budgeted: budgeted,
          month: this.budgetKey,
        }
      )
      this.ngxsStore.dispatch(
        new CategoriesActions.UpdateCategoryBudgeted({
          categoryId: categoryCopy.id!,
          budgeted: categoryCopy.budgeted[this.budgetKey],
          month: this.budgetKey,
        }),
      );
      // this.editCategoryEvent.emit(categoryCopy);
      // this.editCategoryEvent.emit(inflowCategory);
      // check all other categories and assign zero to them if not assigned
      // this.store.assignZeroToUnassignedCategories();
    }
  }

  editCategory(category: Category) {
    // this.editCategoryEvent.emit(category);
    this.menuDropdown.hide();
  }

  deleteCategory(category: Category) {
    // this.deleteCategoryEvent.emit(category);
    this.menuDropdown.hide();
  }

  hideUnhideCategory(category: Category) {
    // @TODO: only hide category when balance is zero
    // this.hideUnhideCategoryEvent.emit(category);
    this.menuDropdown.hide();
  }

  showMoveMenu(category: Category, content: TemplateRef<any>, origin: HTMLElement) {
    console.log(category);
    if (this.moveMenuOverlayRef?.isOpen) {
      return;
    }
    // const options = { ...this.defaultOptions };
    // options.placement = 'left';
    // this.moveDropdown = this.helperService.getDropdownInstance(category.id!, 'moveDropdown', 'moveBtn', options);
    this.moveData = {
      from: {
        category,
        amount: category.balance?.[this.budgetKey]!,
      },
      to: {
        category: null,
      },
    };
    this.moveData$.next(this.moveData);
    this.moveMenuOverlayRef = this.popper.open({
      origin,
      content,
      viewContainerRef: this.viewContainerRef,
    })
    // this.moveDropdown.toggle();
  }

  showCategoryMenu(category: Category, content: TemplateRef<any>, origin: HTMLElement) {
    // this.categoryDropdown = this.helperService.getDropdownInstance(
    //   category.id!,
    //   'moveCategoriesDropdown',
    //   'moveCategoriesBtn',
    //   this.defaultOptions,
    // );
    // this.categoryDropdown.toggle();
    this.categoryDropdownOverlayRef = this.popper.open({
      origin,
      content,
      viewContainerRef: this.viewContainerRef,
    });
  }

  showActivityMenu(category: Category, content: TemplateRef<any>, origin: HTMLElement) {
    // filter category activity transactions
    if (this.activityMenuOverlayRef?.isOpen) {
      return;
    }
    const allTransactions = this.ngxsStore.selectSnapshot(TransactionsState.getNormalizedTransaction);
    const ccAccounts = this.ngxsStore.selectSnapshot(AccountsState.getCreditCardAccounts);
    let categoryTransactions: NormalizedTransaction[] = [];
    if (this.helperService.isCategoryCreditCard(category)) {
      categoryTransactions = <NormalizedTransaction[]>(
        this.helperService.getTransactionsForAccount(allTransactions, [...ccAccounts.map((acc) => acc.id!)])
      );
    } else {
      categoryTransactions = this.helperService.getTransactionsForCategory(allTransactions, [category.id!]);
    }
    const categoryActivity = this.helperService.filterTransactionsBasedOnMonth(
      categoryTransactions,
      this.ngxsStore.selectSnapshot(BudgetsState.getSelectedMonth),
    );
    if (categoryActivity.length) {
      this.categoryActivity$.next({ name: category.name, transactions: categoryActivity });
      this.activityMenuOverlayRef = this.popper.open({
        origin,
        content,
        viewContainerRef: this.viewContainerRef,
      });
    }
  }

  private getElementById<T>(id: string): T | null {
    return document.getElementById(id) as T;
  }

  selectMoveCategory(category: Category) {
    const prevMoveData = this.moveData$.value;
    this.moveData$.next({
      ...prevMoveData,
      to: {
        category: category,
        amount: 0,
      },
    });
    if (this.categoryDropdownOverlayRef?.isOpen) {
      this.categoryDropdownOverlayRef.close();
    }
    // this.categoryDropdown.toggle();
  }

  changeMoveData(value: number) {
    const moveData = this.moveData$.value;
    const category = moveData.from.category;
    if (!category) {
      return;
    }
    if (value === null || value < 0) {
      moveData.from.amount = 0;
    } else {
      if (value > category.balance?.[this.budgetKey]!) {
        console.error("Can't assign more than category's balance");
        moveData.from.amount = category.balance?.[this.budgetKey]!;
      } else {
        moveData.from.amount = value;
      }
    }
    console.log('changeMoveData:', moveData);
    this.moveData$.next({ ...moveData });
  }

  moveBalance() {
    const moveData = this.moveData$.value;
    if (moveData?.from?.amount === null || !moveData?.from?.category?.id || !moveData?.to?.category?.id) {
      return;
    }
    const moveTo = this.ngxsStore.selectSnapshot(
      CategoryGroupsState.getCategory(moveData.to.category?.id, moveData.to.category?.categoryGroupId),
    );

    const moveFrom = this.ngxsStore.selectSnapshot(
      CategoryGroupsState.getCategory(moveData.from.category.id, moveData.from.category.categoryGroupId),
    );
    let refetchMonthBudget = false;
    console.log(moveData);
    console.log('moving from:', {
      categoryId: moveFrom?.id!,
      categoryName: moveFrom?.name,
      budgeted: (moveFrom?.budgeted?.[this.budgetKey] ?? 0) - (this.moveData?.from?.amount ?? 0),
      month: this.budgetKey,
    });
    console.log('moving to:', {
      categoryId: moveTo?.id!,
      categoryName: moveTo?.name,
      budgeted: (moveTo?.budgeted?.[this.budgetKey] ?? 0) + (this.moveData?.from?.amount ?? 0),
      month: this.budgetKey,
    });
    if (moveTo) {
      this.ngxsStore.dispatch(
        new CategoriesActions.UpdateCategoryBudgeted({
          categoryId: moveTo.id!,
          budgeted: (moveTo?.budgeted?.[this.budgetKey] ?? 0) + (this.moveData?.from?.amount ?? 0),
          month: this.budgetKey,
        })
      );
      refetchMonthBudget = true;
    }
    if (moveFrom) {
      this.ngxsStore.dispatch(
        new CategoriesActions.UpdateCategoryBudgeted({
          categoryId: moveFrom.id!,
          budgeted: (moveFrom?.budgeted?.[this.budgetKey] ?? 0) - (this.moveData?.from?.amount ?? 0),
          month: this.budgetKey,
        })
      );
      refetchMonthBudget = true;
    }
    if (refetchMonthBudget)  {
      this.ngxsStore.dispatch(new CategoryGroupsActions.GetCategoryGroups(this.budgetKey))
    }
    this.moveMenuOverlayRef.close();
  }
}
