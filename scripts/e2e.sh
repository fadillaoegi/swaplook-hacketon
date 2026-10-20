#!/usr/bin/env bash
# Runs only against the explicitly supplied, disposable test database.
set -euo pipefail
cd "$(dirname "$0")/.."
root_dir="$PWD"
: "${E2E_DATABASE_URL:?Set E2E_DATABASE_URL to a dedicated disposable PostgreSQL database.}"
e2e_port="${E2E_PORT:-8089}"
case "$e2e_port" in ''|*[!0-9]*) echo 'E2E_PORT must be a TCP port number.' >&2; exit 1;; esac
export SWAPLOOK_BASE_URL="http://127.0.0.1:$e2e_port"
if curl -fsS --connect-timeout 1 --max-time 1 "$SWAPLOOK_BASE_URL/api/health" >/dev/null 2>&1; then
  echo 'E2E port already has a running service. Select a different E2E_PORT.' >&2
  exit 1
fi
run_dir="$(mktemp -d "$root_dir/tmp/e2e.XXXXXX")"
chmod 700 "$run_dir"
export DATABASE_URL="$E2E_DATABASE_URL"
export HTTP_ADDR="127.0.0.1:$e2e_port" APP_ORIGIN="$SWAPLOOK_BASE_URL"
export STATIC_DIR="$root_dir/swaplook_FE/dist" PRIVATE_STORAGE_DIR="$run_dir/images"
export RENDERER_MODE=simulator COOKIE_SECURE=false
export MERCHANT_TOKEN="${MERCHANT_TOKEN:-$(openssl rand -hex 32)}"
"$root_dir/tmp/swaplook-api" >"$run_dir/api.log" 2>&1 &
api_pid=$!
cleanup() { kill "$api_pid" 2>/dev/null || true; wait "$api_pid" 2>/dev/null || true; }
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
deadline=$((SECONDS+30))
ready=false
while (( SECONDS < deadline )); do
  if ! kill -0 "$api_pid" 2>/dev/null; then
    echo 'Test API exited before readiness. Inspect the private tmp/e2e.*/api.log file.' >&2
    exit 1
  fi
  if curl -fsS --connect-timeout 1 --max-time 1 "$SWAPLOOK_BASE_URL/api/health" >/dev/null 2>&1; then ready=true; break; fi
  sleep 0.2
done
if [[ "$ready" != true ]]; then echo 'Test API did not become ready within 30 seconds.' >&2; exit 1; fi
cd swaplook_FE
pnpm exec playwright test
