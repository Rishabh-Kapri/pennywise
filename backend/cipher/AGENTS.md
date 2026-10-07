# Service guidance

Preserve budget scoping in tools, memory, corrections, and streamed events. Tests should use fake providers with deterministic outputs. Run `go vet ./...` and `go test ./...`. The root demo stubs external integrations and does not verify classification quality or live agent execution; report this distinction when validating AI changes.

See root AGENTS.md and docs/development.md for shared commands and architecture.
