import { ChangeDetectionStrategy, Component, OnInit, Renderer2 } from '@angular/core';
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
  isDarkMode: boolean;

  constructor(
    private store: StoreService, 
    private ngxsStore: Store, 
    private renderer: Renderer2
  ) {
    // this.store.initStoreActions();
    
    // Initialize theme based on localStorage or system preference
    this.isDarkMode = localStorage.getItem('theme') === 'dark' || 
                      (!('theme' in localStorage) && window.matchMedia('(prefers-color-scheme: dark)').matches);
    this.updateTheme();
  }

  ngOnInit(): void {
    initFlowbite();
  }

  toggleTheme(): void {
    this.isDarkMode = !this.isDarkMode;
    localStorage.setItem('theme', this.isDarkMode ? 'dark' : 'light');
    this.updateTheme();
  }

  private updateTheme(): void {
    if (this.isDarkMode) {
      this.renderer.addClass(document.body, 'dark');
    } else {
      this.renderer.removeClass(document.body, 'dark');
    }
  }
}
