# react-frontend

The main Pennywise web app: React 19 + TypeScript (strict) + Vite, Redux Toolkit for state, HeroUI components, Tailwind CSS v4, Phosphor icons, Recharts.

## Run

```bash
npm install
npm run dev       # Vite dev server on 5173
npm run build     # tsc -b && vite build
npm run lint      # eslint
```

There is currently no automated test suite; `npm run build` (which type-checks) and `npm run lint` are the CI gates.

## Environment

`.env.local` / `.env.production` (see `.env.example`):

- `VITE_API_URL` — Go API base URL (e.g. `http://localhost:5151/api`)
- `VITE_GOOGLE_CLIENT_ID` — Google OAuth client for login
- `VITE_DEMO_MODE=true` — shows the "Try Demo" button on the login page (requires `DEMO_MODE=true` on the API)

## Structure & conventions

- **Feature folders**: `src/features/<feature>/{components,hooks,store,types}` (transactions, budget, payees, categories, accounts, settings, auth, …)
- **State**: Redux Toolkit slices per feature; use typed `useAppDispatch`/`useAppSelector` from `src/app/hooks.ts`
- **API**: `apiClient` singleton (`src/utils/api.ts`) auto-injects the `Authorization` bearer token (except public auth endpoints) and `x-budget-id` from the selected budget; 401s are retried once via `POST /auth/refresh`
- **Auth**: Google auth-code flow or demo login; session persists in `localStorage` (`pennywise_auth`); routes guarded by `features/auth/components/ProtectedRoute.tsx`
- **Demo account**: `selectIsDemoUser` (auth slice) disables AI config editing and budget creation for `demo@pennywise.local`
- **Websockets**: `features/websocket/WebSocketProvider.tsx` connects to the API's budget-scoped hub for agent streaming events

## Credit card statements

Credit card accounts expose Transactions and Statements views on `/accounts` (`view=statements` selects the latter). The statement UI supports PDF selection, size/type validation (20 MB limit), and local preview. Files are not uploaded, persisted, or analysed yet. The explicitly labelled sample analysis uses the planning excerpts, including charges, evidence, rewards, EMI snapshots, and review checkboxes that last only while the view is mounted. Sample data is never presented as extraction from a selected PDF. Backend extraction, password handling, saved corrections, and ledger matching remain planned in [the statement analysis plan](../docs/credit-card-statement-analysis-plan.md).
