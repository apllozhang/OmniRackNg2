"""Exercise deployment decisions with a fake Docker; never use the real daemon."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


class DeploymentTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        source = Path(__file__).resolve().parents[1]
        shutil.copy(source / "deploy.sh", self.root / "deploy.sh")
        shutil.copy(source / "compose.assets.yml", self.root / "compose.assets.yml")
        (self.root / "scripts").mkdir()
        shutil.copy(source / "scripts/replica_manifest.py", self.root / "scripts/replica_manifest.py")
        self.fake = self.root / "fake-bin"
        self.fake.mkdir()
        docker = self.fake / "docker"
        docker.write_text('''#!/usr/bin/env python3
import os, sys
with open(os.environ['FAKE_CALLS'], 'a') as log:
    log.write(' '.join(sys.argv[1:]) + '\\n')
if sys.argv[1:3] == ['volume', 'ls'] and os.environ.get('EXISTING_VOLUME'):
    print('omnirack-replica_asset-postgres-data')
''')
        docker.chmod(0o755)
        self.calls = self.root / "calls.log"
        self.env = dict(os.environ, PATH=str(self.fake) + os.pathsep + os.environ["PATH"],
                        FAKE_CALLS=str(self.calls), PULL_BASE_IMAGES="false")

    def deploy(self, **overrides):
        return subprocess.run(["bash", "deploy.sh"], cwd=self.root,
                              env=dict(self.env, **overrides), capture_output=True, text=True)

    def test_fresh_deploy_uses_random_secrets_and_cached_base_images(self):
        result = self.deploy()
        self.assertEqual(result.returncode, 0, result.stderr)
        values = dict(line.split("=", 1) for line in (self.root / ".env").read_text().splitlines())
        self.assertEqual(values["ASSET_HTTP_PORT"], "15174")
        self.assertEqual(len(values["ADMIN_PASSWORD"]), 64)
        self.assertNotEqual(values["ADMIN_PASSWORD"], values["POSTGRES_PASSWORD"])
        self.assertEqual((self.root / ".env").stat().st_mode & 0o777, 0o600)
        self.assertIn("compose --env-file .env build\n", self.calls.read_text())
        self.assertNotIn("build --pull", self.calls.read_text())

    def test_redeploy_preserves_secrets_and_pull_is_explicit(self):
        self.assertEqual(self.deploy().returncode, 0)
        before = (self.root / ".env").read_bytes()
        result = self.deploy(PULL_BASE_IMAGES="true")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(before, (self.root / ".env").read_bytes())
        self.assertIn("build --pull", self.calls.read_text())

    def test_missing_config_for_existing_database_stops_before_build_or_up(self):
        result = self.deploy(EXISTING_VOLUME="yes")
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse((self.root / ".env").exists())
        self.assertNotIn(" up ", self.calls.read_text())
        self.assertNotIn(" build", self.calls.read_text())


if __name__ == "__main__":
    unittest.main()
