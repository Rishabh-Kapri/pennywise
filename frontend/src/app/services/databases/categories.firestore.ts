import { Injectable } from '@angular/core';
import { NgxsFirestore } from '@ngxs-labs/firestore-plugin';
import { Category } from 'src/app/models/category.model';

@Injectable({
  providedIn: 'root',
})
export class CategoriesFirestore extends NgxsFirestore<Category[]> {
  protected path = 'categories';
}
