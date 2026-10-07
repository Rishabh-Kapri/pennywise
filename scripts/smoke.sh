#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
# Fresh disposable database on every run, never an existing development volume.
export COMPOSE_PROJECT_NAME="pw-smoke-$(date +%s)-$$"
export DEMO_WEB_PORT=0
mkdir -p artifacts
cleanup() {
  ./scripts/dev.sh logs --no-color > artifacts/smoke-services.log 2>&1 || true
  ./scripts/dev.sh down --volumes --remove-orphans
}
trap cleanup EXIT
./scripts/dev.sh up
address=$(./scripts/dev.sh port web 5173)
export E2E_BASE_URL="http://127.0.0.1:${address##*:}"
npm --prefix react-frontend run test:e2e
