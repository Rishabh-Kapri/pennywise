import { Injectable } from '@angular/core';
import { Emitted, NgxsFirestoreConnect, StreamEmitted } from '@ngxs-labs/firestore-plugin';
import { Action, NgxsOnInit, Selector, State, StateContext } from '@ngxs/store';
import { Payee } from 'src/app/models/payee.model';
import { PayeesFirestore } from 'src/app/services/databases/payees.firestore';
import { PayeesActions } from './payees.action';

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
  @Selector()
  static getAllPayees(state: PayeesStateModel): Payee[] {
    return state.allPayees;
  }

  constructor(
    private ngxsFirestoreConnect: NgxsFirestoreConnect,
    private payeesFs: PayeesFirestore,
  ) {}

  ngxsOnInit(ctx: StateContext<any>): void {
    this.ngxsFirestoreConnect.connect(PayeesActions.GetAllPayees, {
      to: () => this.payeesFs.collection$(),
      connectedActionFinishesOn: 'FirstEmit',
    });
  }

  @Action(StreamEmitted(PayeesActions.GetAllPayees))
  getAllBudgets(
    ctx: StateContext<PayeesStateModel>,
    { action, payload }: Emitted<PayeesActions.GetAllPayees, Payee[]>,
  ) {
    ctx.setState({
      allPayees: payload,
    });
  }
}
