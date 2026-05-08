export type AuthStackParamList = {
  Login: undefined;
};

export type AppTabParamList = {
  Dashboard: undefined;
  Budget: undefined;
  Transactions: { accountId?: string } | undefined;
  Payees: undefined;
  Loans: undefined;
  Settings: undefined;
};
