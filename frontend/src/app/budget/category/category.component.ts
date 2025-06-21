import { AfterViewInit, ChangeDetectionStrategy, Component, OnDestroy, OnInit } from '@angular/core';
import { Store } from '@ngxs/store';
import { Dropdown } from 'flowbite';
import { BehaviorSubject } from 'rxjs';
import { Category, CategoryDTO, InflowCategory } from 'src/app/models/category.model';
import { CategoryGroup } from 'src/app/models/catergoryGroup';
import { CategoryGroupData } from 'src/app/models/state.model';
import { DatabaseService } from 'src/app/services/database.service';
import { HelperService } from 'src/app/services/helper.service';
import { StoreService } from 'src/app/services/store.service';
import { BudgetsState } from 'src/app/store/dashboard/states/budget/budget.state';
import { CategoryGroupsActions } from 'src/app/store/dashboard/states/categoryGroups/categoryGroups.action';
import { CategoryGroupsState } from 'src/app/store/dashboard/states/categoryGroups/categoryGroups.state';

@Component({
  selector: 'app-category',
  templateUrl: './category.component.html',
  styleUrls: ['./category.component.scss'],
  changeDetection: ChangeDetectionStrategy.OnPush,
  standalone: false,
})
export class CategoryComponent implements OnInit, AfterViewInit, OnDestroy {
  groupTargetEl: HTMLElement | null;
  groupTriggerEl: HTMLElement | null;
  groupDropdown: Dropdown;
  groupDropdowns: Record<string, Dropdown> = {};

  categoryGroupName: string;
  categoryName: string;

  destroy$ = new BehaviorSubject<boolean>(false);

  categoryGroups$ = this.ngxsStore.select(CategoryGroupsState.getCategoryGroupData);
  selectedMonth$ = this.ngxsStore.select(BudgetsState.getSelectedMonth);

  isDetailsPanelOpen = false;
  selectedCategory: Category | InflowCategory | null = null;

  constructor(
    private dbService: DatabaseService,
    private helperService: HelperService,
    private ngxsStore: Store,
    public store: StoreService,
  ) {}

  ngOnInit(): void {}

  ngOnDestroy(): void {
    this.destroy$.next(true);
    this.destroy$.complete();
  }

  ngAfterViewInit(): void {
    this.groupTargetEl = document.getElementById('addCategoryGroupDropdown');
    this.groupTriggerEl = document.getElementById('addCategoryGroupBtn');
    this.groupDropdown = new Dropdown(this.groupTargetEl, this.groupTriggerEl);
  }

  showDetails(category: Category | InflowCategory) {
    this.selectedCategory = category;
    this.isDetailsPanelOpen = true;
  }

  closeDetails() {
    this.isDetailsPanelOpen = false;
    this.selectedCategory = null;
  }

  async addCategoryGroup() {
    if (!this.categoryGroupName) {
      this.groupDropdown.hide();
      return;
    }
    const data: CategoryGroup = {
      budgetId: this.ngxsStore.selectSnapshot(BudgetsState.getSelectedBudget)?.id ?? '',
      name: this.categoryGroupName,
      hidden: false,
      deleted: false,
      createdAt: new Date().toISOString(),
    };
    await this.dbService.createCategoryGroup(data);
    this.groupDropdown.hide();
  }

  showDropdown(groupId: string, index: number) {
    const targetEl = document.getElementById(`addCategoryDropdown-${index}`);
    const triggerEl = document.getElementById(`addCategoryBtn-${index}`);
    const dropdown = new Dropdown(targetEl, triggerEl);
    this.groupDropdowns[groupId] = dropdown;
    dropdown.toggle();
  }

  collapseAll() {
    this.ngxsStore.dispatch(new CategoryGroupsActions.ToggleCategoryGroupsCollapse());
  }

  showHideGroupCategories(group: CategoryGroupData) {
    this.ngxsStore.dispatch(new CategoryGroupsActions.ToggleCategoryGroupCollapse(group));
  }

  async addCategory(groupId: string, index: number) {
    if (!this.categoryName) {
      this.groupDropdowns[groupId].hide();
      return;
    }
    const key = this.ngxsStore.selectSnapshot(BudgetsState.getSelectedMonth);
    const category: CategoryDTO = {
      budgetId: this.ngxsStore.selectSnapshot(BudgetsState.getSelectedBudget)?.id ?? '',
      name: this.categoryName,
      categoryGroupId: groupId,
      hidden: false,
      deleted: false,
      createdAt: new Date().toISOString(),
      budgeted: { [key]: 0 },
      note: null,
    };
    await this.dbService.createCategory(category);
    this.groupDropdowns[groupId].hide();
  }

  removeKeys(category: Category | InflowCategory) {
    const data = { ...category };
    if ('balance' in data) {
      delete data.balance;
    }
    if ('activity' in data) {
      delete data.activity;
    }
    delete data.showBudgetInput;
    return data;
  }

  async editCategory(category: Category | InflowCategory) {
    const data: Category | InflowCategory = this.removeKeys(category);
    await this.dbService.editCategory(data);
  }

  async deleteCategory(category: Category) {
    const data = this.removeKeys(category);
    data.deleted = true;
    await this.dbService.editCategory(data);
  }

  async hideUnhideCategory(category: Category) {
    const data = this.removeKeys({ ...category });
    data.hidden = !data.hidden;
    await this.dbService.editCategory(data);
  }
}
