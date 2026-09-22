# 2026_2_TotallySwipes

Репозиторий фронтенда команды Тотали Свайпс с проектом Tinder 🚀

## Участники команды

1. [Домаскин Егор](https://github.com/desitas1701)
2. [Прохоров Савелий](https://github.com/ssavav)
3. [Неделин Никита](https://github.com/zazaza5817)
4. [Поздышев Александр](https://github.com/chaykanet)

## Внешние ссылки

* [Figma](https://www.figma.com/)
* [Frontend](https://github.com/frontend-park-mail-ru/2026_2_TotallySwipes)
* [Deploy](https://meow)
* [Jira](https:/meow)

## Бэкенд: команды Makefile

Перед первым запуском скопировать `local.env.example` → `local.env` и `docker.env.example` → `docker.env`.

* `make run` - приложение на хосте (`go run ./cmd/api`), берёт конфиг из `local.env` (Postgres по `localhost`)
* `make up` - поднять только Postgres в докере, для локальной разработки (используется вместе с `make run`)
* `make up-all` - поднять всё в докере (`docker compose up --build`): Postgres + приложение, конфиг из `docker.env` (Postgres по имени сервиса)
* `make migrate-up` / `make migrate-down` - накатить/откатить миграции через `migrate/migrate` в докере
* `make migrate-create name=...` - создать новую пару файлов миграции в `migrations/`
* `make test` - прогнать тесты (`go test ./...`)

## Правила оформления Pull Requests

1. У вас будет два типа веток: фича(когда создаете что-то новое, модифицируете) и багфикс(при проверке работы сервиса выявлена ошибка либо техническая, либо бизнес-логики - это баг; его нужно фиксить). Соответственно названия веток: `feature/`(номер задачи и краткое описание(максимум пять слов через `-` с маленькой буквы на английском)) и `bugfix/`(номер бага и краткое описание(максимум пять слов через `-` с маленькой буквы на английском)).
2. Где будем все планировать еще не определились.
3. Название PR соответствует названию задачи.
4. В описании PR можно либо краткое описание что сделали, либо просто Ctrl+C Ctrl+V самой задачи
5. Для того, чтобы залить в ветку `main` нужен аппрув [Глеба](https://t.me/Shadow_Nair)