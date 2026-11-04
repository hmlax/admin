"""Publish the disposable SSO CI fixture as server.yaml/data.yaml."""

import json
import urllib.parse
import urllib.request


documents = {
    'server.yaml': {
        'server': {
            'rest': {'addr': ':7788', 'timeout': '30s'},
            'grpc': {'addr': ':7790', 'timeout': '30s'},
        },
        'logger': {'type': 'std'}, 'authz': {'type': 'casbin'},
        'runtime': {'sso': {
            'issuer': 'http://127.0.0.1:8081/realms/gwa',
            'client_id': 'gwa-admin', 'client_secret': 'gwa-secret',
            'redirect_url': 'http://localhost:15889/auth/sso/callback',
            'auto_create': True,
        }},
    },
    'data.yaml': {
        'data': {
            'database': {'driver': 'postgres', 'source': 'host=127.0.0.1 port=5432 user=postgres password=*Abcd123456 dbname=gwa sslmode=disable', 'migrate': True},
            'redis': {'addr': '127.0.0.1:6379', 'password': '*Abcd123456'},
        },
        'oss': {'minio': {'endpoint': '127.0.0.1:9000', 'access_key': 'ci-only', 'secret_key': 'ci-only-secret'}},
        'authn': {'type': 'jwt', 'jwt': {'method': 'HS256', 'key': 'ci-only-shared-test-secret', 'access_token_expires': '5400s', 'refresh_token_expires': '43200s'}},
        'runtime': {'crypto_key': 'f51d66a73d8a0927'},
    },
}

for data_id, content in documents.items():
    fields = {'dataId': data_id, 'group': 'GO_ADMIN_SSO_CI', 'type': 'yaml', 'content': json.dumps(content)}
    request = urllib.request.Request('http://127.0.0.1:8848/nacos/v1/cs/configs',
                                     data=urllib.parse.urlencode(fields).encode())
    with urllib.request.urlopen(request, timeout=15) as response:
        assert response.read().strip() == b'true', 'Nacos fixture publication failed'
    print('Published CI fixture: ' + data_id)
