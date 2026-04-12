# Pennywise

Personal finance/budgeting app with ML-powered transaction classification from email parsing.

## Setup

After cloning, enable git hooks:

```bash
git config core.hooksPath .githooks
```

This runs `go mod vendor` automatically when `go.mod` or `shared/` changes before a commit.
