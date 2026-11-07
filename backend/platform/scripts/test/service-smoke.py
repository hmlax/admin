"""Exercise both built processes against the isolated service-compose.yaml fixture."""
import http.cookiejar
import json
import os
from pathlib import Path
import socket
import subprocess
import tempfile
import time
import urllib.parse
import urllib.error
import urllib.request

ROOT = Path(__file__).resolve().parents[3]
COMPOSE = ['docker', 'compose', '-f', str(Path(__file__).with_name('service-compose.yaml')), '-p', 'gwa-service-smoke']

def port(service, number):
    return subprocess.check_output(COMPOSE + ['port', service, str(number)], text=True).strip()

def available_port():
    with socket.socket() as listener:
        listener.bind(('127.0.0.1', 0))
        return listener.getsockname()[1]

def main():
    env = os.environ.copy()
    grpc_port, http_port = available_port(), available_port()
    group = 'GO_ADMIN_SMOKE_' + str(time.time_ns())
    nacos = 'http://' + port('nacos', 8848)
    env.update(NACOS_ADDR=nacos, NACOS_GROUP=group, NACOS_NAMESPACE_ID='', NACOS_USERNAME='', NACOS_PASSWORD='')
    documents = {
        'server.yaml': {
            'server': {'rest': {'addr': '127.0.0.1:' + str(http_port), 'timeout': '30s'},
                       'grpc': {'addr': '127.0.0.1:' + str(grpc_port), 'timeout': '30s'},},
            'logger': {'type': 'std'}, 'authz': {'type': 'casbin'},
        },
        'data.yaml': {
            'data': {'database': {'driver': 'postgres', 'source': 'postgres://postgres:smoke-postgres@' + port('postgres', 5432) + '/gwa?sslmode=disable', 'migrate': False},
                     'redis': {'addr': port('redis', 6379), 'db': 0}},
            'authn': {'type': 'jwt', 'jwt': {'method': 'HS256', 'key': 'isolated-smoke-shared-secret', 'access_token_expires': '5400s', 'refresh_token_expires': '43200s'}},
            'oss': {'minio': {'endpoint': port('minio', 9000), 'upload_host': port('minio', 9000), 'download_host': port('minio', 9000), 'access_key': 'smoke-user', 'secret_key': 'smoke-object-password', 'use_ssl': False}},
            'runtime': {'crypto_key': 'f51d66a73d8a0927'},
        },
    }
    # Runtime constructors do not perform DDL. Prepare only this Compose fixture.
    schema_env = env.copy()
    schema_env['SERVICE_SMOKE_POSTGRES_DSN'] = documents['data.yaml']['data']['database']['source']
    subprocess.run(['go', 'test', '-tags=integration', './operator/internal/data/store',
                    '-run', '^TestServiceSmokeSchema$', '-count=1'],
                   cwd=ROOT, env=schema_env, check=True)
    def nacos_config(method, data_id, content=None):
        fields = {'dataId': data_id, 'group': group}
        if content is not None:
            fields.update(content=json.dumps(content), type='yaml')
        request = urllib.request.Request(nacos + '/nacos/v1/cs/configs', data=urllib.parse.urlencode(fields).encode(), method=method)
        with urllib.request.urlopen(request, timeout=10) as response:
            assert response.read().strip() == b'true', 'Nacos config operation failed'
    for data_id, content in documents.items():
        nacos_config('POST', data_id, content)
    jar = http.cookiejar.CookieJar()
    opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
    base = 'http://127.0.0.1:' + str(http_port)
    def request(method, path, data=None, token=None, headers=None):
        content = None if data is None else json.dumps(data).encode()
        values = {'Content-Type': 'application/json'}
        if token:
            values['Authorization'] = 'Bearer ' + token
        values.update(headers or {})
        req = urllib.request.Request(base + path, data=content, method=method, headers=values)
        with opener.open(req, timeout=10) as response:
            return json.load(response)
    processes = []
    with tempfile.TemporaryDirectory(prefix='gwa-service-smoke-') as directory:
        logs = []
        try:
            for role in ['operator', 'admin']:
                log = open(Path(directory) / (role + '.log'), 'w')
                logs.append(log)
                binary = env.get(role.upper() + '_TEST_BINARY', '/tmp/gwa-split-' + role)
                processes.append(subprocess.Popen([binary], cwd=ROOT, env=env, stdout=log, stderr=subprocess.STDOUT))
            captcha = None
            for _ in range(60):
                if any(process.poll() is not None for process in processes):
                    raise RuntimeError('a service exited during startup')
                try:
                    captcha = request('GET', '/admin/v1/captcha')
                    break
                # Startup probes retry while operator initializes its services.
                # A terminal failure prints both process logs below.
                except (urllib.error.URLError, ConnectionError, TimeoutError):
                    time.sleep(0.5)
            if captcha is None:
                raise RuntimeError('services did not become ready')
            captcha_id = captcha['captchaId']
            answer = subprocess.check_output(COMPOSE + ['exec', '-T', 'redis', 'redis-cli', '--raw', 'GET', 'go-admin:captcha:' + captcha_id], text=True).strip()
            key = b'f51d66a73d8a0927'.hex()
            password = subprocess.check_output(['openssl', 'enc', '-aes-128-cbc', '-K', key, '-iv', key, '-base64', '-A'], input=b'Abcd@1234').decode()
            login = request('POST', '/admin/v1/login', {'grant_type': 'password', 'client_type': 'admin', 'username': 'admin', 'password': password}, headers={'X-Captcha-Id': captcha_id, 'X-Captcha-Value': answer})
            token = login['access_token']
            assert any(cookie.name == 'refresh_token' and cookie.has_nonstandard_attr('HttpOnly') for cookie in jar), 'HttpOnly refresh cookie missing'
            profile = request('GET', '/admin/v1/me', token=token)
            assert profile.get('username') == 'admin', 'operator context did not resolve current user'
            refreshed = request('POST', '/admin/v1/refresh-token', {'grant_type': 'refresh_token', 'client_type': 'admin'})
            token = refreshed['access_token']
            code = 'service_split_smoke_' + str(time.time_ns())
            request('POST', '/admin/v1/dict/types', {'data': {'typeCode': code, 'typeName': 'Split Smoke', 'isEnabled': True}}, token)
            listing = request('GET', '/admin/v1/dict/types?noPaging=true', token=token)
            dictionary = next(item for item in listing['items'] if item.get('typeCode') == code)
            identifier = dictionary['id']
            request('PUT', '/admin/v1/dict/types/' + str(identifier), {'data': {'typeName': 'Split Smoke Updated'}, 'updateMask': 'typeName'}, token)
            updated = request('GET', '/admin/v1/dict/types/' + str(identifier), token=token)
            assert updated['typeName'] == 'Split Smoke Updated', 'typed CRUD update was lost'
            request('DELETE', '/admin/v1/dict/types?ids=' + str(identifier), token=token)
            remaining = request('GET', '/admin/v1/dict/types?noPaging=true', token=token)
            assert not any(item.get('id') == identifier for item in remaining.get('items', [])), 'deleted dictionary still exists'
            language_codes = {'smoke-' + str(time.time_ns()) + suffix for suffix in ['-A', '-B']}
            request('POST', '/admin/v1/dict/langs/batch', {
                'items': [{'languageCode': code, 'languageName': code, 'nativeName': code} for code in sorted(language_codes)]
            }, token)
            languages = request('GET', '/admin/v1/dict/langs?noPaging=true', token=token)
            created_languages = [item for item in languages['items'] if item.get('languageCode') in language_codes]
            assert len(created_languages) == 2, 'typed language batch did not reach operator'
            for language in created_languages:
                assert language['createdBy'] == profile['id'], 'operator did not assign the authenticated batch actor'
                request('DELETE', '/admin/v1/dict/langs?id=' + str(language['id']), token=token)
            request('POST', '/admin/v1/logout', {}, token)
            try:
                request('GET', '/admin/v1/me', token=token)
                raise AssertionError('revoked access token still accepted')
            except urllib.error.HTTPError as error:
                assert error.code == 401, 'expected revoked-token response 401'
            print('PASS: seeded login, captcha forwarding, HttpOnly refresh cookies, current user, refresh rotation, typed CRUD/batch, logout/revocation')
        except Exception:
            for log in logs:
                log.flush()
            for role in ['operator', 'admin']:
                text = (Path(directory) / (role + '.log')).read_text()
                # Infrastructure fixture logs contain no personal credentials.
                print(role + ' startup tail:\n' + '\n'.join(text.splitlines()[-18:]))
            raise
        finally:
            for process in reversed(processes):
                process.terminate()
            for process in processes:
                try:
                    process.wait(timeout=12)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()
            for log in logs:
                log.close()
            for data_id in documents:
                nacos_config('DELETE', data_id)

if __name__ == '__main__':
    main()
