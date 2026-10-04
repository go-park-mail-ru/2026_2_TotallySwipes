"""Создаёт демо-пользователей через API: регистрация с фото и прохождение текущего теста."""
import argparse
import datetime
import json
import sys
import urllib.error
import urllib.request
import uuid
from pathlib import Path

# Общие демо-данные: имена, интересы и цели знакомства выбираются по кругу.
# Пароль одинаковый для всех аккаунтов; хеширование выполняет API.
PASSWORD = "DemoPass123!"
NAMES = ["Анна", "Мария", "София", "Алиса", "Полина",
         "Екатерина", "Дарья", "Елизавета", "Виктория", "Анастасия",
         "Ксения", "Варвара", "Вероника", "Александра", "Ульяна"]
DESCRIPTIONS = ["Люблю утренние пробежки, книги и разговоры за чашкой кофе.",
                "В свободное время хожу в театр и открываю новые места в городе.",
                "Рисую акварелью, путешествую и собираю впечатления.",
                "Люблю настольные игры, домашнюю выпечку и уютные вечера.",
                "Увлекаюсь фотографией и прогулками на природе.",
                "Хожу в походы, катаюсь на велосипеде и мечтаю увидеть Алтай.",
                "Занимаюсь йогой и люблю спокойные прогулки у воды.",
                "Играю на фортепиано и часто бываю на концертах.",
                "Люблю плавание, кино и встречи с друзьями.",
                "Делаю керамику и ищу вдохновение в музеях.",
                "Танцую, читаю романы и пробую новые рецепты.",
                "Выращиваю цветы и люблю проводить выходные за городом.",
                "Люблю длинные прогулки, выставки и хорошие истории.",
                "Путешествую по России и фотографирую архитектуру.",
                "Ценю чувство юмора, искренность и совместные приключения."]
TAGS = ["музыка", "спорт", "путешествия", "книги", "кино", "игры", "кулинария", "искусство"]
INTENTS = ["Ищу общение", "Ищу половинку", "Ищу встречи"]
# Разрешённые расширения фотографий и MIME-типы для их загрузки.
CONTENT_TYPES = {".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png", ".webp": "image/webp"}


def multipart(fields, files):
    # Собираем multipart/form-data: уникальная граница разделяет поля и файлы.
    boundary = uuid.uuid4().hex
    body = bytearray()
    # Каждое текстовое поле — отдельная часть; повторяющиеся tags образуют список.
    for name, value in fields:
        body += f'--{boundary}\r\nContent-Disposition: form-data; name="{name}"\r\n\r\n{value}\r\n'.encode()
    # Добавляем имя файла, MIME-тип и исходные байты фотографии.
    for name, path in files:
        body += (f'--{boundary}\r\nContent-Disposition: form-data; name="{name}"; filename="{path.name}"\r\n'
                 f'Content-Type: {CONTENT_TYPES[path.suffix.lower()]}\r\n\r\n').encode()
        body += path.read_bytes() + b"\r\n"
    # Закрывающая граница обозначает конец тела запроса.
    body += f"--{boundary}--\r\n".encode()
    return bytes(body), f"multipart/form-data; boundary={boundary}"


def request(url, data=None, content_type=None, token=None):
    # Запрос с телом отправляется как POST, без тела — как GET.
    req = urllib.request.Request(url, data=data, method="POST" if data is not None else "GET")
    if content_type:
        req.add_header("Content-Type", content_type)
    # API авторизует пользователя по cookie, полученной при регистрации.
    if token:
        req.add_header("Cookie", f"access_token={token}")
    # Возвращаем статус, заголовки и тело даже при HTTP-ошибке:
    # вызывающий код решает, пропустить аккаунт или остановить скрипт.
    try:
        with urllib.request.urlopen(req) as resp:
            return resp.status, resp.headers, resp.read()
    except urllib.error.HTTPError as e:
        return e.code, e.headers, e.read()


def access_token(headers):
    # Из Set-Cookie извлекаем только значение access_token, без атрибутов cookie.
    for cookie in headers.get_all("Set-Cookie") or []:
        name, _, rest = cookie.partition("=")
        if name == "access_token":
            return rest.split(";", 1)[0]
    return None


def birth_date(i):
    # Получаем возраст 21–30 лет с добавлением 0–5 месяцев.
    # День ограничен 28, чтобы дата существовала в любом месяце.
    today = datetime.date.today()
    months = today.year * 12 + today.month - 1 - (21 + i % 10) * 12 - i % 6
    return datetime.date(months // 12, months % 12 + 1, min(today.day, 28)).isoformat()


def main():
    # Параметры запуска: адрес API, папка фотографий и число аккаунтов (обычно 15).
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--api", default="http://localhost:8080", help="адрес API")
    parser.add_argument("--photos", default=Path(__file__).parent / "photos", help="каталог с JPG/PNG/WebP для анкет")
    parser.add_argument("--count", type=int, default=len(NAMES), help="сколько пользователей создать")
    args = parser.parse_args()

    # Собираем фотографии в стабильном порядке; без фото регистрация не запускается.
    photos = sorted(p for p in Path(args.photos).glob("*") if p.suffix.lower() in CONTENT_TYPES)
    if not photos:
        sys.exit(f"Положите фото в {args.photos}")
    # Все запросы ниже используют общий префикс версии API.
    api = args.api.rstrip("/") + "/api/v1"

    # Для каждого номера создаём аккаунт demo01@example.com, demo02@example.com и т. д.
    for i in range(1, args.count + 1):
        email = f"demo{i:02}@example.com"
        # Данные регистрации включают анкету и предпочтения поиска.
        fields = [
            ("name", NAMES[(i - 1) % len(NAMES)]),
            ("email", email),
            ("password", PASSWORD),
            ("birth_date", birth_date(i)),
            ("sex", "female"),
            ("search_sex", "all"),
            ("dating_intent", INTENTS[i % 3]),
            ("about_me", DESCRIPTIONS[(i - 1) % len(DESCRIPTIONS)]),
            ("search_age_from", 18),
            ("search_age_to", 100),
        ]
        # Чётным аккаунтам даём 5 интересов, нечётным — 6; внутри анкеты повторов нет.
        fields += [("tags", TAGS[(i - 1 + j) % len(TAGS)]) for j in range(5 + i % 2)]
        # Прикладываем одну фотографию; если их меньше аккаунтов, используем повторно.
        body, content_type = multipart(fields, [("photos", photos[(i - 1) % len(photos)])])

        # Регистрируем аккаунт с анкетой и фото через API.
        status, headers, resp = request(f"{api}/auth/register", body, content_type)
        # Существующие аккаунты пропускаем целиком, включая прохождение теста.
        if status == 409:
            print(f"{email}: уже есть, пропускаю")
            continue
        # Для следующих запросов необходима сессия нового пользователя.
        token = access_token(headers)
        if status != 201 or not token:
            sys.exit(f"{email}: регистрация {status} {resp.decode()}")

        # Загружаем текущий тест, чтобы использовать реальные ID вопросов из БД.
        status, _, resp = request(f"{api}/tests/current", token=token)
        if status != 200:
            sys.exit(f"{email}: тест {status} {resp.decode()}")
        test = json.loads(resp)
        # Генерируем воспроизводимые ответы от 1 до 7, разные для аккаунтов и вопросов.
        answers = [{"question_id": q["id"], "value": 1 + (i * 3 + n * 5) % 7} for n, q in enumerate(test["questions"])]

        # Отправляем ответы: API сохраняет прохождение и рассчитывает тип личности.
        status, _, resp = request(f"{api}/tests/{test['test_id']}/results",
                                  json.dumps({"answers": answers}).encode(), "application/json", token)
        if status != 201:
            sys.exit(f"{email}: ответы {status} {resp.decode()}")
        # Сообщаем об успешном завершении регистрации и прохождения теста.
        print(f"{email}: создан, тип личности {json.loads(resp)['personality_type']}")


# Запускаем наполнение только при прямом вызове файла, а не при импорте.
if __name__ == "__main__":
    main()
