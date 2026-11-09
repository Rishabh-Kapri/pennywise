# API collection

`pennywise.postman_collection.json` holds every HTTP route of the backend: 125 in
go-pennywise-api, 15 in cipher, 5 in go-gmail. It is a Postman v2.1 collection,
which Insomnia, Postman, Bruno and Hoppscotch all import.

- **Insomnia:** Create → Import → From File, pick the JSON.
- **Postman / Bruno:** Import → the JSON file.

## Variables

| Variable | Default | Set by |
|---|---|---|
| `apiUrl`, `cipherUrl`, `gmailUrl` | `http://localhost:5151`, `:5160`, `:5170` | docker-compose ports |
| `accessToken`, `refreshToken` | | `POST /auth/demo` or `POST /auth/google`; `POST /auth/refresh` renews `accessToken` |
| `apiKey` | | `POST /keys` (the secret is returned only once) |
| `userId` | | `GET /auth/users/me` |
| `budgetId` | | `GET /budgets` (first budget, if unset) |
| `accountId`, `payeeId`, `categoryId`, `transactionId`, `keyId`, ... | | you, from earlier responses |
| `internalToken` | | your `INTERNAL_AUTH_TOKEN`, for cipher and gmail |

The token, key, user and budget scripts run in Postman and in Insomnia's scripting.
Elsewhere, copy the values from the responses by hand.

## Getting started

1. Log in: `Pennywise API / auth / POST /auth/demo` (needs `DEMO_MODE=true`), or
   `POST /auth/google` with an OAuth code.
2. `GET /budgets` stores your budget id; budget-scoped requests send it as `X-Budget-ID`.
3. `keys / POST /keys` creates an API key and stores it in `apiKey`.

## Auth

Authenticated requests send `X-API-Key: {{apiKey}}` and also inherit
`Authorization: Bearer {{accessToken}}`. The server checks the API key first. An empty
`X-API-Key` is ignored, so until `apiKey` is set the bearer token is used, and clearing
`apiKey` switches back to it.

New keys only get the `read` scope. So:

- The `X-API-Key` header is **on** for routes that need `read` (or no scope) and
  **off** for routes that need `write`, `delete` or `admin`. Those use the bearer token.
  Turning the header on there returns `403 insufficient permissions`.
- `POST /keys` needs `admin`, so it never sends `X-API-Key` and always uses the login token.

Each request's description names the scope an API key needs for it.

Cipher and gmail are internal services. Their requests send `X-Internal-Token`,
plus `X-User-ID` and `X-Budget-ID` where the route needs them.

## Notes

- Request bodies come from the Go request types: every JSON field, with optional
  pointer fields left out and server-set fields (`id`, `budgetId`, timestamps,
  computed totals) removed. A few common ones (create key, budget, account,
  payee, tag, transaction) have realistic sample values.
- Query parameters are listed but disabled unless the handler has a default; enable
  the ones you need.
- `GET /ws` is the WebSocket upgrade; use a WebSocket client on
  `ws://localhost:5151/ws?token=<accessToken>&budgetId=<budgetId>`.
- The collection is a snapshot. When you add or change a route, update it.
