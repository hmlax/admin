"""Generate the dedicated core gRPC client without importing core business code."""
import argparse
import hashlib
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[2]
RELATIVE = Path('muskdeer/strategy/console/v1/console.proto')
MIRROR = ROOT / 'platform/proto' / RELATIVE
DIGEST = MIRROR.with_suffix('.sha256')
CORE_PACKAGE = 'github.com/hmlax/muskdeer/deer/pb/strategy/console/v1;consolev1'
CLIENT_PACKAGE = 'go-admin/platform/pb/muskdeer/strategy/console/v1;consolev1'

def canonical():
    data = MIRROR.read_text()
    if data.count(CLIENT_PACKAGE) != 1:
        raise ValueError('invalid core client package binding')
    return data.replace(CLIENT_PACKAGE, CORE_PACKAGE)

def check():
    actual = hashlib.sha256(canonical().encode()).hexdigest()
    if actual != DIGEST.read_text().strip():
        raise ValueError('core contract changed without explicit synchronization')

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('action', choices=['sync', 'check', 'generate'])
    parser.add_argument('--source', type=Path)
    args = parser.parse_args()
    if args.action == 'sync':
        if args.source is None:
            parser.error('sync requires --source pointing to the reviewed core proto')
        data = args.source.read_text()
        if data.count(CORE_PACKAGE) != 1:
            raise ValueError('unexpected source contract')
        MIRROR.parent.mkdir(parents=True, exist_ok=True)
        MIRROR.write_text(data.replace(CORE_PACKAGE, CLIENT_PACKAGE))
        DIGEST.write_text(hashlib.sha256(data.encode()).hexdigest() + '\n')
    check()
    if args.action == 'generate':
        subprocess.run([
            'protoc', '-I', str(ROOT / 'platform/proto'),
            '--go_out=.', '--go_opt=module=go-admin',
            '--go-grpc_out=.', '--go-grpc_opt=module=go-admin', str(RELATIVE),
        ], cwd=ROOT, check=True)

if __name__ == '__main__':
    main()
