#!/usr/bin/env python3
"""Portable staging safety tests; no Docker/network/NAS writes."""
import hashlib, importlib.util, io, json, os, pathlib, tarfile, tempfile, unittest
from unittest import mock
script=pathlib.Path(__file__).resolve().with_name('nas-stage-release.py')
spec=importlib.util.spec_from_file_location('stage',script);m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
class StageSafety(unittest.TestCase):
 def setUp(self): self.t=tempfile.TemporaryDirectory();self.root=pathlib.Path(self.t.name)
 def tearDown(self):self.t.cleanup()
 def archive(self, entries):
  p=self.root/'a.tar.gz'
  with tarfile.open(p,'w:gz') as a:
   for name,data,kind in entries:
    i=tarfile.TarInfo(name);i.size=len(data);i.mode=0o755;i.type=kind
    if kind==tarfile.SYMTYPE:i.linkname='/tmp/target'
    a.addfile(i,io.BytesIO(data) if kind==tarfile.REGTYPE else None)
  return p
 def test_exact_regular_archive(self):
  p=self.archive([('pkg/a.py',b'x',tarfile.REGTYPE)])
  m.extract_verified(p,self.root/'out',{'a.py':hashlib.sha256(b'x').hexdigest()},'pkg/')
  self.assertEqual((self.root/'out/a.py').read_bytes(),b'x')
 def test_escaping_duplicates_links_omissions_and_bad_hash_fail(self):
  cases=[ [('pkg//tmp/x',b'x',tarfile.REGTYPE)], [('/tmp/x',b'x',tarfile.REGTYPE)], [('pkg/../x',b'x',tarfile.REGTYPE)],
  [('pkg/a.py',b'x',tarfile.SYMTYPE)], [('pkg/a.py',b'x',tarfile.REGTYPE),('pkg/a.py',b'x',tarfile.REGTYPE)], []]
  for n,entries in enumerate(cases):
   with self.subTest(n=n),self.assertRaises(m.release.ReleaseError):
    m.extract_verified(self.archive(entries),self.root/str(n),{'a.py':hashlib.sha256(b'x').hexdigest()},'pkg/')
  with self.assertRaises(m.release.ReleaseError):m.extract_verified(self.archive([('pkg/a.py',b'y',tarfile.REGTYPE)]),self.root/'bad',{'a.py':hashlib.sha256(b'x').hexdigest()},'pkg/')
 def test_env_snapshot_private_stable_and_never_overwritten(self):
  shared=self.root/'shared';shared.mkdir(mode=0o700); source=shared/'.env';source.write_text('SECRET=x');source.chmod(0o600)
  with mock.patch.object(m,'SHARED',shared):
   target,h=m.snapshot_env('v2',{'version':'v1','env_file':str(source)})
   self.assertEqual(target.stat().st_mode&0o777,0o600)
   self.assertEqual(m.snapshot_env('v2',{'version':'v1','env_file':str(source)})[1],h)
   source.write_text('SECRET=y')
   with self.assertRaises(m.release.ReleaseError):m.snapshot_env('v2',{'version':'v1','env_file':str(source)})
   self.assertEqual(target.read_text(),'SECRET=x')
 def test_verification_inventory_full_copy_and_mutation_rejected(self):
  d=self.root/'verification';d.mkdir();(d/'a').write_bytes(b'x')
  (d/'SHA256SUMS.txt').write_text(hashlib.sha256(b'x').hexdigest()+'  a\n')
  m.copy_verification(d,self.root/'copied')
  (d/'a').write_bytes(b'y')
  with self.assertRaises(m.release.ReleaseError):m.copy_verification(d,self.root/'rejected')
 def test_compatibility_evidence_must_explicitly_cover_current(self):
  p=self.root/'e.json';e={'passed':True,'source_sha256':'a','git_commit':'b','checks':{k:'passed' for k in ['go','frontend','api_contract','integration','restore','schema_compatibility']},'schema_backward_compatible':True,'schema_compatible_from':['v1']};p.write_text(json.dumps(e))
  m.evidence_input(str(p),'a','b',False,'v1')
  with self.assertRaises(m.release.ReleaseError):m.evidence_input(str(p),'a','b',False,'other')
 def test_env_rejects_symlink_parent_and_source(self):
  shared=self.root/'shared';shared.mkdir(mode=0o700);real=self.root/'real';real.mkdir(mode=0o700)
  (real/'v1.env').write_text('SECRET=x');(real/'v1.env').chmod(0o600)
  (shared/'env').symlink_to(real,target_is_directory=True)
  with mock.patch.object(m,'SHARED',shared):
   with self.assertRaises(m.release.ReleaseError):m.snapshot_env('v2',{'version':'v1','env_file':str(shared/'env/v1.env')})
  (shared/'.env').symlink_to(real/'v1.env')
  with mock.patch.object(m,'SHARED',shared):
   with self.assertRaises(m.release.ReleaseError):m.snapshot_env('v2',{'version':'v1','env_file':str(shared/'.env')})
if __name__ == '__main__':
 unittest.main()
