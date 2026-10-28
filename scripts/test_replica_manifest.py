import tempfile
import unittest
from pathlib import Path
from replica_manifest import excluded, inventory


class ManifestTests(unittest.TestCase):
    def test_secret_and_generated_changes_do_not_change_source_identity(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "main.go").write_text("package main")
            before = inventory(root)
            (root / ".env").write_text("PASSWORD=secret")
            (root / ".env.backup.20261028").write_text("PASSWORD=old")
            (root / "artifacts").mkdir()
            (root / "artifacts" / "result.json").write_text("{}")
            self.assertEqual(before, inventory(root))
            (root / "main.go").write_text("package changed")
            self.assertNotEqual(before["source_sha256"], inventory(root)["source_sha256"])

    def test_examples_are_retained_but_dependencies_are_excluded(self):
        self.assertFalse(excluded(".env.asset.example"))
        self.assertFalse(excluded("frontend/package-lock.json"))
        self.assertTrue(excluded("frontend/node_modules/library/index.js"))
        self.assertTrue(excluded("backend/bin/server"))

    def test_symlink_cannot_escape_source_snapshot(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "secret-link").symlink_to("/etc/hosts")
            with self.assertRaisesRegex(ValueError, "symlink"):
                inventory(root)


if __name__ == "__main__":
    unittest.main()
