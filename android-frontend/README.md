# Pennywise Android

Expo React Native Android client for Pennywise.

This app mirrors `react-frontend`:

- feature-first Redux slices under `src/features`
- shared `apiClient` that injects `Authorization` and `x-budget-id`
- the same core API endpoints: `auth`, `budgets`, `accounts`, `transactions`, `categories`, `payees`, `loan-metadata`, `tags`
- Android-focused screens for dashboard, budget, transactions, payees, loans, and settings

## Run

```bash
cd android-frontend
npm install
npm run android:dev
```

For the Android emulator, use `EXPO_PUBLIC_API_URL=http://10.0.2.2:5151/api`. For a physical device, point it at the host machine's LAN IP.

## Push notifications

Transaction pushes (which drive location auto-tagging) need Firebase credentials that are
**not** in this repo. Download `google-services.json` from the Firebase console
(project settings → your Android app, package `dev.pennywise.cloud`) and drop it at
`android-frontend/google-services.json`. It is gitignored on purpose — this repo is public.

`app.config.ts` picks it up automatically for local builds. For EAS builds, upload it once
as a file-type environment variable:

```bash
npx eas-cli env:create --environment preview --name GOOGLE_SERVICES_JSON --type file --value ./google-services.json --visibility secret
npx eas-cli env:create --environment production --name GOOGLE_SERVICES_JSON --type file --value ./google-services.json --visibility secret
```

Push tokens only work on a physical device running a dev client or standalone build —
not Expo Go, and not an emulator. See `AGENTS.md` for details.
