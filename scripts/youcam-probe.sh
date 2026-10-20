#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
root_dir="$PWD"
if [[ ! -f .env ]]; then echo 'Create the ignored server .env first. Never send the API key through chat.' >&2; exit 1; fi
set -a
source .env
set +a
if [[ -z "${YOUCAM_API_KEY:-}" ]]; then echo 'Set YOUCAM_API_KEY in the ignored server .env first. No provider request was made.' >&2; exit 1; fi
person_path="${YOUCAM_PERSON_IMAGE:-.data/youcam/person.jpg}"
garment_path="${YOUCAM_GARMENT_IMAGE:-.data/garments/SL-002.jpg}"
output_path="${YOUCAM_PROBE_DIR:-.data/probe}"
case "$person_path" in /*) ;; *) person_path="$root_dir/$person_path";; esac
case "$garment_path" in /*) ;; *) garment_path="$root_dir/$garment_path";; esac
case "$output_path" in /*) ;; *) output_path="$root_dir/$output_path";; esac
cd swaplook_BE
exec go run ./cmd/probe -person "$person_path" -garment "$garment_path" -output "$output_path" "$@"
