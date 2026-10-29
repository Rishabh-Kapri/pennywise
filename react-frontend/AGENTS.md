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
- Current section ids: `general`, `loans` (`&account=<id>`), `recurring`, `documents`, `reconcile` (`&statement=<id>`), `activity`, `review`, `tags`, `ai`.
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
- AI providers (`features/onboarding/components/AIProvidersPanel`, shown in the wizard and Settings → AI) are the user's own keys and the only models cipher uses. Keys are write-only: omit `apiKey` in a PUT to keep the saved one.
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
