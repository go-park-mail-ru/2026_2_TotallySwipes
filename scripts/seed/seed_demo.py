"""Создаёт демо-пользователей через API: регистрация с фото и прохождение текущего теста."""
import argparse
import datetime
import json
import sys
import urllib.error
import urllib.request
import uuid
from pathlib import Path

PASSWORD = "DemoPass123!"
NAMES = ["Анна", "Мария", "София", "Алиса", "Полина",
         "Екатерина", "Дарья", "Елизавета", "Виктория", "Анастасия",
         "Ксения", "Варвара", "Вероника", "Александра", "Ульяна"]
TAGS = ["музыка", "спорт", "путешествия", "книги", "кино", "игры", "кулинария", "искусство"]
INTENTS = ["Ищу общение", "Ищу половинку", "Ищу встречи"]
CONTENT_TYPES = {".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png", ".webp": "image/webp"}


def multipart(fields, files):
    boundary = uuid.uuid4().hex
    body = bytearray()
    for name, value in fields:
        body += f'--{boundary}\r\nContent-Disposition: form-data; name="{name}"\r\n\r\n{value}\r\n'.encode()
    for name, path in files:
        body += (f'--{boundary}\r\nContent-Disposition: form-data; name="{name}"; filename="{path.name}"\r\n'
                 f'Content-Type: {CONTENT_TYPES[path.suffix.lower()]}\r\n\r\n').encode()
        body += path.read_bytes() + b"\r\n"
    body += f"--{boundary}--\r\n".encode()
    return bytes(body), f"multipart/form-data; boundary={boundary}"


def request(url, data=None, content_type=None, token=None):
    req = urllib.request.Request(url, data=data, method="POST" if data is not None else "GET")
    if content_type:
        req.add_header("Content-Type", content_type)
    if token:
        req.add_header("Cookie", f"access_token={token}")
    try:
        with urllib.request.urlopen(req) as resp:
            return resp.status, resp.headers, resp.read()
    except urllib.error.HTTPError as e:
        return e.code, e.headers, e.read()


def access_token(headers):
    for cookie in headers.get_all("Set-Cookie") or []:
        name, _, rest = cookie.partition("=")
        if name == "access_token":
            return rest.split(";", 1)[0]
    return None


def birth_date(i):
    today = datetime.date.today()
    months = today.year * 12 + today.month - 1 - (21 + i % 10) * 12 - i % 6
    return datetime.date(months // 12, months % 12 + 1, min(today.day, 28)).isoformat()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--api", default="http://localhost:8080", help="адрес API")
    parser.add_argument("--photos", default=Path(__file__).parent / "photos", help="каталог с JPG/PNG/WebP для анкет")
    parser.add_argument("--count", type=int, default=len(NAMES), help="сколько пользователей создать")
    args = parser.parse_args()

    photos = sorted(p for p in Path(args.photos).glob("*") if p.suffix.lower() in CONTENT_TYPES)
    if not photos:
        sys.exit(f"Положите фото в {args.photos}")
    api = args.api.rstrip("/") + "/api/v1"

    for i in range(1, args.count + 1):
        email = f"demo{i:02}@example.com"
        fields = [
            ("name", NAMES[(i - 1) % len(NAMES)]),
            ("email", email),
            ("password", PASSWORD),
            ("birth_date", birth_date(i)),
            ("sex", "female"),
            ("search_sex", "all"),
            ("dating_intent", INTENTS[i % 3]),
            ("about_me", f"Тестовая анкета {NAMES[(i - 1) % len(NAMES)]}. Люблю прогулки, музыку и новые знакомства."),
            ("search_age_from", 18),
            ("search_age_to", 100),
        ]
        fields += [("tags", TAGS[(i - 1 + j) % len(TAGS)]) for j in range(5 + i % 2)]
        body, content_type = multipart(fields, [("photos", photos[(i - 1) % len(photos)])])

        status, headers, resp = request(f"{api}/auth/register", body, content_type)
        if status == 409:
            print(f"{email}: уже есть, пропускаю")
            continue
        token = access_token(headers)
        if status != 201 or not token:
            sys.exit(f"{email}: регистрация {status} {resp.decode()}")

        status, _, resp = request(f"{api}/tests/current", token=token)
        if status != 200:
            sys.exit(f"{email}: тест {status} {resp.decode()}")
        test = json.loads(resp)
        answers = [{"question_id": q["id"], "value": 1 + (i * 3 + n * 5) % 7} for n, q in enumerate(test["questions"])]

        status, _, resp = request(f"{api}/tests/{test['test_id']}/results",
                                  json.dumps({"answers": answers}).encode(), "application/json", token)
        if status != 201:
            sys.exit(f"{email}: ответы {status} {resp.decode()}")
        print(f"{email}: создан, тип личности {json.loads(resp)['personality_type']}")


if __name__ == "__main__":
    main()
