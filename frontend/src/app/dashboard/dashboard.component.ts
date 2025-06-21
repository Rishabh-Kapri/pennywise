import { ChangeDetectionStrategy, Component, AfterViewInit } from '@angular/core';
import { StoreService } from '../services/store.service';
import { SelectedComponent } from '../models/state.model';
import { Store } from '@ngxs/store';
import { AccountsState } from '../store/dashboard/states/accounts/accounts.state';
import { ConfigState } from '../store/dashboard/states/config/config.state';
import { ConfigActions } from '../store/dashboard/states/config/config.action';

@Component({
  selector: 'app-dashboard',
  templateUrl: './dashboard.component.html',
  styleUrls: ['./dashboard.component.scss'],
  changeDetection: ChangeDetectionStrategy.OnPush,
  standalone: false,
})
export class DashboardComponent implements AfterViewInit {
  selectedComponent = SelectedComponent;

  selectedAccount$ = this.ngxsStore.select(AccountsState.getSelectedAccount);
  selectedComponent$ = this.ngxsStore.select(ConfigState.getSelectedComponent);

  constructor(
    private ngxsStore: Store,
    public store: StoreService,
  ) {}

  ngAfterViewInit(): void {
    // No sidebar logic needed
  }

  public selectComponent(component: SelectedComponent) {
    this.ngxsStore.dispatch(new ConfigActions.SetSelectedComponent(component));
  }
}
