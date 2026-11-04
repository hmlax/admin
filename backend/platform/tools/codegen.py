"""Generate operator RPC models and hz-owned admin HTTP adapters and assets."""

import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile


ROOT = Path(__file__).resolve().parents[2]
HTTP_IDL = ROOT / 'admin/idl/proto/admin/v1'
TEMPLATES = {
    'api': 'buf.gen.yaml',
    'openapi': 'buf.admin.openapi.gen.yaml',
    'ts': 'buf.react.admin.typescript.gen.yaml',
}


def project_http(directory, assets_only=False):
    raw = directory / 'source.bin'
    projected = directory / 'http.bin'
    subprocess.run(['buf', 'build', '--as-file-descriptor-set', '-o', str(raw)], cwd=ROOT, check=True)
    command = ['go', 'run', './platform/tools/httpgen', '-descriptor', str(raw), '-http-descriptor', str(projected)]
    if assets_only:
        command.append('-assets-only')
    subprocess.run(command, cwd=ROOT, check=True)
    return projected


def update_hz(directory):
    subprocess.run(['buf', 'generate', 'admin/idl/proto', '--template', 'buf.admin.gen.yaml'], cwd=ROOT, check=True)
    project_http(directory)
    dependencies = directory / 'proto'
    subprocess.run(['buf', 'export', '.', '-o', str(dependencies)], cwd=ROOT, check=True)
    stage = directory / 'admin'
    stage.mkdir()
    shutil.copyfile(ROOT / 'admin/.hz', stage / '.hz')
    for source in sorted(HTTP_IDL.glob('*.proto')):
        if '\nservice ' not in source.read_text():
            continue
        command = [
            os.environ.get('HZ', 'hz'), 'update', '--idl', str(source),
            '--module', 'go-admin', '--out_dir', str(stage),
            '--customize_package', str(ROOT / 'admin/hz.yaml'),
            '--use', 'go-admin/admin/biz/model', '--no_recurse',
            '-I', str(ROOT / 'admin/idl/proto'), '-I', str(dependencies),
        ]
        result = subprocess.run(command, cwd=ROOT, capture_output=True, text=True)
        if result.returncode:
            sys.stderr.write(result.stdout + result.stderr)
            raise subprocess.CalledProcessError(result.returncode, command)
        print('hz update:', source.relative_to(ROOT), flush=True)
    generated = sorted((stage / 'biz/router').glob('*_generated.go'))
    if not generated:
        raise RuntimeError('hz update did not generate any route registrations')
    destination = ROOT / 'admin/biz/router'
    destination.mkdir(parents=True, exist_ok=True)
    names = {path.name for path in generated}
    for old in destination.glob('*_generated.go'):
        if old.name not in names:
            old.unlink()
    for source in generated:
        shutil.copyfile(source, destination / source.name)

def main():
    if len(sys.argv) != 2 or sys.argv[1] not in {*TEMPLATES, 'hz'}:
        raise SystemExit('usage: codegen.py api|hz|openapi|ts')
    target = sys.argv[1]
    if target == 'api':
        sources = sorted((ROOT / 'platform/proto/operator/v1').glob('*.proto'))
        command = ['buf', 'generate', '--template', TEMPLATES[target]]
        for source in sources:
            if not source.name.startswith('model_'):
                command.extend(['--path', str(source.relative_to(ROOT))])
        subprocess.run(command, cwd=ROOT, check=True)
        generated = ROOT / 'platform/pb/operator/v1'
        for source in generated.iterdir():
            source.rename(generated.parent / source.name)
        generated.rmdir()
        return
    with tempfile.TemporaryDirectory(prefix='go-admin-hz-') as work:
        directory = Path(work)
        if target == 'hz':
            update_hz(directory)
            return
        projected = project_http(directory, assets_only=True)
        command = ['buf', 'generate', str(projected), '--template', TEMPLATES[target]]
        names = {source.name for source in HTTP_IDL.glob('*.proto')}
        if target == 'openapi':
            names.add('openapi.proto')
        for name in sorted(names):
            command.extend(['--path', 'admin/v1/' + name])
        subprocess.run(command, cwd=ROOT, check=True)


if __name__ == '__main__':
    main()
