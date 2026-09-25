# Отношения БД Totally Swipes

ER: [er.puml](er.puml). Типы и ограничения: [schema.dbml](schema.dbml).

Все идентификаторы генерируются через `GENERATED ALWAYS AS IDENTITY`.

В таблицах, кроме `profile_version` и `profile_psycho`, для технических временных полей в DDL используются:

```
created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
```

В `profile_version` и `profile_psycho` вместо этой пары используется только
`recorded_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP`.
Все эти временные значения должны быть конечными (`isfinite`). Для пары
`created_at` / `updated_at` действует `updated_at >= created_at`.
`DEFAULT` задаёт время вставки; обновление `updated_at` выполняется явно backend-логикой либо триггером.

Межтабличные ограничения, которые нельзя выразить через `PK`, `FK`, `UNIQUE` или `CHECK`, реализуются backend-логикой либо триггерами.

---

## 1. user

| Атрибут         | Тип    | Ограничения      |
| --------------- | ------ | ---------------- |
| `id`            | bigint | PK, NOT NULL     |
| `name`          | text   | NOT NULL         |
| `email`         | text   | NOT NULL, UNIQUE |
| `password_hash` | text   | NOT NULL         |
| `birth_date`    | date   | NOT NULL         |

CHECK:

```
id > 0
char_length(name) BETWEEN 1 AND 100 AND name ~ '[^[:space:]]'
char_length(email) BETWEEN 1 AND 254 AND email !~ '[[:space:]]'
password_hash ~ '[^[:space:]]'
isfinite(birth_date)
```

Кандидатные ключи:

```
{id}
{email}
```

---

## 2. profile

| Атрибут   | Тип    | Ограничения                     |
| --------- | ------ | ------------------------------- |
| `id`      | bigint | PK, NOT NULL                    |
| `user_id` | bigint | NOT NULL, UNIQUE, FK -> user.id |
| `created_at` | timestamptz | NOT NULL, DEFAULT CURRENT_TIMESTAMP |
| `updated_at` | timestamptz | NOT NULL, DEFAULT CURRENT_TIMESTAMP |

CHECK:

```
id > 0
isfinite(created_at)
isfinite(updated_at)
updated_at >= created_at
```

FK:

```
profile.user_id -> user.id
ON DELETE CASCADE
ON UPDATE RESTRICT
```

Кандидатные ключи:

```
{id}
{user_id}
```

`created_at` — время создания анкеты. `updated_at` — время последнего изменения
анкеты: добавления новой `profile_version` либо сохранения готового результата
теста, который отображается на карточке пользователя. Начало прохождения и
сохранение промежуточных ответов не меняют `profile.updated_at`.
Сохранение версии или готового результата и обновление `profile.updated_at`
выполняются в одной транзакции.

---

## 3. profile_version

`recorded_at` — время создания конкретной версии; версия неизменяема.
Полей `created_at` и `updated_at` нет. Актуальная версия определяется по
максимальному `revision`, а не по времени. Добавление версии обновляет
`profile.updated_at` в той же транзакции.

| Атрибут           | Тип         | Ограничения                         |
| ----------------- | ----------- | ----------------------------------- |
| `id`              | bigint      | PK, NOT NULL                        |
| `profile_id`      | bigint      | NOT NULL, FK -> profile.id          |
| `revision`        | integer     | NOT NULL                            |
| `recorded_at`     | timestamptz | NOT NULL, DEFAULT CURRENT_TIMESTAMP |
| `birth_date`      | date        | NOT NULL                            |
| `sex`             | text        | NOT NULL                            |
| `dating_goal` | text | NULL (цель знакомства) |
| `search_sex`      | text        | NOT NULL                            |
| `search_age_from` | integer     | NOT NULL                            |
| `search_age_to`   | integer     | NOT NULL                            |

UNIQUE:

```
(profile_id, revision)
```

CHECK:

```
id > 0
revision > 0
isfinite(recorded_at)
isfinite(birth_date)
sex IN ('male', 'female')
search_sex IN ('male', 'female', 'all')
search_age_from >= 18
search_age_to >= search_age_from
```

FK:

```
profile_version.profile_id -> profile.id
ON DELETE CASCADE
ON UPDATE RESTRICT
```

Кандидатные ключи:

```
{id}
{profile_id, revision}
```

---

## 4. test

| Атрибут | Тип    | Ограничения  |
| ------- | ------ | ------------ |
| `id`    | bigint | PK, NOT NULL |
| `name`  | text   | NOT NULL     |

CHECK:

```
id > 0
name ~ '[^[:space:]]'
```

Кандидатный ключ:

```
{id}
```

---

## 5. question

| Атрибут        | Тип     | Ограничения             |
| -------------- | ------- | ----------------------- |
| `id`           | bigint  | PK, NOT NULL            |
| `test_id`      | bigint  | NOT NULL, FK -> test.id |
| `operation_id` | integer | NOT NULL                |
| `body`         | text    | NOT NULL                |

CHECK:

```
id > 0
operation_id > 0
body ~ '[^[:space:]]'
```

FK:

```
question.test_id -> test.id
ON DELETE RESTRICT
ON UPDATE RESTRICT
```

Кандидатный ключ:

```
{id}
```

---

## 6. profile_psycho

`recorded_at` — время создания записи прохождения. Оно не меняется при
сохранении ответов и завершении теста. Полей `created_at` и `updated_at` нет;
отдельное время завершения в текущей модели не хранится.
До завершения все веса равны `NULL`; готовый результат содержит все пять весов
и после завершения неизменяем. Если готовый результат отображается на карточке,
его сохранение обновляет `profile.updated_at` в той же транзакции.
Начало прохождения и промежуточные ответы время изменения анкеты не обновляют.

| Атрибут       | Тип         | Ограничения                         |
| ------------- | ----------- | ----------------------------------- |
| `id`          | bigint      | PK, NOT NULL                        |
| `profile_id`  | bigint      | NOT NULL, FK -> profile.id          |
| `test_id`     | bigint      | NOT NULL, FK -> test.id             |
| `revision`    | integer     | NOT NULL                            |
| `recorded_at` | timestamptz | NOT NULL, DEFAULT CURRENT_TIMESTAMP |
| `openness`    | numeric     | NULL                                |
| `conscientiousness`    | numeric     | NULL                                |
| `extraversion`    | numeric     | NULL                                |
| `agreeableness`    | numeric     | NULL                                |
| `neuroticism`    | numeric     | NULL                                |

UNIQUE:

```
(profile_id, revision)
```

CHECK:

```
id > 0
revision > 0
isfinite(recorded_at)
```

Все веса либо одновременно `NULL`, либо одновременно заданы:

```
(
  openness IS NULL
  AND conscientiousness IS NULL
  AND extraversion IS NULL
  AND agreeableness IS NULL
  AND neuroticism IS NULL
)
OR
(
  openness IS NOT NULL
  AND conscientiousness IS NOT NULL
  AND extraversion IS NOT NULL
  AND agreeableness IS NOT NULL
  AND neuroticism IS NOT NULL
)
```

Каждый заданный показатель (`score` в примере ниже) должен быть конечным числом:

```
score <> 'NaN'::numeric
AND score <> 'Infinity'::numeric
AND score <> '-Infinity'::numeric
```

FK:

```
profile_psycho.profile_id -> profile.id
ON DELETE CASCADE
ON UPDATE RESTRICT
```

```
profile_psycho.test_id -> test.id
ON DELETE RESTRICT
ON UPDATE RESTRICT
```

Кандидатные ключи:

```
{id}
{profile_id, revision}
```

---

## 7. user_answer

| Атрибут             | Тип      | Ограничения      |
| ------------------- | -------- | ---------------- |
| `profile_psycho_id` | bigint   | PK, NOT NULL, FK |
| `question_id`       | bigint   | PK, NOT NULL, FK |
| `answer_value`      | smallint | NOT NULL         |

PK:

```
(profile_psycho_id, question_id)
```

CHECK:

```
answer_value BETWEEN 0 AND 7
```

FK:

```
user_answer.profile_psycho_id -> profile_psycho.id
ON DELETE CASCADE
ON UPDATE RESTRICT
```

```
user_answer.question_id -> question.id
ON DELETE RESTRICT
ON UPDATE RESTRICT
```

Межтабличное правило:

```
question.test_id = profile_psycho.test_id
```

проверяется backend-логикой либо триггером.

---

## 8. plan

| Атрибут       | Тип     | Ограничения            |
| ------------- | ------- | ---------------------- |
| `id`          | bigint  | PK, NOT NULL           |
| `name`        | text    | NOT NULL, UNIQUE       |
| `ads_enabled` | boolean | NOT NULL, DEFAULT true |

CHECK:

```
id > 0
char_length(name) BETWEEN 1 AND 100
name ~ '[^[:space:]]'
```

Кандидатные ключи:

```
{id}
{name}
```

---

## 9. subscription

| Атрибут     | Тип         | Ограничения             |
| ----------- | ----------- | ----------------------- |
| `id`        | bigint      | PK, NOT NULL            |
| `user_id`   | bigint      | NOT NULL, FK -> user.id |
| `plan_id`   | bigint      | NOT NULL, FK -> plan.id |
| `starts_at` | timestamptz | NOT NULL                |
| `ends_at`   | timestamptz | NOT NULL                |

CHECK:

```
id > 0
isfinite(starts_at)
isfinite(ends_at)
ends_at > starts_at
```

FK:

```
subscription.user_id -> user.id
ON DELETE CASCADE
ON UPDATE RESTRICT
```

```
subscription.plan_id -> plan.id
ON DELETE RESTRICT
ON UPDATE RESTRICT
```

Периоды подписок одного пользователя не должны пересекаться; это обеспечивается `EXCLUDE` constraint либо backend/trigger-логикой.

---

## 10. tag

| Атрибут | Тип    | Ограничения      |
| ------- | ------ | ---------------- |
| `id`    | bigint | PK, NOT NULL     |
| `name`  | text   | NOT NULL, UNIQUE |

CHECK:

```
id > 0
char_length(name) BETWEEN 1 AND 100
name ~ '[^[:space:]]'
```

Кандидатные ключи:

```
{id}
{name}
```

---

## 11. profile_tag

| Атрибут      | Тип    | Ограничения      |
| ------------ | ------ | ---------------- |
| `profile_id` | bigint | PK, NOT NULL, FK |
| `tag_id`     | bigint | PK, NOT NULL, FK |

PK:

```
(profile_id, tag_id)
```

FK:

```
profile_tag.profile_id -> profile.id
ON DELETE CASCADE
ON UPDATE RESTRICT
```

```
profile_tag.tag_id -> tag.id
ON DELETE RESTRICT
ON UPDATE RESTRICT
```

---

## 12. report_type

| Атрибут | Тип    | Ограничения      |
| ------- | ------ | ---------------- |
| `id`    | bigint | PK, NOT NULL     |
| `name`  | text   | NOT NULL, UNIQUE |

CHECK:

```
id > 0
char_length(name) BETWEEN 1 AND 100
name ~ '[^[:space:]]'
```

Кандидатные ключи:

```
{id}
{name}
```

---

## 13. report

| Атрибут            | Тип    | Ограничения                    |
| ------------------ | ------ | ------------------------------ |
| `id`               | bigint | PK, NOT NULL                   |
| `author_id`        | bigint | NOT NULL, FK -> user.id        |
| `reported_user_id` | bigint | NOT NULL, FK -> user.id        |
| `report_type_id`   | bigint | NOT NULL, FK -> report_type.id |

UNIQUE:

```
(author_id, reported_user_id, report_type_id)
```

CHECK:

```
id > 0
author_id <> reported_user_id
```

FK:

```
report.author_id -> user.id
ON DELETE CASCADE
ON UPDATE RESTRICT
```

```
report.reported_user_id -> user.id
ON DELETE CASCADE
ON UPDATE RESTRICT
```

```
report.report_type_id -> report_type.id
ON DELETE RESTRICT
ON UPDATE RESTRICT
```

Кандидатные ключи:

```
{id}
{author_id, reported_user_id, report_type_id}
```

---

## 14. photo

Файл фотографии хранится в S3, в БД сохраняется только его ключ.

| Атрибут       | Тип      | Ограничения                |
| ------------- | -------- | -------------------------- |
| `id`          | bigint   | PK, NOT NULL               |
| `profile_id`  | bigint   | NOT NULL, FK -> profile.id |
| `storage_key` | text     | NOT NULL, UNIQUE           |
| `position`    | smallint | NOT NULL                   |

UNIQUE:

```
(profile_id, position)
```

CHECK:

```
id > 0
storage_key ~ '[^[:space:]]'
char_length(storage_key) <= 1024
position > 0
```

FK:

```
photo.profile_id -> profile.id
ON DELETE CASCADE
ON UPDATE RESTRICT
```

Максимальное количество фотографий у одного профиля проверяется backend-логикой либо триггером.

---

## 15. photo_comment

| Атрибут     | Тип    | Ограничения              |
| ----------- | ------ | ------------------------ |
| `id`        | bigint | PK, NOT NULL             |
| `author_id` | bigint | NOT NULL, FK -> user.id  |
| `photo_id`  | bigint | NOT NULL, FK -> photo.id |
| `body`      | text   | NOT NULL                 |

CHECK:

```
id > 0
char_length(body) BETWEEN 1 AND 2000
body ~ '[^[:space:]]'
```

FK:

```
photo_comment.author_id -> user.id
ON DELETE CASCADE
ON UPDATE RESTRICT
```

```
photo_comment.photo_id -> photo.id
ON DELETE CASCADE
ON UPDATE RESTRICT
```

Кандидатный ключ:

```
{id}
```

---

## 16. profile_like

| Атрибут      | Тип    | Ограничения                |
| ------------ | ------ | -------------------------- |
| `id`         | bigint | PK, NOT NULL               |
| `author_id`  | bigint | NOT NULL, FK -> user.id    |
| `profile_id` | bigint | NOT NULL, FK -> profile.id |

UNIQUE:

```
(author_id, profile_id)
```

CHECK:

```
id > 0
```

FK:

```
profile_like.author_id -> user.id
ON DELETE CASCADE
ON UPDATE RESTRICT
```

```
profile_like.profile_id -> profile.id
ON DELETE CASCADE
ON UPDATE RESTRICT
```

Запрет лайка собственной анкеты проверяется backend-логикой либо триггером.

---

## 17. match

| Атрибут             | Тип     | Ограничения             |
| ------------------- | ------- | ----------------------- |
| `id`                | bigint  | PK, NOT NULL            |
| `first_user_id`     | bigint  | NOT NULL, FK -> user.id |
| `second_user_id`    | bigint  | NOT NULL, FK -> user.id |
| `access_to_message` | boolean | NOT NULL, DEFAULT true  |

UNIQUE:

```
(first_user_id, second_user_id)
```

CHECK:

```
id > 0
first_user_id < second_user_id
```

FK:

```
match.first_user_id -> user.id
ON DELETE CASCADE
ON UPDATE RESTRICT
```

```
match.second_user_id -> user.id
ON DELETE CASCADE
ON UPDATE RESTRICT
```

`match` создаётся при взаимном лайке; это реализуется backend-логикой либо `AFTER INSERT` trigger на `profile_like`.

---

## 18. message

| Атрибут     | Тип    | Ограничения              |
| ----------- | ------ | ------------------------ |
| `id`        | bigint | PK, NOT NULL             |
| `match_id`  | bigint | NOT NULL, FK -> match.id |
| `sender_id` | bigint | NOT NULL, FK -> user.id  |
| `body`      | text   | NOT NULL                 |

CHECK:

```
id > 0
char_length(body) BETWEEN 1 AND 10000
body ~ '[^[:space:]]'
```

FK:

```
message.match_id -> match.id
ON DELETE CASCADE
ON UPDATE RESTRICT
```

```
message.sender_id -> user.id
ON DELETE CASCADE
ON UPDATE RESTRICT
```

Перед созданием сообщения должно проверяться, что отправитель является участником `match` и `access_to_message = true`; это обеспечивается backend-логикой либо триггером.
