"""Check HTTP -> PostgreSQL log correlation on a running backend."""
import argparse
import json
from pathlib import Path
import subprocess
import time
import urllib.error
import urllib.request
import uuid


def unique_fields(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError(f"Duplicate JSON field: {key}")
        result[key] = value
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--api", default="http://localhost:8080")
    source = parser.add_mutually_exclusive_group(required=True)
    source.add_argument("--log-file", type=Path, help="Local backend.log")
    source.add_argument("--compose", type=Path, help="Compose file; read /logs/backend.log in api")
    parser.add_argument("--env-file", default="docker.env")
    args = parser.parse_args()

    def read_log():
        if args.log_file:
            return args.log_file.read_text()
        return subprocess.run(
            ["docker", "compose", "--env-file", args.env_file, "-f", str(args.compose),
             "exec", "-T", "api", "cat", "/logs/backend.log"],
            check=True, capture_output=True, text=True, timeout=10,
        ).stdout

    # Fail early if the log file cannot be read.
    read_log()
    password = "Smoke-" + uuid.uuid4().hex
    request = urllib.request.Request(
        args.api.rstrip("/") + "/api/v1/auth/login",
        data=json.dumps({"email": f"smoke-{uuid.uuid4().hex}@example.com",
                         "password": password}).encode(),
        headers={"Content-Type": "application/json"}, method="POST",
    )
    try:
        response = urllib.request.urlopen(request, timeout=10)
    except urllib.error.HTTPError as exc:
        response = exc
    with response:
        if response.status != 401:
            raise AssertionError(f"Expected 401, received {response.status}")
        request_id = response.headers.get("X-Request-ID")
        if not request_id:
            raise AssertionError("Missing X-Request-ID response header")
        uuid.UUID(request_id)
        body = json.load(response)
        if body.get("error", {}).get("code") != "INVALID_CREDENTIALS":
            raise AssertionError("Expected INVALID_CREDENTIALS")

    # The response can reach the client before access logging finishes.
    deadline = time.monotonic() + 5
    while True:
        contents = read_log()
        records = [
            json.loads(line, object_pairs_hook=unique_fields)
            for line in contents.splitlines() if request_id in line
        ]
        records = [r for r in records if r.get("request_id") == request_id]
        http = [r for r in records if r.get("msg") == "http request"]
        db = [r for r in records if r.get("msg") == "database query"]
        if http and db:
            break
        if time.monotonic() >= deadline:
            raise AssertionError(f"Missing correlated logs: HTTP={len(http)}, DB={len(db)}")
        time.sleep(0.1)

    if len(http) != 1:
        raise AssertionError(f"Expected one HTTP log, received {len(http)}")
    access = http[0]
    if (access.get("method"), access.get("path"), access.get("status")) != (
        "POST", "/api/v1/auth/login", 401
    ):
        raise AssertionError("Incorrect HTTP log fields")
    for record in [access, *db]:
        duration = record.get("duration_ms")
        if type(duration) not in (int, float) or duration < 0:
            raise AssertionError("Invalid duration_ms")
    if not any(r.get("outcome") == "ok" and
               str(r.get("command_tag", "")).startswith("SELECT") for r in db):
        raise AssertionError("Missing successful SELECT log")
    if password in contents:
        raise AssertionError("Request password leaked into logs")
    print(f"PASS: backend.log contains HTTP and DB logs with request_id={request_id}")


if __name__ == "__main__":
    main()
