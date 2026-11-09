# Jobs pipeline and AI setup: handover

As of 2026-11-08. Deployment is deliberately left out; it continues separately.

## Where things stand

The jobs pipeline now runs a real message from Check through Create locally, using each budget's own keys, models and step settings. All of it is uncommitted on `wip/jev-integration`: about 250 changed files, which also include separate SMS and ports work.

Five things stand between this and a finished feature: lookup can file an ambiguous merchant wrongly, two checks fail, usage isn't recorded, some parts are untested end to end, and the old Temporal flow still needs retiring.

## What was built

| Area | What exists now |
| --- | --- |
| Keys and models | Keys (one per provider) are split from models. Models are chat, decision or embedding, with one default per kind, disable, and ordering. A model is tested before it's saved. |
| Pipeline config | System pipelines live in JSON files validated by a schema. A budget stores only what it changed: adapter, model and fallback per step. |
| Pipeline runtime | Check, Extract, Verify and Predict pick their adapter per budget and job. Fallback chains try the next model when one fails. |
| Decisions | A provider for OpenRouter's Decisions API, called through an SSRF-safe client at the endpoint the user enters. |
| Lookup | Exact `CHANNEL \| MERCHANT` match against `counterparty_aliases`. A miss goes to Predict. |
| Create | `transaction.create` finds the account, payee and category, then uses the same `CreateDeduped` as the rest of the app. Duplicates report as "Already added". |
| Learning | `transaction.learn` writes the source row and updates the alias. Correcting or approving a transaction makes the alias `MANUAL`. |
| Jobs UI | Step line, real step output, failure filter with a clear selected state. Standalone Learn jobs are hidden. |
| Mail watch renewal | Each mailbox gets a daily job schedule. Connecting or resuming turns it on, pausing or removing turns it off. |
| Android | AI keys and models, a Pipelines editor and a Jobs screen. |
| Embeddings | Per-user models in Settings, 1024 dimensions enforced. The server env chain is gone. |
| Evaluation | `cmd/eval-lookup` compares trigram and embedding matching on real aliases, read-only. |
| Migrations | `00012`–`00019`, merged down from 14 files. The merged set builds the same schema as the separate ones did. |

## How a message flows

Each step is its own job, chained by `root_job_id` and `parent_id`. Check through Predict run on the `cipher` queue; the last three run on `pennywise`.

1. **Ingest** accepts an email or SMS and queues `message.received`.
2. **Check** (default: decisions model) asks whether it's a transaction. No ends the message.
3. **Extract** (chat model) reads account, merchant, amount and date.
4. **Verify** (decision model) checks those details against the text. A blank amount or date fails here without a paid call.
5. **Lookup** finds the merchant's alias. A hit skips Predict.
6. **Predict** (chat model) picks a payee and category from the budget's list.
7. **Create** saves the unapproved transaction and its cipher prediction, deduped by hash.
8. **Report** records the outcome for the Activity page.
9. **Learn** saves the source row and alias. It runs after Report, so a learning failure can't lose the outcome.

For a step that uses a model, cipher builds its attempts with `pipeline.Attempts`: the step's model first, then, with fallback on, the owner's other models of that kind in priority order. `buildStep` turns those into one adapter or a chain, and clients are cached by key row, endpoint, base URL and `KeyUpdatedAt`.

Correcting or approving a transaction queues a standalone `transaction.learn` job, which replaces the alias's payee and category.

## Where the code lives

Paths are under `backend/` unless they start with a frontend folder.

| Purpose | Path |
| --- | --- |
| Pipeline files, schema, `Resolve`, `Attempts` | `shared/pipeline/` |
| Picks and builds each step's adapter | `cipher/internal/factory/selector.go` |
| Fallback chains | `cipher/internal/factory/chain.go` |
| Per-model client cache | `cipher/internal/resolver/resolver.go` |
| Old per-budget chat resolver, model tests | `cipher/internal/userllm/resolver.go` |
| Step adapters | `cipher/internal/adapters/{decisions,llm,alias,regex}` |
| Decisions provider | `cipher/agent/decisions/providers/openrouter.go` |
| Message job handlers | `cipher/internal/worker/handlers.go` |
| Rule-based key parsing | `cipher/internal/narration/` |
| Create and learn | `go-pennywise-api/internal/service/messageJob.go` |
| Create, report and learn handlers | `go-pennywise-api/internal/worker/worker.go` |
| Model and pipeline APIs | `go-pennywise-api/internal/service/{aiModel,pipelineStep}.go` |
| SSRF-safe HTTP client | `shared/transport/safedial.go` |
| Recurring jobs | `shared/jobrunner/scheduler.go` |
| Mailbox connect, pause, watch renewal | `ingest/internal/service/mailbox.go` |
| Migrations | `shared/migrations/sql/` |
| Web AI settings | `react-frontend/src/features/onboarding/components/AIProvidersPanel.tsx` |
| Web Jobs and Pipelines | `react-frontend/src/features/jobs/` |
| Android | `android-frontend/src/features/jobs/`, `android-frontend/src/features/ai/` |
| Design notes for sources and aliases | `docs/transaction-sources.md` |

`android-frontend/src/features/jobs/jobTree.ts` is a copy of the web's file, so a change to one needs making in the other.

## Decisions and why

| Decision | Why |
| --- | --- |
| Budgets store only their overrides | Steps nobody changed keep following the system default as it changes. |
| No server-key fallback for AI | Each user brings their own keys. A missing model fails the job with a clear message. |
| A model picked for a step runs ahead of the default | Otherwise picking one would have no effect. |
| The default always runs first; priority orders the rest | One predictable rule for chains and for `DefaultOf`. |
| A disabled default stays the default | Switching it off briefly doesn't lose the choice. |
| Cache clients per key, not per model | A client is bound to a key and endpoint; the model is a request field. |
| Exact lookup, no fuzzy matching | Fuzzy matching only saves one Predict call per new spelling, and a wrong match becomes a saved alias. |
| Lookup uses the rule-parsed key, not the LLM's merchant | Aliases are stored under that key, and it stays local. |
| Learn is a job, after Report | Failures are retryable and visible, and can't block the outcome. |
| Duplicate means the row existed before this job started | A retry finding its own earlier save still counts as created. |
| Embeddings use only the default model, with no fallback | Vectors from different models can't be compared. |
| Email text uses the old flow's conversion | Lookup keys and dedupe hashes have to match existing data. |

**Embedding evaluation, on 188 aliases.** Trigram is only clean at 0.70, finding 7 of 91. Setting aside the Google Play and MakeMyTrip data problems, `bge-m3` is clean at 0.75 and finds 29, and `qwen3-embedding:0.6b` is clean at 0.90 and finds 27. Qwen confuses unrelated people's names at 0.82–0.86, so `bge-m3` is the better fit. The correct counts are inflated by the Uber payee, which holds many unrelated driver names.

## Gotchas

- **Re-vendor every Go service after changing `backend/shared`.** Builds use `-mod=vendor`, and migrations and pipeline JSON are embedded, so they count too.
- **Two resolvers exist.** `internal/resolver` serves pipeline steps by model row. `internal/userllm` still serves the chat agent, agent memory, the old prediction service and the Settings test button. Don't delete it yet.
- **The job runner retries every error.** With fallback on, two models and three attempts can mean six paid calls when everything is failing.
- **Pipeline file IDs are stored in overrides.** Never rename a pipeline, step or adapter ID, and bump `version` when steps change.
- **Model rows carry the encrypted key.** Never pass one to a logger. Use the helpers in `factory/utils.go`.
- **User-entered URLs must use `transport.NewUserURLClient`,** which refuses private addresses unless `ALLOW_PRIVATE_LLM_URLS` is set.
- **`raw_text` is plain text, not the original HTML,** despite what `docs/transaction-sources.md` says, so that it matches existing rows.
- **Ingest reads `CREDENTIALS_KEY`,** but `docker-compose.yml`, `ingest/AGENTS.md` and two code comments say `MAIL_CREDENTIALS_KEY`.
- **A mailbox connected before schedules existed has no schedule.** Reconnect it, or pause and resume it.
- **Messages sent through the API get no source row,** and a retry can add one extra to that merchant's `hit_count`.

## Tested and not tested

| Area | Status |
| --- | --- |
| Real message from Check to Create, locally | Ran; seen in the Jobs screen |
| Duplicate reported as a duplicate | Unit tests only, since the fix |
| Chain logic, adapters, step selection, create and learn | Unit tests pass |
| Aliases, sources, models, job queries | Database tests pass on a throwaway copy |
| Merged migrations | Same schema as the separate files; AI models migration run up and down with sample data |
| Correcting a transaction, then a new message following it | Not tried through the app |
| A fallback chain with a model really failing | Unit tests only |
| Disable and reorder controls in Settings | Type-checked; not confirmed in a browser |
| Android screens | Type-checked and bundled; not run on a device |

**Failing now:**

- `go-gmail` doesn't compile: `cmd/main.go:274` calls `NewSmsHandler` with too few arguments.
- Four tests in `shared/adapters/postgres` fail: `TestGetByCatIdAndMonth`, `TestCreate`, `TestUpdateBudgetedByCatIdAndMonth`, `TestUpdateCarryoverByCatIdAndMonth`. This session didn't touch that code, and the cause hasn't been looked at.

## Next steps

In suggested order. Deployment is tracked separately.

**Get it clean**

- [ ] Fix the `go-gmail` compile error, and find out why the four `shared/adapters/postgres` tests fail.
- [ ] Split the 250 changed files into reviewable commits, separating the SMS and ports work.
- [ ] Make `docker-compose.yml`, `ingest/AGENTS.md` and the code comments agree on `CREDENTIALS_KEY`.
- [ ] Update the old migration numbers in `docs/job-queue.md`.

**Finish the feature**

- [ ] Add an "ambiguous" flag on aliases, so merchants like Google Play and CRED CLUB always go to Predict. Today every such charge lands on one payee.
- [ ] Correct the `RAZ*MAKEMYTRIP` alias, filed as Redbus.
- [ ] Record model usage from job results. Each step shows it in its output, but `llmusage` doesn't store it.
- [ ] Make `ObservedDecisions` record spans and errors. It's a pass-through today, and its field is misspelled `telelmetry`.
- [ ] Add a non-retryable error to `jobrunner`, following `queue.Permanent`, so "no model set up" stops retrying.
- [ ] Try the untested paths: a correction followed by a new message, a real fallback, and Android on a device.

**Retire the old flow**

- [ ] Decide whether the old embedding learning goes. It fails today unless the embedding model is set up, and only the Temporal flow uses it.
- [ ] Move model testing and `Probe` out of `userllm` and `adapters/decisions`, then delete that adapter's old `client.go` and `model.go`.
- [ ] Decide how the chat agent picks a model, then delete `userllm`.
- [ ] Remove the Temporal variables from `go-gmail` and `workflows` once the flow is gone. The API and cipher no longer connect to Temporal; their activity and workflow code is kept but not wired in.

**Later**

- [ ] Reconciliation against statements: the matching tiers in `docs/transaction-sources.md`.
- [ ] Rule out or build fuzzy matching. If it's wanted, passing the closest known merchants to Predict as hints is the lower-risk route.
- [ ] Add the embedding results to `docs/transaction-sources.md`, and update its "No cloud LLM" line, which no longer matches the pipeline.
- [ ] Live updates on the Android Jobs screen.
