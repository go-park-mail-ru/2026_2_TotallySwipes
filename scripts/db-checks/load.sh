#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/../.."
source scripts/db-checks/stand.sh

if [[ $# -lt 1 || $# -gt 2 ]]; then
  echo "Использование: bash scripts/db-checks/load.sh {read-history|send-message|all} [STAY_ALIVE]"
  exit 1
fi

test_type="$1"
stay_alive=0

case "$test_type" in
  read-history|send-message|all) ;;
  *) echo "Неизвестный сценарий: $test_type"; exit 1 ;;
esac

if [[ $# == 2 ]]; then

  if [[ "$2" != STAY_ALIVE ]]; then
    echo "Второй аргумент может быть только STAY_ALIVE"
    exit 1
  fi

  stay_alive=1
fi

initialize_stand load "$stay_alive"
start_database
start_monitoring

# Каждый дочерний сценарий получает имя уже подготовленного стенда.
case "$test_type" in

  read-history)
    bash scripts/db-checks/pgbench/read-history.sh "$project"
    ;;

  send-message)
    bash scripts/db-checks/pgbench/send-message.sh "$project"
    ;;

  all)
    bash scripts/db-checks/pgbench/read-history.sh "$project"
    bash scripts/db-checks/pgbench/send-message.sh "$project"
    ;;
esac

echo "PASS: нагрузочное тестирование завершено"
