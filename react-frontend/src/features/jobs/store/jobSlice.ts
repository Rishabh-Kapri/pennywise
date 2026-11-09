import { apiClient, LoadingState } from '@/utils';
import type { RootState } from '@/app';
import { createAsyncThunk, createSlice, type PayloadAction } from '@reduxjs/toolkit';
import type { JobDetail, JobFilters, JobList, JobsState } from '../types/job.types';

export const JOBS_PAGE_SIZE = 30;

const initialFilters: JobFilters = {
  errorCode: '',
  jobId: '',
  offset: 0,
};

const initialState: JobsState = {
  list: null,
  listLoading: LoadingState.IDLE,
  error: null,
  filters: initialFilters,
  updatedAt: null,
  details: {},
  detailLoading: LoadingState.IDLE,
  actingJobId: null,
};

function jobsQuery(filters: JobFilters): string {
  const params = new URLSearchParams({ limit: String(JOBS_PAGE_SIZE) });
  if (filters.errorCode) params.set('errorCode', filters.errorCode);
  if (filters.jobId) params.set('jobId', filters.jobId);
  if (filters.offset) params.set('offset', String(filters.offset));
  return params.toString();
}

export const fetchJobs = createAsyncThunk<JobList, void, { state: RootState }>(
  'jobs/fetchJobs',
  async (_, { getState }) => {
    return await apiClient.get<JobList>(`jobs?${jobsQuery(getState().jobs.filters)}`);
  },
);

export const fetchJobDetail = createAsyncThunk<JobDetail, string>('jobs/fetchJobDetail', async (id) => {
  return await apiClient.get<JobDetail>(`jobs/${id}`);
});

export const runJobAction = createAsyncThunk<
  void,
  { id: string; action: 'retry' | 'cancel' | 'run-again' | 'dismiss' },
  { state: RootState }
>('jobs/runJobAction', async ({ id, action }, { dispatch }) => {
  await apiClient.post<{ status: string }, Record<string, never>>(`jobs/${id}/${action}`, {});
  // Running again and dismissing remove the job, so there's no detail to reload.
  const removed = action === 'run-again' || action === 'dismiss';
  await Promise.all([dispatch(fetchJobs()), ...(removed ? [] : [dispatch(fetchJobDetail(id))])]);
});

/** Queues every failed job with this error code again. Resolves to how many. */
export const retryFailedJobs = createAsyncThunk<number, string, { state: RootState }>(
  'jobs/retryFailedJobs',
  async (errorCode, { dispatch }) => {
    const { retried } = await apiClient.post<{ retried: number }, { errorCode: string }>('jobs/retry-failed', {
      errorCode,
    });
    await dispatch(fetchJobs());
    return retried;
  },
);

const jobSlice = createSlice({
  name: 'jobs',
  initialState,
  reducers: {
    /** Changing a filter goes back to the first page. */
    jobFiltersChanged: (state, action: PayloadAction<Partial<JobFilters>>) => {
      state.filters = { ...state.filters, offset: 0, ...action.payload };
    },
    jobDetailsCleared: (state) => {
      state.details = {};
    },
  },
  extraReducers: (builder) => {
    builder
      .addCase(fetchJobs.pending, (state) => {
        state.listLoading = LoadingState.PENDING;
      })
      .addCase(fetchJobs.fulfilled, (state, action) => {
        state.listLoading = LoadingState.SUCCESS;
        state.list = action.payload;
        state.updatedAt = new Date().toISOString();
        state.error = null;
      })
      .addCase(fetchJobs.rejected, (state, action) => {
        state.listLoading = LoadingState.ERROR;
        state.error = action.error.message ?? 'Failed to load jobs';
      })
      .addCase(fetchJobDetail.pending, (state) => {
        state.detailLoading = LoadingState.PENDING;
      })
      .addCase(fetchJobDetail.fulfilled, (state, action) => {
        state.detailLoading = LoadingState.SUCCESS;
        state.details[action.payload.id] = action.payload;
      })
      .addCase(fetchJobDetail.rejected, (state) => {
        state.detailLoading = LoadingState.ERROR;
      })
      .addCase(runJobAction.pending, (state, action) => {
        state.actingJobId = action.meta.arg.id;
      })
      .addCase(runJobAction.fulfilled, (state) => {
        state.actingJobId = null;
      })
      .addCase(runJobAction.rejected, (state) => {
        state.actingJobId = null;
      });
  },
});

export const { jobFiltersChanged, jobDetailsCleared } = jobSlice.actions;

export default jobSlice.reducer;
