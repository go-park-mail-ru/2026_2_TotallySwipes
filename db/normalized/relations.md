# Отношения БД Totally Swipes

ER: [er.puml](er.puml). Типы и ограничения: [schema.dbml](schema.dbml).

Все атрибуты скалярные. DEFAULT CURRENT_TIMESTAMP задан для recorded_at. Остальные значения задаются явно; NULL у всех пяти весов означает незавершённое прохождение. Генерация id предполагается через identity в DDL. Технические created_at/updated_at опущены на этапе нормализации.


## 1. user — Учётная запись

Хранит имя, email и хеш пароля пользователя.

| Атрибут | Тип | Ограничения поля |
| --- | --- | --- |
| `id` | bigint | pk, not null |
| `name` | text | not null |
| `email` | text | not null, unique |
| `password_hash` | text | not null |

Проверки CHECK:

- `id > 0`.
- `char_length(name) BETWEEN 1 AND 100 AND name ~ '[^[:space:]]'`.
- `char_length(email) BETWEEN 1 AND 254 AND email !~ '[[:space:]]'`.
- `password_hash ~ '[^[:space:]]'`.

email уникален при текущем правиле сравнения БД; формат email проверяется приложением.

Кандидатные ключи: `{id}, {email}`.

```text
Relation user:
{id} -> name, email, password_hash
{email} -> id
```

Все значения атомарны (1НФ). Кандидатные ключи простые, частичных зависимостей нет (2НФ). Неключевые атрибуты не определяют другие атрибуты (3НФ). Оба определителя в базисе — кандидатные ключи, поэтому выполняется НФБК.

## 2. profile — Анкета и её владелец

Связывает единственную анкету с её владельцем. Изменяемые поля находятся в profile_version.

| Атрибут | Тип | Ограничения поля |
| --- | --- | --- |
| `id` | bigint | pk, not null |
| `user_id` | bigint | not null, unique |

Проверки CHECK:

- `id > 0`.

user_id — FK на user.id. У пользователя должна быть ровно одна анкета; FK и UNIQUE обеспечивают только владельца и не более одной анкеты.

Кандидатные ключи: `{id}, {user_id}`.

```
Relation profile:
{id} -> user_id
{user_id} -> id
```

Значения атомарны (1НФ). Оба атрибута являются кандидатными ключами, неключевых атрибутов нет (2НФ и 3НФ). В обеих нетривиальных зависимостях определитель — ключ (НФБК).

## 3. profile_version — Версия анкеты

Неизменяемый снимок полей анкеты. Редактирование создаёт новую версию; текущей считается версия с максимальным revision для profile_id.

| Атрибут | Тип | Ограничения поля |
| --- | --- | --- |
| `id` | bigint | pk, not null |
| `profile_id` | bigint | not null |
| `revision` | integer | not null |
| `recorded_at` | timestamptz | not null, default: `CURRENT_TIMESTAMP` |
| `birth_date` | date | not null |
| `sex` | text | not null |
| `search_sex` | text | not null |
| `search_age_from` | integer | not null |
| `search_age_to` | integer | not null |

Проверки CHECK:

- `id > 0`.
- `revision > 0`.
- `isfinite(recorded_at)`.
- `isfinite(birth_date)`.
- `sex IN ('male', 'female')`.
- `search_sex IN ('male', 'female', 'all')`.
- `search_age_from >= 18 AND search_age_to >= search_age_from`.

profile_id — FK на profile.id. UNIQUE(profile_id, revision) запрещает повтор номера версии. У анкеты должна быть минимум одна версия. Возраст владельца на момент записи — не менее 18 лет (проверка приложения).

Кандидатные ключи: `{id}, {profile_id, revision}`.

```
Relation profile_version:
{id} -> profile_id, revision, recorded_at, birth_date, sex, search_sex, search_age_from, search_age_to
{profile_id, revision} -> id
```

Значения атомарны (1НФ). Поля снимка определяются всей парой (profile_id, revision), а не одной её частью (2НФ). Зависимостей между неключевыми атрибутами не задано (3НФ). Оба определителя — кандидатные ключи (НФБК).

## 4. test — Тест

Определяет конкретную редакцию психологического теста. Использованный набор вопросов и смысл операций неизменяемы; новая редакция — новая запись test.

| Атрибут | Тип | Ограничения поля |
| --- | --- | --- |
| `id` | bigint | pk, not null |
| `name` | text | not null |

Проверки CHECK:

- `id > 0`.
- `name ~ '[^[:space:]]'`.



Кандидатные ключи: `{id}`.

```
Relation test:
{id} -> name
```

Значения атомарны (1НФ), ключ простой (2НФ). Название не уникально и не определяет id; транзитивных зависимостей нет (3НФ). Единственный определитель — ключ id (НФБК).

## 5. question — Вопрос

Хранит текст вопроса, ссылку на тест и код операции расчёта. operation_id — код backend, не внешний ключ.

| Атрибут | Тип | Ограничения поля |
| --- | --- | --- |
| `id` | bigint | pk, not null |
| `test_id` | bigint | not null |
| `operation_id` | integer | not null |
| `body` | text | not null |

Проверки CHECK:

- `id > 0`.
- `body ~ '[^[:space:]]'`.

test_id — FK на test.id. Допустимые operation_id определяются реестром операций; перечень пока не задан.

Кандидатные ключи: `{id}`.

```
Relation question:
{id} -> test_id, operation_id, body
```

Значения атомарны (1НФ), ключ простой (2НФ). Операция и текст могут повторяться и не определяют остальные поля (3НФ). Единственный определитель — ключ id (НФБК).

## 6. profile_psycho — Прохождение психологического теста

Хранит одно прохождение теста: владельца, тест, номер версии, время создания записи и пять итоговых характеристик. Повторное прохождение создаёт новую запись; завершённый результат неизменяем.

| Атрибут | Тип | Ограничения поля |
| --- | --- | --- |
| `id` | bigint | pk, not null |
| `profile_id` | bigint | not null |
| `test_id` | bigint | not null |
| `revision` | integer | not null |
| `recorded_at` | timestamptz | not null, default: `CURRENT_TIMESTAMP` |
| `weight_1` | numeric | null |
| `weight_2` | numeric | null |
| `weight_3` | numeric | null |
| `weight_4` | numeric | null |
| `weight_5` | numeric | null |

Проверки CHECK:

- `id > 0`.
- `revision > 0`.
- `isfinite(recorded_at)`.
- До завершения все пять весов NULL; при завершении все пять заполнены.
- Каждый заданный вес — конечное число: NaN и ±Infinity запрещены.

profile_id и test_id — FK на profile.id и test.id. На анкету приходится 0..N прохождений. Все пять весов либо одновременно NULL (незавершённая попытка), либо одновременно заданы (готовый результат). UNIQUE(profile_id, revision) запрещает повтор номера версии в пределах анкеты независимо от теста. Новая попытка получает следующий revision; номер после создания неизменяем. Нумерация независима от revision в profile_version. Текущий результат выбранного теста — готовая запись с максимальным revision; незавершённая новая версия его не заменяет. Готовый результат требует ответов на все вопросы непустого теста; FK и CHECK не проверяют полноту и формулу.

Кандидатные ключи: `{id}, {profile_id, revision}`.

```
Relation profile_psycho:
{id} -> profile_id, test_id, revision, recorded_at, weight_1, weight_2, weight_3, weight_4, weight_5
{profile_id, revision} -> id
```

Пять весов — отдельные скалярные значения (1НФ). Результат зависит от всей пары (profile_id, revision), а не от одной её части (2НФ). Владелец и тест не определяют результат: прохождений может быть несколько; зависимостей между весами не задано (3НФ). Оба определителя — кандидатные ключи (НФБК).

## 7. user_answer — Ответ в прохождении

Хранит ответ на вопрос в конкретном прохождении. После завершения прохождения ответ неизменяем.

| Атрибут | Тип | Ограничения поля |
| --- | --- | --- |
| `profile_psycho_id` | bigint | not null |
| `question_id` | bigint | not null |
| `answer_value` | smallint | not null |

Проверки CHECK:

- `answer_value BETWEEN 0 AND 7`.

PK(profile_psycho_id, question_id); оба поля — FK. 0 — «точно нет», 7 — «точно да». Отсутствие строки означает отсутствие ответа. Вопрос должен принадлежать тесту прохождения: question.test_id = profile_psycho.test_id; отдельные FK это равенство не обеспечивают.

Кандидатные ключи: `{profile_psycho_id, question_id}`.

```
Relation user_answer:
{profile_psycho_id, question_id} -> answer_value
```

Значения атомарны (1НФ). Ответ зависит от всей пары: одна попытка содержит ответы на разные вопросы, один вопрос имеет разные ответы в разных попытках (2НФ). Других нетривиальных зависимостей не задано (3НФ). Определитель — составной ключ (НФБК).
