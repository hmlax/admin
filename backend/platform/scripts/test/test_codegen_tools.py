"""A missing generator must leave the checked-in frontend client intact."""
import hashlib
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[3]


class CodegenToolTests(unittest.TestCase):
    def test_missing_typescript_plugin_preserves_generated_client(self):
        client = ROOT.parent / 'frontend/src/api/generated/admin/v1/index.ts'
        before = hashlib.sha256(client.read_bytes()).hexdigest()
        with tempfile.TemporaryDirectory() as empty_path:
            environment = dict(os.environ, PATH=empty_path)
            result = subprocess.run(
                [sys.executable, str(ROOT / 'platform/tools/codegen.py'), 'ts'],
                cwd=ROOT, env=environment, capture_output=True, text=True, check=False,
            )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('run make plugin', result.stderr)
        self.assertEqual(hashlib.sha256(client.read_bytes()).hexdigest(), before)


if __name__ == '__main__':
    unittest.main()
