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

Redis пока запускается с постоянными параметрами из Compose. Конфигурация nginx находится
в `deploy/nginx/default.conf`; публичный домен — `totallyswipes.ru`.

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
Перед запуском Gateway создать общую сеть с фронтендом, как описано ниже.

```bash
docker compose -f docker-compose.microservices.yml up -d --build auth profile matching chat gateway
```

Точки входа Chat, Matching и WebSocket Gateway подключены; Auth и Profile ещё требуют
доработки. Проверка nginx не подтверждает работоспособность всего приложения.
Readiness gRPC в Compose пока не настроен: service_started проверяет запуск процесса,
а не готовность RPC.

`GRPC_REQUEST_TIMEOUT` ограничивает длительность обычных RPC Gateway, включая команды
из WebSocket. Подписка `SubscribeEvents` живёт до закрытия соединения и этим таймаутом
не ограничивается. Более короткий дедлайн входящего контекста сохраняется.

При SIGINT/SIGTERM Gateway перестаёт принимать запросы, отправляет открытым WebSocket
код закрытия 1001, отменяет их подписки и ждёт завершения циклов. Обычные HTTP-запросы
могут завершиться в пределах `SHUTDOWN_TIMEOUT`; после его истечения соединения
закрываются принудительно. Затем закрываются gRPC-клиенты. В Compose для Gateway
задан `stop_grace_period: 15s`, больше стандартного `SHUTDOWN_TIMEOUT=10s`.
При увеличении `SHUTDOWN_TIMEOUT` нужно также увеличить `stop_grace_period`.

## Nginx и сеть

Nginx находится в профиле `edge`, поэтому по умолчанию не запускается. Он завершает TLS
и направляет запросы:

- `https://totallyswipes.ru/api/...` — HTTP API Gateway;
- `https://totallyswipes.ru/media/...` — фотографии через Gateway;
- `wss://totallyswipes.ru/ws` — WebSocket Gateway;
- остальные пути — HTTP-сервер фронтенда `frontend:80`.

В `proxy_pass` для Gateway нет завершающего `/`: исходный путь запроса сохраняется.
`proxy_read_timeout 90s` у WebSocket ограничивает паузу между чтениями от Gateway,
а не полную длительность соединения. Gateway отправляет ping каждые 25 секунд.

На сервере должны быть DNS-запись домена на IP этой машины, открытые входящие порты
80/443 и действующий сертификат именно для `totallyswipes.ru`. Compose подключает
`certs/fullchain.pem` и `certs/privkey.pem` в nginx; эти файлы не коммитятся.
После обновления сертификата выполнить перезагрузку конфигурации nginx.

Фронтенд и бэкенд запускаются разными Compose-проектами. Один раз создать общую сеть:

```bash
docker network create totallyswipes-edge
```

Бэкенд уже подключает Gateway и nginx к этой внешней сети. В Compose фронтенда добавить
следующий фрагмент к существующему сервису, сохранив его остальные параметры и сети:

```yaml
services:
  frontend:
    networks:
      default:
      edge:
        aliases:
          - frontend

networks:
  default:
  edge:
    name: totallyswipes-edge
    external: true
```

Здесь принято, что HTTP-сервер фронтенда слушает порт 80 внутри контейнера. Если порт
другой, изменить `server frontend:80 resolve` в nginx. Переходы по URL SPA должен
обрабатывать HTTP-сервер фронтенда (например, через `try_files ... /index.html`).
Только внешний nginx публикует порты 80/443 хоста: публикацию этих портов у фронтенда
нужно убрать. `localhost` внутри nginx указывает на сам контейнер nginx.

В локальном `deploy/env/gateway.env` задать:

```dotenv
CORS_ORIGINS=https://totallyswipes.ru
WS_ALLOWED_ORIGINS=https://totallyswipes.ru
COOKIE_SECURE=true
TLS_CERT_FILE=
TLS_KEY_FILE=
```

Origin — адрес страницы браузера с протоколом HTTPS, без пути и завершающего `/`.
Это не `wss://totallyswipes.ru/ws`. Nginx передаёт исходный Origin в Gateway;
проверки CORS и WebSocket Origin выполняются в Gateway. При одном домене фронтенда
и API дополнительные CORS-заголовки от nginx не нужны. В прежнем шаблоне был домен
`totally-swipes.ru` с дефисом; локальные env-файлы нужно обновить под выбранный домен.

Gateway работает без собственного TLS. Cookie Secure включён в расчёте на внешний HTTPS;
для локального HTTP его потребуется отключить.

После подготовки сертификатов и запуска фронтенда и Gateway проверить и запустить nginx:

```bash
docker compose -f docker-compose.microservices.yml --profile edge run --rm --no-deps nginx nginx -t
docker compose -f docker-compose.microservices.yml --profile edge up -d nginx
```

Перезагрузка после изменения конфигурации или сертификата:

```bash
docker compose -f docker-compose.microservices.yml --profile edge exec nginx nginx -t
docker compose -f docker-compose.microservices.yml --profile edge exec nginx nginx -s reload
```

Сети:

- `edge` (`totallyswipes-edge`) — Nginx, Gateway и фронтенд из отдельного Compose;
- `backend` — Gateway и микросервисы, с исходящим доступом к внешним API;
- `data` — сервисы и хранилища, внутренняя сеть Docker.

Gateway не подключён к data-сети. PostgreSQL, Redis, MinIO и gRPC-порты не публикуются
на хост. Только Nginx публикует 80/443, когда включён его профиль.

Проверка конфигурации без запуска:

```bash
docker compose -f docker-compose.microservices.yml --profile edge --profile tools config --quiet
```
