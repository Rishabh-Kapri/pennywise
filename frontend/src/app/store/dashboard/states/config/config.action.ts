import { SelectedComponent } from 'src/app/models/state.model';

export namespace ConfigActions {
  export class SetStateLoadingStatus {
    static readonly type = '[Config] SetStateLoadingStatus';
    constructor(readonly payload: boolean) {}
  }
  export class SetSelectedComponent {
    static readonly type = '[Config] SetSelectedComponent';
    constructor(readonly payload: SelectedComponent) {}
  }
}
