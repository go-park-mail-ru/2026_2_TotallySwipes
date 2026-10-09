"""Создаёт демо-пользователей через API: регистрация, онбординг анкеты, фото и прохождение теста."""
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
# Ключи из справочника tag (миграция 000002); подписи показывает фронтенд.
TAGS = ["coffee", "books", "music", "hiking", "bicycle", "travel", "photo",
        "board_games", "cooking", "running", "painting", "movies", "concerts", "animals"]
# Ключи целей знакомства; подписи показывает фронтенд.
GOALS = ["friendship", "relationship", "casual"]
# Разрешённые расширения фотографий и MIME-типы для их загрузки.
CONTENT_TYPES = {".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png", ".webp": "image/webp"}


def multipart(files):
    # Собираем multipart/form-data: уникальная граница разделяет файлы.
    boundary = uuid.uuid4().hex
    body = bytearray()
    # Добавляем имя файла, MIME-тип и исходные байты фотографии.
    for name, path in files:
        body += (f'--{boundary}\r\nContent-Disposition: form-data; name="{name}"; filename="{path.name}"\r\n'
                 f'Content-Type: {CONTENT_TYPES[path.suffix.lower()]}\r\n\r\n').encode()
        body += path.read_bytes() + b"\r\n"
    # Закрывающая граница обозначает конец тела запроса.
    body += f"--{boundary}--\r\n".encode()
    return bytes(body), f"multipart/form-data; boundary={boundary}"


def request(url, data=None, content_type=None, token=None, method=None):
    # Запрос с телом по умолчанию отправляется как POST, без тела — как GET.
    req = urllib.request.Request(url, data=data, method=method or ("POST" if data is not None else "GET"))
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

        # Регистрация - только почта и пароль; анкета создаётся пустой.
        # Если аккаунт уже есть (прошлый запуск мог упасть посередине), входим
        # и дозаполняем только то, чего не хватает: анкету, фото и тест.
        credentials = json.dumps({"email": email, "password": PASSWORD}).encode()
        status, headers, resp = request(f"{api}/auth/register", credentials, "application/json")
        created = status == 201
        if status == 409:
            status, headers, resp = request(f"{api}/auth/login", credentials, "application/json")
        # Для следующих запросов необходима сессия пользователя.
        token = access_token(headers)
        if status not in (200, 201) or not token:
            sys.exit(f"{email}: вход {status} {resp.decode()}")
        missing = json.loads(resp)["missing"]

        # Онбординг одним PATCH: обязательные поля, описание и интересы.
        # Чётным аккаунтам даём 5 интересов, нечётным — 6; внутри анкеты повторов нет.
        if any(field != "photos" for field in missing):
            profile = {
                "name": NAMES[(i - 1) % len(NAMES)],
                "birth_date": birth_date(i),
                "sex": "female",
                "dating_goal": GOALS[i % 3],
                "about_me": DESCRIPTIONS[(i - 1) % len(DESCRIPTIONS)],
                "search_sex": "all",
                "search_age_from": 18,
                "search_age_to": 100,
                "tags": [TAGS[(i - 1 + j) % len(TAGS)] for j in range(5 + i % 2)],
            }
            status, _, resp = request(f"{api}/profile/me", json.dumps(profile).encode(), "application/json", token, "PATCH")
            if status != 200:
                sys.exit(f"{email}: анкета {status} {resp.decode()}")

        # Загружаем одну фотографию; если их меньше аккаунтов, используем повторно.
        if "photos" in missing:
            body, content_type = multipart([("photo", photos[(i - 1) % len(photos)])])
            status, _, resp = request(f"{api}/profile/me/photos", body, content_type, token)
            if status != 201:
                sys.exit(f"{email}: фото {status} {resp.decode()}")

        # Тест проходим, только если он ещё не пройден.
        status, _, resp = request(f"{api}/tests/results/me", token=token)
        if status not in (200, 404):
            sys.exit(f"{email}: результат теста {status} {resp.decode()}")
        if status == 200:
            print(f"{email}: уже есть, пропускаю" if not missing else f"{email}: дозаполнен, тест уже пройден")
            continue

        # Загружаем текущий тест, чтобы взять ID вопросов из ответа API.
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
        print(f"{email}: {'создан' if created else 'дозаполнен'}, тип личности {json.loads(resp)['personality_type']}")


# Запускаем наполнение только при прямом вызове файла, а не при импорте.
if __name__ == "__main__":
    main()
