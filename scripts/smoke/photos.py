#!/usr/bin/env python3
"""Isolated API/PostgreSQL/MinIO photo smoke test; no production env files."""
import hashlib
import http.cookiejar
import json
import os
from pathlib import Path
import secrets
import subprocess
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request

ROOT = Path(__file__).resolve().parents[2]
COMPOSE = ['docker', 'compose', '-f', str(Path(__file__).with_name('compose.yml'))]
BASE = 'http://127.0.0.1:28080'


def compose(*args, **kwargs):
    return subprocess.run([*COMPOSE, *args], check=True, **kwargs)


def wait_url(url, process=None):
    for _ in range(120):
        if process is not None and process.poll() is not None:
            raise RuntimeError('API exited before becoming ready')
        try:
            with urllib.request.urlopen(url, timeout=1) as response:
                if response.status == 200:
                    return
        except (OSError, urllib.error.URLError):
            pass
        time.sleep(.25)
    raise RuntimeError('Readiness timeout: ' + url)


def scenario():
    client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))

    def request(path, method='GET', data=None, content_type='application/json', expected=200):
        if isinstance(data, dict):
            data = json.dumps(data).encode()
        req = urllib.request.Request(BASE + '/api/v1' + path, method=method, data=data,
                                     headers={'Content-Type': content_type})
        try:
            response = client.open(req, timeout=15)
        except urllib.error.HTTPError as exc:
            response = exc
        with response:
            payload = response.read()
            assert response.status == expected, (path, response.status, payload.decode(errors='replace'))
            return json.loads(payload)

    request('/auth/register', 'POST', {'email': 'photo-smoke@example.com', 'password': 'PhotoTest123!'}, expected=201)
    request('/profile/me', 'PATCH', {'name': 'Фото тест', 'birth_date': '2000-01-01',
                                      'sex': 'male', 'dating_goal': 'relationship'})
    request('/filters/me', 'PUT', {'sex': 'all', 'age_from': 18, 'age_to': 100})
    files = sorted((ROOT / 'db/data/cats').glob('*.jpg'))[:2]
    assert len(files) == 2, 'Need at least two JPEG files in db/data/cats'
    uploaded = []
    for file in files:
        data = file.read_bytes()
        boundary = 'photo-smoke-' + secrets.token_hex(16)
        body = (f'--{boundary}\r\nContent-Disposition: form-data; name="photo"; filename="{file.name}"\r\n'
                'Content-Type: image/jpeg\r\n\r\n').encode() + data + f'\r\n--{boundary}--\r\n'.encode()
        result = request('/profile/me/photos', 'POST', body, 'multipart/form-data; boundary=' + boundary, 201)
        photo = result['photos'][-1]
        parsed = urllib.parse.urlsplit(photo['url'])
        assert parsed.netloc == '127.0.0.1:29000', 'URL must be directly accessible to the browser'
        with urllib.request.urlopen(photo['url'], timeout=10) as response:
            assert response.status == 200
            assert response.headers.get_content_type() == 'image/jpeg'
            assert hashlib.sha256(response.read()).digest() == hashlib.sha256(data).digest(), 'Downloaded bytes differ'
        anonymous = urllib.parse.urlunsplit(parsed._replace(query=''))
        try:
            urllib.request.urlopen(anonymous, timeout=10).close()
        except urllib.error.HTTPError as exc:
            assert exc.code == 403, exc.code
        else:
            raise AssertionError('Bucket unexpectedly permits anonymous access')
        uploaded.append(photo)
        print('PASS upload and signed download:', file.name)

    expected_ids = [p['id'] for p in reversed(uploaded)]
    result = request('/profile/me/photos/order', 'PUT', {'photo_ids': expected_ids})
    assert [p['id'] for p in result['photos']] == expected_ids
    removed = uploaded[0]
    result = request('/profile/me/photos/' + str(removed['id']), 'DELETE')
    assert [p['id'] for p in result['photos']] == [uploaded[1]['id']]
    try:
        urllib.request.urlopen(removed['url'], timeout=10).close()
    except urllib.error.HTTPError as exc:
        assert exc.code == 404, exc.code
        assert b'NoSuchKey' in exc.read(), 'Deletion must remove the object, not merely revoke access'
    else:
        raise AssertionError('Deleted object still exists in MinIO')
    result = request('/profile/me/photos/' + str(uploaded[1]['id']), 'DELETE', expected=409)
    assert result['error']['code'] == 'LAST_PHOTO'
    with urllib.request.urlopen(uploaded[1]['url'], timeout=10) as response:
        assert response.status == 200
    print('PASS reorder, object deletion and last-photo protection')


def main():
    process = None
    # Only this dedicated Compose project is removed, including its test data.
    with tempfile.TemporaryDirectory(prefix='photo-smoke-') as directory:
        binary = Path(directory) / 'api'
        subprocess.run(['go', 'build', '-o', str(binary), './cmd/api'], cwd=ROOT, check=True)
        try:
            compose('up', '-d', '--wait', '--wait-timeout', '60')
            wait_url('http://127.0.0.1:29000/minio/health/ready')
            for migration in sorted((ROOT / 'db/migrations').glob('*.up.sql')):
                compose('exec', '-T', 'postgres', 'psql', '-X', '-q', '-v', 'ON_ERROR_STOP=1',
                        '-U', 'photo_test', '-d', 'photo_test', input=migration.read_bytes(), stdout=subprocess.DEVNULL)
            env = dict(os.environ, DATABASE_URL='postgres://photo_test:photo_test@127.0.0.1:25432/photo_test?sslmode=disable',
                       REDIS_ADDR='127.0.0.1:26379', JWT_SECRET=secrets.token_hex(32), COOKIE_SECURE='false',
                       PORT='28080', MINIO_PUBLIC_ENDPOINT='127.0.0.1:29000', MINIO_ENDPOINT='127.0.0.1:29000', MINIO_ACCESS_KEY='photo_test',
                       MINIO_SECRET_KEY='photo-test-password', MINIO_BUCKET='photo-smoke', MINIO_USE_TLS='false',
                       MINIO_PHOTO_URL_TTL='15m')
            with tempfile.TemporaryFile() as log:
                process = subprocess.Popen([str(binary)], cwd=ROOT, env=env, stdout=log, stderr=subprocess.STDOUT)
                try:
                    wait_url(BASE + '/health', process)
                    scenario()
                except BaseException:
                    log.seek(0)
                    print(log.read().decode(errors='replace'))
                    raise
                finally:
                    process.terminate()
                    try:
                        process.wait(timeout=5)
                    except subprocess.TimeoutExpired:
                        process.kill()
                        process.wait()
        finally:
            compose('down', '--volumes')
    print('PASS photo smoke test')


if __name__ == '__main__':
    main()
