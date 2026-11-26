#!/usr/bin/env bash

# Этот файл подключают два сценария; отдельно его запускать не нужно.
set -euo pipefail

initialize_benchmark() {
  project="${1:?Передайте имя тестового Compose-проекта}"
  scenario="$2"
  duration="${PGBENCH_DURATION:-60}"
  client_counts="${PGBENCH_CLIENTS:-1 5 10}"

  if [[ ! "$project" =~ ^totallyswipes-(load|access)-[0-9]+-[0-9]+$ ]]; then
    echo "Ожидалось имя отдельного тестового проекта totallyswipes-load-..."
    exit 1
  fi

  if [[ ! "$duration" =~ ^[1-9][0-9]*$ || -z "${client_counts// /}" ]]; then
    echo "Длительность и число клиентов должны быть положительными целыми числами"
    exit 1
  fi

  for clients in $client_counts; do

    if [[ ! "$clients" =~ ^[1-9][0-9]*$ ]]; then
      echo "Некорректное число клиентов: $clients"
      exit 1
    fi
  done

  compose=(docker compose -p "$project" -f scripts/db-checks/compose.yml)
  artifact_root="arifacts/pgbench/$project"
  mkdir -p "$artifact_root"
  lock_dir="$artifact_root/.running"

  if ! mkdir "$lock_dir"; then
    echo "Для этого стенда уже выполняется сценарий. Запускайте их последовательно."
    exit 1
  fi

  result_dir="$artifact_root/$scenario-$(date +%Y%m%d-%H%M%S)-$$"
  mkdir -p "$result_dir"
  trap finish_benchmark EXIT
  exec > >(tee "$result_dir/run.log") 2>&1

  echo "Проект: $project; сценарий: $scenario"
  echo "Клиенты: $client_counts; длительность каждого запуска: ${duration}s"
  "${compose[@]}" exec -T postgres pgbench --version
}

finish_benchmark() {
  status=$?
  trap - EXIT

  if ! bash scripts/db-checks/collect-logs.sh "$project"; then
    status=1
  fi

  rmdir "$lock_dir" || true
  echo "Результаты: $result_dir"
  exit "$status"
}

prepare_data() {
  # Каждый запуск начинает с одинаковой истории: 10 чатов по 1000 сообщений.
  "${compose[@]}" exec -T -e PGPASSWORD=admin_test_only postgres \
    psql -X -h 127.0.0.1 -U access_admin -d access_test \
    -v ON_ERROR_STOP=1 < scripts/db-checks/pgbench/prepare.sql
}

run_pgbench() {
  local clients="$1"
  local sql_file="$2"

  echo "Старт: $(date -u +%Y-%m-%dT%H:%M:%SZ); клиентов: $clients"

  "${compose[@]}" exec -T \
    -e PGPASSWORD=chat_app_test_only \
    -e "PGAPPNAME=pgbench-$scenario-$clients" \
    postgres pgbench \
    -h 127.0.0.1 -U chat_app \
    --no-vacuum --protocol=prepared \
    --client="$clients" --jobs=1 --time="$duration" \
    --progress=5 --report-per-command --random-seed=42 \
    --file=/dev/stdin access_test < "$sql_file" 2>&1 | tee "$result_dir/clients-$clients.log"

  echo "Завершение: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
}
