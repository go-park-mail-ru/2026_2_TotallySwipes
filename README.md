# 2026_2_TotallySwipes

Репозиторий бэкенда команды Тотали Свайпс с проектом Tinder 🚀

## Участники команды

1. [Домаскин Егор](https://github.com/desitas1701)
2. [Прохоров Савелий](https://github.com/ssavav)
3. [Неделин Никита](https://github.com/zazaza5817)
4. [Поздышев Александр](https://github.com/chaykanet)

## Внешние ссылки

* [Figma](https://www.figma.com/)
* [Frontend](https://github.com/frontend-park-mail-ru/2026_2_TotallySwipes)
* [Deploy](http://161.104.105.207)
* [Jira](https://totallyswipes.atlassian.net/jira/software/projects/TS/boards/1?filter=&groupBy=none)

## Запуск проекта на машине

Используется один файл `docker-compose.microservices.yml`.

Нужны Docker с Compose 2.30+ и доступ к реестрам образов. На сервере порты 80/443 должны
быть свободны и доступны снаружи. Все команды ниже выполняются из корня репозитория.

### 1. Получить код и подготовить env

```bash
git clone --branch dev3 https://github.com/go-park-mail-ru/2026_2_TotallySwipes.git
cd 2026_2_TotallySwipes

for template in deploy/env/*.env.example; do
  target="${template%.example}"
  if [ ! -e "$target" ]; then
    cp "$template" "$target"
    chmod 600 "$target"
  fi
done
```

Заполнить созданные `deploy/env/*.env`, заменив значения `replace-*`:

| Файл | Что настроить |
| --- | --- |
| `postgres.env` | Администратор PostgreSQL, имя БД и его `DATABASE_URL` для миграций |
| `auth.env` | Подключение `auth_app`, Redis, JWT-секрет и адрес Profile |
| `profile.env` | Подключение `profile_app`, Redis, MinIO, ключ геокодера и `PUBLIC_URL` |
| `matching.env` | Подключение `matching_app`, Redis, адреса Profile/Chat и токен Timepad |
| `chat.env` | Подключение `chat_app`, адрес Profile и MinIO для медиа |
| `gateway.env` | Адреса gRPC-сервисов, JWT-секрет, Origin и настройки HTTP/WebSocket |
| `minio.env` | Администратор MinIO: `MINIO_ROOT_USER` и `MINIO_ROOT_PASSWORD` |

`JWT_SECRET` у Auth и Gateway должен совпадать. В `DATABASE_URL` каждого приложения
указать пароль соответствующей роли PostgreSQL; роли создаются на шаге 3.
Спецсимволы пароля внутри URL кодируются. Внутри Docker используются имена
`postgres`, `redis`, `minio`, `auth`, `profile`, `matching`, `chat`, а не `localhost`.
Настройки старого монолита из корневого `.env` этим запуском не используются как
`env_file` приложений; их настройки находятся в `deploy/env/`.

### 2. Создать сеть и запустить хранилища

Один раз создать общие сети для фронтенда/Gateway и хранилищ.
Если соответствующая сеть уже есть, повторять её создание не нужно:

```bash
docker network create totallyswipes-edge
docker network create --internal totallyswipes-data

docker compose -f docker-compose.microservices.yml up -d --wait postgres redis minio
```

### 3. Применить миграции и создать роли PostgreSQL

```bash
docker compose -f docker-compose.microservices.yml --profile tools run --rm migrate
```

На новой БД один раз создать роли:

```bash
docker compose -f docker-compose.microservices.yml exec -T postgres \
  sh -c 'exec psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1' \
  < db/security/roles.sql
```

Открыть psql для установки паролей:

```bash
docker compose -f docker-compose.microservices.yml exec postgres \
  sh -c 'exec psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"'
```

Внутри psql выполнить команды. Пароли четырёх приложений должны совпадать
с их env-файлами; для роли мониторинга задать отдельный пароль:

```text
\password auth_app
\password profile_app
\password matching_app
\password chat_app
\password monitor
\q
```

Выдать права сервисам:

```bash
docker compose -f docker-compose.microservices.yml exec -T postgres \
  sh -c 'exec psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1' \
  < db/security/grants.sql
```

Если роли уже созданы, пропустить `roles.sql` и установку паролей.
После новых миграций `grants.sql` можно применять повторно.

### 4. Подготовить MinIO

Шаблоны Profile и Chat используют пользователей `profile_storage` и `chat_storage`.
В MinIO нужно создать эти учётные записи, bucket `photos` и `chat-media`, выдать
каждому пользователю права чтения, записи и удаления объектов в своём bucket.
Пароли пользователей должны совпадать с `MINIO_SECRET_KEY` соответствующего сервиса.
Env-файлы сами не создают пользователей и политики MinIO.

Для первого локального запуска можно использовать администратора MinIO:
в `profile.env` и `chat.env` задать `MINIO_ACCESS_KEY` и `MINIO_SECRET_KEY`, совпадающие
с `MINIO_ROOT_USER` и `MINIO_ROOT_PASSWORD` из `minio.env`. Сервисы создадут отсутствующие
bucket при запуске. Для серверного запуска используются отдельные пользователи,
описанные выше.

### 5. Подготовить nginx и фронтенд

Текущий nginx настроен на `totallyswipes.ru`. Домен должен указывать на IP машины,
а сертификат и ключ должны лежать в файлах:

```text
certs/fullchain.pem
certs/privkey.pem
```

В `gateway.env`:

```dotenv
CORS_ORIGINS=https://totallyswipes.ru
WS_ALLOWED_ORIGINS=https://totallyswipes.ru
COOKIE_SECURE=true
TLS_CERT_FILE=
TLS_KEY_FILE=
```

В `profile.env` — `PUBLIC_URL=https://totallyswipes.ru`. HTTPS/WSS завершается
на nginx, соединение nginx → Gateway использует HTTP/WS.

В Compose фронтенда добавить общую сеть к существующему сервису, сохранив его
остальные настройки. В этой сети нужен alias `frontend` и внутренний HTTP-порт 80:

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

Если внутренний порт фронтенда другой, изменить `frontend:80` в
`deploy/nginx/default.conf`. Только внешний nginx публикует порты хоста 80/443;
у контейнера фронтенда эти публикации нужно убрать. Запустить фронтенд его командой
`docker compose up -d --build` из репозитория фронтенда.

При другом домене изменить `server_name` и HTTPS-редирект в nginx, сертификат,
`PUBLIC_URL`, `CORS_ORIGINS` и `WS_ALLOWED_ORIGINS`.

### 6. Запустить весь бэкенд

```bash
docker compose -f docker-compose.microservices.yml --profile edge up -d --build
```

`--profile edge` включает nginx. Без этого параметра приложения и хранилища
запустятся, но внешний nginx не будет поднят. Порты Gateway и хранилищ отдельно
на хост не публикуются.

Проверить состояние и посмотреть логи:

```bash
docker compose -f docker-compose.microservices.yml --profile edge ps
docker compose -f docker-compose.microservices.yml --profile edge logs -f
curl --fail https://totallyswipes.ru/health
```

После запуска доступны:

- фронтенд: `https://totallyswipes.ru/`;
- API: `https://totallyswipes.ru/api/v1/...`;
- WebSocket: `wss://totallyswipes.ru/ws`.

Для WebSocket требуется cookie `access_token`, полученная при входе.
Сеть `totallyswipes-edge` общая для двух Compose-проектов; внутренние сети
`backend` создаётся автоматически, `data` (`totallyswipes-data`) — на шаге 2.

### Повторный запуск и остановка

После изменения кода повторить команду запуска с `--build`. Новые миграции сначала
применить командой из шага 3, затем повторно выполнить `grants.sql`.

```bash
docker compose -f docker-compose.microservices.yml --profile edge down
```

Остановка без `--volumes` сохраняет данные PostgreSQL, Redis, MinIO и логи.
Подробные настройки сервисов находятся в [deploy/README.md](deploy/README.md).

## Старый монолит и команды Makefile

Для монолита используются корневой `.env`, `docker-compose.yml` и точка входа
`cmd/api`. Его запуск описывается отдельно от микросервисов:

- `make run` — запустить монолит на хосте; адреса хранилищ в `.env` должны быть доступны с хоста;
- `make up` — запустить PostgreSQL и Redis для разработки монолита;
- `make up-all` — запустить Compose монолита;
- `make migrate-up` / `make migrate-down` — применить/откатить миграции через Compose монолита;
- `make migrate-create name=...` — создать пару миграций в `db/migrations/`;
- `make test` — выполнить Go-тесты;
- `make proto` — сгенерировать protobuf для всех сервисов; нужны `protoc`, `protoc-gen-go`, `protoc-gen-go-grpc`.

## Правила оформления Pull Requests

1. У вас будет два типа веток: фича(когда создаете что-то новое, модифицируете) и багфикс(при проверке работы сервиса выявлена ошибка либо техническая, либо бизнес-логики - это баг; его нужно фиксить). Соответственно названия веток: `feature/`(номер задачи и краткое описание(максимум пять слов через `-` с маленькой буквы на английском)) и `bugfix/`(номер бага и краткое описание(максимум пять слов через `-` с маленькой буквы на английском)).
2. Где будем все планировать еще не определились.
3. Название PR соответствует названию задачи.
4. В описании PR можно либо краткое описание что сделали, либо просто Ctrl+C Ctrl+V самой задачи
5. Для того, чтобы залить в ветку `main` нужен аппрув [Глеба](https://t.me/Shadow_Nair)

### Интеграционные тесты

`make test-integration` поднимает отдельные PostgreSQL/PostGIS и Redis через Docker
Compose, применяет все миграции и запускает тесты с `-tags=integration -race`.
Нужны Go и работающий Docker с Compose. Порты назначаются автоматически;
каждый запуск получает отдельный Compose project. Контейнеры и данные удаляются
при завершении, включая ошибку тестов. Рабочие базы приложения не используются.
Redis-тесты работают с настоящим сервером, без miniredis и моков.

Обычный `go test ./...` не включает интеграционные тесты с тегом `integration`.
Workflow выполняет тесты без этого тега с детектором гонок и формирует отчёт покрытия.
При ручном запуске с тегом нужны `TEST_HISTORY_DATABASE_URL` (PostGIS со всеми
миграциями) и `TEST_REDIS_ADDR`. Используйте только выделенную тестовую среду:
Redis-тесты очищают БД 15, PostgreSQL-тестам нужна пустая база с миграциями.
