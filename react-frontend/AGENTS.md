# AGENTS.md — react-frontend

React 19 + Vite + Redux Toolkit web client for Pennywise. Actively developed (the Angular `frontend/` is legacy).

## Commands
- `npm run dev` — Vite dev server
- `npm run build` — `tsc -b && vite build` (type-check is the test gate)
- `npm run lint` — ESLint (clean; keep it that way)
- `npm run preview` — serve the build
- No test suite. Don't look for `*.test.*` / `*.spec.*`.

## Layout
- `src/app/` — `store.ts` (root reducer + middleware), `App.tsx` (router), `hooks.ts`, `middlewares.ts`
- `src/features/<name>/` — `components/`, `hooks/`, `store/`, `types/` (+ `goals/` in category)
- `src/components/common/` — shared widgets (Modal, Popover, DateSelector, AgentChat, …)
- `src/components/layout/` — `Layout`, `Navbar`, `Dashboard`, `Header`; `Sidebar` is exported but not mounted
- `src/utils/api.ts` — `apiClient` singleton; `src/config/env.ts` — env flags
- `src/styles/` — `index.css` (`@import 'tailwindcss'`), `theme.css`; HeroUI theme in `hero.ts`

## Conventions
- Redux Toolkit slices per feature in `features/*/store/`, wired in `src/app/store.ts`.
- Use `useAppDispatch` / `useAppSelector` from `src/app/hooks.ts`, not the raw react-redux hooks.
- `apiClient` (`src/utils/api.ts`) injects `Authorization` and `x-budget-id` from store state. Don't pass the budget id by hand. Skips the header for endpoints containing `budgets`.
- On 401, `apiClient` refreshes the token once and retries; a failed refresh resets auth and goes to `/login`.
- UI: HeroUI components, `@phosphor-icons/react` icons, Tailwind v4 utilities, CSS Modules (`*.module.css`, the dominant styling pattern), Recharts for charts.
- Path alias `@/` → `src/`.
- Explicit TypeScript interfaces; `strict` is on.

## Navigation
- The top `Navbar` (`src/components/layout/Navbar/Navbar.tsx`, `NAV_ITEMS`) is the only nav chrome. It holds daily destinations only: Home (`/dashboard`), Transactions, Budget, Accounts, Payees, Reports.
- Occasional or configuration screens are Settings sections at `/settings?section=<id>`. Register new ones in `SECTIONS` in `src/features/settings/components/Settings.tsx` with a `group` (account / budget / automation), a description (shown as the section header) and search `keywords`. Set `ownHeader` if the section renders its own title, or it gets two.
- Current section ids: `general`, `loans` (`&account=<id>`), `recurring`, `documents`, `reconcile` (`&statement=<id>`), `mail`, `activity`, `jobs`, `review`, `tags`, `ai`.
- Setup: `features/budget/BudgetOnboarding` creates a budget; then `Layout` shows `features/onboarding/OnboardingWizard` (instead of the app) until `budgets.onboarding.step` is `done`. Demo users never see it. Progress is server-side (`PATCH /onboarding`), so every step writes real data rather than wizard state. `GettingStartedChecklist` on the dashboard ticks items from real data, not from the wizard's record.
- Legacy paths redirect to Settings: `/activity`, `/recurring`, `/review` → `/settings?section=…`; `/loans/:id` → `/settings?section=loans&account=<id>`.

## Gotchas
- Receipt/document bodies need auth, and `<img src>` can't send headers. Fetch with `apiClient.fetchBlobUrl('documents/<id>/content')` and use the returned object URL.
- Category goal progress is computed client-side in `src/features/category/goals/goalProgress.ts` from the month's budgeted/activity/balance. The API stores only the goal definition.
- Demo mode: `VITE_DEMO_MODE=true` shows the "Try Demo" login (needs `DEMO_MODE=true` on the API). `selectIsDemoUser` (`features/auth/store`) gates budget creation.
- `VITE_API_URL` sets the API base; the fallback in `src/config/env.ts` is `http://localhost:5151/api`.
- `features/reconcile/` is the real statement reconciliation (backed by `/api/statements`). `features/statements/` is a separate credit-card analysis preview that uses only sample data. It is not wired to the API (`docs/credit-card-statement-analysis-plan.md`).
- Lazy-load heavy routes (see `App.tsx`); settings sections are split the same way.

- Account logos: `accounts.bank` holds a logo id from `features/accounts/bankLogos.ts`; accounts without one fall back to matching the name. Credit cards render as an outlined card, other accounts as the bare logo (`BankLogo`). Loan accounts are created through `LoanAccountForm`, the rest through `AccountFormModal` (new and edit; `PATCH /accounts/:id` changes name, bank, suffix and closed, never the type). Account suffix = last digits; the API rejects one already used in the budget.
- Transaction pickers (`features/transactions/components/popovers`): payee/account single-select go through `SearchSelect`, dates through `MiniCalendar`, all on the common `Popover`. Don't bring back HeroUI Autocomplete/Calendar there: Calendar took 100ms+ to open, and HeroUI's layered `!important` outlines fight our CSS Modules.
- Payee rules (`features/payees`): match strings are unique per budget and the API upserts on them, so POSTing an existing string under another payee *moves* the rule. A rule with no category is skipped by cipher. `payeeUtils.testRules` mirrors the server's EXACT-then-PATTERN (ILIKE) lookup; keep the two in step.
- Dashboard figures come from `features/dashboard/store` (its own 6-month transaction window plus `reports/income-expense`), never the shared `transactions` slice: app start, budget switch, the websocket and the Transactions page reload that slice with a 30-row or filtered page. `reports/income-expense` counts opening balances as income (payee `metadata.startingBalPayeeId`); subtract them wherever "income" means earnings.
- Reports (`features/reports`): the Overview/Spending/Income/Investing tabs share one `reports/cashflow` response, classified in `cashFlow.ts`. Any transfer between an on-budget account and an asset/liability/loan account counts as invested/withdrawn/debt, never income or spending, whatever category its on-budget leg carries (users filed fund sales both to Ready to Assign and straight to categories). Group treatments (spending / one-off / saving) are guessed from the group name and overridden per device in localStorage. Loan balances come from `GET /reports/loans` (amortized server-side from `loan_metadata` and the payments booked to the loan account: each payment clears accrued interest first). Net worth and `LoanOverview` both use it; don't go back to original-minus-payments, which counts interest as principal.
- AI setup (`features/onboarding/components/AIProvidersPanel`, shown in the wizard and Settings → AI) is the user's own keys (one per provider) and the models run with them (chat or decision, one default per kind); cipher uses nothing else. Keys are write-only: omit `apiKey` in a PUT to keep the saved one. Models are tested by the server before they're saved, so a failed create is a 422 shown in the form. "Has AI" means a chat model on an enabled key (`usableModels`, `fetchHasChatModel`), not just a saved key. Provider labels live in `features/onboarding/aiProviders.ts`.
- Jobs (`features/jobs`, Settings → Jobs): one row per email, not per job. Rows needing attention act from `RowActions`: Retry a dead job, Run again a stopped message (a new run replaces the stopped one, keyed by `ReceivedMessage.RunID`), Create by hand (the add-transaction form via router state, which dismisses the row once saved) and Dismiss (soft-deletes the row's jobs, keeping their idempotency keys). `jobTree.ts` turns a flat job list into chains (`buildRows`), then runs (`buildRun`) with progress; `emailGroup` decides what shows and what folds away. `index.css` resets margin and padding at one class's specificity, so Jobs CSS rules that set either need the `.page` prefix. A job with one child stays on its parent's row; a new row starts at a fork, or at a `message.received` under a mailbox job. The email's outcome comes from its `pipeline.report` label. Attempts come from `job_events` split on `claimed`, not the attempt number, because a manual retry restarts the numbers. The screen has no polling: it loads on open and on the Refresh button, until jobs get websocket events. Search and the Show filter are client-side, on the loaded page; a pasted job ID goes to the API.
- Pipelines (`features/jobs/components/PipelineView`, Jobs → Pipelines, `&view=pipeline&pipeline=<id>`): every system pipeline from `GET /pipelines` with the budget's overrides; each pipeline has its own steps, so never hardcode step IDs. Only changed fields are sent (`PUT /pipelines/:pipeline/steps/:step`, null = follow the default); `DELETE` resets a step. The server computes `effective` and `warning`; don't recompute them client-side.
- Email sync (`features/mail/useMailConnections.ts`, used by `MailSettings` and onboarding's `EmailStep`, web only): a separate Google consent for `gmail.readonly`, independent of login. The code goes to `POST /mail-connections`; the API forwards it to ingest. It passes `prompt: 'consent'` outside the library's types, because without the consent screen Google sends no refresh token. `EmailBackfill` imports the last 30 days through `/mail-connections/:id/backfill`: every email with automatic import on, or only the ticked ones. `onboarding.emailsImportedAt` stops step 4 offering it twice; `MailSettings` offers it for any mailbox that still has access.
- Toasts: use `toast` from `@/utils` (styled in `utils/toast.module.css`); give a short title plus a `description`, and use `errorMessage(err, fallback)` for failures instead of putting raw server errors in the title.

## Key files
| Purpose | Path |
|---|---|
| Store | `src/app/store.ts` |
| Router | `src/app/App.tsx` |
| API client | `src/utils/api.ts` |
| Settings sections | `src/features/settings/components/Settings.tsx` |
| Navbar | `src/components/layout/Navbar/Navbar.tsx` |
| Goal progress | `src/features/category/goals/goalProgress.ts` |
| Document library | `src/features/documents/` |
| Auth (ProtectedRoute, slice) | `src/features/auth/` |
| Env flags | `src/config/env.ts` |
