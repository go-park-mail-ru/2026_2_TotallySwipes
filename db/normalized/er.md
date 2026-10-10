# ER-диаграмма Totally Swipes

Диаграмма отражает ограничения SQL из [миграций](../migrations/). Назначение полей и бизнес-правила описаны в [relations.md](relations.md).

`||` — ровно одна запись, `o|` — ноль или одна, `o{` — ноль или много. `PK` — первичный ключ, `FK` — внешний ключ, `UK` — уникальный ключ. Комментарии `U1` обозначают поля одного составного уникального ключа внутри таблицы.

У пользователя может быть **0..1 анкета**, у каждой анкеты — ровно один пользователь. `UNIQUE (profile.user_id)` сохраняется. Backend при регистрации создаёт пользователя и анкету в одной транзакции. Профиль без версий SQL также допускает, поэтому связь с `profile_version` показана как `0..N`.

Теги связаны с `profile_version` через `profile_tag.profile_version_id`. Каждая версия имеет собственный набор тегов, включая пустой; составной первичный ключ — `(profile_version_id, tag_id)`. Прямой связи `profile_tag` с `profile` нет.

Фотографии и лайки остаются связаны с `profile`. Новая версия анкеты не требует переноса лайков; версия анкеты на момент лайка не фиксируется. Исторические версии сохраняют поля анкеты и наборы тегов, но не состав фотографий.

Ограничения, не выражаемые линиями ER-диаграммы:

- `subscription_no_overlap` запрещает пересечение периодов `[starts_at, ends_at)` одного пользователя через `EXCLUDE USING gist`; соседние периоды разрешены. `CHECK` требует конечных границ и положительной длительности.
- Триггеры запрещают `UPDATE` строк `user_answer`, `profile_tag`, `profile_like` и `profile_version`.
- `profile_psycho_no_rewrite` запрещает обновление готового результата и изменение идентификаторов и времени записи незавершённого. Пять показателей либо все `NULL`, либо все заполнены — это обеспечивает `CHECK`.
- Триггеры запрещают прямой `DELETE` строк `user_answer`, `profile_tag`, `profile_like`, `profile_version` и `profile_psycho`. Каскадное удаление при удалении владельца сохраняется. Добавление связей к старым версиям и ответов к готовым результатам пока не запрещено.

`profile_psycho.personality_type` хранит тип, показанный после теста. Допустим `NULL`; непустое значение требует заполненных показателей и защищено от изменения вместе с готовым результатом.

```mermaid
erDiagram
    user {
        bigint id PK
        text email UK
        text password_hash
        timestamptz created_at
        timestamptz updated_at
    }

    profile {
        bigint id PK
        bigint user_id FK, UK
        timestamptz created_at
        timestamptz updated_at
    }

    profile_version {
        bigint id PK
        bigint profile_id FK, UK "U1"
        integer revision UK "U1"
        timestamptz recorded_at
        text name "NULL"
        date birth_date "NULL"
        text sex "NULL"
        text dating_goal "NULL"
        text about_me
        text education
        text work
        text smoking
        text alcohol
        smallint height
    }

    search_filter {
        bigint user_id PK, FK
        text sex
        smallint age_from
        smallint age_to
        timestamptz created_at
        timestamptz updated_at
    }

    profile_psycho {
        bigint id PK
        bigint profile_id FK, UK "U1"
        integer revision UK "U1"
        timestamptz recorded_at
        numeric openness
        numeric conscientiousness
        numeric extraversion
        numeric agreeableness
        numeric neuroticism
        text personality_type "NULL или один из 16 типов"
    }

    user_answer {
        bigint profile_psycho_id PK, FK
        smallint question_no PK "1..10"
        smallint answer_value
        timestamptz created_at
    }

    plan {
        bigint id PK
        text name UK
        boolean ads_enabled
        timestamptz created_at
        timestamptz updated_at
    }

    subscription {
        bigint id PK
        bigint user_id FK
        bigint plan_id FK
        timestamptz starts_at
        timestamptz ends_at
        timestamptz created_at
        timestamptz updated_at
    }

    tag {
        bigint id PK
        text name UK
        timestamptz created_at
        timestamptz updated_at
    }

    profile_tag {
        bigint profile_version_id PK, FK
        bigint tag_id PK, FK
        timestamptz created_at
    }

    report_type {
        bigint id PK
        text name UK
        timestamptz created_at
        timestamptz updated_at
    }

    report {
        bigint id PK
        bigint author_id FK, UK "U1"
        bigint reported_user_id FK, UK "U1"
        bigint report_type_id FK, UK "U1"
        timestamptz created_at
        timestamptz updated_at
    }

    photo {
        bigint id PK
        bigint profile_id FK, UK "U1"
        text storage_key UK
        smallint position UK "U1"
        timestamptz created_at
        timestamptz updated_at
    }

    photo_comment {
        bigint id PK
        bigint author_id FK
        bigint photo_id FK
        text body
        timestamptz created_at
        timestamptz updated_at
    }

    profile_like {
        bigint id PK
        bigint author_id FK, UK "U1"
        bigint profile_id FK, UK "U1"
        timestamptz created_at
    }

    match {
        bigint id PK
        bigint first_user_id FK, UK "U1"
        bigint second_user_id FK, UK "U1"
        boolean access_to_message
        timestamptz created_at
        timestamptz updated_at
    }

    message {
        bigint id PK
        bigint match_id FK
        bigint sender_id FK
        text body
        timestamptz created_at
        timestamptz updated_at
    }

    user ||..o| profile : "user_id"
    user ||..o| search_filter : "user_id"
    profile ||..o{ profile_version : "profile_id"
    profile ||..o{ profile_psycho : "profile_id"
    profile_psycho ||--o{ user_answer : "profile_psycho_id"
    user ||..o{ subscription : "user_id"
    plan ||..o{ subscription : "plan_id"
    profile_version ||--o{ profile_tag : "profile_version_id"
    tag ||--o{ profile_tag : "tag_id"
    user ||..o{ report : "author_id"
    user ||..o{ report : "reported_user_id"
    report_type ||..o{ report : "report_type_id"
    profile ||..o{ photo : "profile_id"
    user ||..o{ photo_comment : "author_id"
    photo ||..o{ photo_comment : "photo_id"
    user ||..o{ profile_like : "author_id"
    profile ||..o{ profile_like : "profile_id"
    user ||..o{ match : "first_user_id"
    user ||..o{ match : "second_user_id"
    match ||..o{ message : "match_id"
    user ||..o{ message : "sender_id"
```
