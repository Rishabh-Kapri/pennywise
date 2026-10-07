# Service guidance

Changes affect all Go services: run root `make check-go`. Preserve canonical context propagation and verified-internal auth semantics. DB tests must never use hard-coded hosts or application credentials. Legacy monthly-budget integration tests require a disposable PENNYWISE_TEST_DATABASE_URL; absence means skip, explicit connection failure means fail.

See root AGENTS.md and docs/development.md for shared commands and architecture.
