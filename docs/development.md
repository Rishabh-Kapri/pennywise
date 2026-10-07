# Agent development workflow

Requires Docker with Compose v2+, Go 1.26, Node 22+, npm, and Bash. Run commands from the repository root.

```sh
make setup       # install locked JS dependencies, Chromium, and Go dependencies
make dev         # build/start the isolated demo; prints its web address
make check       # all five Go modules: vet/tests; React: lint/build
make smoke       # fresh database migrations + real API/browser smoke test
make status
make logs
make stop        # stop demo; retain its database
```

On Linux, Chromium may require OS packages: `cd react-frontend && npx playwright install --with-deps chromium`.

## Demo and isolation

`compose.demo.yml` runs PostgreSQL/pgvector, Redis, API, React, and an integration stub. Login using **Try Demo**; the API seeds synthetic accounts, transactions, and a budget. The stack does not read the root `.env` or need Google, Gmail, Temporal, Ollama, or cloud AI credentials. Integration requests return a deterministic 503 with an explanatory message; this environment tests budgeting, not AI output or email ingestion. Use the existing `docker-compose.yml` and service READMEs for real integrations.

React uses a same-origin `/api` proxy, including websockets. Only the web port is published, on loopback. Source edits hot-reload in React; run `make dev` again after backend edits or dependency changes to rebuild. Demo data survives `make stop`.

Each checkout/worktree gets a Compose project name derived from its absolute path, isolating networks and database volumes. For simultaneous worktrees, choose a free port with `DEMO_WEB_PORT=5174 make dev`, or `DEMO_WEB_PORT=0 make dev` for automatic allocation. `COMPOSE_PROJECT_NAME` can override the project name; use the same override for subsequent commands. `./scripts/dev.sh port web 5173` prints the assigned port.

`make smoke` always creates its own project and fresh database, then deletes only that project's resources. It retains service logs in `artifacts/` and browser failure screenshots/traces in `react-frontend/test-results/`. CI uploads these artifacts. To run browser tests against your existing demo: `E2E_BASE_URL=http://127.0.0.1:5173 npm --prefix react-frontend run test:e2e`.

If Docker's bridge DNS cannot download build dependencies on Linux, `DEMO_BUILD_NETWORK=host make dev` uses host networking during builds only. Normal container networking remains isolated.

## Validation and database tests

`make check-go` tests all active Go modules, including shared and its consumers. `make check-web` runs React lint/build. CI runs the same commands on PRs and pushes to dev/master, plus `make smoke`. Configure repository branch protection to require the Validate jobs; workflow files alone do not enable branch protection.

Legacy shared monthly-budget integration tests are opt-in using `PENNYWISE_TEST_DATABASE_URL`, pointing only at a disposable test database. They create their own simplified table and should not run against a migrated application database. Unit tests require no database. The browser smoke test validates the actual migrated schema and persistence.

Fresh demo databases run every migration normally. Legacy JSON seeds are skipped when their files are absent in the container. Do not use `baseline` to initialize an empty database: that command marks versions 1–6 applied without creating their schema.

## Task and review contract

Use `.github/ISSUE_TEMPLATE/development-task.md` to define observable acceptance criteria, affected services, edge cases, and verification before implementation. Keep changes scoped; preserve unrelated working-tree edits. For API changes, update all affected clients and add request/response coverage. For cross-service changes, test the producing and consuming sides. Budget IDs and verified internal auth must remain enforced.

Before handing work back, report what changed, checks actually run, and any remaining limitations. Use screenshots/traces for UI failures and correlation IDs for backend debugging. Never claim a skipped check passed.

## API contracts

`contracts/core.schema.json` defines the core login, budgets, accounts, and transaction-create response shapes. Playwright validates actual responses with Ajv during `make smoke`. Additive fields are allowed; missing fields and incompatible types fail. Extend this initial contract and its exercised endpoints as features grow; it is not a complete API specification. Update relevant React and Android types together when changing a contract.

Reference: [Playwright test configuration](https://playwright.dev/docs/test-configuration) and [Ajv validation](https://ajv.js.org/guide/getting-started).
