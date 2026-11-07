"""Verify the running infra-docker deployment without exposing tokens or secrets."""

import http.cookiejar
import json
import os
from pathlib import Path
import subprocess
import urllib.error
import urllib.parse
import urllib.request

try:
    import yaml
except ImportError as error:
    raise SystemExit("Install verification dependency: python3 -m pip install PyYAML==6.0.2") from error


ROOT = Path(__file__).resolve().parents[3]
COMPOSE = ['docker', 'compose', '--env-file', str(ROOT / '.env.infra'),
           '-f', str(ROOT / 'docker-compose.infra.yaml')]



def ordered_menus(items):
    # The existing menu tree builder does not promise sibling ordering.
    # Compare the same route content rather than independent SQL traversal order.
    result = []
    for item in items:
        node = dict(item)
        if 'children' in node:
            node['children'] = ordered_menus(node['children'])
        result.append(node)
    return sorted(result, key=lambda node: (node.get('name', ''), node.get('path', '')))


def main():
    config = json.loads(subprocess.check_output(COMPOSE + ['config', '--format', 'json']))
    env = config['services']['admin']['environment']
    nacos = os.environ.get('NACOS_PUBLISH_URL', 'http://127.0.0.1:8848').rstrip('/')
    login = urllib.request.Request(nacos + '/nacos/v1/auth/users/login',
                                   data=urllib.parse.urlencode({
                                       'username': env['NACOS_USERNAME'],
                                       'password': env['NACOS_PASSWORD'],
                                   }).encode())
    with urllib.request.urlopen(login, timeout=10) as response:
        nacos_token = json.load(response)['accessToken']
    for service in ['operator.grpc', 'admin.http']:
        query = urllib.parse.urlencode({
            'serviceName': service, 'groupName': env['NACOS_GROUP'],
            'namespaceId': env.get('NACOS_NAMESPACE_ID', ''),
            'accessToken': nacos_token, 'healthyOnly': 'true',
        })
        with urllib.request.urlopen(nacos + '/nacos/v1/ns/instance/list?' + query, timeout=10) as response:
            instances = json.load(response).get('hosts', [])
        assert any(instance.get('healthy') and instance.get('enabled') for instance in instances), service + ' has no healthy instance'
        print('PASS: healthy Nacos registration ' + service)

    documents = {}
    for data_id in ['server.yaml', 'data.yaml']:
        query = urllib.parse.urlencode({'dataId': data_id, 'group': env['NACOS_GROUP'],
                                       'tenant': env.get('NACOS_NAMESPACE_ID', ''), 'accessToken': nacos_token})
        with urllib.request.urlopen(nacos + '/nacos/v1/cs/configs?' + query, timeout=10) as response:
            documents[data_id] = yaml.safe_load(response.read())
    data = documents['data.yaml']
    redis = data['data']['redis']
    base = os.environ.get('API_SMOKE_URL', 'http://127.0.0.1:7788')
    opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))

    def request(method, path, body=None, token=None, headers=None):
        values = {'Content-Type': 'application/json'}
        if token:
            values['Authorization'] = 'Bearer ' + token
        values.update(headers or {})
        data = None if body is None else json.dumps(body).encode()
        with opener.open(urllib.request.Request(base + path, data=data, headers=values, method=method), timeout=10) as response:
            return json.load(response)

    request('GET', '/admin/v1/sso/login-info')
    try:
        request('GET', '/admin/v1/me')
        raise AssertionError('unauthenticated request was accepted')
    except urllib.error.HTTPError as error:
        assert error.code == 401, 'expected unauthenticated HTTP 401'
    captcha = request('GET', '/admin/v1/captcha')
    answer = subprocess.check_output([
        'docker', 'exec', '-e', 'REDISCLI_AUTH=' + redis.get('password', ''), 'infra-redis',
        'redis-cli', '-n', str(redis.get('db', 0)), '--raw', 'GET',
        'go-admin:captcha:' + captcha['captchaId'],
    ], text=True).strip()
    key = data['runtime']['crypto_key'].encode().hex()
    password = subprocess.check_output([
        'openssl', 'enc', '-aes-128-cbc', '-K', key, '-iv', key, '-base64', '-A',
    ], input=os.environ.get('ADMIN_SMOKE_PASSWORD', 'Abcd@1234').encode()).decode()
    login = request('POST', '/admin/v1/login', {
        'grant_type': 'password', 'client_type': 'admin',
        'username': os.environ.get('ADMIN_SMOKE_USERNAME', 'admin'), 'password': password,
    }, headers={'X-Captcha-Id': captcha['captchaId'], 'X-Captcha-Value': answer.lower()})
    token = login['access_token']
    user = request('GET', '/admin/v1/me', token=token)
    assert user.get('username') == os.environ.get('ADMIN_SMOKE_USERNAME', 'admin'), 'profile mismatch'
    initial = request('GET', '/admin/v1/initial-context', token=token)
    navigation = request('GET', '/admin/v1/routes', token=token)
    permissions = request('GET', '/admin/v1/perm-codes', token=token)
    assert ordered_menus(initial.get('menus', [])) == ordered_menus(navigation.get('items', [])), 'initial menus mismatch'
    assert initial.get('permissions', []) == permissions.get('codes', []), 'initial permissions mismatch'
    assert initial.get('hiddenFields', []) == permissions.get('hiddenFields', []), 'initial field permissions mismatch'
    refreshed = request('POST', '/admin/v1/refresh-token', {
        'grant_type': 'refresh_token', 'client_type': 'admin',
    })
    token = refreshed['access_token']
    request('POST', '/admin/v1/logout', {}, token=token)
    try:
        request('GET', '/admin/v1/me', token=token)
        raise AssertionError('revoked token was accepted')
    except urllib.error.HTTPError as error:
        assert error.code == 401, 'expected revoked-token HTTP 401'
    print('PASS: Nacos discovery, lowercase captcha login, profile, initial menus/permissions, refresh cookie and logout/revocation')


if __name__ == '__main__':
    try:
        main()
    except urllib.error.HTTPError as error:
        # Request URLs may contain Nacos access tokens; do not print them.
        raise SystemExit('Deployment check failed (HTTP ' + str(error.code) + ')') from error
