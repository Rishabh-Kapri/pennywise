import type { JobStatus } from './job.types';

export interface JobScheduleRun {
  jobId: string;
  status: JobStatus;
  scheduledFor: string;
  completedAt?: string;
  lastError?: string;
}

export interface JobSchedule {
  id: string;
  type: string;
  /** What it's for, such as the mailbox address. */
  label?: string;
  cron: string;
  timezone: string;
  enabled: boolean;
  nextRunAt: string;
  lastRun?: JobScheduleRun;
}

export interface JobScheduleList {
  /** False when the server has no job store. */
  enabled: boolean;
  schedules: JobSchedule[];
}
