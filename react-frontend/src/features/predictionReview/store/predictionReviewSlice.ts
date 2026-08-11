import { createAsyncThunk, createSlice, type PayloadAction } from '@reduxjs/toolkit';
import { apiClient, LoadingState } from '@/utils';
import type { RootState } from '@/app';
import type {
  PredictionReviewItem,
  PredictionReviewRequest,
  PredictionReviewState,
} from '../types/predictionReview.types';

export const fetchReviewQueue = createAsyncThunk<PredictionReviewItem[], boolean | undefined>(
  'predictionReview/fetchQueue',
  async (includeReviewed = false) => {
    const query = includeReviewed ? '?includeReviewed=true&limit=100' : '?limit=100';
    const data = await apiClient.get<PredictionReviewItem[] | null>(`predictions/review${query}`);
    return data ?? [];
  },
);

export const reviewPrediction = createAsyncThunk<
  PredictionReviewItem,
  { id: string; request: PredictionReviewRequest }
>('predictionReview/review', async ({ id, request }) => {
  return await apiClient.post<PredictionReviewItem, PredictionReviewRequest>(
    `predictions/review/${id}`,
    request,
  );
});

const initialState: PredictionReviewState = {
  items: [],
  loading: LoadingState.IDLE,
  includeReviewed: false,
  submittingId: null,
  error: null,
};

const predictionReviewSlice = createSlice({
  name: 'predictionReview',
  initialState,
  reducers: {
    setIncludeReviewed: (state, action: PayloadAction<boolean>) => {
      state.includeReviewed = action.payload;
      state.loading = LoadingState.IDLE;
    },
  },
  extraReducers: (builder) => {
    builder
      .addCase(fetchReviewQueue.pending, (state) => {
        state.loading = LoadingState.PENDING;
        state.error = null;
      })
      .addCase(fetchReviewQueue.fulfilled, (state, action) => {
        state.loading = LoadingState.SUCCESS;
        state.items = action.payload;
      })
      .addCase(fetchReviewQueue.rejected, (state, action) => {
        state.loading = LoadingState.ERROR;
        state.error = action.error.message ?? 'Failed to load review queue';
      })
      .addCase(reviewPrediction.pending, (state, action) => {
        state.submittingId = action.meta.arg.id;
        state.error = null;
      })
      .addCase(reviewPrediction.fulfilled, (state, action) => {
        state.submittingId = null;
        if (state.includeReviewed) {
          const index = state.items.findIndex((item) => item.id === action.payload.id);
          if (index >= 0) {
            state.items[index] = action.payload;
          }
        } else {
          // pending-only view: a triaged row leaves the queue
          state.items = state.items.filter((item) => item.id !== action.payload.id);
        }
      })
      .addCase(reviewPrediction.rejected, (state, action) => {
        state.submittingId = null;
        state.error = action.error.message ?? 'Failed to submit review';
      });
  },
});

export const { setIncludeReviewed } = predictionReviewSlice.actions;

export default predictionReviewSlice.reducer;

// Selectors
export const selectReviewItems = (state: RootState) => state.predictionReview.items;
export const selectReviewLoading = (state: RootState) => state.predictionReview.loading;
export const selectReviewIncludeReviewed = (state: RootState) =>
  state.predictionReview.includeReviewed;
export const selectReviewSubmittingId = (state: RootState) => state.predictionReview.submittingId;
export const selectReviewError = (state: RootState) => state.predictionReview.error;
