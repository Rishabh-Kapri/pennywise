import { ChangeDetectionStrategy, Component } from '@angular/core';
import { StoreService } from '../services/store.service';
import { SelectedComponent } from '../models/state.model';

@Component({
  selector: 'app-dashboard',
  templateUrl: './dashboard.component.html',
  styleUrls: ['./dashboard.component.scss'],
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class DashboardComponent {
  selectedComponent = SelectedComponent;
  constructor(public store: StoreService) {}
}
