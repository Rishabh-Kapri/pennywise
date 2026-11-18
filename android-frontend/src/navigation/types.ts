import type { NavigatorScreenParams } from '@react-navigation/native';

export type AuthStackParamList = {
  Login: undefined;
};

export type AppTabParamList = {
  Dashboard: undefined;
  Budget: undefined;
  Transactions:
    | {
        accountId?: string;
        categoryId?: string;
        categoryName?: string;
        payeeId?: string;
        payeeName?: string;
        month?: string;
      }
    | undefined;
  Reports: undefined;
};

export type RootStackParamList = {
  Main: NavigatorScreenParams<AppTabParamList>;
  Settings: undefined;
  Documents: undefined;
  Reconcile: undefined;
  Statement: { id: string };
  Tags: undefined;
  Jobs: undefined;
  Pipelines: undefined;
  AI: undefined;
  Loans: undefined;
  Payees: undefined;
  Penny: undefined;
  /** a budget category, for the budget's selected month */
  Category: { id: string };
};
