import { AfterViewInit, Component, OnDestroy, OnInit } from '@angular/core';
import { Dropdown } from 'flowbite';
import { BehaviorSubject } from 'rxjs';
import { Category, CategoryDTO, InflowCategory } from 'src/app/models/category.model';
import { CategoryGroup } from 'src/app/models/catergoryGroup';
import { DatabaseService } from 'src/app/services/database.service';
import { HelperService } from 'src/app/services/helper.service';
import { StoreService } from 'src/app/services/store.service';

@Component({
  selector: 'app-category',
  templateUrl: './category.component.html',
  styleUrls: ['./category.component.scss'],
})
export class CategoryComponent implements OnInit, AfterViewInit, OnDestroy {
  groupTargetEl: HTMLElement | null;
  groupTriggerEl: HTMLElement | null;
  groupDropdown: Dropdown;
  groupDropdowns: Record<string, Dropdown> = {};

  categoryGroupName: string;
  categoryName: string;

  destroy$ = new BehaviorSubject<boolean>(false);

  constructor(private dbService: DatabaseService, public store: StoreService, private helperService: HelperService) {}

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

  async addCategoryGroup() {
    if (!this.categoryGroupName) {
      this.groupDropdown.hide();
      return;
    }
    const data: CategoryGroup = {
      budgetId: this.store.selectedBudet,
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

  async addCategory(groupId: string, index: number) {
    if (!this.categoryName) {
      this.groupDropdowns[groupId].hide();
      return;
    }
    const key = this.store.selectedMonth;
    const category: CategoryDTO = {
      budgetId: this.store.selectedBudet,
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
    console.log('editing category', category.name);
    const data: Category | InflowCategory = this.removeKeys(category);
    await this.dbService.editCategory(data);
  }

  async deleteCategory(category: Category) {
    console.log('deleting category', category.name);
    const data = this.removeKeys(category);
    data.deleted = true;
    await this.dbService.editCategory(data);
  }

  async hideCategory(category: Category) {
    console.log('hiding category', category.name);
    const data = this.removeKeys(category);
    data.hidden = true;
    await this.dbService.editCategory(data);
  }
}
