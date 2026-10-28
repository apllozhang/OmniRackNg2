#!/usr/bin/env python3
import importlib.util
import json
from pathlib import Path
import subprocess
import tarfile
import tempfile
import unittest
from unittest import mock
from types import SimpleNamespace

spec = importlib.util.spec_from_file_location('omni_workspace', Path(__file__).with_name('omni-workspace.py'))
omni = importlib.util.module_from_spec(spec)
spec.loader.exec_module(omni)


class WorkspaceTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        repo = self.root / 'repo'
        repo.mkdir()
        def git(*args):
            return subprocess.run(['git', '-C', str(repo), *args], check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        git('init')
        git('config', 'user.name', 'Workspace test')
        git('config', 'user.email', 'test@example.invalid')
        (repo / 'app.txt').write_text('before\n')
        (repo / '.gitignore').write_text('ignored.log\n.env\n')
        git('add', '.')
        git('commit', '-m', 'baseline')
        self.args = SimpleNamespace(task_id='audit-one', actor='test-ai', goal='repair defect')

    def handoff_args(self, **kwargs):
        values = dict(task_id='audit-one', summary='implemented; verification pending', next_steps='run integration',
                      status='handoff', evidence=[], check=[], risk=['not yet deployed'])
        values.update(kwargs)
        return SimpleNamespace(**values)

    def test_collision_preserves_task(self):
        first = omni.task_new(self.root, self.args)
        with self.assertRaises(ValueError):
            omni.task_new(self.root, self.args)
        record = json.loads((self.root / 'tasks/audit-one/TASK.json').read_text())
        self.assertEqual(first['base_commit'], record['base_commit'])
        self.assertEqual(record['state'], 'active')

    def test_dirty_handoff_captures_patch_and_untracked_excludes_env(self):
        omni.task_new(self.root, self.args)
        tree = self.root / 'worktrees/audit-one'
        (tree / 'app.txt').write_text('after\n')
        (tree / 'new.py').write_text('print(42)\n')
        (tree / '.env').write_text('JWT_SECRET=do-not-copy\n')
        (tree / 'ignored.log').write_text('ignored data\n')
        snapshot = omni.handoff(self.root, self.handoff_args())
        latest = self.root / 'handoffs/latest'
        self.assertTrue(latest.is_symlink())
        self.assertIn('+after', (latest / 'working-tree.patch').read_text())
        with tarfile.open(latest / 'untracked.tar.gz') as archive:
            self.assertEqual(archive.getnames(), ['new.py'])
        self.assertEqual(snapshot['branch'], 'work/audit-one')
        self.assertIn(' M app.txt', snapshot['git_status'])
        self.assertEqual(snapshot['state'], 'handoff')
        self.assertEqual((latest / 'HANDOFF.json').stat().st_mode & 0o777, 0o600)

    def test_rejects_unsafe_id_and_symlink_untracked(self):
        for task_id in ['../escape', '.', 'ok/escape', 'BAD', 'ab', 'a..b']:
            with self.assertRaises(ValueError):
                omni.require_id(task_id)
        omni.task_new(self.root, self.args)
        (self.root / 'worktrees/audit-one/link').symlink_to('/etc/passwd')
        with self.assertRaises(ValueError):
            omni.handoff(self.root, self.handoff_args())
        self.assertFalse((self.root / 'handoffs/latest').exists())

    def test_missing_evidence_does_not_publish_latest(self):
        omni.task_new(self.root, self.args)
        with self.assertRaises(ValueError):
            omni.handoff(self.root, self.handoff_args(evidence=['evidence/missing.json']))
        self.assertFalse((self.root / 'handoffs/latest').exists())

    def test_concurrent_content_edit_refuses_publication(self):
        omni.task_new(self.root, self.args)
        tree = self.root / 'worktrees/audit-one'
        (tree / 'app.txt').write_text('first edit\n')
        original_git = omni.git
        def changing_git(directory, *args, **kwargs):
            result = original_git(directory, *args, **kwargs)
            if args[:2] == ('diff', '--no-ext-diff'):
                (tree / 'app.txt').write_text('concurrent second edit\n')
            return result
        with mock.patch.object(omni, 'git', side_effect=changing_git):
            with self.assertRaisesRegex(ValueError, 'contents changed'):
                omni.handoff(self.root, self.handoff_args())
        self.assertFalse((self.root / 'handoffs/latest').exists())

    def test_unexpected_branch_refuses_handoff(self):
        omni.task_new(self.root, self.args)
        tree = self.root / 'worktrees/audit-one'
        omni.git(tree, 'checkout', '-b', 'other-branch')
        with self.assertRaises(ValueError):
            omni.handoff(self.root, self.handoff_args())

    def test_changed_secret_file_refuses_complete_and_excludes_snapshot(self):
        omni.task_new(self.root, self.args)
        tree = self.root / 'worktrees/audit-one'
        (tree / 'secret.pem').write_text('credential-content\n')
        with self.assertRaises(ValueError):
            omni.handoff(self.root, self.handoff_args(status='complete'))
        snapshot = omni.handoff(self.root, self.handoff_args())
        self.assertEqual(snapshot['excluded_secret_paths'], ['secret.pem'])
        with tarfile.open(self.root / 'handoffs/latest/untracked.tar.gz') as archive:
            self.assertEqual(archive.getnames(), [])


if __name__ == '__main__':
    unittest.main()
