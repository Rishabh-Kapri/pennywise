# frontend (Angular, legacy)

> **Status: legacy/maintenance.** Active UI development happens in [`react-frontend`](../react-frontend). This Angular 17 app predates it and still contains Firestore remnants from an earlier architecture.

Angular 17 + NGXS state management + SCSS/Tailwind.

## Run

```bash
npm install
npm start         # ng serve on port 5000 (0.0.0.0)
npm run build     # production build into dist/
npm test          # Karma/Jasmine unit tests
```

## Conventions

- NGXS actions/selectors/states in `src/app/store/dashboard/states/`
- Constructor-based dependency injection (not `inject()`)
- Explicit model interfaces in `src/app/models/` (e.g. `transaction.model.ts`)
- `HeadersInterceptor` injects `X-Budget-ID` on API calls
