#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
if [[ ! -f .env ]]; then
  echo 'Create .env from .env.example and configure a merchant token first.' >&2
  exit 1
fi
set -a
source .env
set +a
cd swaplook_BE
exec go run ./cmd/api
