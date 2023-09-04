import { AfterViewInit, Component, EventEmitter, Input, Output } from '@angular/core';
import { Parser } from 'expr-eval';
import { Dropdown, DropdownOptions } from 'flowbite';
import { take } from 'rxjs';
import { Category, InflowCategory } from 'src/app/models/category.model';
import { StoreService } from 'src/app/services/store.service';
/**
 * Get category data
 */
@Component({
  selector: 'app-category-item',
  templateUrl: './category-item.component.html',
  styleUrls: ['./category-item.component.scss'],
})
export class CategoryItemComponent implements AfterViewInit {
  @Input() categories: Category[];
  @Input() budgetKey: string;
  @Output() editCategoryEvent = new EventEmitter<Category | InflowCategory>();
  @Output() deleteCategoryEvent = new EventEmitter<Category>();
  @Output() hideCategoryEvent = new EventEmitter<Category>();

  menuDropdown: Dropdown;
  moveDropdown: Dropdown;
  categoryDropdown: Dropdown;
  parser = new Parser();
  moveData = {
    from: { categoryId: '', amount: 0 },
    to: { categoryId: '', name: '' },
  };
  defaultOptions: DropdownOptions = {
    placement: 'bottom',
    triggerType: 'click',
    offsetSkidding: 0,
    offsetDistance: 10,
    delay: 300,
    ignoreClickOutsideClass: false,
  };
  constructor(public store: StoreService) {}

  ngAfterViewInit(): void {
    // add info for assignment to categories
    // 1. category 1 - assignments - '2023-11': 2000
    //                               '2023-10': 1000
  }

  /**
   * Generic method for dropdown
   */
  getDropdownInstance(
    category: Category,
    targetIdPrefix: string,
    triggerIdPrefix: string,
    options?: DropdownOptions
  ): Dropdown {
    const targetEl = document.getElementById(`${targetIdPrefix}-${category.id}`);
    const triggerEl = document.getElementById(`${triggerIdPrefix}-${category.id}`);
    const dropdown = new Dropdown(targetEl, triggerEl, options);
    return dropdown;
  }

  showEditMenu(category: Category) {
    this.menuDropdown = this.getDropdownInstance(category, 'menuDropdown', 'menuBtn', this.defaultOptions);
    this.menuDropdown.toggle();
  }

  showBudgetInput(category: Category) {
    category.showBudgetInput = true;
  }

  // Budget to the category
  hideBudgetInput(category: Category, event: any) {
    const currentBudget = category.budgeted[this.budgetKey];
    try {
      category.showBudgetInput = false;
      const expr = this.parser.parse(event.target.value);
      category.budgeted[this.budgetKey] = expr.evaluate();
    } catch (err) {
      // do nothing
      category.budgeted[this.budgetKey] = category.budgeted[this.budgetKey];
    }
    if (category.budgeted[this.budgetKey] !== currentBudget) {
      // check if inflow has the amount that is being budgeted
      const budgeted = category.budgeted[this.budgetKey];
      const inflowCategory = this.store.inflowCategory$.value!;
      const balance = inflowCategory.budgeted;
      const diff = budgeted - currentBudget;
      console.log('DIFF:', diff);
      if (diff <= balance) {
        // subtract from inflow
        inflowCategory.budgeted -= diff;
      } else {
        console.log('show alert for unavailable money');
        // unassign the budgeted
        category.budgeted[this.budgetKey] = currentBudget;
      }
      console.log(category, inflowCategory);
      this.editCategoryEvent.emit(category);
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

  hideCategory(category: Category) {
    this.hideCategoryEvent.emit(category);
    this.menuDropdown.hide();
  }

  showMoveMenu(category: Category) {
    const options = { ...this.defaultOptions };
    options.placement = 'left';
    this.moveDropdown = this.getDropdownInstance(category, 'moveDropdown', 'moveBtn', options);
    this.moveData = {
      from: {
        categoryId: category.id!,
        amount: category.balance?.[this.budgetKey]!,
      },
      to: {
        categoryId: '',
        name: '',
      },
    };
    this.moveDropdown.toggle();
  }

  showCategoryMenu(category: Category) {
    this.categoryDropdown = this.getDropdownInstance(
      category,
      'moveCategoriesDropdown',
      'moveCategoriesBtn',
      this.defaultOptions
    );
    this.categoryDropdown.toggle();
  }

  selectMoveCategory(category: Category) {
    console.log(category);
    this.moveData.to.name = category.name;
    this.moveData.to.categoryId = category.id!;
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
    let moveTo: Category;
    this.store.categories$.pipe(take(1)).subscribe((categories) => {
      moveTo = categories.find((cat) => cat.id === this.moveData.to.categoryId) as Category;
      const moveFrom = this.categories.find((cat) => cat.id === this.moveData.from.categoryId);
      if (moveTo) {
        moveTo.budgeted[this.budgetKey] += this.moveData.from.amount;
        // moveTo.balance[this.budgetKey] += this.moveData.from.amount;
      }
      if (moveFrom) {
        moveFrom.budgeted[this.budgetKey] -= this.moveData.from.amount;
        // moveFrom.balance[this.budgetKey] -= this.moveData.from.amount;
      }
      this.editCategoryEvent.emit(moveFrom);
      this.editCategoryEvent.emit(moveTo);
    });
  }
}