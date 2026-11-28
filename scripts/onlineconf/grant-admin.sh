#!/usr/bin/env bash
set -euo pipefail

if [[ $# != 1 || -z "$1" ]]; then
    echo "Использование: bash scripts/onlineconf/grant-admin.sh <email>" >&2
    exit 1
fi

repository_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
project="${PROJECT:-totallyswipes-microservices}"

# Подключаемся к работающему PostgreSQL под владельцем БД.
docker compose -p "$project" -f "$repository_dir/docker-compose.microservices.yml" \
    exec -T -e "ONLINECONF_ADMIN_EMAIL=$1" postgres sh -ec '
        exec psql -X -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1
    ' <<'SQL'
\getenv admin_email ONLINECONF_ADMIN_EMAIL

BEGIN;

SELECT revision FROM onlineconf.state WHERE id = 1 FOR UPDATE;

SELECT id AS target_user_id
FROM public."user"
WHERE email = lower(btrim(:'admin_email'))
\gset

-- Нет пользователя — psql завершится с ошибкой, транзакция будет отменена.
WITH inserted AS (
    INSERT INTO onlineconf.admins (user_id)
    VALUES (:'target_user_id'::BIGINT)
    ON CONFLICT (user_id) DO NOTHING
    RETURNING *
), recorded AS (
    INSERT INTO onlineconf.history (entity, action, "after")
    SELECT 'admin', 'grant', to_jsonb(inserted) FROM inserted
    RETURNING id
)
UPDATE onlineconf.state SET revision = revision + 1
WHERE id = 1 AND EXISTS (SELECT 1 FROM recorded);

SELECT user_id AS admin_user_id FROM onlineconf.admins
WHERE user_id = :'target_user_id'::BIGINT;

COMMIT;
SQL
