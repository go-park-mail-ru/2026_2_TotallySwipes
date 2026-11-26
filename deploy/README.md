# Конфигурация микросервисов

`docker-compose.microservices.yml` — отдельная конфигурация, основной Compose монолита
не меняется. Имя проекта, сети и volumes отдельные: `totallyswipes-microservices`.

## Env-файлы

В `deploy/env` находятся шаблоны `*.env.example` и локальные копии `*.env`.
Локальные копии исключены из Git и Docker build context. На другом компьютере
создать их из шаблонов:

```bash
for template in deploy/env/*.env.example; do
  target="${template%.example}"
  if [ ! -e "$target" ]; then
    cp "$template" "$target"
    chmod 600 "$target"
  fi
done
```

Заменить значения `replace-*` перед использованием. Реальные секреты не коммитить.

- `postgres.env` — администратор БД и подключение мигратора.
- `gateway.env` — HTTP, JWT-проверка, адреса gRPC, настройки WebSocket.
- `auth.env` — роль `auth_app`, Redis, выпуск JWT.
- `profile.env` — роль `profile_app`, Redis, геокодер, фотографии.
- `matching.env` — роль `matching_app`, Redis, Profile, Chat и Timepad.
- `chat.env` — роль `chat_app`, Profile и медиа.
- `minio.env` — административная учётная запись MinIO.

`JWT_SECRET` в Auth и Gateway должен совпадать. Пароли в DATABASE_URL должны совпадать
с паролями соответствующих ролей PostgreSQL. Спецсимволы в паролях внутри URL кодируются.
Файлы env задаются через env_file; ссылки на контейнеры используют DNS-имена Compose,
например `postgres:5432`, а не localhost.

Redis пока запускается с постоянными параметрами из Compose. Nginx env-файл пока не нужен:
его конфигурация будет отдельно написана в `deploy/nginx/default.conf`.

## Подготовка инфраструктуры

```bash
docker compose -f docker-compose.microservices.yml up -d postgres redis minio
docker compose -f docker-compose.microservices.yml run --rm migrate
```

На новой БД создать роли, затем выдать права:

```bash
docker compose -f docker-compose.microservices.yml exec -T postgres \
  sh -c 'exec psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1' \
  < db/security/roles.sql

docker compose -f docker-compose.microservices.yml exec -T postgres \
  sh -c 'exec psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1' \
  < db/security/grants.sql
```

`roles.sql` выполняется один раз; повторный запуск с существующими ролями завершится
ошибкой. `grants.sql` можно применять повторно после новых миграций.

Далее войти администратором через psql и задать пароли через `\password auth_app`,
`\password profile_app`, `\password matching_app`, `\password chat_app`, `\password monitor`.
Сами env-файлы пользователей БД не создают и не устанавливают им пароли.

Пользователи MinIO `profile_storage` и `chat_storage` также должны быть созданы отдельно
с доступом к своим bucket: `photos` и `chat-media`. Значения в env — предложенный контракт;
автоматическая подготовка пользователей и политик MinIO в этот Compose пока не включена.

## Запуск приложений

Общий `deploy/Dockerfile.microservices` собирает выбранную точку входа через SERVICE_PATH.

```bash
docker compose -f docker-compose.microservices.yml up -d --build auth profile matching chat gateway
```

Это заготовка окружения: точки входа gRPC-сервисов сейчас пустые и завершаются сразу;
Gateway тоже ещё дорабатывается. Стек нельзя считать работающим приложением до реализации
серверов и проверки сборки. Переменные GRPC_PORT и LOG_LEVEL должны читаться будущими
конфигами сервисов; Compose только передаёт их окружению. Readiness gRPC пока не настроен:
service_started проверяет запуск процесса, а не готовность RPC.

## Nginx и сеть

`deploy/nginx/default.conf` намеренно пустой. Nginx находится в профиле `edge`, поэтому
по умолчанию не запускается. После отдельной настройки конфигурации он будет завершать
TLS и отправлять HTTP/WS-запросы в `gateway:8080`.

Gateway работает без собственного TLS. Cookie Secure включён в расчёте на внешний HTTPS;
для локального HTTP его потребуется отключить.

Сети:

- `edge` — Nginx и Gateway;
- `backend` — Gateway и микросервисы, с исходящим доступом к внешним API;
- `data` — сервисы и хранилища, внутренняя сеть Docker.

Gateway не подключён к data-сети. PostgreSQL, Redis, MinIO и gRPC-порты не публикуются
на хост. Только Nginx публикует 80/443, когда включён его профиль.

Проверка конфигурации без запуска:

```bash
docker compose -f docker-compose.microservices.yml --profile edge --profile tools config --quiet
```
