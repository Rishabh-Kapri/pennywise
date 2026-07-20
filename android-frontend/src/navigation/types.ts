import type { NavigatorScreenParams } from '@react-navigation/native';

export type AuthStackParamList = {
  Login: undefined;
};

export type AppTabParamList = {
  Dashboard: undefined;
  Budget: undefined;
  Transactions: { accountId?: string } | undefined;
  Payees: undefined;
  Loans: undefined;
  Penny: undefined;
};

export type RootStackParamList = {
  Main: NavigatorScreenParams<AppTabParamList>;
  Settings: undefined;
};
