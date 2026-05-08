import { createAsyncThunk, createSlice, type PayloadAction } from '@reduxjs/toolkit';
import { apiClient } from '../../../utils/api';
import { LoadingState } from '../../../utils/constants';
import type { RootState } from '../../../app/store';
import {
  BudgetAccountType,
  LoanAccountType,
  TrackingAccountType,
  type Account,
  type AccountState
} from '../types';

const initialState: AccountState = {
  selectedAccount: null,
  allAccounts: [],
  trackingAccounts: [],
  budgetAccounts: [],
  loanAccounts: [],
  loading: LoadingState.IDLE,
  error: null
};

function filterBudgetAccounts(accounts: Account[]) {
  return accounts.filter(
    (account) =>
      [BudgetAccountType.CHECKING, BudgetAccountType.SAVINGS, BudgetAccountType.CREDIT_CARD].includes(
        account.type as BudgetAccountType
      ) && !account.closed
  );
}

function filterTrackingAccounts(accounts: Account[]) {
  return accounts.filter(
    (account) =>
      [TrackingAccountType.ASSET, TrackingAccountType.LIABILITY].includes(account.type as TrackingAccountType) &&
      !account.closed
  );
}

function filterLoanAccounts(accounts: Account[]) {
  return accounts.filter(
    (account) => Object.values(LoanAccountType).includes(account.type as LoanAccountType) && !account.closed
  );
}

export const fetchAllAccounts = createAsyncThunk<Account[]>('accounts/fetchAllAccounts', async () => {
  return apiClient.get<Account[]>('accounts');
});

const accountSlice = createSlice({
  name: 'accounts',
  initialState,
  reducers: {
    setSelectedAccount: (state, action: PayloadAction<Account | null>) => {
      state.selectedAccount = action.payload;
    }
  },
  extraReducers: (builder) => {
    builder
      .addCase(fetchAllAccounts.pending, (state) => {
        state.loading = LoadingState.PENDING;
        state.error = null;
      })
      .addCase(fetchAllAccounts.fulfilled, (state, action) => {
        state.loading = LoadingState.SUCCESS;
        state.allAccounts = action.payload;
        state.budgetAccounts = filterBudgetAccounts(action.payload);
        state.trackingAccounts = filterTrackingAccounts(action.payload);
        state.loanAccounts = filterLoanAccounts(action.payload);
      })
      .addCase(fetchAllAccounts.rejected, (state, action) => {
        state.loading = LoadingState.ERROR;
        state.error = action.error.message ?? 'Failed to load accounts';
      });
  }
});

export const { setSelectedAccount } = accountSlice.actions;
export default accountSlice.reducer;

export const selectAccountInfoFromId = (state: RootState, id?: string) => {
  if (!id) {
    return {
      id: '',
      name: 'All Accounts',
      balance: state.accounts.allAccounts.reduce((sum, account) => sum + (account.balance ?? 0), 0)
    };
  }
  const account = state.accounts.allAccounts.find((item) => item.id === id);
  return { id: account?.id ?? '', name: account?.name ?? '', balance: account?.balance ?? 0 };
};

export const selectLoanAccounts = (state: RootState) => state.accounts.loanAccounts;
