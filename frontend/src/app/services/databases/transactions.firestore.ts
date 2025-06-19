import { Injectable } from '@angular/core';
import { NgxsFirestore } from '@ngxs-labs/firestore-plugin';
import { Transaction } from 'firebase/firestore';

@Injectable({
  providedIn: 'root',
})
export class TransactionsFirestore extends NgxsFirestore<Transaction[]> {
  protected path = 'transactions';
}
