import assert from 'node:assert';
import { finish, test } from './harness.mts';
import { buildRows, buildRun, emailGroup, messageSteps, type JobRow, type JobRun } from '../src/features/jobs/jobTree.ts';
import { mergeBySource, splitByDay } from '../src/features/jobs/runGroups.ts';
import type { JobSummary, JobTree } from '../src/features/jobs/types.ts';

function job(id: string, type: string, parentId?: string, extra: Partial<JobSummary> = {}): JobSummary {
  return {
    id,
    parentId,
    type,
    queue: 'pennywise',
    status: 'succeeded',
    attempt: 1,
    maxAttempts: 3,
    runAt: '2026-11-08T00:00:00Z',
    createdAt: '2026-11-08T00:00:00Z',
    ...extra
  };
}

function row(jobs: JobSummary[]): JobRow {
  const tree: JobTree = { rootId: jobs[0].id, jobs };
  const built = buildRows(tree);
  assert.ok(built);
  return built;
}

await test('Learn only appears once the message has a learn job', () => {
  const stopped = row([job('a', 'message.received'), job('b', 'pipeline.report', 'a', { label: 'not_transaction' })]);
  assert.ok(!messageSteps(stopped).some((step) => step.label === 'Learn'));

  const learned = row([
    job('a', 'message.received'),
    job('b', 'transaction.create', 'a'),
    job('c', 'pipeline.report', 'b', { label: 'transaction_created' }),
    job('d', 'transaction.learn', 'c')
  ]);
  assert.equal(messageSteps(learned).at(-1)?.label, 'Learn');
  assert.equal(emailGroup(learned), 'created');
});

await test('a duplicate skips Create and is not counted as created', () => {
  const duplicate = row([
    job('a', 'message.received'),
    job('b', 'transaction.create', 'a'),
    job('c', 'pipeline.report', 'b', { label: 'transaction_duplicate' })
  ]);
  assert.equal(messageSteps(duplicate).find((step) => step.label === 'Create')?.state, 'skipped');
  assert.equal(emailGroup(duplicate), 'skipped');
});

await test('a dead job needs attention', () => {
  const dead = row([job('a', 'message.received', undefined, { status: 'dead' })]);
  assert.equal(emailGroup(dead), 'attention');
});

function message(id: string, createdAt: string, source = 'sms'): JobRun {
  const run = buildRun({ rootId: id, jobs: [job(id, 'message.received', undefined, { source, createdAt })] });
  assert.ok(run);
  return run;
}

await test('messages from one source fold into one row per day', () => {
  // Newest first, like the API.
  const runs = [
    message('c', '2026-11-08T12:00:00'),
    message('b', '2026-11-08T09:00:00'),
    message('e', '2026-11-08T08:00:00', 'api'),
    message('a', '2026-11-07T12:00:00')
  ];
  const merged = mergeBySource(runs);
  assert.equal(merged.length, 3);
  assert.equal(merged[0].emailCount, 2);
  assert.equal(merged[0].id, 'b');
  assert.equal(merged[0].startedAt, '2026-11-08T09:00:00');
  assert.equal(merged[1].startedAt, undefined);

  const days = splitByDay(merged);
  assert.deepEqual(days.map((day) => day.runs.length), [2, 1]);
});

finish();
