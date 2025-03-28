import { CategoryGroup } from 'src/app/models/catergoryGroup';

export namespace CategoryGroupsActions {
  export class GetAllCategoryGroups {
    static readonly type = '[CategoryGroups] GetAll';
  }
  export class SetCategoryGroupData {
    static readonly type = '[CategoryGroups] SetGroupData';
  }
}
