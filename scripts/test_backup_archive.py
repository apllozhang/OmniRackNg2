import importlib.util
import io
import json
import tarfile
import tempfile
import unittest
from pathlib import Path

spec = importlib.util.spec_from_file_location('backup_archive', Path(__file__).with_name('verify-backup-archive.py'))
backup = importlib.util.module_from_spec(spec)
spec.loader.exec_module(backup)


class BackupArchiveTests(unittest.TestCase):
    def source(self, root):
        source = root / 'payload'
        (source / 'config').mkdir(parents=True)
        (source / 'database.dump').write_bytes(b'postgres dump sample')
        (source / 'config/.env').write_text('EXAMPLE=not-a-real-secret', encoding='utf-8')
        backup.create_manifest(source)
        return source

    def test_round_trip_and_manifest(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            source = self.source(root)
            with tarfile.open(root / 'backup.tar.gz', 'w:gz') as tar:
                tar.add(source, arcname='.')
            result = backup.unpack(root / 'backup.tar.gz', root / 'restored')
            self.assertEqual(len(result['files']), 2)
            self.assertEqual((root / 'restored/database.dump').read_bytes(), b'postgres dump sample')

    def test_modified_payload_rejected(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            source = self.source(root)
            (source / 'database.dump').write_bytes(b'changed malicious dump')
            with tarfile.open(root / 'backup.tar.gz', 'w:gz') as tar:
                tar.add(source, arcname='.')
            with self.assertRaises(ValueError):
                backup.unpack(root / 'backup.tar.gz', root / 'restored')

    def test_traversal_and_symlinks_rejected_before_extraction(self):
        for name, kind in [('../outside', tarfile.REGTYPE), ('/absolute', tarfile.REGTYPE), ('link', tarfile.SYMTYPE)]:
            with self.subTest(name=name), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                with tarfile.open(root / 'bad.tar.gz', 'w:gz') as tar:
                    info = tarfile.TarInfo(name)
                    info.type = kind
                    info.linkname = '/etc/passwd' if kind == tarfile.SYMTYPE else ''
                    info.size = 0
                    tar.addfile(info, io.BytesIO())
                with self.assertRaises(ValueError):
                    backup.unpack(root / 'bad.tar.gz', root / 'restored')
                self.assertFalse((root / 'outside').exists())

    def test_nonempty_restore_destination_refused(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            output = root / 'existing'
            output.mkdir()
            (output / 'important.txt').write_text('keep')
            with self.assertRaises(ValueError):
                backup.unpack(root / 'missing.tar.gz', output)
            self.assertEqual((output / 'important.txt').read_text(), 'keep')


if __name__ == '__main__':
    unittest.main()
