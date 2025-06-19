import { ChangeDetectionStrategy, Component, OnInit } from '@angular/core';
import { initFlowbite } from 'flowbite';
import { StoreService } from './services/store.service';
import { Store } from '@ngxs/store';
import { ngxsFirestoreConnections } from '@ngxs-labs/firestore-plugin';

@Component({
  selector: 'app-root',
  templateUrl: './app.component.html',
  styleUrls: ['./app.component.scss'],
  changeDetection: ChangeDetectionStrategy.OnPush,
  standalone: false,
})
export class AppComponent implements OnInit {
  title = 'pennywise';
  ngxsFirestoreState$ = this.ngxsStore.select(ngxsFirestoreConnections);

  constructor(private store: StoreService, private ngxsStore: Store) {
    // this.store.init();
    this.store.initStoreActions();
  }

  ngOnInit(): void {
    initFlowbite();
  }
}
