# react-frontend

The main Pennywise web app: React 19 + TypeScript (strict) + Vite, Redux Toolkit for state, HeroUI components, Tailwind CSS v4, Phosphor icons, Recharts.

## Run

```bash
npm install
npm run dev       # Vite dev server on 5173
npm run build     # tsc -b && vite build
npm run lint      # eslint
```

CI runs `npm run build` (which type-checks), `npm run lint`, and Playwright browser tests. From the repository root, `make dev` starts a credential-free demo and `make smoke` runs the browser suite against a fresh disposable database. For an existing demo, run `npm run test:e2e` with optional `E2E_BASE_URL`. See [Development workflow](../docs/development.md) for setup, isolation, and failure artifacts.

## Environment

`.env.local` / `.env.production` (see `.env.example`):

- `VITE_API_URL` — Go API base URL (e.g. `http://localhost:5151/api`)
- `VITE_GOOGLE_CLIENT_ID` — Google OAuth client for login
- `VITE_DEMO_MODE=true` — shows the "Try Demo" button on the login page (requires `DEMO_MODE=true` on the API)

`npm run dev` binds to `127.0.0.1:5173` and fails if that port is already occupied. If you also run the Docker demo, start it with `DEMO_WEB_PORT=5174 make dev` and open `http://127.0.0.1:5174`. The demo intentionally sets an empty Google client ID; Google sign-in appears on the directly running frontend when `VITE_GOOGLE_CLIENT_ID` is configured. Restart Vite after changing environment values.

## Structure & conventions

- **Feature folders**: `src/features/<feature>/{components,hooks,store,types}` (transactions, budget, payees, categories, accounts, settings, auth, …)
- **State**: Redux Toolkit slices per feature; use typed `useAppDispatch`/`useAppSelector` from `src/app/hooks.ts`
- **API**: `apiClient` singleton (`src/utils/api.ts`) auto-injects the `Authorization` bearer token (except public auth endpoints) and `x-budget-id` from the selected budget; 401s are retried once via `POST /auth/refresh`
- **Auth**: Google auth-code flow or demo login; session persists in `localStorage` (`pennywise_auth`); routes guarded by `features/auth/components/ProtectedRoute.tsx`
- **Demo account**: `selectIsDemoUser` (auth slice) disables AI config editing and budget creation for `demo@pennywise.local`
- **Websockets**: `features/websocket/WebSocketProvider.tsx` connects to the API's budget-scoped hub for agent streaming events
