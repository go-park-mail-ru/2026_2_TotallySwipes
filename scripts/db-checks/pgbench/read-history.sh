#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/../../.."
source scripts/db-checks/pgbench/common.sh
initialize_benchmark "${1:-}" read-history

for clients in $client_counts; do
  prepare_data
  run_pgbench "$clients" scripts/db-checks/pgbench/read-history.sql
done
