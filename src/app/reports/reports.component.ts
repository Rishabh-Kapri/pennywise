import { Component, TemplateRef, ViewContainerRef } from '@angular/core';
import { StoreService } from '../services/store.service';
import { Account } from '../models/account.model';
import { Observable, combineLatest, of, switchMap } from 'rxjs';
import { CommonModule } from '@angular/common';
import { PopoverRef } from '../services/popover-ref';
import { PopoverService } from '../services/popover.service';

@Component({
  selector: 'app-reports',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './reports.component.html',
  styleUrl: './reports.component.scss',
})
export class ReportsComponent {
  accountFilterOverlayRef: PopoverRef;
  dateFilterOverlayRef: PopoverRef;
  accountData$: Observable<{ name: string; accounts: Account[] }[]>;

  constructor(private popper: PopoverService, private viewContainerRef: ViewContainerRef, public store: StoreService) {
    this.accountData$ = combineLatest([this.store.budgetAccounts$, this.store.trackingAccounts$]).pipe(
      switchMap(([budgetAccounts, trackingAccounts]) => {
        const groupData = [
          { name: 'Budget Accounts', accounts: budgetAccounts },
          { name: 'Tracking Accounts', accounts: trackingAccounts },
        ];
        return of(groupData);
      })
    );
  }

  showAccountFilter(content: TemplateRef<any>, origin: HTMLElement) {
    if (this.accountFilterOverlayRef?.isOpen) return;
    this.accountFilterOverlayRef = this.popper.open({ origin, content, viewContainerRef: this.viewContainerRef });
  }

  showDateFilter(content: TemplateRef<any>, origin: HTMLElement) {
    if (this.dateFilterOverlayRef?.isOpen) return;
    this.dateFilterOverlayRef = this.popper.open({ origin, content, viewContainerRef: this.viewContainerRef });
  }
}
