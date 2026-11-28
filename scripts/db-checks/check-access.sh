#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/../.."
source scripts/db-checks/stand.sh

if [[ $# != 0 ]]; then
  echo "Использование: bash scripts/db-checks/check-access.sh"
  exit 1
fi

allowed() {
  local role="$1"
  local query="$2"
  echo "Проверка разрешённого запроса: $role"
  sql "$role" "${role}_test_only" -c "$query"
  echo "PASS: разрешённый запрос выполнен"
}

denied() {
  local role="$1"
  local query="$2"
  local output
  echo "Проверка запрещённого запроса: $role — $query"

  if output=$(sql "$role" "${role}_test_only" -c "$query" 2>&1); then
    echo "FAIL: запрещённый запрос выполнился"
    return 1
  fi

  echo "$output"

  if [[ "$output" != *"42501"* ]]; then
    echo "FAIL: ожидался SQLSTATE 42501 (недостаточно прав)"
    return 1
  fi

  echo "PASS: получен ожидаемый отказ в доступе"
}

initialize_stand check-access 0
start_database

for role in auth_app profile_app matching_app chat_app onlineconf_app monitor; do
  echo "Проверка подключения: $role"
  actual=$(sql "$role" "${role}_test_only" -Atc 'SELECT current_user;')

  if [[ "$actual" != "$role" ]]; then
    echo "FAIL: ожидали $role, подключились как $actual"
    exit 1
  fi

  echo "PASS: подключение под $actual"

  if sql "$role" wrong_password -c 'SELECT 1;'; then
    echo "FAIL: подключение с неверным паролем разрешено"
    exit 1
  fi

  echo "PASS: неверный пароль отклонён"
  denied "$role" 'CREATE TABLE public.forbidden_table (id bigint);'
done

allowed auth_app "BEGIN; WITH account AS (
  INSERT INTO public.\"user\" (email, password_hash)
  VALUES ('access-test@example.com', 'test_hash') RETURNING id
) INSERT INTO public.profile (user_id) SELECT id FROM account; ROLLBACK;"

allowed profile_app 'SELECT id FROM public.profile LIMIT 1;'
allowed matching_app 'SELECT id FROM public."match" LIMIT 1;'

allowed onlineconf_app 'SELECT id, email FROM public."user" LIMIT 1;'
allowed onlineconf_app "BEGIN;
    INSERT INTO onlineconf.configs (scope, name, kind, value_type, value)
    VALUES ('profile', 'access_check', 'config', 'number', '6');
    UPDATE onlineconf.state SET revision = revision + 1 WHERE id = 1;
    ROLLBACK;"
denied onlineconf_app 'SELECT password_hash FROM public."user";'
denied onlineconf_app 'DELETE FROM onlineconf.history;'
allowed matching_app 'SELECT id, match_id FROM public.chat LIMIT 1;'
allowed chat_app 'SELECT id, is_disabled FROM public."user" LIMIT 1;'
allowed monitor 'SELECT datname, numbackends FROM pg_catalog.pg_stat_database;'

denied auth_app 'SELECT * FROM public.message;'
denied profile_app 'SELECT password_hash FROM public."user";'
denied matching_app 'SELECT password_hash FROM public."user";'
denied chat_app 'SELECT password_hash FROM public."user";'
denied monitor 'UPDATE public."user" SET is_disabled = true WHERE false;'

echo "PASS: все проверки подключения и доступа пройдены"
