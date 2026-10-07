# Service guidance

Preserve handler → service → repository layering. Enforce auth and budget ownership in middleware; never trust raw internal headers. Keep transfer, carryover, and prediction correction side effects in the transaction service. Run `go vet ./...` and `go test ./...`; schema changes also require root `make smoke` on a fresh database. Do not baseline empty databases or renumber existing migrations.

See root AGENTS.md and docs/development.md for shared commands and architecture.
