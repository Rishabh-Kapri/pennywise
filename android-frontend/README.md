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
