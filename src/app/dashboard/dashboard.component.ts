import { ChangeDetectionStrategy, Component } from '@angular/core';
import { StoreService } from '../services/store.service';
import { SelectedComponent } from '../models/state.model';
import { Store } from '@ngxs/store';
import { AccountsState } from '../store/dashboard/states/accounts/accounts.state';
import { ConfigState } from '../store/dashboard/states/config/config.state';

@Component({
  selector: 'app-dashboard',
  templateUrl: './dashboard.component.html',
  styleUrls: ['./dashboard.component.scss'],
  changeDetection: ChangeDetectionStrategy.OnPush,
  standalone: false,
})
export class DashboardComponent {
  selectedComponent = SelectedComponent;

  selectedAccount$ = this.ngxsStore.select(AccountsState.getSelectedAccount);
  selectedComponent$ = this.ngxsStore.select(ConfigState.getSelectedComponent);

  constructor(
    private ngxsStore: Store,
    public store: StoreService,
  ) {}
}
