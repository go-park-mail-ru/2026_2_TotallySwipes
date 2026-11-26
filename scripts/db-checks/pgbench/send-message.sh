#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/../../.."
source scripts/db-checks/pgbench/common.sh
initialize_benchmark "${1:-}" send-message

for clients in $client_counts; do
  prepare_data
  run_pgbench "$clients" scripts/db-checks/pgbench/send-message.sql

  # Проверяем, что параллельные отправки не создали пропуски или дубликаты seq.
  consistent=$("${compose[@]}" exec -T -e PGPASSWORD=chat_app_test_only postgres \
    psql -X -h 127.0.0.1 -U chat_app -d access_test -v ON_ERROR_STOP=1 -Atc '
      SELECT bool_and(last_seq = message_count AND last_seq = maximum_seq AND minimum_seq = 1)
      FROM (
          SELECT sequence.last_seq, count(message.id) AS message_count,
                 max(message.seq) AS maximum_seq, min(message.seq) AS minimum_seq
          FROM public.chat_sequence AS sequence
          JOIN public.message AS message ON message.chat_id = sequence.chat_id
          WHERE sequence.chat_id BETWEEN 900000001 AND 900000010
          GROUP BY sequence.chat_id, sequence.last_seq
      ) AS checks;')

  if [[ "$consistent" != t ]]; then
    echo "FAIL: обнаружено нарушение последовательности сообщений"
    exit 1
  fi

  echo "PASS: seq уникальны, идут без пропусков и согласованы со счётчиками"
done
