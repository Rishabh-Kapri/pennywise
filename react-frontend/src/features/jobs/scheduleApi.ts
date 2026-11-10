import { apiClient } from '@/utils/api';
import type { JobScheduleList } from './types/schedule.types';

export const fetchJobSchedules = () => apiClient.get<JobScheduleList>('jobs/schedules');
