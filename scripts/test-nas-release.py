#!/usr/bin/env python3
"""Release safety regression tests. No Docker, HTTP or NAS writes."""
import copy
import fcntl
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest import mock

spec = importlib.util.spec_from_file_location('nas_release', Path(__file__).with_name('nas-release.py'))
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)

class ReleaseSafety(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        root = Path(self.temp.name).resolve()
        self.paths = mock.patch.multiple(m, ROOT=root / 'releases', SHARED=root / 'shared', CURRENT=root / 'current')
        self.paths.start()
        m.ROOT.mkdir()
        (m.SHARED / 'env').mkdir(parents=True)
        self.directory, self.record = self.make_release('v2')
        self.old_dir, self.old = self.make_release('v1')
        m.CURRENT.symlink_to(self.old_dir)
    def tearDown(self):
        self.paths.stop()
        self.temp.cleanup()
    def make_release(self, version):
        directory = m.ROOT / version
        directory.mkdir()
        env = m.SHARED / 'env' / (version + '.env')
        env.write_text('JWT_SECRET=do-not-print\n')
        env.chmod(0o600)
        (directory / 'compose.json').write_text('{}')
        evidence = {'passed': True, 'source_sha256': 'a' * 64, 'git_commit': 'b' * 40,
                    'checks': {k: 'passed' for k in ('go', 'frontend', 'api_contract', 'integration', 'restore', 'schema_compatibility')}}
        (directory / 'checks.json').write_text(json.dumps(evidence))
        record = {'schema_version': 2, 'version': version, 'source_sha256': 'a' * 64,
                  'git_commit': 'b' * 40, 'env_file': str(env), 'env_sha256': m.digest(env),
                  'compose_files': [str(directory / 'compose.json')],
                  'file_sha256': {name: m.digest(directory / name) for name in ('compose.json', 'checks.json')},
                  'images': {component: {'tag': 'omnirack-replica-' + component + ':' + version,
                                        'id': 'sha256:' + digit * 64} for component, digit in [('backend', 'c'), ('frontend', 'd')]},
                  'validation': {'passed': True, 'evidence_file': 'checks.json', 'schema_backward_compatible': True,
                                 'rollback_compatible_from': ['v1']}}
        (directory / 'RELEASE.json').write_text(json.dumps(record))
        return directory, record
    def topology(self):
        return {'name': m.PROJECT, 'services': {
            'postgres': {'image': 'postgres:16-alpine', 'volumes': [{'type': 'volume', 'source': 'asset-postgres-data', 'target': '/var/lib/postgresql/data'}]},
            'backend': {'image': self.record['images']['backend']['tag']},
            'frontend': {'image': self.record['images']['frontend']['tag'], 'ports': [{'published': '15174', 'target': 80}]}},
            'volumes': {'asset-postgres-data': {'name': m.PROJECT + '_asset-postgres-data'}},
            'networks': {'default': {'name': m.PROJECT + '_default'}}}
    def test_rejects_path_escape_and_symlink_release(self):
        for value in ('.', '..', '../v2', '/v2', 'v..2', '', 'v/2', 'v2\n'):
            with self.subTest(value=value), self.assertRaises(m.ReleaseError):
                m.version_id(value)
        (m.ROOT / 'alias').symlink_to(self.directory)
        with self.assertRaises(m.ReleaseError):
            m.release_dir('alias')
    def test_rejects_member_traversal_symlink_and_double_slash(self):
        (self.directory / 'alias').symlink_to(self.old_dir / 'checks.json')
        for path in ('/tmp/x', '../v1/checks.json', 'x//y', './checks.json', 'alias'):
            with self.subTest(path=path), self.assertRaises(m.ReleaseError):
                m.member(self.directory, path)
    def test_metadata_checks_hashes_secrets_and_evidence(self):
        m.validate_metadata(self.directory, self.record)
        for change in ('digest', 'env-mode', 'wrong-source', 'missing-restore', 'schema', 'external-compose'):
            record = copy.deepcopy(self.record)
            evidence_path = self.directory / 'checks.json'
            original = evidence_path.read_text()
            if change == 'digest':
                record['file_sha256']['checks.json'] = '0' * 64
            elif change == 'env-mode':
                Path(record['env_file']).chmod(0o644)
            elif change == 'wrong-source':
                record['source_sha256'] = '1' * 64
            elif change == 'missing-restore':
                evidence = json.loads(original)
                del evidence['checks']['restore']
                evidence_path.write_text(json.dumps(evidence))
                record['file_sha256']['checks.json'] = m.digest(evidence_path)
            elif change == 'schema':
                record['schema_version'] = 1
            else:
                record['compose_files'] = ['/tmp/compose.json']
            with self.subTest(change=change), self.assertRaises(m.ReleaseError):
                m.validate_metadata(self.directory, record)
            Path(record['env_file']).chmod(0o600)
            evidence_path.write_text(original)
    def test_topology_rejects_other_instances_mounts_and_privilege(self):
        m.validate_topology(self.topology(), self.record)
        modifications = [
            lambda x: x.update(name='omnirackng2'),
            lambda x: x['services']['frontend']['ports'][0].update(published='15173'),
            lambda x: x['services']['backend'].update(volumes=[{'type': 'bind', 'source': '/'}]),
            lambda x: x['services']['postgres'].update(ports=[{'published': '5432'}]),
            lambda x: x['services']['backend'].update(privileged=True),
            lambda x: x['services']['backend'].update(network_mode='host'),
            lambda x: x['services']['backend'].update(image='other:latest'),
            lambda x: x['services']['frontend'].update(environment={'JWT_SECRET': 'secret'}),
            lambda x: x['services']['backend'].update(entrypoint=['/bin/sh']),
            lambda x: x['services'].update(extra={'image': 'anything'}),
            lambda x: x['volumes']['asset-postgres-data'].update(name='another_db'),
            lambda x: x['networks']['default'].update(external=True),
        ]
        for index, change in enumerate(modifications):
            config = self.topology()
            change(config)
            with self.subTest(index=index), self.assertRaises(m.ReleaseError):
                m.validate_topology(config, self.record)
    def activation_mocks(self):
        def validate(version):
            return (self.directory, self.record) if version == 'v2' else (self.old_dir, self.old)
        return mock.patch.object(m, 'validate', side_effect=validate)
    def test_lock_taken_before_reading_current(self):
        def read_current():
            # A second open cannot lock while activation owns the lock.
            with (m.SHARED / 'activation.lock').open('a') as lock:
                with self.assertRaises(BlockingIOError):
                    fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
            return 'v1'
        with self.activation_mocks(), mock.patch.object(m, 'current_version', side_effect=read_current), mock.patch.object(m, 'perform'):
            m.activate('v2')
        self.assertEqual(m.CURRENT.resolve(), self.directory)
    def test_failure_rolls_back_application_and_preserves_pointer(self):
        with self.activation_mocks(), mock.patch.object(m, 'perform', side_effect=[m.ReleaseError('fail'), None]) as perform:
            with self.assertRaisesRegex(m.ReleaseError, 'previous application restored'):
                m.activate('v2')
            self.assertEqual(perform.call_count, 2)
        self.assertEqual(m.CURRENT.resolve(), self.old_dir)
        events = [json.loads(line)['event'] for line in (m.SHARED / 'release-manager-history.jsonl').read_text().splitlines()]
        self.assertEqual(events[-1], 'application_rollback_succeeded')
    def test_failed_rollback_records_unknown_running_state(self):
        with self.activation_mocks(), mock.patch.object(m, 'perform', side_effect=m.ReleaseError('fail')):
            with self.assertRaisesRegex(m.ReleaseError, 'rollback failed'):
                m.activate('v2')
        last = json.loads((m.SHARED / 'release-manager-history.jsonl').read_text().splitlines()[-1])
        self.assertEqual(last['running_state'], 'unknown')
        self.assertEqual(m.CURRENT.resolve(), self.old_dir)
    def test_journal_failure_after_healthy_does_not_rollback(self):
        original = m.journal
        def journal(event):
            if event['event'] == 'application_healthy':
                raise OSError('disk full')
            original(event)
        with self.activation_mocks(), mock.patch.object(m, 'perform') as perform, mock.patch.object(m, 'journal', side_effect=journal):
            with self.assertRaises(OSError):
                m.activate('v2')
            self.assertEqual(perform.call_count, 1)
        self.assertEqual(m.CURRENT.resolve(), self.old_dir)
    def test_changed_environment_rejected_before_any_mutation(self):
        self.record['env_sha256'] = '0' * 64
        with self.activation_mocks(), mock.patch.object(m, 'perform') as perform:
            with self.assertRaisesRegex(m.ReleaseError, 'environment changed'):
                m.activate('v2')
            perform.assert_not_called()
    def test_perform_pins_image_ids_and_changes_only_app_services(self):
        commands = []
        def run(command):
            commands.append(command)
            # Final -f is our private immutable override.
            pinned = Path(command[command.index('up') - 1])
            override = json.loads(pinned.read_text())
            self.assertEqual(override['services']['backend']['image'], self.record['images']['backend']['id'])
            self.assertEqual(override['services']['frontend']['pull_policy'], 'never')
            return ''
        with mock.patch.object(m, 'run', side_effect=run), mock.patch.object(m, 'healthy'):
            m.perform(self.record)
        self.assertEqual(commands[0][-2:], ['backend', 'frontend'])
        self.assertIn('--no-deps', commands[0])
        self.assertIn('--no-build', commands[0])
    def test_adoption_checks_identity_then_only_updates_metadata(self):
        self.record['kind'] = 'rollback_snapshot'
        self.record['baseline_of'] = 'v1'
        with self.activation_mocks(), mock.patch.object(m, 'healthy') as healthy, mock.patch.object(m, 'perform') as perform:
            m.adopt_baseline('v2')
        healthy.assert_called_once_with(self.record)
        perform.assert_not_called()
        self.assertEqual(m.current_version(), 'v2')
    def test_adoption_rejects_changed_images_without_mutation(self):
        self.record['kind'] = 'rollback_snapshot'
        self.record['baseline_of'] = 'v1'
        self.record['images']['backend']['id'] = 'sha256:' + '0' * 64
        with self.activation_mocks(), mock.patch.object(m, 'healthy') as healthy, mock.patch.object(m, 'perform') as perform:
            with self.assertRaisesRegex(m.ReleaseError, 'snapshot image differs'):
                m.adopt_baseline('v2')
        healthy.assert_not_called()
        perform.assert_not_called()
        self.assertEqual(m.current_version(), 'v1')
    def test_cannot_upgrade_to_rollback_snapshot(self):
        self.record['kind'] = 'rollback_snapshot'
        with self.activation_mocks(), mock.patch.object(m, 'perform') as perform:
            with self.assertRaisesRegex(m.ReleaseError, 'cannot be activated'):
                m.activate('v2')
        perform.assert_not_called()
    def test_explicit_rollback_checks_current_compatibility_direction(self):
        self.record['kind'] = 'rollback_snapshot'
        self.record['validation']['rollback_compatible_from'] = []
        self.old['validation']['rollback_compatible_from'] = ['v2']
        with self.activation_mocks(), mock.patch.object(m, 'perform') as perform:
            m.activate('v2', rollback=True)
        perform.assert_called_once_with(self.record)
        self.assertEqual(m.current_version(), 'v2')
    def test_same_version_activation_is_idempotent_without_self_compatibility(self):
        with self.activation_mocks(), mock.patch.object(m, 'healthy') as healthy, mock.patch.object(m, 'perform') as perform:
            m.activate('v1')
        healthy.assert_called_once_with(self.old)
        perform.assert_not_called()
    def test_atomic_pointer_update_uses_unique_temporary(self):
        m.point_current(self.directory)
        self.assertEqual(m.current_version(), 'v2')
        self.assertEqual(list(m.CURRENT.parent.glob('current.*.next')), [])

if __name__ == '__main__':
    unittest.main()
