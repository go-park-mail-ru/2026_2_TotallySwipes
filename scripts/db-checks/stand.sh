#!/usr/bin/env bash

# Общие функции подготовки и удаления изолированного стенда.
set -euo pipefail

initialize_stand() {
  local kind="$1"
  stay_alive="$2"
  project="totallyswipes-$kind-$(date +%s)-$$"
  compose=(docker compose -p "$project" -f scripts/db-checks/compose.yml)
  log_file="arifacts/$kind/$project/run.log"
  mkdir -p "$(dirname "$log_file")"
  exec > >(tee "$log_file") 2>&1

  trap cleanup_stand EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM
  echo "Тестовый проект: $project"
}

cleanup_stand() {
  local status=$?
  trap - EXIT INT TERM

  if ! bash scripts/db-checks/collect-logs.sh "$project"; then
    echo "FAIL: не удалось сохранить логи PostgreSQL"
    status=1
  fi

  if [[ "$stay_alive" == 1 ]]; then
    echo "Стенд оставлен запущенным: $project"
    echo "Grafana: http://localhost:3000/d/postgres-basics (admin / grafana_test_only)"
    echo "Остановка: docker compose -p $project -f scripts/db-checks/compose.yml --profile monitoring down --volumes"
  else

    if ! "${compose[@]}" --profile monitoring down --volumes; then
      status=1
    fi
  fi

  echo "Лог запуска: $log_file"
  exit "$status"
}

sql() {
  local role="$1"
  local password="$2"
  shift 2
  "${compose[@]}" exec -T -e "PGPASSWORD=$password" postgres \
    psql -X -h 127.0.0.1 -U "$role" -d access_test \
    -v ON_ERROR_STOP=1 -v VERBOSITY=verbose "$@"
}

start_database() {
  "${compose[@]}" up -d --wait --wait-timeout 120 postgres

  for migration in db/migrations/*.up.sql; do
    echo "Миграция: $migration"
    sql access_admin admin_test_only < "$migration"
  done

  sql access_admin admin_test_only < db/security/roles.sql
  sql access_admin admin_test_only < db/security/grants.sql

  sql access_admin admin_test_only <<'SQL'
ALTER ROLE auth_app PASSWORD 'auth_app_test_only';
ALTER ROLE profile_app PASSWORD 'profile_app_test_only';
ALTER ROLE matching_app PASSWORD 'matching_app_test_only';
ALTER ROLE chat_app PASSWORD 'chat_app_test_only';
ALTER ROLE onlineconf_app PASSWORD 'onlineconf_app_test_only';
ALTER ROLE monitor PASSWORD 'monitor_test_only';
SQL

}

start_monitoring() {
  "${compose[@]}" --profile monitoring up -d --wait --wait-timeout 180
  echo "Grafana: http://localhost:3000/d/postgres-basics (admin / grafana_test_only)"
  echo "Prometheus: http://localhost:9090"
}
