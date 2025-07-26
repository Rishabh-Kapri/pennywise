import { Injectable } from "@angular/core";
import { NgxsFirestore } from "@ngxs-labs/firestore-plugin";
import { Budget } from "src/app/models/budget.model";

@Injectable({
  providedIn: 'root'
})
export class BudgetsFirestore extends NgxsFirestore<Budget[]> {
  protected path = 'budgets';
}
