# Service guidance

Keep workflow code deterministic; external I/O belongs in activities. Preserve request metadata propagation, task queues, retry behavior, and compatibility with running workflows. Run `go vet ./...` and `go test ./...`. Root demo does not run Temporal; use workflow test environments for orchestration changes.

See root AGENTS.md and docs/development.md for shared commands and architecture.
