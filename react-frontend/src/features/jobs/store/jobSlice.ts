import { apiClient, LoadingState } from '@/utils';
import type { RootState } from '@/app';
import { createAsyncThunk, createSlice, type PayloadAction } from '@reduxjs/toolkit';
import type { JobDetail, JobFilters, JobList, JobsState, JobUpdate } from '../types/job.types';

export const JOBS_PAGE_SIZE = 60;
/** The most the API returns in one list (maxJobTreeLimit). */
export const JOBS_MAX_LIMIT = 1000;

const initialFilters: JobFilters = {
  errorCode: '',
  jobId: '',
  limit: JOBS_PAGE_SIZE,
};

const initialState: JobsState = {
  list: null,
  listLoading: LoadingState.IDLE,
  error: null,
  filters: initialFilters,
  updatedAt: null,
  liveVersion: 0,
  latest: {},
  statementVersion: 0,
  details: {},
  detailLoading: LoadingState.IDLE,
  actingJobId: null,
};

function jobsQuery(filters: JobFilters): string {
  const params = new URLSearchParams({ limit: String(filters.limit) });
  if (filters.errorCode) params.set('errorCode', filters.errorCode);
  if (filters.jobId) params.set('jobId', filters.jobId);
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
  { id: string; action: 'retry' | 'cancel' | 'run-again' | 'continue' | 'dismiss' },
  { state: RootState }
>('jobs/runJobAction', async ({ id, action }, { dispatch }) => {
  await apiClient.post<{ status: string }, Record<string, never>>(`jobs/${id}/${action}`, {});
  // These remove the job or move the message on, so there's no detail to reload.
  const removed = action === 'run-again' || action === 'continue' || action === 'dismiss';
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
    /** Changing a filter goes back to the newest jobs. */
    jobFiltersChanged: (state, action: PayloadAction<Partial<JobFilters>>) => {
      state.filters = { ...state.filters, limit: JOBS_PAGE_SIZE, ...action.payload };
    },
    /** Shows the change right away; the reload it prompts brings the rest. */
    jobUpdated: (state, action: PayloadAction<JobUpdate>) => {
      state.liveVersion += 1;
      const { jobId, type, status, progress } = action.payload;
      state.latest[jobId] = { ...state.latest[jobId], ...action.payload, progress: progress ?? state.latest[jobId]?.progress };
      if (type?.startsWith('statement.') && status !== 'queued' && status !== 'running') {
        state.statementVersion += 1;
      }
      for (const tree of state.list?.trees ?? []) {
        const job = tree.jobs.find((candidate) => candidate.id === jobId);
        if (!job) continue;
        job.status = status;
        if (progress) job.progress = progress;
        break;
      }
      const detail = state.details[jobId];
      if (detail) {
        detail.status = status;
        if (progress) detail.progress = progress;
      }
    },
    jobsMayHaveChanged: (state) => {
      state.liveVersion += 1;
      // Updates may have been missed while the socket was down.
      state.statementVersion += 1;
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

export const { jobFiltersChanged, jobDetailsCleared, jobUpdated, jobsMayHaveChanged } = jobSlice.actions;

export default jobSlice.reducer;
