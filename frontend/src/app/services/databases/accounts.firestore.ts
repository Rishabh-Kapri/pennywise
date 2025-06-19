import { Injectable } from "@angular/core";
import { NgxsFirestore } from "@ngxs-labs/firestore-plugin";
import { Account } from "src/app/models/account.model";

@Injectable({
  providedIn: 'root'
})
export class AccountsFirestore extends NgxsFirestore<Account[]> {
  protected path = 'accounts';
}
