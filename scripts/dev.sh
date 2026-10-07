#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
# Each checkout has independent Compose resources. Override for CI or explicit names.
export COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-pw-$(printf '%s' "$PWD" | cksum | cut -d' ' -f1)}"
compose=(docker compose --env-file /dev/null -f compose.demo.yml)
if [[ "${1:-}" == up ]]; then
  "${compose[@]}" up --build --renew-anon-volumes -d --wait --wait-timeout 180
  "${compose[@]}" port web 5173
else
  "${compose[@]}" "$@"
fi
