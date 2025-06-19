import { Category } from 'src/app/models/category.model';
import { CategoryGroupData } from 'src/app/models/state.model';

export namespace CategoryGroupsActions {
  export class GetAllCategoryGroups {
    static readonly type = '[CategoryGroups] GetAllCategoryGroups';
    constructor(readonly budgetId: string) {}
  }
  export class SetCategoryGroupData {
    static readonly type = '[CategoryGroups] SetCategoryGroupData';
  }
  export class ToggleCategoryGroupsCollapse {
    static readonly type = '[CategoryGroups] ToggleCategoryGroupsCollapse';
  }
  export class ToggleCategoryGroupCollapse {
    static readonly type = '[CategoryGroups] ToggleCategoryGroupCollapse';
    constructor(readonly payload: CategoryGroupData) {}
  }
  export class UpdateCategoryInGroup {
    static readonly type = '[CategoryGroups]UpdateCategoryInGroup';
    constructor(
      readonly groupId: string,
      readonly categoryId: string,
      readonly data: Partial<Category>,
    ) {}
  }
}
