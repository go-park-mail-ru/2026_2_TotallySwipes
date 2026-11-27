#!/usr/bin/env bash
set -euo pipefail

app_dir="${1:?Укажите каталог приложения}"
export IMAGE_PREFIX="${2:?Укажите префикс образов GHCR}"
export IMAGE_TAG="${3:?Укажите Git SHA релиза}"

if [[ ! "$IMAGE_TAG" =~ ^[a-f0-9]{40}$ ]]; then
  echo "Git SHA должен содержать 40 шестнадцатеричных символов" >&2
  exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "Для установки паролей ролей PostgreSQL нужен python3" >&2
  exit 1
fi

release_dir="$app_dir/releases/$IMAGE_TAG"
project="totallyswipes-${IMAGE_TAG:0:12}"
core_project="totallyswipes-microservices"
apps=(auth profile matching chat gateway)
compose=(docker compose -f "$release_dir/docker-compose.microservices.yml")
export DEPLOYMENT_ID="${IMAGE_TAG:0:12}"
export NGINX_CONFIG_DIR="$app_dir/nginx/conf.d"
export NGINX_UPSTREAM_DIR="$app_dir/nginx/upstreams"
export CERTS_DIR="$app_dir/certs"
public_host="${PUBLIC_HOST:-totally-swipes.ru}"
previous_project="${LEGACY_PROJECT:-app}"

mkdir -p "$app_dir" "$NGINX_CONFIG_DIR" "$NGINX_UPSTREAM_DIR"
exec 9>"$app_dir/deploy.lock"
flock -n 9

if [[ -f "$app_dir/current-project" ]]; then
  previous_project=$(cat "$app_dir/current-project")
fi

if [[ ! "$previous_project" =~ ^[a-z0-9][a-z0-9_-]*$ ]]; then
  echo "Некорректное имя предыдущего Compose-проекта" >&2
  exit 1
fi

backup_dir=$(mktemp -d)
candidate_started=0
config_changed=0
completed=0
proxy_running=0

if [[ $(docker inspect -f '{{.State.Running}}' totallyswipes-nginx 2>/dev/null || true) == true ]]; then
  proxy_running=1
  mounted_config=$(docker inspect -f '{{range .Mounts}}{{if eq .Destination "/etc/nginx/conf.d"}}{{.Source}}{{end}}{{end}}' totallyswipes-nginx)

  if [[ "$mounted_config" != "$NGINX_CONFIG_DIR" ]]; then
    echo "Перед первым деплоем перезапустите nginx с каталогом $NGINX_CONFIG_DIR" >&2
    exit 1
  fi
fi

for item in "$NGINX_CONFIG_DIR/default.conf" "$NGINX_UPSTREAM_DIR/gateway.conf"; do
  if [[ -f "$item" ]]; then
    cp "$item" "$backup_dir/$(basename "$item")"
  fi
done

finish() {
  local status=$?
  trap - EXIT

  if [[ "$status" != 0 && "$completed" == 0 ]]; then
    echo "Деплой не завершён. Предыдущие приложения не останавливаем." >&2

    if [[ "$config_changed" == 1 ]]; then
      docker logs --tail 100 totallyswipes-nginx || true
      docker inspect --format \
        '{{.Name}}: status={{.State.Status}}, exit={{.State.ExitCode}}, error={{.State.Error}}, ports={{json .NetworkSettings.Ports}}, bindings={{json .HostConfig.PortBindings}}' \
        totallyswipes-nginx || true
    fi

    # Сначала выводим причину сбоя, пока откат ещё не удалил контейнеры.
    if [[ "$candidate_started" == 1 ]]; then
      "${compose[@]}" -p "$project" ps --all || true
      "${compose[@]}" -p "$project" logs --no-color --tail 100 "${apps[@]}" || true

      for container in $(docker ps -aq --filter "label=com.docker.compose.project=$project"); do
        docker inspect --format \
          '{{.Name}}: status={{.State.Status}}, exit={{.State.ExitCode}}, oom={{.State.OOMKilled}}{{if .State.Health}}{{range .State.Health.Log}}{{println}}{{.Output}}{{end}}{{end}}' \
          "$container" || true
      done
    else
      "${compose[@]}" -p "$core_project" ps --all postgres redis minio || true
      "${compose[@]}" -p "$core_project" logs --no-color --tail 100 postgres redis minio || true
    fi

    local restored=1

    if [[ "$config_changed" == 1 && "$proxy_running" == 1 ]]; then
      for item in default.conf gateway.conf; do
        if [[ ! -f "$backup_dir/$item" ]]; then
          restored=0
        fi
      done

      if [[ "$restored" == 1 ]]; then
        cp "$backup_dir/default.conf" "$NGINX_CONFIG_DIR/default.conf"
        cp "$backup_dir/gateway.conf" "$NGINX_UPSTREAM_DIR/gateway.conf"
        if ! docker exec totallyswipes-nginx nginx -t || ! docker exec totallyswipes-nginx nginx -s reload; then
          restored=0
        fi
      fi
    elif [[ "$config_changed" == 1 ]]; then
      if ! "${compose[@]}" -p "$core_project" --profile edge stop nginx; then
        restored=0
      fi
    fi

    if [[ "$restored" == 1 && "$candidate_started" == 1 && "$previous_project" != "$project" ]]; then
      "${compose[@]}" -p "$project" down || true
    elif [[ "$restored" == 0 ]]; then
      echo "Проверьте nginx вручную: оба комплекта приложений оставлены запущенными." >&2
    fi
  elif [[ "$status" != 0 ]]; then
    echo "Новая версия активна, но остановка предыдущих приложений завершилась ошибкой." >&2
  fi

  rm -rf -- "$backup_dir"
  exit "$status"
}
trap finish EXIT
trap 'exit 130' INT
trap 'exit 143' TERM HUP

for network in totallyswipes-edge totallyswipes-data; do
  if ! docker network inspect "$network" >/dev/null 2>&1; then
    if [[ "$network" == totallyswipes-data ]]; then
      docker network create --internal "$network"
    else
      docker network create "$network"
    fi
  fi
done

# Хранилища всегда принадлежат одному постоянному проекту.
"${compose[@]}" -p "$core_project" up -d --no-recreate --wait postgres redis

# MinIO применяет новые учётные данные из env при пересоздании контейнера.
"${compose[@]}" -p "$core_project" up -d --no-deps --wait minio

"${compose[@]}" -p "$project" pull "${apps[@]}"
"${compose[@]}" -p "$core_project" --profile tools run --rm -T --no-deps migrate
"${compose[@]}" -p "$core_project" exec -T postgres \
  sh -ec 'exec psql -X -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1' \
  < "$release_dir/db/security/roles.sql"

# Пароли берём из env сервисов и передаём в psql через stdin без вывода в лог.
if ! python3 "$release_dir/scripts/deploy/postgres-passwords.py" "$release_dir/deploy/env" \
  | "${compose[@]}" -p "$core_project" exec -T postgres \
    sh -ec 'exec psql -X -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1' \
    >/dev/null 2>&1; then
  echo "Не удалось установить пароли ролей PostgreSQL из env сервисов" >&2
  exit 1
fi

"${compose[@]}" -p "$core_project" exec -T postgres \
  sh -ec 'exec psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1' \
  < "$release_dir/db/security/grants.sql"

# Другой project позволяет проверить новую версию, пока предыдущая ещё работает.
candidate_started=1
"${compose[@]}" -p "$project" up -d --no-build --no-deps --wait --wait-timeout 180 "${apps[@]}"
"${compose[@]}" -p "$project" exec -T gateway healthcheck \
  -http http://127.0.0.1:8080/health

# Директории монтируются в nginx целиком, поэтому замена файлов видна контейнеру.
config_changed=1
cp "$release_dir/deploy/nginx/default.conf" "$NGINX_CONFIG_DIR/default.conf.next"
mv "$NGINX_CONFIG_DIR/default.conf.next" "$NGINX_CONFIG_DIR/default.conf"
printf 'server gateway-%s:8080 resolve;\n' "$DEPLOYMENT_ID" > "$NGINX_UPSTREAM_DIR/gateway.conf.next"
mv "$NGINX_UPSTREAM_DIR/gateway.conf.next" "$NGINX_UPSTREAM_DIR/gateway.conf"
"${compose[@]}" -p "$core_project" --profile edge run --rm --no-deps nginx nginx -t

if [[ "$proxy_running" == 1 ]]; then
  "${compose[@]}" -p "$core_project" --profile edge up -d --no-build --no-deps --no-recreate nginx
  docker exec totallyswipes-nginx nginx -s reload
else
  # После неудачного запуска пересоздаём контейнер вместе с его сетевым окружением.
  "${compose[@]}" -p "$core_project" --profile edge up -d --no-build --no-deps --force-recreate nginx
fi

routed=0
for attempt in {1..15}; do
  if http_status=$(curl --noproxy '*' --fail --silent --show-error --max-time 5 \
    --resolve "$public_host:443:127.0.0.1" --write-out '%{http_code}' --output /dev/null "https://$public_host/health"); then
    if [[ "$http_status" == 200 ]]; then
      routed=1
      break
    fi
  fi
  sleep 1
done

if [[ "$routed" != 1 ]]; then
  echo "Проверка /health через nginx не вернула HTTP 200" >&2
  exit 1
fi

printf '%s\n' "$project" > "$app_dir/current-project.next"
mv "$app_dir/current-project.next" "$app_dir/current-project"
completed=1

# Завершаем только приложения: api у монолита или пять прежних микросервисов.
if [[ "$previous_project" != "$project" ]]; then
  for service in gateway api matching chat profile auth; do
    old_ids=$(docker ps -q \
      --filter "label=com.docker.compose.project=$previous_project" \
      --filter "label=com.docker.compose.service=$service")
    if [[ -n "$old_ids" ]]; then
      while IFS= read -r container; do
        docker stop --time 30 "$container"
      done <<< "$old_ids"
    fi
  done
fi

echo "Деплой завершён: $project. Предыдущие приложения остановлены, хранилища сохранены."
