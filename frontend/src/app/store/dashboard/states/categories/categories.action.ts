import { Category } from 'src/app/models/category.model';

export namespace CategoriesActions {
  export class GetCategories {
    static readonly type = '[Categories] GetCategories';
  }
  export class GetAllCategories {
    static readonly type = '[Categories] GetAllCategories';
    constructor(readonly budgetId: string) {}
  }
  export class SetInflowCategoryBalance {
    static readonly type = '[Categories] SetInflowCategoryBalance';
  }
  export class UpdateCategory {
    static readonly type = '[Category] UpdateCategory';
    constructor(readonly payload: Category) {}
  }
}
