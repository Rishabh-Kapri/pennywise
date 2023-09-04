import { Component, OnInit } from '@angular/core';
import { initFlowbite } from 'flowbite';
import { StoreService } from './services/store.service';

@Component({
  selector: 'app-root',
  templateUrl: './app.component.html',
  styleUrls: ['./app.component.scss'],
})
export class AppComponent implements OnInit {
  title = 'pennywise';

  constructor(private store: StoreService) {
    this.store.init();
  }

  ngOnInit(): void {
    initFlowbite();
  }
}
