# Job queue

How Pennywise runs background work without Temporal: a Postgres-backed job
queue behind a port, a shared worker runtime, and the ingest flow built on
top of it. This document records what exists, the decisions behind it, what
we learned building it, and what is left.

Status as of 2026-11-04: the job queue, history and ingest's inbound
side work end to end on Postgres. The mailbox worker currently only *lists*
new mail; the full sync, the message pipeline's logic and several runner
fixes are still to do (see [Next steps](#next-steps)).

---

## Contents

1. [Why](#why)
2. [The picture](#the-picture)
3. [Building blocks](#building-blocks)
   - [The job port](#the-job-port)
   - [Adapters](#adapters)
   - [ObservedBroker: job history](#observedbroker-job-history)
   - [Factory: ConnectJob](#factory-connectjob)
   - [The worker runtime (jobrunner)](#the-worker-runtime-jobrunner)
   - [Schema](#schema)
   - [Queues and routing](#queues-and-routing)
   - [Request metadata on jobs](#request-metadata-on-jobs)
4. [Job lifecycle](#job-lifecycle)
5. [Guarantees and how they're kept](#guarantees-and-how-theyre-kept)
6. [Ingest on top of the queue](#ingest-on-top-of-the-queue)
   - [The message pipeline](#the-message-pipeline)
7. [Decisions](#decisions)
8. [Learnings](#learnings)
9. [Current state](#current-state)
10. [Next steps](#next-steps)
11. [Working with it locally](#working-with-it-locally)

---

## Why

The email pipeline ran on Temporal (`EmailToTransactionWorkflow`, activity
workers in go-gmail and the API, a separate `workflows` service). Temporal
gave us durable retries, but its server and workers used more memory than a
personal budgeting app on Railway should, and every step of the pipeline was
shaped around workflow and activity boundaries.

What we actually need from it is small:

- run a unit of work later, on whichever service handles it;
- retry it with backoff when it fails, and give up after N attempts;
- never run the same unit of work twice by accident (Gmail pushes and
  Pub/Sub both deliver at least once);
- see what happened to a job afterwards.

Postgres already holds everything else, so it can orchestrate retries too:
a `jobs` table, `SELECT … FOR UPDATE SKIP LOCKED` to claim work, and an event
log for history. The design keeps the broker behind a port so the storage can
change later (Redis, SQS, RabbitMQ) without touching the services.

---

## The picture

```
                       producers                                    consumers
                 ┌───────────────────────┐                   ┌──────────────────────┐
  Gmail push ──► │ ingest: Pub/Sub recv. │──mailbox.changed─►│ ingest worker        │
                 └───────────────────────┘     (ingest q.)   │  FetchSince ─────────┼──┐
  SMS / API  ──► ┌───────────────────────┐                   └──────────────────────┘  │
                 │ ingest: HTTP handler  │──┐                                          │
                 └───────────────────────┘  │   message.received (one per budget)      │
                                            ├──────────────(cipher queue)◄─────────────┘
                                            ▼
                                     ┌──────────────┐        ┌──────────────────────┐
                                     │  jobs table  │◄──────►│ cipher worker        │
                                     │ (Postgres)   │ claim  │  classify (to build) │
                                     └──────┬───────┘        └──────────────────────┘
                                            │ every state change
                                            ▼
                                     ┌──────────────┐
                                     │  job_events  │  history (ObservedBroker → JobLog)
                                     └──────────────┘
```

Every service talks to the queue only through `job.Broker`. Which storage is
behind it is a config choice (`JOB_DRIVER`).

---

## Building blocks

### The job port

`backend/shared/ports/job/` defines the contracts. No driver types appear
here.

**`Broker`**: moving jobs through their states.

| Method | What it does |
|---|---|
| `Enqueue(ctx, type, payload, opts...)` | Create a job. With an idempotency key that already exists for that type, returns the existing job with `Duplicate: true`. |
| `ClaimJob(ctx, queue, workerID)` | Take the next due job from one queue, highest priority then oldest. Never waits: with nothing ready it returns `JOBS_UNAVAILABLE`. |
| `Heartbeat(ctx, claim)` | "Still working on it", so the reaper doesn't take it back. |
| `UpdateProgress(ctx, claim, progress)` | Store partial progress (JSON) and heartbeat. |
| `MarkCompleted(ctx, claim, result, duration)` | Success; store the result. |
| `MarkPending(ctx, job, claim, lastError, runAt)` | Failed attempt; run again at `runAt`. |
| `MarkDead(ctx, claim, lastError, reason)` | Give up. |
| `RequeueStaleRunning(ctx, threshold, reaperID)` | The reaper: running jobs whose heartbeat is older than `threshold` go back to queued, or to dead when out of attempts. Returns a count. |

**`Claim`** (`JobID`, `WorkerID`, `Attempt`) is proof of ownership. Every
write after the claim includes it, and the adapter only updates the row while
`locked_by`, `attempt` and `status = 'running'` all still match. If not, the
write fails with `JOB_CLAIM_LOST`: the reaper took the job back, someone
cancelled it, or a newer attempt holds it. The worker must stop and not retry
the write.

**`JobStore`**: reading and admin actions (`GetJobByID`, `ListJobs`,
`ListByRoot`, `CancelJob`, `RetryDead`). Built and available, not yet used by
any service.

**`JobLog`**: `Append(ctx, JobEvent)`, the history sink.

**Enqueue options** are functional options: `WithBudgetID`,
`WithIdempotencyKey`, `WithQueue`, `WithRootJobID`, `WithParentID`.
`ResolveEnqueueOptions` fills defaults (3 attempts, run now) and resolves the
queue (see [Queues and routing](#queues-and-routing)).

### Adapters

**Postgres** (`backend/shared/adapters/postgres/job.go`): the real one.

- `Enqueue`: `INSERT … ON CONFLICT DO NOTHING RETURNING id`. Without a
  conflict target, `DO NOTHING` covers both unique indexes (idempotency key and
  schedule tick). No row back means a duplicate; the existing ID is then looked
  up.
- `ClaimJob`: one statement, so the claim is atomic:
  ```sql
  UPDATE jobs SET status = 'running', attempt = attempt + 1,
         locked_by = $1, locked_at = now(), heartbeat_at = now(), …
  WHERE id = (
      SELECT id FROM jobs
      WHERE queue = $2 AND status = 'queued' AND deleted_at IS NULL AND run_at <= now()
      ORDER BY priority DESC, run_at
      LIMIT 1
      FOR UPDATE SKIP LOCKED)
  RETURNING …
  ```
  `SKIP LOCKED` makes a second worker skip a row the first is claiming instead
  of waiting on it, so two workers never get the same job.
- `Mark*`, `Heartbeat`, `UpdateProgress`: one `UPDATE … WHERE id AND
  locked_by AND attempt AND status = 'running'`; zero rows means
  `JOB_CLAIM_LOST`.
- `RequeueStaleRunning`: one `UPDATE` with a `CASE` (out of attempts → dead,
  else queued), using the database clock so app servers with skewed clocks
  agree.
- `locked_by`, `locked_at`, `heartbeat_at` are never cleared: they describe
  the last attempt, and ownership checks also require `status = 'running'`.
- The Postgres `JobLog` (`NewPgJobLogAdapter`) inserts into `job_events`.
  `JobStore.CancelJob` and `RetryDead` write their event in the same
  transaction as the change.

**Memory** (`backend/shared/adapters/memory/`): a Go map guarded by a mutex,
for tests and for learning how the port behaves. The memory `JobLog` prints
events to the console. `MarkCompleted`, `MarkPending`, `Heartbeat`,
`UpdateProgress` and `RequeueStaleRunning` are still unimplemented, and there
is no memory `JobStore`. A memory queue lives inside one process, so it can
never connect two services (see [Learnings](#learnings)).

### ObservedBroker: job history

`job.ObservedBroker` (`backend/shared/ports/job/observed.go`) is a decorator:
it wraps any `Broker`, is itself a `Broker`, and after each call that changed
state it appends a `JobEvent` through a `JobLog`:

| Call | Event |
|---|---|
| `Enqueue` (new job only) | `enqueued` |
| `ClaimJob` | `claimed` |
| `MarkCompleted` | `completed` |
| `MarkPending` | `retry_scheduled` (with error and next run time) |
| `MarkDead` | `dead` (with reason) |

`Heartbeat`, `UpdateProgress`, duplicates and failed calls record nothing,
because nothing changed.

The history write is **best-effort**: it happens after the broker call,
possibly in another database, on a context that keeps the caller's values but
not its cancellation (`context.WithoutCancel` plus a 5 s timeout), so history
is still written during shutdown. If the append fails, the state change
stands and the failure is logged; returning it would make the runner retry a
job that actually completed.

Putting history in a decorator, not in each adapter, means a future Redis
broker gets the same history for free, written to Postgres.

### Factory: ConnectJob

`factory.ConnectJob(ctx, JobConfig)` (`backend/shared/factory/job.go`) is
the only code outside `adapters/` that names a job adapter:

| `JOB_DRIVER` | Broker | History | Store |
|---|---|---|---|
| `postgres` | Postgres broker on `JOB_BROKER_URL`, wrapped in `ObservedBroker` | Postgres `JobLog` on `JOB_STORE_URL` | Postgres `JobStore` on `JOB_STORE_URL` |
| `memory` | memory broker, wrapped in `ObservedBroker` | console | none (`nil`) |
| unset / `none` | none: `ConnectJob` returns `nil` | | |

`acquirePool` shares one pool when both URLs (or the main `DATABASE_URL`) are
the same, so jobs can join the application's transactions (see the outbox
discussion under [Next steps](#next-steps)).

### The worker runtime (jobrunner)

`backend/shared/jobrunner/runner.go` turns the port into running workers.

```go
w, _ := jobrunner.NewWorker(broker,
    jobrunner.WithQueue(model.QueueIngest),   // required
    jobrunner.WithConcurrency(1),
    jobrunner.WithPrefix("ingest-worker"))
w.RegisterJobHandler(model.JobTypeMailboxChanged, handler)
go w.Run(ctx)   // returns after ctx is cancelled and in-flight jobs finish
```

- **Slots:** `concurrency` goroutines, each looping claim → execute. Worker
  IDs are `<prefix>-<n>` and become `locked_by`.
- **Handlers:** `func(ctx, *Job) (json.RawMessage, error)`. A panic is
  recovered and treated as a failure.
- **Outcome:** success → `MarkCompleted` with the handler's result and the
  attempt's duration. Error → retry with backoff, or dead once
  `attempt >= MaxAttempts`. No handler for the type → dead. A lost claim while
  persisting is logged and ignored.
- **Backoff:** exponential with full jitter:
  `wait = random(0, min(maxBackoff, baseBackoff × 2^(attempt−1)))`, defaults
  1 s and 5 min. Jitter spreads retries of jobs that failed together.
- **Shutdown:** `Run` waits for `ctx` (or `Stop`), then gives in-flight jobs
  up to `shutdownTimeout` (default 30 s).

Services wrap it in a `worker.Setup`: cipher in
`backend/cipher/internal/worker`, ingest in `backend/ingest/internal/worker`.
Ingest's `Setup` takes the broker `main` already opened and returns a channel
that closes when the worker has stopped; `main` waits on it before closing
the database.

### Schema

Migrations in `backend/shared/migrations/sql`:

**`00015_jobs.sql`**: types `job_status` (`queued`, `running`, `cancelled`,
`succeeded`, `dead`) and `job_event_type`, and three tables:

- **`jobs`**: one row per unit of work. Notable columns: `type`, `queue`
  (00016), `payload` JSONB, `status`, `priority` (higher first),
  `max_attempts`/`attempt`, `idempotency_key`, `metadata` JSONB (see below),
  `budget_id`, `root_job_id`/`parent_id` (job chains), `schedule_id`/
  `scheduled_for` (cron ticks), `progress`/`result` JSONB, `run_at`,
  `locked_by`/`locked_at`/`heartbeat_at`, `last_error`.
- **`job_events`**: append-only history: job, event type, status, attempt,
  actor (`ingest`, `cipher-worker-1`, `reaper`, `admin`), error, reason,
  duration, next run time.
- **`job_schedules`**: recurring jobs (cron, timezone, queue, next run). Run by
  `jobrunner.RunScheduler`, which ingest starts. Its one user today is mailbox
  watch renewal.

Indexes, each for one query:

| Index | Serves |
|---|---|
| unique `(type, idempotency_key)` where key is set | Enqueue dedupe |
| unique `(schedule_id, scheduled_for)` where scheduled | a cron tick fires once, even with two schedulers |
| `(queue, priority DESC, run_at)` where `status = 'queued'` | ClaimJob. Partial, so it only holds waiting jobs and stays tiny however much history piles up |
| `(heartbeat_at)` where `status = 'running'` | the reaper |
| `(root_job_id)` | `ListByRoot`, and `ON DELETE SET NULL` without a table scan |
| `(budget_id, created_at DESC)` | per-budget listing, and the budget foreign key |

**`00016_job_queues.sql`**: adds `jobs.queue` (existing rows, test data, go
to `unrouted`), makes it `NOT NULL`, changes the idempotency index to
`(queue, type, idempotency_key)`, and rebuilds the claim index with `queue`
first.

### Queues

Every job lives on one named queue. This stops a worker from claiming another
service's job, which used to happen and **killed** the job, because a worker
dead-letters any type it has no handler for.

There is no routing table. Every `Enqueue` names its queue with
`job.WithQueue`, and `ResolveEnqueueOptions` refuses a job without one. A
worker claims from a list of queues (`ClaimJob(ctx, queues, workerID)`,
`queue = ANY($2)`), e.g. `cipher` and `cipher-backfill`.

Fan-out (one job type to several queues) is one `Enqueue` per queue, one row
each, because a row has a single status and claim. Dedupe is per
`(queue, type, idempotency_key)`, so each queue keeps its own copy.

`backend/shared/model/jobs.go` holds the queue names (`QueueCipher`,
`QueueIngest`) and every job type, with who queues it and its payload.

| Job type | Queue | Produced by | Payload |
|---|---|---|---|
| `message.received` | `cipher` | ingest (HTTP API, mailbox sync) | `model.ReceivedMessage` |
| `mailbox.changed` | `ingest` | ingest Pub/Sub receiver | `model.MailboxChanged` |
| `mailbox.backfill` | `ingest` | ingest `POST /api/ingest/connections/:id/backfill` | `model.MailboxBackfill` |
| `mailbox.watch.start` / `.stop` | `ingest` | not yet | not yet |
| `email.extract` / `.lookup` / `.predict` | `cipher` | not yet (placeholder handlers) | not yet |

### Request metadata on jobs

A job carries `job.Metadata`: the correlation ID and origin service of the
request that enqueued it, captured by `job.MetadataFromContext` and stored in
`jobs.metadata`. A worker restores it with `Metadata.Context(ctx)` so its logs
and outbound calls share the enqueuer's correlation ID. These are the same two
fields the Temporal propagator carries.

It deliberately never holds the user, the budget or the "verified internal"
flag. Anything that can write a `jobs` row could otherwise make its job run
as a trusted service or as a user. The budget has its own column.

---

## Job lifecycle

```
             Enqueue                ClaimJob
   (none) ──────────► queued ───────────────────► running
                        ▲  ▲                        │
                        │  │ MarkPending            │ MarkCompleted
                        │  └──── (retry at run_at) ─┤──────────────────► succeeded
                        │                           │
    RetryDead (attempt=0)                           │ MarkDead / attempts used up
                        │                           ▼
                        └───────────────────────── dead
                                                    ▲
             RequeueStaleRunning: running with a stale heartbeat
               → queued, or → dead when attempt >= max_attempts

   queued ── CancelJob ──► cancelled        (a running job can't be cancelled
                                             from the database)
```

`attempt` counts claims. A job enqueued with the default 3 attempts runs at
most 3 times; the third failure marks it dead.

---

## Guarantees and how they're kept

| Guarantee | Mechanism |
|---|---|
| Two workers never run the same attempt | `FOR UPDATE SKIP LOCKED` in the single-statement claim |
| A stale worker can't overwrite a newer attempt | every write matches `locked_by` + `attempt` + `status = 'running'` → `JOB_CLAIM_LOST` |
| A redelivered push or retried request doesn't create a second job | idempotency keys; unique index on `(type, idempotency_key)` |
| A cron tick fires once | unique index on `(schedule_id, scheduled_for)` |
| A worker only sees work it can handle | queues; `ClaimJob` filters by queue |
| A crashed worker's job isn't lost | heartbeat + reaper (built, **not yet scheduled**) |
| History matches reality | best-effort: written after the change; failures logged, not returned |

Delivery is **at least once**. A handler can run twice for the same job (a
crash after the work but before `MarkCompleted`, a reaped job that was in
fact still running), so handlers must be idempotent. For ingest that holds
naturally: re-fetching the same emails enqueues jobs whose keys already
exist.

---

## Ingest on top of the queue

The ingest service (`backend/ingest`) receives messages that might be bank
transactions and queues them for cipher. It is the first real user of the
queue.

### Sources

- **Email (Gmail): notify, then fetch.** A push only says "mailbox X changed,
  history ID N". Turning that into emails takes a fetch with the stored
  credentials and cursor, so it becomes a `mailbox.changed` job.
- **SMS and API: the message is in the request.** No fetch is needed, so they
  go straight to `IngestService.Accept`.

```
Gmail push ──► mailbox.changed ──► FetchSince ──┐
                (email only)                    │
SMS (phone app) ────────────────────────────────┼──► message.received ──► cipher
API call ───────────────────────────────────────┘     (every source)
```

### Pieces

- **`IngestService.Accept`** (`internal/service/ingest.go`): validates an
  `InboundMessage` and enqueues one `message.received` job **per budget**,
  keyed `budget:source:provider:externalID`. The budget is part of the key
  because the index enforces uniqueness across all budgets.
- **HTTP API** (`POST /api/ingest/messages`): API key (`write` scope) or JWT,
  budget from `X-Budget-ID`. Clients send an `externalId`, or a `receivedAt`
  from which a content hash is derived, so retries are safe. 256 KB cap.
- **Pub/Sub receiver** (`internal/pubsub`, moved from go-gmail): each push
  becomes a `mailbox.changed` job keyed `gmail:account:historyId`, then is
  acked. Malformed pushes are acked (redelivery can't fix them); enqueue
  failures are nacked for redelivery. Off unless `SUB_NAME` is set.
- **Worker** (`internal/worker`): handles `mailbox.changed`. **Currently list
  only**: `MailConnectionRepository.GetByAccount` →
  `MailProvider.FetchSince`, with the list as the job's result; nothing is
  queued and the cursor isn't saved.
- **`MailboxService.Sync`** (`internal/service/mailbox.go`): the full flow,
  written and unit tested, not yet wired: find the connection, skip if
  disabled or out of budgets, keep only budgets that still exist,
  `FetchSince`, `Accept` each message, **then** save the cursor. Problems a
  retry can't fix (no connection, token refused, cursor expired) complete as
  `skipped` and are recorded on the connection, because the runner retries
  every error.

### Mail connections

A mailbox Pennywise may read is a row in `mail_connections` (migration
`00014`), separate from login identity (`google_provider_users`). Model
`model.MailConnection`, port `repository.MailConnectionRepository`, Postgres
adapter in shared, available as `database.MailConnection` from
`factory.ConnectDB`.

- `budget_ids UUID[]`: one mailbox feeds every listed budget; empty means
  disabled. No foreign key (arrays can't have one, and budgets are
  soft-deleted), so readers skip budgets that no longer exist.
- `credentials BYTEA`: sealed by the provider adapter (AES-256-GCM via
  `utils.SecretBox`, key `CREDENTIALS_KEY`); opaque everywhere else.
- `sync_cursor`: the provider's bookmark (Gmail history ID), opaque outside
  the adapter.
- `watch_expires_at`, `status` (`active`, `needs_reconnect`, `error`),
  `last_sync_at`, `last_error`.

### The Gmail adapter

`MailProvider` (`internal/ports`) has `Watch`, `StopWatch`, `FetchSince`
and `Search`. The Gmail adapter (`internal/adapters/gmail`) is go-gmail's
code moved behind that port, with fixes:

- reads every page of history (go-gmail read only the first);
- a failed message fetch fails the whole fetch and returns no cursor
  (go-gmail returned nothing and no error, so emails were silently lost);
- finds HTML at any depth and falls back to plain text (go-gmail missed
  single-part and nested emails);
- passes `ctx` to every Gmail call;
- `MAIL_AUTH_FAILED` when Google refuses the token (the user must
  reconnect), `MAIL_CURSOR_EXPIRED` when history is older than Gmail keeps.

Adapters are built by `internal/factory.NewMailProviders` from
`MAIL_PROVIDERS`, and picked per connection through a registry keyed by
`Connection.Provider`.

---

### The message pipeline

Every `message.received` job walks a chain of jobs, each queued by the one
before it and carrying what the steps so far found. Each step is one job, so
a failed LLM call retries that step and not the ones already paid for.

| Job | Queue | Port or service | Next |
| --- | --- | --- | --- |
| `message.received` | `cipher` | `CheckTransaction` | `message.extract`, or report `not_transaction` |
| `message.extract` | `cipher` | `Extract` | `message.verify` |
| `message.verify` | `cipher` | `Verifier` | `message.lookup`, or report `unverified` |
| `message.lookup` | `cipher` | `MemoryLookup` | `transaction.create` on a hit, `message.predict` on a miss |
| `message.predict` | `cipher` | `Predictor` | `transaction.create`, or report `no_prediction` |
| `transaction.create` | `pennywise` | `MessageJobService` | report `transaction_created` |
| `pipeline.report` | `pennywise` | `MessageJobService` | none |

Payloads are in `shared/model/messagePipeline.go`. Every job uses the
message's idempotency key, and the dedupe index includes the job type, so a
retried step gets back the job it already queued.

Not written yet: only `CheckTransaction` has real adapters. `Extract`,
`Verifier`, `MemoryLookup` and `Predictor` use the stand-ins in
`cipher/internal/adapters/unimplemented`, and `MessageJobService.CreateTransaction` is stubbed.
These return `NOT_IMPLEMENTED`, so a job fails, retries and goes dead instead
of finishing as if it had done the work. A job that dies is not reported: the
last step of the chain never runs, so `pipeline.report` doesn't see it.

The whole chain needs one shared job store (`JOB_DRIVER=postgres`): the memory
broker lives inside one process.

### The Jobs screen

Settings → Jobs shows the budget's jobs, served by `/api/jobs`. Each top-level
job is a tree: a mailbox sync with one row per message, or a lone message. A
chain of single children stays on one row of steps, and only forks indent.
Clicking a step shows its attempts, errors, input and output.

"Failing now" groups dead and retrying jobs by the error code at the start of
`last_error`, so one cause repeated many times reads as one line.

Limits: an attempt that dies without saving a result shows as "No result
recorded", and the reaper records no per-job event.

## Decisions

| # | Decision | Why | Alternatives considered |
|---|---|---|---|
| 1 | Postgres orchestrates retries; Temporal goes | memory cost; Postgres already holds everything | keep Temporal; Redis-only queue |
| 2 | Broker behind a port; Postgres first, Redis/SQS/RabbitMQ later | storage can change without touching services | call Postgres directly |
| 3 | `jobs` doubles as the outbox | a job enqueued in the same transaction as a business write commits with it | separate outbox table plus relay now (deferred, see next steps) |
| 4 | History in a decorator (`ObservedBroker`) over a `JobLog` port | any broker adapter gets the same history; adapters stay free of logging | events written inside each adapter's transaction (atomic, but tied to Postgres) |
| 5 | History is best-effort | returning an append failure would retry a job that completed | fail the call |
| 6 | `ClaimJob` never waits; the runner polls | waiting and polling are runner policy, not storage | sleep or ticker inside the adapter (rejected: slowed real work, ignored shutdown, would be copied into every adapter) |
| 7 | Claim ownership via `(locked_by, attempt, status)` | a reaped or replaced attempt can't overwrite a newer one | trust the worker |
| 8 | Idempotency key unique per `(queue, type, key)`; ingest keys include the budget | dedupe at the database; the index is global across budgets | in-memory dedupe (go-gmail's maps, lost on restart) |
| 9 | Job metadata = correlation ID + origin service only | stored values must not grant trust | store the whole `RequestMetadata` |
| 10 | Named queues; a worker claims from a list of them | workers stop claiming (and killing) each other's jobs; maps onto real brokers, which are queue-based | filter claims by job type (simpler, but routing tied to type and a poor fit for SQS/RabbitMQ) |
| 11 | No routing: the producer names the queue on every `Enqueue`; fan-out is one enqueue per queue | simplest; no config or table to keep in sync | routes in config (`JOB_ROUTES`), or workers registering their handlers in a `job_routes` table (rejected as too complex) |
| 12 | Unfixable handler outcomes complete as `skipped` | the runner has no "permanent error"; retrying a revoked token is pointless | add permanent errors to the runner (possible later) |
| 13 | Ingest worker concurrency 1 | two syncs of one mailbox race on its cursor | per-mailbox locking |
| 14 | Enqueue first, save the cursor last | a crash in between re-fetches; idempotency keys drop the repeats | save first (loses mail on a crash) |
| 15 | Driving adapters (HTTP, Pub/Sub, worker) have no port of their own; they call service interfaces | ports are for what the core calls and you want to swap | interfaces around receivers |

---

## Learnings

**Concurrency and context**

- A `context.Background()` is never cancelled: a loop waiting on it never
  ends. Shutdown needs a cancellable root (`signal.NotifyContext`) passed down
  to every worker.
- Order in the claim loop matters. Checking `job == nil` before `err` made
  every slot spin at full speed on an empty queue, and on Ctrl+C the
  cancellation error was swallowed by `continue`, so `wg.Wait()` never
  returned.
- An inverted condition (`if !HasCode(err, JOBS_UNAVAILABLE)`) made the
  runner log "claimed job" only when nothing was claimed, and **drop** real
  jobs: claimed, never run, stuck as `running`.
- `<-ctx.Done()` after an unexpected error parks a slot until shutdown. With
  concurrency 1, one database blip stops the worker.
- `defer conn.Close()` in a setup function that starts goroutines closes the
  pool as soon as setup returns, under the running workers.
- `time.Sleep` can't be interrupted; `select` on `ctx.Done()` and
  `time.After` can.
- A persist after the handler (mark completed, dead) must survive shutdown but
  keep the correlation ID: `context.WithoutCancel(jobCtx)` plus a timeout, not
  `context.Background()`.

**Database**

- A partial unique index needs its `WHERE` repeated in `ON CONFLICT`, or
  `ON CONFLICT DO NOTHING` with no target.
- Partial indexes keep the hot query (queued jobs) small no matter how much
  history accumulates.
- An `INSERT` listing 9 columns with 8 placeholders fails every time, silently
  in best-effort history. An integration test caught it.
- `bytea` stores text as its characters: inserting hex without
  `decode(…, 'hex')` stored `b332…` as ASCII, and AES-GCM rejected it ("message
  authentication failed").
- A memory queue is a Go map inside one process. Two services can never share
  it; connecting services needs a real broker (Postgres here).
- goose compares version numbers only. The production database still carries
  the pre-squash history (version 18 in the old numbering; `baseline` only
  accepts the old final version 29), so `up` treats
  the new `00014`/`00015` as already applied or missing. See next steps.

**Gmail**

- The history ID is a per-mailbox counter that only rises. Reading history
  consumes nothing; the bookmark is ours. Any number of readers can read from
  any point, which is why local tests can't affect production.
- A push carries only `{emailAddress, historyId}`; fetching starts from our
  stored cursor, so out-of-order pushes don't matter.
- History is kept for about a week; older cursors return 404 and need a new
  watch plus a backfill search.
- A watch expires after about 7 days and must be renewed. One watch per
  mailbox: watching to another topic redirects production's pushes.
- One Pub/Sub subscription delivers each message to one consumer. Two services
  pulling the same subscription split the pushes between them.

**Design**

- A dependency direction rule settles where code goes: whatever knows *how*
  something arrives (HTTP headers, Pub/Sub messages, job rows) is a driving
  adapter; the port only says *what* happens.
- Real code exposed port gaps abstract design missed: `Watch` must return the
  new cursor and expiry; disabling needs `StopWatch`; `Search` needed fields.
- Every error retrying can't fix needs a non-error outcome, or it burns all
  attempts (a missing mail provider config retried three times).

---

## Current state

| Area | State |
|---|---|
| Job port, Postgres adapter, history, factory | working, integration tested |
| Queues | working; producers name the queue, claims take a list |
| Memory adapter | enqueue and claim only |
| `JobStore` | built, unused |
| Runner | runs, retries with backoff; issues listed below |
| Reaper (`RequeueStaleRunning`) | built, never called |
| Heartbeats | none sent; long jobs could be reaped once the reaper runs |
| `job_schedules` | working; `jobrunner.RunScheduler` checks every minute, started by ingest |
| Ingest HTTP API → `message.received` | working |
| Ingest Pub/Sub receiver → `mailbox.changed` | working, off by default |
| Ingest worker | list only (`FetchSince`), nothing queued |
| `MailboxService.Sync` | written and unit tested, not wired |
| Cipher `message.received` handler | not written (placeholder handlers only) |
| Watch renewal | working; one `mailbox.watch:<connection id>` schedule per mailbox, daily at 03:00 UTC |
| go-gmail / Temporal pipeline | still the production path |

---

## Next steps

Roughly in order.

1. **Fix the runner.**
   - Take a list of queues (`ClaimJob` now takes `[]string`), read from
     `JOB_QUEUES`. Until then `shared/jobrunner` doesn't compile.
   - Wait-and-retry on unexpected claim errors instead of parking the slot.
   - Replace `time.Sleep` polling with a `select` on `ctx.Done()`.
   - Build each job's context from `job.Metadata.Context` (and the job's
     budget) so handler logs carry the enqueuer's correlation ID.
   - Use `context.WithoutCancel(jobCtx)` for persist calls.
   - Optionally support permanent errors so handlers can dead-letter
     immediately.
2. **Send heartbeats and run the reaper.** A ticker in the runner per
   in-flight job; a scheduled `RequeueStaleRunning`. Make it return the
   reaped jobs so the decorator can record `reaped` events.
3. **Wire the real sync.** Switch the ingest worker from list-only to
   `MailboxService.Sync`.
4. **Fill in the message pipeline.** The jobs and handlers exist; the logic
   doesn't yet (see [The message pipeline](#the-message-pipeline)).
5. **Watch lifecycle.** Renewal is done: connecting or resuming a mailbox turns
   on its schedule, pausing or removing it turns it off. `mailbox.watch.stop`
   is still unused.
6. **Connect flow.** "Connect Gmail" in onboarding with its own consent:
   OAuth code → sealed credentials → `MailConnection.Upsert` → `Watch`. Not in
   the `MailProvider` port yet.
7. **Fill in history details.** `duration_ms` on completed events; budget
   scoping for any API over `JobStore` (it currently lists every budget's
   jobs).
8. **Migrate production's migration state.** Reconcile the pre-squash goose
   history on Railway (bring the schema to the squashed set, reset version
   rows, drop the hand-made job tables), then apply `00014`–`00016`. Done once
   locally by hand.
9. **Cut over from Temporal.** Point Gmail's Pub/Sub at ingest (its own
    subscription first, to compare side by side), retire go-gmail's
    `pkg/pubsub` and the `EmailToTransactionWorkflow`, then the `workflows`
    service.
10. **Later: other brokers.** A Redis/SQS adapter with the `jobs` table as the
    outbox and a relay that publishes from it. Enqueues that must commit with a
    business write should then go through an outbox port rather than the
    broker directly.

---

## Working with it locally

**Database and migrations.** The local Postgres is a copy of Railway's, in
container `pennywise-local-pg` (Postgres 18 with pgvector) on `localhost:5432`.

```bash
cd backend/go-pennywise-api
export DATABASE_URL='postgres://pennywise:pennywise@localhost:5432/pennywise?sslmode=disable'
make migrate-status
make migrate-up
```

**Run the services against Postgres jobs.** In each service's `.env`:
`JOB_DRIVER=postgres` and the same `JOB_BROKER_URL` / `JOB_STORE_URL`.
Ingest also needs `MAIL_PROVIDERS=gmail`, `CREDENTIALS_KEY`, and the Google
settings (`GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `PROJECT_ID`,
`PUBSUB_TOPIC`). Leave `SUB_NAME` unset locally: pulling production's
subscription would take pushes away from go-gmail.

**Migrate a Gmail connection.** `cmd/seal` seals a refresh token (stdin) into
hex using the same key ingest opens with. Store it with
`decode('<hex>', 'hex')`, and copy the cursor and watch expiry from
`google_provider_users` so the connection continues where the old pipeline is.

**Trigger the worker by hand:**

```sql
INSERT INTO jobs (type, queue, idempotency_key, payload)
VALUES ('mailbox.changed', 'ingest', 'gmail:you@gmail.com:manual-1',
        '{"provider":"gmail","account":"you@gmail.com","cursor":"manual-1"}');

SELECT status, attempt, last_error, result FROM jobs
WHERE idempotency_key = 'gmail:you@gmail.com:manual-1';
```

Use a new `manual-N` per run. The cursor in the payload is informational;
fetching starts from the connection's stored cursor, which the list-only
worker never moves.

**Inspect history:**

```sql
SELECT type, status, attempt, actor, error, reason, created_at
FROM job_events WHERE job_id = '<id>' ORDER BY id;
```

**Integration tests** need a Postgres role that can create databases:

```bash
docker run -d --rm --name pw-test-pg -e POSTGRES_USER=pw -e POSTGRES_PASSWORD=pw \
  -p 127.0.0.1:55439:5432 pgvector/pgvector:pg17
cd backend/shared
PENNYWISE_TEST_DATABASE_URL='postgres://pw:pw@127.0.0.1:55439/pw?sslmode=disable' \
  go test ./adapters/postgres/
docker rm -f pw-test-pg
```
