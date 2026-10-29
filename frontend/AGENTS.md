# frontend (Angular 17 + NGXS)

Legacy client, maintenance only. Active UI development happens in `react-frontend/`. This app still contains Firestore/Firebase remnants from an earlier architecture (`src/app/services/databases/*.firestore.ts`); do not build new features here.

## Commands

```bash
npm install
npm start                # ng serve on port 5000
npm run build            # ng build -> dist/pennywise
npm test                 # ng test (Karma + Jasmine)
npx ng test --include="**/name.spec.ts"   # single spec
```

## Conventions

- NGXS states live in `src/app/store/dashboard/states/` (and `store/auth/`), decorated with `@State<...>`.
- Prefer constructor injection; a handful of files use `inject()`, so don't rewrite them unless touching that code.
- Explicit model interfaces in `src/app/models/` (e.g. `transaction.model.ts`).
- Styling: SCSS per component plus Tailwind (`src/styles.scss`).
- `HeadersInterceptor` (`src/app/services/headers-interceptor.service.ts`) adds `X-Budget-ID` to API calls.
- Tests: 15 `*.spec.ts` files; Karma runs them in a real browser, so `npm test` needs Chrome.
