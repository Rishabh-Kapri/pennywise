import { Category } from 'src/app/models/category.model';

export namespace CategoriesActions {
  export class GetCategories {
    static readonly type = '[Category] GetCategories';
  }
  export class GetAllCategories {
    static readonly type = '[Category] GetAllCategories';
    constructor(readonly budgetId: string) { }
  }
  export class SetInflowCategoryBalance {
    static readonly type = '[Category] SetInflowCategoryBalance';
  }
  export class CreateCategory {
    static readonly type = '[Category] CreateCategory';
    constructor(readonly payload: Category) {}
  }
  export class UpdateCategory {
    static readonly type = '[Category] UpdateCategory';
    constructor(readonly payload: Category) { }
  }
  export class UpdateCategoryBudgeted {
    static readonly type = '[Category] UpdateCategoryBudgeted';
    constructor(
      readonly payload: {
        categoryId: string;
        budgeted: number;
        month: string;
      },
    ) { }
  }
}
