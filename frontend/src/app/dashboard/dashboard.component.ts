import { ChangeDetectionStrategy, Component, AfterViewInit, OnInit } from '@angular/core';
import { StoreService } from '../services/store.service';
import { SelectedComponent } from '../models/state.model';
import { Store } from '@ngxs/store';
import { AccountsState } from '../store/dashboard/states/accounts/accounts.state';
import { ConfigState } from '../store/dashboard/states/config/config.state';
import { ConfigActions } from '../store/dashboard/states/config/config.action';
import { BudgetsActions } from '../store/dashboard/states/budget/budget.action';
import { BudgetsState } from '../store/dashboard/states/budget/budget.state';
import { TransactionsActions } from '../store/dashboard/states/transactions/transaction.action';
import { AccountsActions } from '../store/dashboard/states/accounts/accounts.action';
import { PayeesActions } from '../store/dashboard/states/payees/payees.action';
import { CategoriesActions } from '../store/dashboard/states/categories/categories.action';
import { CategoryGroupsActions } from '../store/dashboard/states/categoryGroups/categoryGroups.action';

@Component({
  selector: 'app-dashboard',
  templateUrl: './dashboard.component.html',
  styleUrls: ['./dashboard.component.scss'],
  changeDetection: ChangeDetectionStrategy.OnPush,
  standalone: false,
})
export class DashboardComponent implements OnInit, AfterViewInit {
  selectedComponent = SelectedComponent;

  selectedAccount$ = this.ngxsStore.select(AccountsState.getSelectedAccount);
  selectedComponent$ = this.ngxsStore.select(ConfigState.getSelectedComponent);

  constructor(
    private ngxsStore: Store,
    public store: StoreService,
  ) { }

  ngOnInit(): void {
    this.ngxsStore.dispatch(new BudgetsActions.GetAllBudgets()).subscribe(() => {
      const selectedBudget = this.ngxsStore.selectSnapshot(BudgetsState.getSelectedBudget);
      const month = this.ngxsStore.selectSnapshot(BudgetsState.getSelectedMonth);
      console.log(selectedBudget, selectedBudget?.id);
      this.ngxsStore.dispatch([
        new TransactionsActions.GetNormalisedTransaction(),
        new PayeesActions.GetPayees(),
        new CategoriesActions.GetCategories(),
        new CategoryGroupsActions.GetCategoryGroups(month),
        new AccountsActions.GetAccounts(),
      ])
    });
  }

  ngAfterViewInit(): void {
    // No sidebar logic needed
  }

  public selectComponent(component: SelectedComponent) {
    this.ngxsStore.dispatch(new ConfigActions.SetSelectedComponent(component));
  }
}
