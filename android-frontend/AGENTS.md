# AGENTS.md — android-frontend

Expo (SDK 54, RN 0.81) Android client mirroring `react-frontend`. Android only; iOS is off (`supportsTablet: false`, no ios config).

## Identity
- Package and scheme: `dev.pennywise.cloud` (`app.config.ts`). Google OAuth redirect is built by `expo-auth-session` (`LoginScreen.tsx`), not set in code; the Android OAuth client must register `dev.pennywise.cloud:/oauthredirect`.
- Env (`src/config/env.ts`, `EXPO_PUBLIC_*`): `EXPO_PUBLIC_API_URL` (default `http://10.0.2.2:5151/api`), `EXPO_PUBLIC_INGEST_URL` (default `http://10.0.2.2:5180/api`, emulator only, so every EAS environment that forwards SMS must set it), `EXPO_PUBLIC_GOOGLE_CLIENT_ID`, `EXPO_PUBLIC_ANDROID_GOOGLE_CLIENT_ID`. Template: `.env.example`.

## Commands
- `npm run typecheck` (`tsc --noEmit`)
- `npm test` (node strip-types runner over `test/`)
- `npm run android:dev` (Metro; needs a dev client already installed)
- Dev client: `npm run android` (local SDK) or `npx eas build --profile preview --platform android`
- Export check: `npx expo export --platform android --output-dir <dir outside repo>`
- `npm run lint` is broken: `eslint` is not a dependency and there is no ESLint config.

## OTA updates (EAS Update)
- Expo account `rishabhkapri`, EAS project id `285e8ad2-57bb-47e7-85f4-331d05616208` (`app.config.ts`). Check login with `npx eas-cli whoami`. Preview APKs listen on channel `preview` with runtime = `version`; builds made before 2026-10-23 have no `expo-updates` and never receive updates.
- `expo-updates` with `runtimeVersion: { policy: 'appVersion' }`; build profiles map to channels (`preview` → `preview`, `production` → `production`). Don't switch to the fingerprint policy: the EAS-side `google-services.json` path differs from local, so the fingerprints never match and the EAS build fails in "Configure expo-updates".
- Ship JS/asset changes: `npx eas-cli update --channel preview --environment preview --message "…"`. Pass `--environment` so the `EXPO_PUBLIC_*` values come from EAS; they're inlined at bundle time, and a local `.env` would otherwise leak into the update.
- EAS resolves account-level (SHARED) variables *after* project ones, so a shared `EXPO_PUBLIC_*` silently overrides the project value. Before `eas update`, run `npx eas-cli env:list preview --format long` and check that each name appears once. A stray shared `EXPO_PUBLIC_ANDROID_GOOGLE_CLIENT_ID` (the web id) once broke sign-in ("Custom scheme URIs are not allowed for 'WEB' client type"). Undo a bad update with `npx eas-cli update:roll-back-to-embedded --channel preview --runtime-version <version> -p android`.
- From `0.1.1`, `updates.checkAutomatically` is `'NEVER'`: the app only takes an update from Settings → App → Check for updates (`features/settings/components/AppUpdates.tsx`). The `0.1.0` APK still checks on every launch and applies on the next cold start. Changing this setting is native config and needs a new APK.
- The app only accepts an update created *after* its embedded bundle. An update published while its APK is still building is treated as older and never offered ("Up to date"). Publish once the build has finished.
- An update only reaches builds with the same `version` (`app.config.ts`). Adding or upgrading a native module or editing native config needs a new APK: bump `version` first, or an OTA update could reach an APK without the native code it needs.

## Architecture
- Navigation: React Navigation (native-stack + bottom-tabs), not Expo Router. `src/App.tsx` owns the navigators. `src/app/` is the Redux store, not a route folder. `app.config.ts` `extra.router.root` is a leftover.
- Bottom tabs (`AppTabParamList`): Dashboard, Budget, Transactions, Reports, Payees, Penny (AgentChat).
- Root stack (`src/navigation/types.ts`, `RootStackParamList`): Main, Settings, Documents, Tags, Jobs, Pipelines, AI, Loans. Occasional screens are pushed here; there is no settings shell. Settings groups them under Library (Documents, Tags, Loans) and Automation (Jobs, Pipelines, AI). Stack screens use `components/StackHeader`.
- State: feature slices in `src/features/*/store/`, registered in `src/app/store.ts`. Typed hooks in `src/app/hooks.ts`.
- Transport: `src/utils/api.ts` (`apiClient`) is the only REST client. It injects `Authorization` and `x-budget-id`, and refreshes on 401. It is wired to the store at boot, so it is unusable headlessly.
- Headless code (widget task handler, location snap task) must use `src/utils/headlessApi.ts`. It reads tokens from AsyncStorage and does not clear the session on failure.
- Realtime: `src/features/websocket/WebSocketProvider.tsx` (agent stream, transaction created).
- Entry: `index.js` registers the app and the widget task handler. `setupLocationSnap()` runs at module scope in `App.tsx`.

## UI conventions
- Theme tokens in `src/theme.ts` mirror react-frontend's dark theme; `chartColors`/`chartSeries` are the web's `--rp-*` report colors. Don't hardcode colors in screens.
- Bottom sheets: `components/Sheet.tsx`. Inside an existing Modal use `SheetOverlay` / `PickerOverlay` / `DatePickerOverlay` (nested Modals misbehave on Android); otherwise `Sheet` / `PickerModal`. Android runs edge-to-edge, so the window doesn't resize for the keyboard: sheets pad themselves with `useKeyboardHeight`, and a list sheet uses `size="tall"` so filtering can't collapse it under the keyboard.
- Mount editors only while open (`{draft ? <TransactionEditor key=… /> : null}`) so pickers and the keypad never survive a close.
- Amounts are typed on `AmountKeypad` (no system keyboard) and accept sums; evaluation is `utils/expression.ts`.
- Charts: `components/charts.tsx` (bars are Views; lines and rings use `react-native-svg`, already in the native build via lucide).

## Data ported from react-frontend
- Jobs (`features/jobs`): `jobTree.ts` is a copy of the web's; keep the two in step. AI keys and models are in `features/ai/llmSetup.ts` and `AIModelsPanel.tsx`.
- Reports (`features/reports`): `cashFlow.ts` is a verbatim copy of the web's; keep the two in step. Group treatments persist per device in AsyncStorage.
- Dashboard (`features/dashboard/store`): its own 6-month transaction window plus `reports/income-expense`, never the shared transactions slice (same rule as the web).
- Loans: balances come from `GET /reports/loans` (amortized server-side); don't go back to original-minus-payments.
- Payees: `payeeUtils.ts` and the slice mirror the web (`payees/stats`, `payees/rules`, `payees/merge`).

## Native modules
- `react-native-pdf` (needs `react-native-blob-util`), `react-native-document-scanner-plugin`, `react-native-android-widget`, expo-notifications, expo-location, expo-task-manager.
- Local module `modules/sms-receiver` (autolinked from `modules/`): a manifest `SMS_RECEIVED` receiver that starts headless task `SmsReceived` (registered in `index.js`), plus `getInboxMessages` (needs `READ_SMS`). Task key lives in `SmsHeadlessTaskService.TASK_KEY` and `SMS_TASK_KEY`; keep them equal.
- None of these run in Expo Go. Rebuild the dev client after adding or updating one.
- EAS uploads only files git doesn't ignore. Keep `.gitignore`'s `/android/` rooted: unrooted, it hid `modules/sms-receiver/android`, the build passed without the module, and the APK opened to a grey screen. Check with `git check-ignore -v modules/*/android/build.gradle`.

## Documents
- View: `ensureDocumentCached` (`src/features/documents/documentFile.ts`) downloads into the app cache. `DocumentViewer` renders from there, so nothing reaches the gallery.
- Save: `saveDocumentToDevice` copies the cached bytes into a folder chosen via Storage Access Framework.
- Scan: `TransactionAttachments.tsx` posts pages to `transactions/:id/documents/scan`.

## Location snap and push
- Push: `registerDevicePushToken` (`src/features/notifications/push.ts`) posts the Expo token to `devices/push-token` on boot.
- On `transaction.created`, a fix under 15 min old is PATCHed to `transactions/:id/location` with source `auto`. Otherwise (no permission or stale fix) the transaction is queued in AsyncStorage (1h TTL, max 10) for a "Tag location?" prompt.
- The prompt is shown on notification tap and on AppState change. "Attach" saves a fresh fix as `manual`.

## SMS forwarding
- `features/sms/smsForwarder.ts` POSTs `{sender, body, receivedAt, source}` to ingest's `ingest/sms` (`EXPO_PUBLIC_INGEST_URL`, budget-scoped, headlessApi), not the main API. `source` is `live` or `inbox`; ingest ignores it and dedupes on sender + body + `receivedAt`, so both paths must send the same time: `getInboxMessages` reads `DATE_SENT` to match the live timestamp. Only alphanumeric sender IDs are sent; phone-number senders never leave the device.
- Failed sends wait in an AsyncStorage outbox (max 50), retried on the next SMS and on app start. Signed out drops them.
- `RECEIVE_SMS` is requested on boot, after the push permission (concurrent permission dialogs get dropped). `READ_SMS` is requested by Settings → SMS → Import recent messages (`smsInbox.ts`, always the last 30 days, so re-imports resend).
- Any non-2xx from ingest keeps the message in the outbox and shows the error. SMS permissions are Play-restricted; fine for sideloaded APKs.

## Parity gaps vs react-frontend
- Not ported: Recurring, Prediction Review, report filters (account/category/tag) and custom report ranges, loan creation. Delete-account Danger Zone is also not ported.
- Android does not show category goals.

## Key files
| Purpose | Path |
|---|---|
| Root navigation and boot | `src/App.tsx`, `src/navigation/types.ts` |
| Store | `src/app/store.ts` |
| REST client | `src/utils/api.ts` |
| Headless REST client | `src/utils/headlessApi.ts` |
| Native config and plugins | `app.config.ts`, `eas.json` |
| Documents | `src/features/documents/documentFile.ts` |
| Location snap, prompts | `src/features/notifications/locationSnapTask.ts` |
| Push registration | `src/features/notifications/push.ts` |
| Widgets | `src/features/widgets/`, `index.js` |
| SMS forwarding | `src/features/sms/`, `modules/sms-receiver/` |
| Realtime | `src/features/websocket/WebSocketProvider.tsx` |
