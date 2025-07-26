import { ChangeDetectionStrategy, Component, OnInit } from '@angular/core';
import { StoreService } from '../services/store.service';
import { Store } from '@ngxs/store';
import { CategoriesState } from '../store/dashboard/states/categories/categories.state';
import { BudgetsState } from '../store/dashboard/states/budget/budget.state';

@Component({
  selector: 'app-budget',
  templateUrl: './budget.component.html',
  styleUrls: ['./budget.component.scss'],
  changeDetection: ChangeDetectionStrategy.OnPush,
  standalone: false,
})
export class BudgetComponent implements OnInit {
  inflowCategory$ = this.ngxsStore.select(CategoriesState.getInflowWithBalance);
  selectedMonth$ = this.ngxsStore.select(BudgetsState.getSelectedMonth);

  constructor(
    private ngxsStore: Store,
    public store: StoreService,
  ) {}

  ngOnInit(): void {}
}
