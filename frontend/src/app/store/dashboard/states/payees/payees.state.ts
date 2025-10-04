import { Injectable } from '@angular/core';
import { Connected, Emitted, NgxsFirestoreConnect, StreamEmitted } from '@ngxs-labs/firestore-plugin';
import { Action, NgxsOnInit, Selector, State, StateContext, Store, createSelector } from '@ngxs/store';
import { Payee } from 'src/app/models/payee.model';
import { PayeesFirestore } from 'src/app/services/databases/payees.firestore';
import { PayeesActions } from './payees.action';
import { STARTING_BALANCE_PAYEE } from 'src/app/constants/general';
import { query, where } from 'firebase/firestore';
import { HttpService } from 'src/app/services/http.service';

export interface PayeesStateModel {
  allPayees: Payee[];
}
@State<PayeesStateModel>({
  name: 'payees',
  defaults: {
    allPayees: [],
  },
})
@Injectable()
export class PayeesState implements NgxsOnInit {
  readonly ENDPOINT = 'payees';

  @Selector()
  static getAllPayees(state: PayeesStateModel): Payee[] {
    return state.allPayees;
  }
  @Selector()
  static getStartingBalancePayee(state: PayeesStateModel): Payee | null {
    return state.allPayees.find((payee) => payee.name === STARTING_BALANCE_PAYEE) ?? null;
  }

  static getPayeeFromId(id: string): (state: PayeesStateModel) => Payee | null {
    return createSelector([PayeesState], (state: PayeesStateModel) => {
      const foundPayee = state.allPayees.find((payee) => payee.id === id) ?? null;
      return JSON.parse(JSON.stringify(foundPayee));
    });
  }

  constructor(
    private ngxsStore: Store,
    private ngxsFirestoreConnect: NgxsFirestoreConnect,
    private payeesFs: PayeesFirestore,
    private httpService: HttpService,
  ) { }

  ngxsOnInit() {
    // this.ngxsFirestoreConnect.connect(PayeesActions.GetAllPayees, {
    //   to: () => this.payeesFs.collection$((ref) => query(ref, where('budgetId', '==', 'Mm1kjyD58NQnNzOfx460'))),
    //   connectedActionFinishesOn: 'FirstEmit',
    // });
  }

  // @Action(PayeesActions.GetAllPayees)
  // initPayeesStream(ctx: StateContext<PayeesStateModel>, { budgetId }: PayeesActions.GetAllPayees) {
  //   this.ngxsFirestoreConnect.connect(PayeesActions.GetAllPayees, {
  //     to: () => this.payeesFs.collection$((ref) => query(ref, where('budgetId', '==', budgetId))),
  //     // connectedActionFinishesOn: 'FirstEmit',
  //   });
  // }
  //
  // @Action(StreamEmitted(PayeesActions.GetAllPayees))
  // getAllPayees(ctx: StateContext<PayeesStateModel>, { payload }: Emitted<PayeesActions.GetAllPayees, Payee[]>) {
  //   console.log("PAYEES:::", payload);
  //   ctx.patchState({
  //     allPayees: payload,
  //   });
  // }

  @Action(PayeesActions.GetPayees)
  getPayees(ctx: StateContext<PayeesStateModel>) {
    this.httpService.get<Payee[]>(this.ENDPOINT).subscribe({
      next: (payees) => {
        ctx.patchState({
          allPayees: payees ?? [],
        });
      },
    });
  }

  @Action(PayeesActions.CreatePayee)
  createPayee(ctx: StateContext<PayeesStateModel>, { payload }: PayeesActions.CreatePayee) {
    this.httpService.post<Partial<Payee>>(this.ENDPOINT, payload).subscribe({
      next: (res) => {
        ctx.dispatch(new PayeesActions.GetPayees());
      },
    });
  }
}
