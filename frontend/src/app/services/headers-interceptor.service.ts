import { HttpEvent, HttpHandler, HttpInterceptor, HttpRequest } from '@angular/common/http';
import { Injectable } from '@angular/core';
import { Store } from '@ngxs/store';
import { Observable } from 'rxjs';
import { BudgetsState } from '../store/dashboard/states/budget/budget.state';
import { environment } from 'src/environment/environment';

@Injectable()
export class HeadersInterceptor implements HttpInterceptor {
  constructor(private ngxsStore: Store) { }

  intercept(req: HttpRequest<any>, next: HttpHandler): Observable<HttpEvent<any>> {
    const selectedBudget = this.ngxsStore.selectSnapshot(BudgetsState.getSelectedBudget);
    const selectedBudgetId = selectedBudget?.id ?? '';

    if (req.url.startsWith(environment.apiUrl) && selectedBudgetId) {
      const modifiedReq = req.clone({
        headers: req.headers.set('X-Budget-ID', selectedBudgetId),
      });
      return next.handle(modifiedReq);
    }
    return next.handle(req);
  }
}
