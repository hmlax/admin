"""Publish this application's Nacos configuration using the local Compose env."""

import json
import os
from pathlib import Path
import subprocess
import urllib.error
import urllib.parse
import urllib.request


ROOT = Path(__file__).resolve().parents[3]
COMPOSE = ['docker', 'compose', '--env-file', str(ROOT / '.env.infra'),
           '-f', str(ROOT / 'docker-compose.infra.yaml')]


def main():
    result = subprocess.run(COMPOSE + ['config', '--format', 'json'],
                            check=True, capture_output=True, text=True)
    config = json.loads(result.stdout)
    env = config['services']['operator']['environment']
    # The SDK uses the container hostname; the publisher runs on the host.
    base = os.environ.get('NACOS_PUBLISH_URL', 'http://127.0.0.1:8848').rstrip('/')

    def request(path, fields):
        data = urllib.parse.urlencode(fields).encode()
        with urllib.request.urlopen(urllib.request.Request(base + '/nacos' + path,
                                                          data=data), timeout=15) as response:
            return response.read()

    try:
        login = json.loads(request('/v1/auth/users/login', {
            'username': env['NACOS_USERNAME'], 'password': env['NACOS_PASSWORD'],
        }))
        token = login['accessToken']
        for data_id in ['server.yaml', 'data.yaml']:
            source = ROOT / 'platform/deploy/infra/nacos' / data_id
            if not source.is_file():
                raise SystemExit('Create local publication file from its .example: ' + str(source))
            content = source.read_text()
            body = request('/v1/cs/configs', {
                'accessToken': token, 'dataId': data_id,
                'group': env['NACOS_GROUP'], 'tenant': env.get('NACOS_NAMESPACE_ID', ''),
                'type': 'yaml', 'content': content,
            })
            if body.strip() != b'true':
                raise RuntimeError('Nacos did not accept configuration for ' + data_id)
            print('Published Nacos configuration: ' + env['NACOS_GROUP'] + '/' + data_id)
    except urllib.error.HTTPError as error:
        # Avoid printing response bodies or request URLs containing access tokens.
        raise SystemExit('Nacos rejected the request (HTTP ' + str(error.code) +
                         '); check credentials and config write permissions') from error


if __name__ == '__main__':
    main()
