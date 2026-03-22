import {
  createAsyncThunk,
  createSlice,
  type PayloadAction,
} from '@reduxjs/toolkit';
import {
  BudgetAccountType,
  LoanAccountType,
  TrackingAccountType,
  type Account,
  type AccountState,
} from '../types/account.types';
import { apiClient, LoadingState } from '@/utils';
import type { RootState } from '@/app';

const initialState: AccountState = {
  selectedAccount: null,
  allAccounts: [],
  trackingAccounts: [],
  budgetAccounts: [],
  loanAccounts: [],
  loading: LoadingState.IDLE,
  error: null,
};

function filterTrackingAccounts(accounts: Account[]) {
  return accounts.filter(
    (acc) =>
      [TrackingAccountType.ASSET, TrackingAccountType.LIABILITY].includes(
        acc.type as TrackingAccountType,
      ) && !acc.closed,
  );
}

function filterBudgetAccounts(accounts: Account[]) {
  return accounts.filter(
    (acc) =>
      [
        BudgetAccountType.SAVINGS,
        BudgetAccountType.CHECKING,
        BudgetAccountType.CREDIT_CARD,
      ].includes(acc.type as BudgetAccountType) && !acc.closed,
  );
}

function filterLoanAccounts(accounts: Account[]) {
  return accounts.filter(
    (acc) =>
      Object.values(LoanAccountType).includes(
        acc.type as LoanAccountType,
      ) && !acc.closed,
  );
}

export const fetchAllAccounts = createAsyncThunk<Account[]>(
  'accounts/fetchAllAccounts',
  async () => {
    return await apiClient.get('accounts');
  },
);

const accountSlice = createSlice({
  name: 'accounts',
  initialState,
  reducers: {
    setSelectedAccount: (state, action: PayloadAction<Account>) => {
      state.selectedAccount = action.payload;
    },
    setBudgetAccounts: (state) => {
      state.budgetAccounts = filterBudgetAccounts(state.allAccounts);
    },
    setTrackingAccounts: (state) => {
      state.trackingAccounts = filterTrackingAccounts(state.allAccounts);
    },
    setLoanAccounts: (state) => {
      state.loanAccounts = filterLoanAccounts(state.allAccounts);
    },
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
        state.error = null;
      })
      .addCase(fetchAllAccounts.rejected, (state, action) => {
        state.loading = LoadingState.ERROR;
        state.error = action.error.message ?? null;
        state.allAccounts = [];
        state.trackingAccounts = [];
        state.budgetAccounts = [];
        state.loanAccounts = [];
      });
  },
});

export const {
  setSelectedAccount,
  setBudgetAccounts,
  setTrackingAccounts,
  setLoanAccounts,
} = accountSlice.actions;

export default accountSlice.reducer;

export const selectAccountInfoFromId = (state: RootState, id: string) => {
  if (id === '') {
    return {
      id: '',
      name: 'All Accounts',
      balance: state.accounts.allAccounts.reduce(
        (a, b) => a + (b?.balance ?? 0),
        0,
      ),
    };
  } else {
    const account = state.accounts.allAccounts.find((acc) => acc.id === id);
    return {
      id: account?.id ?? '',
      name: account?.name ?? '',
      balance: account?.balance ?? 0,
    };
  }
};

export const selectLoanAccounts = (state: RootState) =>
  state.accounts.loanAccounts;

