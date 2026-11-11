#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
project="totallyswipes-integration-$$"
compose=(docker compose -p "$project" -f scripts/integration/compose.yml)
cleanup() { "${compose[@]}" down --volumes; }
trap cleanup EXIT
"${compose[@]}" up -d --wait
for migration in db/migrations/*.up.sql; do
  "${compose[@]}" exec -T -e PGPASSWORD=integration postgres psql -h 127.0.0.1 -v ON_ERROR_STOP=1 -U integration -d integration < "$migration"
done
pg_addr=$("${compose[@]}" port postgres 5432)
export TEST_HISTORY_DATABASE_URL="postgres://integration:integration@${pg_addr}/integration?sslmode=disable"
export TEST_REDIS_ADDR=$("${compose[@]}" port redis 6379)
go test -tags=integration -race -count=1 -timeout=2m ./internal/repository/postgres/integration_tests ./internal/repository/redis/integration_tests
