# file-parser (experimental)

Clojure service scaffold for bulk transaction uploads (parsing statement files into Pennywise transactions). Early-stage: an HTTP handler skeleton exists (`src/file_parser/core.clj`, default port `4000` via `PORT`), but the parsing/bulk-upload flow is not production-ready.

## Run

```bash
clojure -M:run-m          # start the service
clojure -T:build test     # run tests
clojure -T:build ci       # tests + uberjar into target/
```

## Layout

- `src/file_parser/core.clj` — entry point / HTTP server
- `src/file_parser/handler.clj` — request handlers
- `src/file_parser/bulk_upload.clj` — bulk-upload flow (stub)

Not part of the docker-compose stack or CI deploy.
