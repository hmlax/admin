"""Run real tenant onboarding and RBAC against an isolated Compose fixture.

Only the dedicated test Compose project is created and removed. Existing app
containers, Nacos configuration, databases and historical QA pointers are untouched.
"""

import argparse
import json
import os
from pathlib import Path
import secrets
import socket
import subprocess
import sys
import time
import traceback
import urllib.parse
import urllib.request

from tenant_lifecycle import Infrastructure, Lifecycle, Report, ROOT, private_json, require
from tenant_permissions import run as verify_permissions


class FixtureInfrastructure(Infrastructure):
    def __init__(self, crypto_key, redis):
        self.key = crypto_key.encode().hex()
        self.redis = redis


def available_port():
    with socket.socket() as listener:
        listener.bind(('127.0.0.1', 0))
        return listener.getsockname()[1]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--project', default='go-admin-core-e2e')
    parser.add_argument('--binary-dir', type=Path, required=True)
    args = parser.parse_args()
    require(args.project.startswith('go-admin-core-e2e'), 'only a dedicated core-e2e project is permitted')
    compose = ['docker', 'compose', '-f', str(Path(__file__).with_name('service-compose.yaml')), '-p', args.project]
    report = Report(ROOT / 'platform/artifacts/core-framework-e2e', '通用后台：真实租户与权限端到端验收')
    processes, logs, published = [], [], []
    credentials = {'ready': False, 'environment': 'isolated and removed after verification'}
    failure = None
    documents = {}
    group = 'GO_ADMIN_CORE_E2E_' + report.run_id
    nacos = None

    def mapped_port(service, number):
        return subprocess.check_output(compose + ['port', service, str(number)], text=True, timeout=10).strip()

    def configure(method, data_id, content=None):
        fields = {'dataId': data_id, 'group': group}
        if content is not None:
            fields.update(content=json.dumps(content), type='yaml')
        request = urllib.request.Request(nacos + '/nacos/v1/cs/configs',
                                         data=urllib.parse.urlencode(fields).encode(), method=method)
        with urllib.request.urlopen(request, timeout=10) as response:
            require(response.read().strip() == b'true', 'isolated Nacos config operation failed')

    try:
        with report.step('启动专用隔离数据库、Redis、MinIO 与 Nacos'):
            subprocess.run(compose + ['up', '-d', '--wait', '--wait-timeout', '90'],
                           cwd=ROOT, check=True, timeout=110)
            # The Redis container is resolved by this project's service, never by a shared name.
            redis_id = subprocess.check_output(compose + ['ps', '-q', 'redis'], text=True).strip()
            require(bool(redis_id), 'isolated Redis container missing')
            os.environ['E2E_REDIS_CONTAINER'] = redis_id
            crypto_key = secrets.token_hex(8)
            report.secrets.append(crypto_key)
            grpc_port, http_port = available_port(), available_port()
            base = 'http://127.0.0.1:' + str(http_port)
            os.environ['E2E_BASE_URL'] = base
            os.environ['E2E_TENANT_BASE_URL'] = base
            os.environ['ADMIN_SMOKE_USERNAME'] = 'admin'
            os.environ['ADMIN_SMOKE_PASSWORD'] = 'Abcd@1234'
            report.secrets.append('Abcd@1234')
            nacos = 'http://' + mapped_port('nacos', 8848)
            documents = {
                'server.yaml': {
                    'server': {
                        'rest': {'addr': '127.0.0.1:' + str(http_port), 'timeout': '30s'},
                        'grpc': {'addr': '127.0.0.1:' + str(grpc_port), 'timeout': '30s'},
                    },
                    'logger': {'type': 'std'}, 'authz': {'type': 'casbin'},
                },
                'data.yaml': {
                    'data': {
                        'database': {'driver': 'postgres', 'source': 'postgres://postgres:smoke-postgres@' + mapped_port('postgres', 5432) + '/gwa?sslmode=disable'},
                        'redis': {'addr': mapped_port('redis', 6379), 'db': 0},
                    },
                    'authn': {'type': 'jwt', 'jwt': {
                        'method': 'HS256', 'key': secrets.token_urlsafe(32),
                        'access_token_expires': '5400s', 'refresh_token_expires': '43200s',
                    }},
                    'oss': {'minio': {
                        'endpoint': mapped_port('minio', 9000), 'upload_host': mapped_port('minio', 9000),
                        'download_host': mapped_port('minio', 9000), 'access_key': 'smoke-user',
                        'secret_key': 'smoke-object-password', 'use_ssl': False,
                    }},
                    'runtime': {'crypto_key': crypto_key},
                },
            }
            report.secrets.append(documents['data.yaml']['authn']['jwt']['key'])
            infra = FixtureInfrastructure(crypto_key, documents['data.yaml']['data']['redis'])
            env = dict(os.environ, NACOS_ADDR=nacos, NACOS_GROUP=group, NACOS_NAMESPACE_ID='', NACOS_USERNAME='', NACOS_PASSWORD='')
            env['SERVICE_SMOKE_POSTGRES_DSN'] = documents['data.yaml']['data']['database']['source']

        with report.step('仅为专用测试库创建模型结构并发布独立 Nacos 配置'):
            subprocess.run(['go', 'test', '-tags=integration', './operator/internal/data/store',
                            '-run', '^TestServiceSmokeSchema$', '-count=1'], cwd=ROOT, env=env, check=True, timeout=90)
            for name, content in documents.items():
                configure('POST', name, content)
                published.append(name)

        with report.step('启动当前源码构建的 Hertz admin 与 Kratos operator'):
            for role, name in [('operator', 'server'), ('admin', 'admin')]:
                binary = args.binary_dir.resolve() / name
                require(binary.is_file(), 'missing freshly built ' + role + ' binary')
                log = (report.directory / (role + '.log')).open('w')
                logs.append(log)
                role_env = dict(env, NACOS_LOG_DIR=str(report.directory / (role + '-nacos-log')),
                                NACOS_CACHE_DIR=str(report.directory / (role + '-nacos-cache')))
                process = subprocess.Popen([str(binary)], cwd=ROOT, env=role_env, stdout=log, stderr=subprocess.STDOUT)
                processes.append(process)
                if role == 'operator':
                    deadline = time.monotonic() + 60
                    while True:
                        require(process.poll() is None, 'operator exited during startup; inspect its private log')
                        try:
                            with socket.create_connection(('127.0.0.1', grpc_port), timeout=0.5):
                                break
                        except OSError:
                            if time.monotonic() >= deadline:
                                raise RuntimeError('operator did not start within 60 seconds')
                            time.sleep(0.25)
            flow = Lifecycle(report, infra)
            flow.new_client().wait_ready()
            require(all(p.poll() is None for p in processes), 'an application exited during startup')

        flow.run()
        credentials.update(flow.credentials())
        verify_permissions(report, credentials, infra)
        report.ready = True
    except Exception as error:
        report.ready = False
        failure = report.redact(error)
        report.failure_trace = report.redact(traceback.format_exc())
        print('FAIL: ' + failure, file=sys.stderr, flush=True)
    finally:
        for process in reversed(processes):
            process.terminate()
        for process in processes:
            try:
                process.wait(timeout=12)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait(timeout=5)
        for log in logs:
            log.close()
        for name in published:
            try:
                configure('DELETE', name)
            except Exception as error:
                failure = failure or report.redact(error)
        cleanup = subprocess.run(compose + ['down', '--volumes'], cwd=ROOT, capture_output=True, text=True, timeout=45)
        if cleanup.returncode:
            failure = failure or 'isolated Compose cleanup failed'
        credentials['ready'] = False  # The throwaway deployment no longer exists.
        credentials['verified_before_teardown'] = report.ready
        for values in report.resources.values():
            if isinstance(values, list):
                for item in values:
                    if isinstance(item, dict) and 'retained' in item:
                        item['retained'] = False
        report.resources['environment'] = {'project': args.project, 'removed': cleanup.returncode == 0}
        report.save(os.environ.get('E2E_BASE_URL', ''), credentials, failure)
        for role in ['operator', 'admin']:
            path = report.directory / (role + '.log')
            if path.exists():
                path.chmod(0o600)
    return 1 if failure else 0


if __name__ == '__main__':
    sys.exit(main())
