# Карта API микросервисов

Рабочий проект распределения API от 18 ноября 2026 года на основе [архитектуры](schema.md) и текущего кода. Существующие маршруты перечислены по [newRouter](../cmd/api/main.go); новые RPC, HTTP-пути и WebSocket-события ниже — предложения, не реализованные контракты.

## Как читать и дополнять документ

Описываем три уровня отдельно:

1. Внешний контракт: HTTP-маршрут или WebSocket-команда, доступный фронтенду.
2. Прикладная операция и владеющий ею сервис: какие правила он проверяет и какие данные изменяет.
3. Внутренние зависимости: gRPC-вызовы других сервисов, без прямого доступа к их таблицам.

Существующий Go-интерфейс не переносится в Protobuf автоматически. Интерфейсы репозиториев остаются внутри владельца данных. Новый RPC описывает прикладную операцию, а не отдельный SQL-запрос. Внешние DTO и HTTP-статусы сохраняет Gateway; точные поля сверяем с [DTO](../internal/handler/dto), [обработчиками](../internal/handler) и [OpenAPI](swagger/openapi.yaml).

Статусы: **перенос** — операция существует; **разделение** — существует, но пересекает будущие границы; **изменение** — требуется новое поведение; **новое** — операция пока отсутствует. Имена RPC предварительные. Все пути в таблицах существующего API имеют префикс `/api/v1`, кроме явно указанных инфраструктурных маршрутов.

## Существующий внешний API

### Auth

| HTTP | Текущая реализация | Целевая операция | Статус / изменение |
|---|---|---|---|
| POST `/auth/register` | `AuthService.Register`, `UserRepo.CreateUser` | `Auth.Register` | Разделение: user в Auth, `Profile.EnsureProfile` по gRPC; восстановление частичного сбоя |
| POST `/auth/login` | `AuthService.Login` | `Auth.Login` | Разделение: заполненность через `Profile.GetProfileStatus` |
| POST `/auth/refresh` | `AuthService.Refresh` | `Auth.Refresh` | Перенос: ротация сессий остаётся в Auth |
| POST `/auth/logout` | `AuthService.Logout` | `Auth.Logout` | Перенос: отзыв refresh-сессии |
| GET `/auth/csrf` | `AuthHandler.CSRF` | Локальная операция Gateway | HTTP-защита, отдельный бизнес-RPC не нужен |

Cookies устанавливает Gateway. Токены выпускает Auth. Клиент не передаёт произвольный `user_id` вместо доказательства авторизации.

### Profile

| HTTP | Текущая реализация | Предлагаемый RPC Profile | Статус / примечание |
|---|---|---|---|
| GET `/profile/me` | `ProfileService.GetMyProfile` | `GetMyProfile` | Перенос |
| PATCH `/profile/me` | `ProfileService.UpdateProfile` | `UpdateProfile` | Перенос; сохранить различие отсутствующего поля, очистки и значения |
| GET `/profile/me/short` | `ProfileService.GetShortProfile` | `GetShortProfile` | Перенос |
| POST `/profile/me/photos` | `ProfileService.AddPhoto` | `AddPhoto` | Перенос; внешний multipart требует отдельного транспортного адаптера |
| PUT `/profile/me/photos/order` | `ProfileService.ReorderPhotos` | `ReorderPhotos` | Перенос |
| DELETE `/profile/me/photos/{photo_id}` | `ProfileService.DeletePhoto` | `DeletePhoto` | Перенос |
| GET `/filters/me` | `FilterService.Get` | `GetSearchFilter` | Перенос |
| PUT `/filters/me` | `FilterService.Set` | `SetSearchFilter` | Перенос |
| GET `/tests/current` | `PsychoTestService.GetCurrentTest` | `GetCurrentTest` | Перенос |
| POST `/tests/{test_id}/results` | `PsychoTestService.SubmitTestAnswers` | `SubmitTestAnswers` | Перенос; расчёт Big Five остаётся локальным в Profile |
| GET `/tests/results/me` | `PsychoTestService.GetMyTestResult` | `GetMyTestResult` | Перенос |
| POST `/location/me/preview` | `LocationService.GetLocationPreview` | `PreviewLocation` | Перенос; обратное геокодирование и preview |
| PUT `/location/me` | `LocationService.SaveLocation` | `SaveLocation` | Перенос; сохранить применение `preview_id` |
| GET `/location/me` | `LocationService.GetLocation` | `GetLocation` | Перенос |

Прямое геокодирование, подписки и отдельный API справочника тегов не имеют самостоятельных маршрутов в текущем роутере. Их внешние контракты нужно определить отдельно; не считаем их уже реализованными endpoints.

### Matching & Meeting

| HTTP | Текущая реализация | Предлагаемый RPC Matching | Статус / изменение |
|---|---|---|---|
| GET `/feed` | `ProfileService.GetNextFeed` | `GetFeed` | Разделение: кандидаты из Profile, свайпы и совместимость в Matching |
| POST `/feed/swipe` | `SwipeService.Swipe`, `SaveSwipeAndMatch` | `Swipe` | Изменение: самостоятельный суперлайк, запрет его отмены, удаление обычного мэтча после дизлайка |
| GET `/matches` | `MatchService.GetMatches` | `GetMatches` | Разделение: заменить JOIN с профилями пакетным RPC к Profile |
| GET `/matches/summary` | `MatchService.GetSummary` | `GetMatchSummary` | Перенос |
| POST `/matches/{match_id}/seen` | `MatchService.MarkSeen` | `MarkMatchSeen` | Перенос; просмотр мэтча не равен прочтению сообщений |
| GET `/profiles/{user_id}` | `ProfileService.GetMatchedProfile` | `GetMatchedProfile` | Предлагается перенести orchestration в Matching: проверка мэтча, загрузка анкеты из Profile, расчёт совместимости |
| POST `/matches/{match_id}/search-places` | `EventService.SearchEventsByCity` | `CreatePlaceSearch` | Перенос; интеграция Timepad в Matching |
| GET `/matches/{match_id}/place-searches/{search_id}` | `EventService.GetSearchEvents` | `GetPlaceSearch` | Перенос |
| POST `/matches/{match_id}/place-searches/{search_id}/results` | `EventService.SubmitResults` | `SubmitPlaceSelection` | Изменение: при пересечении надёжно публиковать системное сообщение в Chat |

Путь `/profiles/{user_id}` сохраняется. По текущему коду это не произвольное чтение профиля: нужны мэтч и заполненная анкета, в ответ входят `match_id` и карточка с совместимостью. Предлагаемая маршрутизация в Matching сохраняет эти ограничения без обратного вызова Profile → Matching. Владельцем самой анкеты остаётся Profile.

### Инфраструктура и выдача файлов

| Текущий маршрут без префикса | Сейчас | Целевое размещение |
|---|---|---|
| GET `/health` | `handler.GetHealth` | Health Gateway; отдельные health/readiness для сервисов |
| GET/HEAD `/media/{key}` | `PhotoHandler.Get` → MinIO | Сохранить внешний URL; Profile владеет фото, способ передачи бинарных данных через Gateway уточнить |
| GET/HEAD `/cats/*` | `http.FileServer` | Сохранить статическую выдачу через Gateway или Nginx |

## Внутренний API между сервисами

Эти RPC не следует автоматически публиковать как внешние HTTP endpoints. Входы ниже — смысловые поля, а не окончательная Protobuf-схема. Идентичность инициатора передаётся доверенным способом; внутренний доступ ограничивается ролью вызывающего сервиса.

| Кто вызывает → владелец | Предлагаемый RPC | Вход → результат | Основание в текущем коде / правила |
|---|---|---|---|
| Auth → Profile | `EnsureProfile` | `user_id` → состояние анкеты и недостающие поля | Создание profile сейчас внутри `UserRepo.CreateUser`; повтор не очищает анкету |
| Auth → Profile | `GetProfileStatus` | `user_id` → заполненность, `missing` | Замена зависимости `ProfileMissingChecker.Missing` |
| Matching → Profile | `SearchCandidates` | пользователь, курсор, размер пачки → кандидаты, следующий курсор | Переработка `GetCandidatesByCursorAndLimit`: фильтры и география локальны, чужие swipe-таблицы не читаются |
| Matching → Profile | `GetProfilesBatch` | список ID и контекст просмотра → нужные карточки и Big Five | Замена обращений к `ProfileRepository`, `MatchViewerRepository` и межсервисных JOIN; ограничить размер пачки |
| Chat → Matching | `CanMessage` | отправитель, получатель/контекст мэтча → разрешение и причина отказа | Новый метод: действующий мэтч и отсутствие блокировки |
| Chat → Matching | `GetMatchParticipants` | `match_id` → участники и состояние | Новый метод для проверки состава создаваемого чата |
| Matching → Chat | `EnsureChat` | `match_id` → `chat_id` | Новый метод; один чат на мэтч, повторы безопасны |
| Matching → Chat | `PublishEventIntersection` | мэтч, поиск, версия результата, события → системное сообщение | Новый метод; дедупликация по результату поиска |
| Matching → Chat | `DeliverUserNotification` | `event_id`, адресат, тип и данные → подтверждение приёма | Предложение для онлайн-уведомления о суперлайке; факт и просмотр хранятся в Matching |
| Chat / Profile → владелец статуса аккаунта | `GetAccountStatus` или распространение изменений статуса | пользователь → модераторский статус | Новый контракт; владелец предварительно Auth, окончательно не выбран |

Для `EnsureChat` Matching сначала фиксирует мэтч, затем вызывает Chat: обратная проверка состава чата не должна ждать транзакцию Matching или повторно инициировать создание чата. Для отложенных уведомлений используются устойчивые повторы и дедупликация. Устаревший вызов создания чата не должен возобновлять удалённый мэтч.

Big Five возвращается только доверенным потребителям в необходимом объёме. `GetProfilesBatch` не должен означать неограниченный публичный доступ к любым анкетам. Географические вычисления остаются в Profile; точные координаты не требуется раскрывать фронтенду или передавать Matching без необходимости.

## Новые внешние операции: предложения

Пути ниже нужно согласовать перед реализацией. Они не обнаружены в текущем роутере.

| Владелец | Предлагаемый внешний API | Назначение |
|---|---|---|
| Matching | GET `/api/v1/sympathies` | Входящие симпатии, пометка суперлайка, пагинация |
| Matching | POST `/api/v1/sympathies/{id}/seen` | Просмотр симпатии; отдельно от `MarkMatchSeen` |
| Matching | PUT/DELETE `/api/v1/blocks/{user_id}` | Заблокировать / разблокировать собеседника |
| Chat | GET `/api/v1/chats` | Чаты, включая пустые, превью и непрочитанные |
| Chat | GET `/api/v1/chats/by-match/{match_id}` | Найти чат по мэтчу; только участникам |
| Chat | GET `/api/v1/chats/{chat_id}/messages` | История сообщений с курсором |
| Chat | POST `/api/v1/chats/{chat_id}/attachments` | Загрузить голосовое/видеокружок; лимиты и формат уточнить |
| Chat | GET `/api/v1/chats/{chat_id}/attachments/{attachment_id}` | Получить медиа после проверки прав |
| Chat | WebSocket, предлагаемый путь `/ws` | Отправка сообщений и доставка событий всех чатов вкладки |

Для суперлайка используем существующий `POST /feed/swipe` с `action=super_like`, не добавляем дублирующий маршрут. Сейчас успешный ответ — `204 No Content`, поэтому нельзя молча заменить его ответом с `chat_id`. Для перехода в чат нужно выбрать совместимый сценарий: получить `match_id` из обновлённого списка мэтчей и найти чат, либо добавить согласованный отдельный lookup по собеседнику, либо явно согласовать новую версию ответа. При асинхронном создании чата интерфейс учитывает временную готовность.

Модераторский API, подписки и Reports пока не определяем как готовый контракт: статус аккаунта нужен для поиска и подключения WebSocket, но роли модерации и операции управления ещё требуют решения.

### Предлагаемый WebSocket-протокол Chat

| Направление | Команда / событие | Основные данные и проверка |
|---|---|---|
| Клиент → Chat | `message.send` | `request_id`, `client_message_id`, `chat_id`, текст/вложения; участник + `CanMessage` |
| Клиент → Chat | `message.edit` | `request_id`, `chat_id`, `message_id`, текст; авторство и правила редактирования |
| Клиент → Chat | `message.delete` | `request_id`, `chat_id`, `message_id`; авторство и правила удаления |
| Клиент → Chat | `chat.read` | `request_id`, `chat_id`, позиция прочтения; участие, допустимая позиция, только продвижение вперёд |
| Chat → клиент | `command.result` / `command.error` | `request_id`, результат/код ошибки; подтверждение обработки команды, не прочтения |
| Chat → участники | `chat.created` | `event_id`, `chat_id`, `match_id`; показать и пустой чат |
| Chat → участники | `message.created/updated/deleted` | `event_id`, `chat_id`, сообщение/ID, версия изменения |
| Chat → участники | `chat.read_updated` | `event_id`, `chat_id`, читатель, позиция прочтения |
| Chat → подключения пользователя | `chat.unread_updated` | `event_id`, `chat_id`, актуальный счётчик |
| Chat → получатель | `sympathy.received` | `event_id`, идентификатор симпатии, отправитель, пометка суперлайка; источник — Matching |

Это предложение транспорта команд: HTTP-дубли отправки/редактирования не обязательны. WebSocket handler вызывает прикладную логику Chat локально; превращать его обращения к собственному сервису в сетевой gRPC не требуется. Все подключения авторизованы, события проверяются по получателям. После reconnect клиент синхронизирует состояние; точный механизм согласования снимка и новых событий ещё предстоит определить.

## Что делать с существующими интерфейсами

| Сейчас | Целевое изменение |
|---|---|
| Интерфейсы `internal/handler/*` | Сохранить как порты HTTP-слоя, где возможно; реализовать адаптеры gRPC-клиентов, преобразующие protobuf в текущие model/DTO и ошибки |
| `ProfileRepository` | Разделить операции владения анкетой и зависимости формирования ленты/мэтчей; SQL-репозиторий остаётся в Profile, Matching получает узкий клиентский интерфейс |
| `ProfileMissingChecker` | Реализовать адаптером к Profile для Auth |
| `MatchViewerRepository` | Заменить в Matching интерфейсом клиента Profile, не удалённым SQL-репозиторием |
| `CompatibilityService.CalculateBigFive` | Выделить локальный калькулятор теста в Profile |
| `CompatibilityService.CalculateDistance` | Оставить локальный калькулятор совместимости в Matching; отдельный RPC для каждой пары не нужен |
| `PsychoTestService` | Зависеть только от калькулятора Big Five и своих Profile-репозиториев |
| `ProfileService.GetNextFeed`, `feedItem` | Разделить поиск/данные анкеты и сборку ленты с совместимостью в Matching |
| `ProfileService.GetMatchedProfile` | Перенести orchestration и проверку мэтча в Matching, чтение анкеты через Profile |
| `SwipeRepository.SaveSwipeAndMatch` | Оставить в Matching; убрать доступ к таблицам Profile, переработать идентификацию цели и правила мэтча |
| `EventProvider`, `EventRepository`, `EventMatchRepository` | Остаются внутренними зависимостями Matching; добавить клиент Chat для системного сообщения |
| `PhotoStorage`, `PhotoURLProvider`, `PhotoReader` | Фото анкеты принадлежат Profile; Gateway не должен обращаться к MinIO через собственную бизнес-логику |

## Шаблон подробного контракта операции

После согласования распределения для каждого RPC заполняем карточку. Только затем фиксируем `.proto` и новые HTTP/WS-контракты.

```text
Операция и владелец:
Статус: перенос / разделение / изменение / новое
Внешний HTTP endpoint или WS-команда (если есть):
Текущий Go-интерфейс, реализация и DTO:
Кто вызывает и какие права нужны:
Вход: поля, типы, обязательность, ограничения, семантика отсутствия:
Выход: поля, типы, сортировка, пагинация:
Бизнес-правила и изменяемые данные:
Внутренние gRPC-вызовы:
Граница транзакции и поведение при частичном сбое:
Идемпотентность, конкурентные запросы и повторы:
Доменные ошибки → gRPC status/details → прежний HTTP-ответ:
События, получатели и порядок доставки:
Проверки совместимости с существующим API:
Открытые решения:
```

Первыми подробно описать `Register`, `SearchCandidates/GetFeed`, `Swipe`, `EnsureChat/CanMessage` и `SubmitPlaceSelection/PublishEventIntersection`: именно здесь проходят границы данных, транзакций и новых продуктовых правил.
