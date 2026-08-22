import { createAsyncThunk, createSlice, type PayloadAction } from '@reduxjs/toolkit';
import { apiClient } from '../../../utils/api';
import { LoadingState } from '../../../utils/constants';
import type { PipelineRun, PipelineRunDetail, PipelineState } from '../types';

const RUNS_FETCH_LIMIT = 50;

const initialState: PipelineState = {
  runs: [],
  runsLoading: LoadingState.IDLE,
  error: null,
  eventsByRunId: {},
  detailLoading: LoadingState.IDLE,
  retryingRunId: null
};

export const fetchPipelineRuns = createAsyncThunk<PipelineRun[]>('pipeline/fetchPipelineRuns', async () => {
  return apiClient.get<PipelineRun[]>(`pipeline/runs?limit=${RUNS_FETCH_LIMIT}`);
});

export const fetchPipelineRunDetail = createAsyncThunk<PipelineRunDetail, string>(
  'pipeline/fetchPipelineRunDetail',
  async (runId) => {
    return apiClient.get<PipelineRunDetail>(`pipeline/runs/${runId}`);
  }
);

export const retryPipelineRun = createAsyncThunk<{ status: string; run: PipelineRun }, string>(
  'pipeline/retryPipelineRun',
  async (runId) => {
    return apiClient.post<{ status: string; run: PipelineRun }>(`pipeline/runs/${runId}/retry`, {});
  }
);

function mergeRun(state: PipelineState, run: PipelineRun) {
  const index = state.runs.findIndex((existing) => existing.id === run.id);
  if (index === -1) {
    state.runs.unshift(run);
    return;
  }
  // Websocket delivery is best-effort; ignore stale updates.
  if (new Date(run.updatedAt) >= new Date(state.runs[index].updatedAt)) {
    state.runs[index] = run;
  }
}

const pipelineSlice = createSlice({
  name: 'pipeline',
  initialState,
  reducers: {
    pipelineRunUpdated: (state, action: PayloadAction<PipelineRun>) => {
      mergeRun(state, action.payload);
    }
  },
  extraReducers: (builder) => {
    builder
      .addCase(fetchPipelineRuns.pending, (state) => {
        state.runsLoading = LoadingState.PENDING;
        state.error = null;
      })
      .addCase(fetchPipelineRuns.fulfilled, (state, action) => {
        state.runsLoading = LoadingState.SUCCESS;
        state.runs = action.payload ?? [];
        state.error = null;
      })
      .addCase(fetchPipelineRuns.rejected, (state, action) => {
        state.runsLoading = LoadingState.ERROR;
        state.error = action.error.message ?? 'Failed to load pipeline runs';
      })
      .addCase(fetchPipelineRunDetail.pending, (state) => {
        state.detailLoading = LoadingState.PENDING;
      })
      .addCase(fetchPipelineRunDetail.fulfilled, (state, action) => {
        state.detailLoading = LoadingState.SUCCESS;
        state.eventsByRunId[action.payload.run.id] = action.payload.events ?? [];
        mergeRun(state, action.payload.run);
      })
      .addCase(fetchPipelineRunDetail.rejected, (state) => {
        state.detailLoading = LoadingState.ERROR;
      })
      .addCase(retryPipelineRun.pending, (state, action) => {
        state.retryingRunId = action.meta.arg;
      })
      .addCase(retryPipelineRun.fulfilled, (state) => {
        state.retryingRunId = null;
      })
      .addCase(retryPipelineRun.rejected, (state) => {
        state.retryingRunId = null;
      });
  }
});

export const { pipelineRunUpdated } = pipelineSlice.actions;

export default pipelineSlice.reducer;

export const selectPipelineRuns = (state: { pipeline: PipelineState }) => state.pipeline.runs;
export const selectPipelineRunsLoading = (state: { pipeline: PipelineState }) => state.pipeline.runsLoading;
export const selectPipelineError = (state: { pipeline: PipelineState }) => state.pipeline.error;
export const selectActivePipelineRunCount = (state: { pipeline: PipelineState }) =>
  state.pipeline.runs.filter((run) => run.status === 'running' || run.status === 'waiting_retry').length;
