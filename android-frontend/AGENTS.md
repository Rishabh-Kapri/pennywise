# Agent Guidelines for Pennywise Android

## Overview

`android-frontend` is the Expo React Native Android client for Pennywise. It mirrors the active React web frontend at a feature level, but it has Android-specific OAuth, deep-link, and build requirements.

## App identity

- Expo app name: `Pennywise`
- Expo slug: `pennywise-android`
- Android package: `dev.pennywise.cloud`
- App URL scheme: `dev.pennywise.cloud`
- Google AuthSession redirect in installed APKs: `dev.pennywise.cloud:/oauthredirect`

Keep the package and scheme aligned. If the scheme does not match the Google redirect URI, Google login can complete in the browser but fail to return to the app.

## Stack

- Expo SDK 54
- React 19
- React Native 0.81.5
- Redux Toolkit + React Redux
- React Navigation bottom tabs + native stack
- Expo AuthSession + WebBrowser for Google login
- AsyncStorage for local auth persistence
- Lucide React Native for icons

## Project structure

`android-frontend` follows the same feature-first shape as `react-frontend`, adapted for Expo React Native. Keep new code inside the closest existing feature folder unless it is truly shared across screens.

```text
android-frontend/
├── App.tsx                 # thin root export to src/App.tsx
├── index.js                # Expo registerRootComponent entrypoint
├── app.json                # Expo app identity, scheme, Android package, EAS metadata
├── eas.json                # EAS build profiles
├── babel.config.js         # Expo + Reanimated Babel config
├── package.json            # scripts and pinned Expo-compatible native deps
├── tsconfig.json           # strict TypeScript config and @/* alias
├── android/                # generated native Android project; avoid hand edits unless needed
└── src/
    ├── App.tsx             # providers, auth gate, onboarding gate, tabs, API probe
    ├── app/                # Redux store, typed hooks, cross-feature middleware
    ├── components/         # shared native UI primitives
    ├── config/             # environment/runtime config
    ├── features/           # feature-owned screens, slices, types, utilities
    ├── navigation/         # React Navigation param-list types
    ├── theme.ts            # shared color/spacing/radius/type tokens
    └── utils/              # API client, auth helpers, dates, storage, constants
```

Important: `src/app` is a Redux/application folder, not an Expo Router route folder. This app uses React Navigation, not Expo Router. `app.json` sets `extra.router.root` to `app` and `package.json` uses `index.js` so Expo does not treat `src/app` as the router root.

## Source layout

### Root shell

- `index.js` registers the app with Expo.
- `App.tsx` re-exports `src/App.tsx`.
- `src/App.tsx` owns top-level providers, auth hydration, budget loading, onboarding fallback, bottom tabs, and the WebSocket provider.
- `src/navigation/types.ts` defines the stack/tab route names and params.

### App infrastructure

- `src/app/store.ts` registers Redux reducers: `accounts`, `agent`, `auth`, `budgets`, `categories`, `loans`, `payees`, `tags`, and `transactions`.
- `src/app/hooks.ts` provides typed Redux hooks.
- `src/app/middlewares.ts` coordinates cross-feature data loading when budgets or months change.
- `src/utils/api.ts` is the only shared API transport. Do not create feature-specific fetch wrappers that bypass it.
- `src/utils/storage.ts` is the AsyncStorage auth persistence layer.

### Shared UI

- `src/components/AppText.tsx` centralizes text styling.
- `src/components/Button.tsx` is the shared command button.
- `src/components/Card.tsx` is for individual grouped content, not full page sections.
- `src/components/Screen.tsx` is the scroll/safe-area page wrapper.
- `src/components/SectionHeader.tsx` and `LoadingStateView.tsx` are shared display helpers.
- `src/theme.ts` is the single source for colors, spacing, radii, typography, and shadows.

### Feature folders

Each feature folder should own its domain types and Redux slice. Screen files should stay inside the feature that owns the workflow.

| Feature | Structure | Responsibility |
|---------|-----------|----------------|
| `accounts` | `types.ts`, `store/accountSlice.ts` | account records, tracking/loan selectors, selected-account state |
| `agent` | `types.ts`, `store/agentSlice.ts`, `components/AgentChat.tsx` | Penny agent chat, model selection, conversation history, streaming response state |
| `auth` | `types.ts`, `store/authSlice.ts`, `screens/LoginScreen.tsx` | auth hydration, Google login, refresh/logout, token persistence |
| `budget` | `types.ts`, `constants.ts`, `store/budgetSlice.ts`, `screens/*` | budget list/selection, selected month, onboarding, monthly budget UI |
| `category` | `types.ts`, `store/categorySlice.ts` | categories, category groups, inflow category, budgeted amount updates |
| `dashboard` | `screens/DashboardScreen.tsx` | month overview, account balances, summary cards |
| `loans` | `types.ts`, `store/loanSlice.ts`, `screens/LoansScreen.tsx`, `utils/payoffCalculator.ts` | loan metadata, payoff projections, loan account display |
| `payees` | `types.ts`, `store/payeeSlice.ts`, `screens/PayeesScreen.tsx` | payee list/search/create flows |
| `settings` | `screens/SettingsScreen.tsx` | profile display, budget switching, logout |
| `tags` | `types.ts`, `store/tagSlice.ts` | tag loading and normalized tag state |
| `transactions` | `types.ts`, `store/transactionSlice.ts`, `screens/TransactionsScreen.tsx` | transaction list, search, create/edit, approval/status updates |
| `websocket` | `WebSocketProvider.tsx` | authenticated budget-scoped WebSocket connection and transaction refresh events |

### Data flow

1. `hydrateAuth()` restores stored access/refresh tokens from AsyncStorage.
2. Authenticated users trigger `fetchAllBudgets()`.
3. A selected budget triggers the cross-feature middleware to fetch accounts, transactions, categories/category groups, inflow, payees, loan metadata, tags, and inflow category.
4. Changing the selected month refreshes month-scoped category groups.
5. Budget updates refresh inflow data.
6. WebSocket transaction-created events refresh transactions for the current budget.

When adding a new feature, prefer this pattern:

```text
src/features/<feature>/
├── types.ts
├── store/<feature>Slice.ts
├── screens/<FeatureScreen>.tsx
└── utils/                 # only if the feature has non-UI domain logic
```

## Key files

| Purpose | Path |
|---------|------|
| Expo config | `app.json` |
| EAS build profiles | `eas.json` |
| Babel config | `babel.config.js` |
| App shell/routes/tabs | `src/App.tsx` |
| Theme tokens | `src/theme.ts` |
| Env config | `src/config/env.ts` |
| API client | `src/utils/api.ts` |
| Store setup | `src/app/store.ts` |
| Auth screen | `src/features/auth/screens/LoginScreen.tsx` |
| Auth slice/storage flow | `src/features/auth/store/authSlice.ts` |
| WebSocket provider | `src/features/websocket/WebSocketProvider.tsx` |

## Build, test, run

```bash
cd android-frontend
npm install
npm run typecheck
npx expo export --platform android --output-dir /tmp/pennywise-android-export
```

Expo Go/dev server:

```bash
cd android-frontend
npm run android:dev
```

EAS preview APK:

```bash
cd android-frontend
npx eas-cli build -p android --profile preview --clear-cache
```

Local native Android builds require a local Android SDK. If the user does not want Android Studio/SDK installed, use EAS cloud builds instead of `npx expo run:android`.

## Environment variables

Local `.env`:

```env
EXPO_PUBLIC_API_URL=http://<host>:5151/api
EXPO_PUBLIC_GOOGLE_CLIENT_ID=<web-oauth-client-id>.apps.googleusercontent.com
EXPO_PUBLIC_ANDROID_GOOGLE_CLIENT_ID=<android-oauth-client-id>.apps.googleusercontent.com
```

For EAS builds, define the same values in the EAS `preview` environment. `EXPO_PUBLIC_*` values are embedded into the JS bundle at build time; changing local `.env` after building does not affect an installed APK.

```bash
npx eas-cli env:create --environment preview --name EXPO_PUBLIC_API_URL --value "http://<host>:5151/api" --visibility plaintext
npx eas-cli env:create --environment preview --name EXPO_PUBLIC_GOOGLE_CLIENT_ID --value "<web-client-id>.apps.googleusercontent.com" --visibility plaintext
npx eas-cli env:create --environment preview --name EXPO_PUBLIC_ANDROID_GOOGLE_CLIENT_ID --value "<android-client-id>.apps.googleusercontent.com" --visibility plaintext
```

Backend API must also have:

```env
GOOGLE_ANDROID_CLIENT_ID=<same-android-client-id>.apps.googleusercontent.com
```

## API client behavior

`src/utils/api.ts` uses `EXPO_PUBLIC_API_URL` as the base URL. It automatically:

- adds `Authorization: Bearer <accessToken>` except on public auth endpoints
- adds `x-budget-id` when a selected budget exists, except for budget endpoints
- retries a 401 once via `POST /auth/refresh`
- probes `GET /api` once at app startup through `probeRoot()` so Metro/backend logs show API connectivity

For an Android emulator, the API URL can use `http://10.0.2.2:5151/api`. For a physical phone, use the host machine LAN IP or a reachable deployed API URL. `localhost` means the phone itself.

## Google OAuth notes

The Android app does not use the same OAuth exchange as the React web app.

React web login uses Google Identity Services `auth-code` flow and the backend exchanges the code with `redirect_uri=postmessage`, web client ID, and web client secret.

Installed Android APK login uses Expo AuthSession with PKCE:

- `responseType: ResponseType.Code`
- `shouldAutoExchangeCode: false`
- Android client ID from `EXPO_PUBLIC_ANDROID_GOOGLE_CLIENT_ID`
- redirect URI from AuthSession, usually `dev.pennywise.cloud:/oauthredirect`
- `codeVerifier` sent to the API

The app sends `{ code, redirectUri, codeVerifier }` to `POST /auth/google`. The Go API exchanges mobile codes with `GOOGLE_ANDROID_CLIENT_ID`, the same `redirectUri`, and the PKCE `codeVerifier`. Do not remove these fields or reuse the web `postmessage` exchange for Android; Google will reject it with `invalid_grant`.

Google Cloud Android OAuth client must use:

```text
Application type: Android
Package name: dev.pennywise.cloud
SHA-1: the keystore fingerprint that signed the installed APK
Custom URI scheme: enabled
```

For EAS preview APKs, use the EAS preview signing certificate SHA-1 from:

```bash
npx eas-cli credentials -p android
```

Local debug keystore SHA-1 and EAS preview SHA-1 are different. Use the SHA-1 for the APK actually installed on the phone.

The app requests Gmail scope (`https://mail.google.com/`). If the Google OAuth consent screen is in testing, the account must be listed as a test user.

## Signing credentials

Never commit Android signing credentials.

Ignored artifacts include:

- `*.jks`
- `*.keystore`
- `credentials.json`
- `android/credentials.json`

The SHA-1 fingerprint is safe to put in Google Cloud. The keystore file itself is secret.

## Theme

`src/theme.ts` should match `react-frontend/src/styles/index.css`.

Current core tokens:

- primary: `#4483a2`
- background: `#1f1f1e`
- surface: `#1f1f1f`
- surfaceStrong: `#323232`
- text: `#f6f6f6`
- muted: `#a9a9a9`
- border: `#626262`

Avoid reintroducing the old green/beige Android-only palette.

## Dependency caveats

- `babel-preset-expo` must be a top-level dev dependency because Babel resolves it from the app root.
- `react-native-reanimated@4.1.1` requires `react-native-worklets@0.5.x`; this project pins `react-native-worklets` to `0.5.1`. Do not let it float to `0.8.x`, or EAS Android builds fail in `assertWorkletsVersionTask`.

## Maintenance

When changing app identity, OAuth flow, EAS profiles, environment variables, theme tokens, or API client behavior, update this file in the same PR.
