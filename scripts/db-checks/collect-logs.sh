#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/../.."
project="${1:?Передайте имя Compose-проекта}"
log_dir="arifacts/postgres/$project"
mkdir -p "$log_dir"

log_file="$log_dir/postgres-$(date +%Y%m%d-%H%M%S)-$$.log"
docker compose -p "$project" -f scripts/db-checks/compose.yml \
  logs --no-color --timestamps postgres > "$log_file" 2>&1

echo "Логи PostgreSQL: $log_file"
