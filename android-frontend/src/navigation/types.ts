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
  Payees: undefined;
  Penny: undefined;
};

export type RootStackParamList = {
  Main: NavigatorScreenParams<AppTabParamList>;
  Settings: undefined;
  Documents: undefined;
  Tags: undefined;
  Activity: undefined;
  AI: undefined;
  Loans: undefined;
};
