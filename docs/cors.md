# Прямые запросы frontend к backend: CORS

Frontend и backend работают на одном сервере, но браузер обращается к backend напрямую, без Nginx и reverse proxy. API сохраняет префикс `/api/v1`.

Один сервер не означает один origin: разные порты, хосты или схемы HTTP/HTTPS создают разные origins. Для запросов между ними на backend настраиваем CORS с cookies.

## Адреса окружений

| Окружение | Полный origin фронтенда | Полный origin backend |
| --- | --- | --- |
| Локальная разработка | `http://localhost:5173` | `http://localhost:8080` |
| Сервер | Уточнить | Уточнить |

Origin содержит схему, хост и порт, если он нестандартный, без пути и завершающего `/`. Для локальной разработки согласован origin фронтенда `http://localhost:5173`.

В белый список backend добавляются точные origins фронтенда. `localhost` и `127.0.0.1` считаются разными хостами.

## Согласованные настройки CORS

| Настройка | Значение |
| --- | --- |
| Разрешённые origins | Локально: `http://localhost:5173`; серверный адрес уточняется. Без `*` |
| Разрешённые методы | `GET`, `POST` |
| Разрешённые заголовки запроса | `Content-Type` |
| Credentials | `Access-Control-Allow-Credentials: true` |
| Preflight | Проверить запрашиваемые origin, метод и заголовки; для разрешённого `OPTIONS` вернуть `204` без проверки JWT |
| Кеш preflight | `Access-Control-Max-Age: 600` |
| Vary | `Origin`; для preflight также `Access-Control-Request-Method` и `Access-Control-Request-Headers` |

Backend возвращает в `Access-Control-Allow-Origin` один адрес, совпавший с белым списком. Произвольный Origin отражать в ответ нельзя.

CORS middleware подключается снаружи роутера до JWT-проверки, чтобы обрабатывать preflight и добавлять заголовки к ответам для разрешённого origin, включая ошибки `401`, `404`, `405`. Обычные защищённые запросы по-прежнему проверяются JWT middleware. CORS не заменяет авторизацию.

Это согласованная конфигурация; CORS middleware пока не реализован.

## Запросы с фронтенда

`API_ORIGIN` — полный адрес backend из конфигурации фронтенда, без завершающего `/` и без `/api/v1`. Относительный URL `/api/v1/...` без proxy уйдёт на сервер фронтенда, поэтому используем адрес backend:

```js
const API_ORIGIN = "http://localhost:8080"; // локальная разработка
const response = await fetch(`${API_ORIGIN}/api/v1/tests/current`, {
  credentials: "include",
});
```

```js
const response = await fetch(`${API_ORIGIN}/api/v1/tests/${testId}/results`, {
  method: "POST",
  credentials: "include",
  headers: {
    "Content-Type": "application/json",
  },
  body: JSON.stringify({ answers }),
});
```

`credentials: "include"` указывается также при регистрации, входе и выходе. JWT хранится в HttpOnly cookie `access_token`: JavaScript его не читает. Заголовками `Cookie` и `Origin` управляет браузер; `Authorization` в текущей схеме не используется. Preflight браузер отправляет автоматически.

## Доступность backend и cookies

- Браузеру нужен доступ к адресу и порту backend. В Docker Compose API по умолчанию опубликован на `127.0.0.1`; для прямого доступа с других компьютеров в серверном окружении потребуется `API_BIND=0.0.0.0` и доступный порт API. Локальные настройки этим не заменяем.
- Текущий Go-сервер использует `ListenAndServe`, то есть HTTP. HTTPS для прямого подключения нужно настроить отдельно; фронтенд с HTTPS не должен обращаться к HTTP API.
- Cookies сейчас используют `SameSite=Lax`. Разные порты при одинаковых схеме и хосте требуют CORS, но сами по себе не требуют менять SameSite.
- Если frontend и backend окажутся на разных сайтах, cookie-настройки нужно согласовать отдельно: одного CORS и `credentials: "include"` недостаточно.
- Для обычной локальной разработки по HTTP используется `COOKIE_SECURE=false`; для HTTPS — `COOKIE_SECURE=true`.

Справка: [MDN — CORS](https://developer.mozilla.org/en-US/docs/Web/HTTP/Guides/CORS), [MDN — Fetch credentials](https://developer.mozilla.org/en-US/docs/Web/API/Request/credentials).
