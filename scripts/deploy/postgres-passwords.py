#!/usr/bin/env python3

import sys
from pathlib import Path
from urllib.parse import unquote, urlsplit


def password_statement(env_dir: Path, service: str) -> str:
    role = f"{service}_app"
    prefix = "DATABASE_URL="
    values = [
        line[len(prefix):]
        for line in (env_dir / f"{service}.env").read_text().splitlines()
        if line.startswith(prefix)
    ]

    if len(values) != 1:
        raise ValueError

    url = urlsplit(values[0])

    if url.scheme not in ("postgres", "postgresql") or url.hostname != "postgres":
        raise ValueError

    if unquote(url.username or "", errors="strict") != role or not url.password:
        raise ValueError

    password = unquote(url.password, errors="strict")

    if any(character in password for character in "\0\r\n"):
        raise ValueError

    # Пароль становится SQL-литералом; кавычки экранируем независимо от формата URL.
    password = password.replace("'", "''")
    return f"ALTER ROLE \"{role}\" PASSWORD '{password}';"


def main() -> int:
    if len(sys.argv) != 2:
        print("usage: postgres-passwords.py <env-directory>", file=sys.stderr)
        return 1

    statements = []

    for service in ("auth", "profile", "matching", "chat", "onlineconf"):
        try:
            statements.append(password_statement(Path(sys.argv[1]), service))
        except (OSError, ValueError, UnicodeError):
            # Значение DATABASE_URL и исключение могут содержать пароль.
            print(f"invalid DATABASE_URL in {service}.env; expected {service}_app at postgres", file=sys.stderr)
            return 1

    # Выводим SQL только после проверки всех файлов сервисов.
    print("BEGIN;")
    print("SET LOCAL standard_conforming_strings = on;")
    print("\n".join(statements))
    print("COMMIT;")
    return 0


if __name__ == "__main__":
    sys.exit(main())
