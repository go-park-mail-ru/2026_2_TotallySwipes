#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/../../.."
project="${1:?Передайте имя Compose-проекта, напечатанное load.sh}"
compose=(docker compose -p "$project" -f scripts/db-checks/compose.yml)
log_file="arifacts/load/${project}/load-$(date +%Y%m%d-%H%M%S)-$$.log"
mkdir -p "$(dirname "$log_file")"
exec > >(tee "$log_file") 2>&1

cleanup() {
  status=$?
  trap - EXIT
  if ! bash scripts/db-checks/collect-logs.sh "$project"; then
    status=1
  fi
  echo "Лог нагрузки: $log_file"
  exit "$status"
}

trap cleanup EXIT

# Нагрузка использует только отдельную тестовую БД, созданную load.sh.
"${compose[@]}" exec -T -e PGPASSWORD=admin_test_only postgres \
  psql -X -h 127.0.0.1 -U access_admin -d access_test -v ON_ERROR_STOP=1 <<'SQL'
CREATE TABLE IF NOT EXISTS public.monitoring_demo (
    id BIGINT PRIMARY KEY,
    value BIGINT NOT NULL
);
TRUNCATE public.monitoring_demo;
SQL

for iteration in {1..30}; do
  "${compose[@]}" exec -T -e PGPASSWORD=admin_test_only postgres \
    psql -X -h 127.0.0.1 -U access_admin -d access_test -v ON_ERROR_STOP=1 <<'SQL'
INSERT INTO public.monitoring_demo (id, value)
SELECT id, 0 FROM generate_series(1, 1000) AS id;
UPDATE public.monitoring_demo SET value = value + 1;
DELETE FROM public.monitoring_demo;
SQL
  sleep 2
done

echo "Нагрузка завершена. На графиках INSERT, UPDATE и DELETE появится рост."
