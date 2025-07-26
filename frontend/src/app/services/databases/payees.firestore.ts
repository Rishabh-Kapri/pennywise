import { Injectable } from '@angular/core';
import { NgxsFirestore } from '@ngxs-labs/firestore-plugin';
import { Payee } from 'src/app/models/payee.model';

@Injectable({
  providedIn: 'root',
})
export class PayeesFirestore extends NgxsFirestore<Payee[]> {
  protected path = 'payees';
}
