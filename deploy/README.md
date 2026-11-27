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

Сервисы `auth` и `profile` реализованы: это gRPC-серверы поверх общей логики из `internal/`
(репозитории, сервисы, хранилища). Точки входа `matching` и `chat` пока пустые и завершаются сразу,
поэтому ручки ленты, мэтчей и чата через Gateway ещё не работают.

- `profile` — анкета, фото (загрузка, удаление, порядок, отдача файла для `/media/...`),
  психологический тест, фильтр поиска, геолокация и внутренние RPC для matching/chat:
  `SearchCandidates`, `GetProfilesBatch`, `GetProfileChatSummaries`.
- `auth` — регистрация, вход, refresh и logout. Список незаполненных полей анкеты при входе
  запрашивает у `profile` по gRPC, поэтому стартует после него.

Общий слой gRPC:

- `internal/grpcserver` — интерсепторы (recovery, request ID из метаданных `x-request-id`,
  логирование, перевод ошибок), health-сервис, reflection и graceful shutdown по `SHUTDOWN_TIMEOUT`;
- `internal/grpcclient` — клиентское соединение: проброс request ID, таймаут `GRPC_REQUEST_TIMEOUT`,
  восстановление доменных ошибок;
- `internal/grpcerr` — реестр доменных ошибок `internal/model` ↔ gRPC-код и `ErrorInfo.reason`.
  Благодаря ему Gateway получает те же `model.Err*`, что и монолит, и отвечает теми же HTTP-кодами.
  Новую доменную ошибку нужно добавить в реестр, иначе клиент получит `Internal`.

Сообщения gRPC ограничены `validate.PhotoMaxSize + 1 МБ` на обеих сторонах, чтобы проходили фото.
Readiness gRPC в Compose не настроен: service_started проверяет запуск процесса, а не готовность RPC.
Проверить сервис вручную можно через health/reflection, например
`grpcurl -plaintext profile:50051 list` из контейнера в сети `backend`.

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
