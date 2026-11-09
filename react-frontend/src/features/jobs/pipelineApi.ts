import { apiClient } from '@/utils/api';
import type {
  PipelineCatalog,
  PipelineId,
  PipelineStep,
  PipelineStepId,
  UpdatePipelineStepPayload,
} from './types/pipeline.types';

export const fetchPipelines = () => apiClient.get<PipelineCatalog>('pipelines');

const stepPath = (pipeline: PipelineId, step: PipelineStepId) =>
  `pipelines/${encodeURIComponent(pipeline)}/steps/${encodeURIComponent(step)}`;

export const updatePipelineStep = (pipeline: PipelineId, step: PipelineStepId, payload: UpdatePipelineStepPayload) =>
  apiClient.put<PipelineStep, UpdatePipelineStepPayload>(stepPath(pipeline, step), payload);

/** Deletes the budget's override, so the step follows the default again. */
export const resetPipelineStep = (pipeline: PipelineId, step: PipelineStepId) =>
  apiClient.delete<PipelineStep>(stepPath(pipeline, step));
