import { Component } from '@angular/core';
import { StoreService } from '../services/store.service';
import { SelectedComponent } from '../models/state.model';

@Component({
  selector: 'app-dashboard',
  templateUrl: './dashboard.component.html',
  styleUrls: ['./dashboard.component.scss'],
})
export class DashboardComponent {
  selectedComponent = SelectedComponent;
  constructor(public store: StoreService) {}
}
