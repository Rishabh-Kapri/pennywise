import { useAppSelector } from '@/app/hooks';
import { errorMessage, errorText } from '@/utils';
import { ApiError } from '@/utils/api';
import { CheckCircle, CircleNotch, MinusCircle, WarningCircle, type Icon } from '@phosphor-icons/react';
import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { formatAgo, formatTime } from '../jobTime';
import { fetchJobSchedules } from '../scheduleApi';
import type { JobStatus } from '../types/job.types';
import type { JobSchedule, JobScheduleList, JobScheduleRun } from '../types/schedule.types';
import jobStyles from './Jobs.module.css';
import styles from './Schedules.module.css';

const MAILBOX_RENEWAL = 'mailbox.watch.start';

const TITLES: Record<string, string> = {
  [MAILBOX_RENEWAL]: 'Keep email sync active',
};

const RUN_STATUS: Record<JobStatus, { label: string; icon: Icon }> = {
  queued: { label: 'Queued', icon: CircleNotch },
  running: { label: 'Running', icon: CircleNotch },
  succeeded: { label: 'Succeeded', icon: CheckCircle },
  dead: { label: 'Failed', icon: WarningCircle },
  cancelled: { label: 'Cancelled', icon: MinusCircle },
};

/** Plain words for a daily schedule; anything else shows its cron expression. */
function describeTiming(schedule: JobSchedule): string {
  const [minute, hour, ...rest] = schedule.cron.trim().split(/\s+/);
  const daily = rest.length === 3 && rest.every((field) => field === '*');
  if (daily && /^\d{1,2}$/.test(minute) && /^\d{1,2}$/.test(hour)) {
    return `Every day at ${hour.padStart(2, '0')}:${minute.padStart(2, '0')} ${schedule.timezone}`;
  }
  return `Cron ${schedule.cron} (${schedule.timezone})`;
}

function LastRun({ run }: { run?: JobScheduleRun }) {
  if (!run) return <div className={styles.run}>Hasn't run yet</div>;
  const { label, icon: StatusIcon } = RUN_STATUS[run.status];
  const failed = run.status === 'dead';
  return (
    <>
      <div className={`${styles.run} ${failed ? styles.failed : ''}`}>
        <StatusIcon size={15} weight={failed ? 'fill' : 'regular'} />
        <span>
          Last run: {label.toLowerCase()} {formatAgo(run.completedAt ?? run.scheduledFor)}
        </span>
      </div>
      {failed && run.lastError && (
        <div className={styles.error} title={run.lastError}>
          {errorText(run.lastError)}
        </div>
      )}
    </>
  );
}

function ScheduleRow({ schedule }: { schedule: JobSchedule }) {
  return (
    <li className={styles.row}>
      <div className={styles.head}>
        <span className={styles.title}>
          {TITLES[schedule.type] ?? schedule.type}
          {schedule.label && <span className={styles.account}> · {schedule.label}</span>}
        </span>
        <span className={styles.state}>{schedule.enabled ? 'On' : 'Off'}</span>
      </div>
      <div className={styles.timing}>
        {describeTiming(schedule)}
        {schedule.enabled && ` · Next ${formatTime(schedule.nextRunAt)}`}
      </div>
      <LastRun run={schedule.lastRun} />
    </li>
  );
}

/** The budget's recurring jobs. Read only. */
export function ScheduleView() {
  const budgetId = useAppSelector((state) => state.budgets.selectedBudget?.id);
  const [list, setList] = useState<JobScheduleList | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let stale = false;
    fetchJobSchedules()
      .then((loaded) => {
        if (stale) return;
        setList(loaded);
        setError(null);
      })
      .catch((err) => {
        if (stale) return;
        setError(
          err instanceof ApiError && err.status === 404
            ? "This server doesn't support schedules yet."
            : errorMessage(err, "Couldn't load the schedules."),
        );
      });
    return () => {
      stale = true;
    };
  }, [budgetId]);

  if (error) return <div className={jobStyles.empty}>{error}</div>;
  if (!list) return <div className={jobStyles.empty}>Loading schedules…</div>;
  if (!list.enabled) {
    return (
      <div className={jobStyles.empty}>
        The job queue is off. Set <code>JOB_DRIVER=postgres</code> on the API to see schedules here.
      </div>
    );
  }
  if (list.schedules.length === 0) return <div className={jobStyles.empty}>Nothing runs on a schedule yet.</div>;

  return (
    <div className={styles.root}>
      {list.schedules.some((schedule) => schedule.type === MAILBOX_RENEWAL) && (
        <p className={styles.intro}>
          Each mailbox renews its link with Gmail once a day so new emails keep arriving. Pause a mailbox in{' '}
          <Link to="/settings?section=mail">Mail settings</Link> to turn its renewal off.
        </p>
      )}
      <ul className={styles.list}>
        {list.schedules.map((schedule) => (
          <ScheduleRow key={schedule.id} schedule={schedule} />
        ))}
      </ul>
    </div>
  );
}
