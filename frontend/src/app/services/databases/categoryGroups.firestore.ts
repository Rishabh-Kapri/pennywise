import { Injectable } from '@angular/core';
import { NgxsFirestore } from '@ngxs-labs/firestore-plugin';
import { CategoryGroup } from 'src/app/models/catergoryGroup';

@Injectable({
  providedIn: 'root',
})
export class CategoryGroupsFirestore extends NgxsFirestore<CategoryGroup[]> {
  protected path = 'categoryGroups';
}
