#!/usr/bin/env python3
"""Stage immutable NAS replica releases, without changing current or containers.

Full: --version V --verification /abs/verification --evidence /abs/passed.json
Baseline: --version V --baseline --evidence /abs/runtime-and-restore.json
Run on NAS 103 from a clean committed repository. Secrets are copied privately.
Failed staging directories are retained for inspection; final releases are never overwritten.
"""
import argparse
import datetime as dt
import fcntl
import hashlib
import importlib.util
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('nas_release', HERE / 'nas-release.py')
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)
sys.path.insert(0, str(HERE))
from replica_manifest import inventory

ROOT = release.ROOT
SHARED = release.SHARED
REPO = HERE.parent
MAX_ARCHIVE_BYTES = 2 * 1024 * 1024 * 1024


def write_json(path, value):
    path.write_text(json.dumps(value, indent=2, ensure_ascii=False) + '\n')


def absolute_file(value):
    path = Path(value)
    release.require(path.is_absolute(), 'absolute input path required')
    return release.regular(path)


def git(*args):
    return release.run(['git', '-C', str(REPO), *args])


def safe_relative(name):
    release.require(isinstance(name, str) and name and '\\' not in name and not name.startswith('/')
                    and all(part not in ('', '.', '..') for part in name.split('/')), 'unsafe archive/file path')
    return PurePosixPath(name)


def extract_verified(archive_path, destination, expected, prefix=''):
    """Extract only bounded unique regular files and require an exact hash inventory."""
    seen = set()
    total = 0
    with tarfile.open(archive_path, 'r:*') as archive:
        for entry in archive:
            release.require(entry.isfile() and not entry.issym() and not entry.islnk(), 'archive may contain only regular files')
            safe_relative(entry.name)
            if prefix:
                release.require(entry.name.startswith(prefix), 'archive prefix mismatch')
                name = entry.name[len(prefix):]
            else:
                name = entry.name
            safe_relative(name)
            release.require(name not in seen and len(seen) < 20000, 'duplicate or excessive archive entries')
            seen.add(name)
            total += entry.size
            release.require(0 <= entry.size <= MAX_ARCHIVE_BYTES and total <= MAX_ARCHIVE_BYTES, 'archive exceeds extraction budget')
            release.require(name in expected, 'unrecorded archive member: ' + name)
            path = destination.joinpath(*PurePosixPath(name).parts)
            path.parent.mkdir(parents=True, exist_ok=True)
            h = hashlib.sha256()
            source = archive.extractfile(entry)
            release.require(source is not None, 'archive member has no data')
            with source, path.open('xb') as output:
                for chunk in iter(lambda: source.read(1024 * 1024), b''):
                    h.update(chunk)
                    output.write(chunk)
            release.require(h.hexdigest() == expected[name], 'archive content hash mismatch: ' + name)
            path.chmod(0o755 if entry.mode & 0o111 else 0o644)
    release.require(seen == set(expected), 'archive omits recorded files')


def archive_inventory(path, prefix=''):
    """Read hashes, preserving strict validation; used for generated package manifest only."""
    result = {}
    total = 0
    with tarfile.open(path, 'r:*') as archive:
        for entry in archive:
            release.require(entry.isfile(), 'archive contains non-file entry')
            safe_relative(entry.name)
            release.require(entry.name.startswith(prefix), 'archive prefix mismatch')
            name = entry.name[len(prefix):]
            safe_relative(name)
            release.require(name not in result and len(result) < 20000, 'duplicate archive member')
            total += entry.size
            release.require(0 <= entry.size <= MAX_ARCHIVE_BYTES and total <= MAX_ARCHIVE_BYTES, 'archive exceeds extraction budget')
            h = hashlib.sha256()
            stream = archive.extractfile(entry)
            release.require(stream is not None, 'archive member has no data')
            with stream:
                for chunk in iter(lambda: stream.read(1024 * 1024), b''):
                    h.update(chunk)
            result[name] = h.hexdigest()
    return result


def copy_verification(source, destination):
    release.require(source.is_absolute() and source.is_dir() and not source.is_symlink(), 'verification directory must be absolute and regular')
    sums = {}
    for line in release.regular(source / 'SHA256SUMS.txt').read_text().splitlines():
        release.require(len(line) > 66 and line[64:66] == '  ' and re.fullmatch(r'[0-9a-f]{64}', line[:64]), 'invalid verification checksum line')
        name = line[66:]
        safe_relative(name)
        release.require(name not in sums, 'duplicate verification checksum')
        sums[name] = line[:64]
    actual = set()
    destination.mkdir()
    for base, dirs, names in os.walk(source, followlinks=False):
        for directory in dirs:
            release.require(not (Path(base) / directory).is_symlink(), 'verification directory contains symlink')
        for name in names:
            original = release.regular(Path(base) / name)
            relative = original.relative_to(source).as_posix()
            safe_relative(relative)
            actual.add(relative)
            expected = sums.get(relative)
            if relative != 'SHA256SUMS.txt':
                release.require(expected is not None and release.digest(original) == expected, 'verification checksum mismatch: ' + relative)
            target = destination / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(original, target)
            target.chmod(0o755 if original.stat().st_mode & 0o111 else 0o644)
            if expected:
                release.require(release.digest(target) == expected, 'verification changed while copying')
    release.require(actual == set(sums) | {'SHA256SUMS.txt'}, 'verification checksum inventory is incomplete')


def evidence_input(path, source_hash, commit, baseline, previous):
    evidence = release.json_file(absolute_file(path))
    required = {'runtime', 'restore'} if baseline else {'go', 'frontend', 'api_contract', 'integration', 'restore', 'schema_compatibility'}
    release.require(evidence.get('passed') is True and evidence.get('source_sha256') == source_hash
                    and evidence.get('git_commit') == commit, 'evidence identity/passed flag mismatch')
    checks = evidence.get('checks', {})
    release.require(all(checks.get(name) == 'passed' for name in required), 'required verification evidence is absent or failed')
    if not baseline:
        release.require(evidence.get('schema_backward_compatible') is True and
                        previous in evidence.get('schema_compatible_from', []), 'schema compatibility evidence does not cover current release')
    return evidence


def snapshot_env(version, historical):
    # Never overwrite an existing snapshot. A failed stage can safely reuse identical bytes.
    source = Path(historical['env_file'])
    expected_snapshot = SHARED / 'env' / (release.version_id(historical['version']) + '.env')
    release.require(source in (SHARED / '.env', expected_snapshot) and not SHARED.is_symlink()
                    and not source.parent.is_symlink(), 'historical environment path is outside the private shared directory or symlinked')
    release.regular(source)
    release.require(source.stat().st_mode & 0o077 == 0, 'historical environment file must be private')
    data = source.read_bytes()
    directory = SHARED / 'env'
    directory.mkdir(mode=0o700, exist_ok=True)
    release.require(not directory.is_symlink() and directory.stat().st_mode & 0o077 == 0, 'private environment directory required')
    target = directory / (version + '.env')
    if target.exists() or target.is_symlink():
        release.regular(target)
        release.require(target.stat().st_mode & 0o077 == 0 and target.read_bytes() == data, 'environment snapshot already exists with different content or permissions')
    else:
        fd, temporary_name = tempfile.mkstemp(prefix='.' + version + '.', dir=directory)
        temporary = Path(temporary_name)
        try:
            with os.fdopen(fd, 'wb') as stream:
                stream.write(data)
                stream.flush()
                os.fsync(stream.fileno())
            # Hard-link publication gives no-overwrite atomic semantics.
            os.link(temporary, target)
        finally:
            temporary.unlink(missing_ok=True)
    fd = os.open(directory, os.O_DIRECTORY)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)
    return target, hashlib.sha256(data).hexdigest()


def image_spec(component, version, image_id):
    return {'tag': 'omnirack-replica-' + component + ':' + version, 'id': image_id}


def inspect(reference):
    return json.loads(release.run(['docker', 'image', 'inspect', reference]))[0]


def ensure_unused_tag(tag):
    result = subprocess.run(['docker', 'image', 'inspect', tag], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    release.require(result.returncode != 0, 'target image tag already exists; do not overwrite it: ' + tag)


def tag_snapshot(component, version, image_id):
    record = image_spec(component, version, image_id)
    ensure_unused_tag(record['tag'])
    release.run(['docker', 'tag', image_id, record['tag']])
    release.require(release.image_id(record['tag']) == image_id, 'snapshot tag identity mismatch')
    return record


def build_image(component, version, base_id, stage, source_hash, commit):
    context = stage / 'runtime' / component
    context.mkdir(parents=True)
    # Content-addressed local base alias, checked around the build; never a mutable baseline tag.
    base_tag = 'omnirack-replica-base-' + component + ':sha256-' + base_id.removeprefix('sha256:')
    existing = subprocess.run(['docker', 'image', 'inspect', base_tag], capture_output=True, text=True)
    if existing.returncode == 0:
        release.require(json.loads(existing.stdout)[0]['Id'] == base_id, 'content-addressed base tag was overwritten')
    else:
        release.run(['docker', 'tag', base_id, base_tag])
    release.require(release.image_id(base_tag) == base_id, 'base image identity mismatch')
    base = inspect(base_id)
    labels = ('LABEL org.opencontainers.image.version=' + json.dumps(version) +
              ' org.opencontainers.image.revision=' + json.dumps(commit) +
              ' com.omnirack.source-sha256=' + json.dumps(source_hash) + '\n')
    if component == 'backend':
        shutil.copyfile(stage / 'verification/omnirack-server', context / 'omnirack-server')
        dockerfile = ('FROM ' + base_tag + '\n' + labels +
                      'WORKDIR /app\nCOPY --chmod=0755 omnirack-server /app/omnirack-server\n'
                      'ENTRYPOINT ["/app/omnirack-server"]\n')
    else:
        shutil.copytree(stage / 'frontend', context / 'html')
        shutil.copyfile(stage / 'source/frontend/nginx/default.conf', context / 'default.conf')
        (context / 'default.conf').chmod(0o644)
        for item in [context / 'html', *(context / 'html').rglob('*')]:
            item.chmod(0o755 if item.is_dir() else 0o644)
        dockerfile = ('FROM ' + base_tag + '\n' + labels +
                      'USER root\nRUN rm -rf /usr/share/nginx/html && mkdir -p /usr/share/nginx/html\n'
                      'COPY html /usr/share/nginx/html\nCOPY default.conf /etc/nginx/conf.d/default.conf\n')
    (context / 'Dockerfile').write_text(dockerfile)
    tag = image_spec(component, version, '')['tag']
    ensure_unused_tag(tag)
    build = subprocess.run(['docker', 'build', '--pull=false', '-t', tag, str(context)], capture_output=True, text=True)
    (context / 'build.log').write_text(build.stdout + build.stderr)
    (context / 'build.log').chmod(0o600)
    release.require(build.returncode == 0, 'Docker build failed; inspect private staging runtime/' + component + '/build.log')
    release.require(release.image_id(base_tag) == base_id, 'base tag changed during build')
    built = inspect(tag)
    layers = base['RootFS']['Layers']
    release.require(built['RootFS']['Layers'][:len(layers)] == layers, 'built image does not descend from recorded base layers')
    release.require(built['Config'].get('Labels', {}).get('com.omnirack.source-sha256') == source_hash, 'built image source label mismatch')
    record = image_spec(component, version, built['Id'])
    record['base_id'] = base_id
    record['base_tag'] = base_tag
    return record


def compose_document(record, postgres_image):
    return {'services': {
        'postgres': {'image': postgres_image, 'restart': 'unless-stopped',
                     'environment': {key: '${' + key + '}' for key in ('POSTGRES_DB', 'POSTGRES_USER', 'POSTGRES_PASSWORD')},
                     'volumes': ['asset-postgres-data:/var/lib/postgresql/data']},
        'backend': {'image': record['images']['backend']['tag'], 'restart': 'unless-stopped',
                    'env_file': [record['env_file']], 'environment': {
                        'DATABASE_URL': 'host=postgres user=${POSTGRES_USER} password=${POSTGRES_PASSWORD} dbname=${POSTGRES_DB} port=5432 sslmode=disable TimeZone=UTC',
                        'HTTP_ADDR': ':8080'}},
        'frontend': {'image': record['images']['frontend']['tag'], 'restart': 'unless-stopped',
                     'ports': ['0.0.0.0:15174:80']}},
        'volumes': {'asset-postgres-data': {'name': release.PROJECT + '_asset-postgres-data', 'external': True}},
        'networks': {'default': {'name': release.PROJECT + '_default'}}}


def stage_release(args):
    os.umask(0o077)
    version = release.version_id(args.version)
    release.require(ROOT.is_dir() and not ROOT.is_symlink() and SHARED.is_dir() and not SHARED.is_symlink(), 'NAS release/shared directories required')
    release.require(not (ROOT / version).exists() and not (ROOT / version).is_symlink(), 'release ID already exists')
    with (SHARED / 'activation.lock').open('a') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        previous = release.current_version()
        _, historical = release.load(previous)
        source = inventory(REPO)
        if args.baseline:
            commit = historical.get('git_commit')
            source_hash = historical.get('source_sha256')
        else:
            release.require(args.verification is not None, '--verification required for full releases')
            release.require(not git('status', '--porcelain'), 'repository must be clean and committed before staging')
            commit = git('rev-parse', 'HEAD')
            source_hash = source['source_sha256']
        evidence = evidence_input(args.evidence, source_hash, commit, args.baseline, previous)
        for component in ('backend', 'frontend'):
            actual = release.run(['docker', 'inspect', release.PROJECT + '-' + component + '-1', '--format', '{{.Image}}'])
            release.require(actual == historical['images'][component]['id'], 'running app differs from current release; resolve before staging')
        candidate = Path(tempfile.mkdtemp(prefix='.' + version + '.stage-', dir=ROOT))
        try:
            write_json(candidate / 'validation-evidence.json', evidence)
            env, env_hash = snapshot_env(version, historical)
            record = {'schema_version': 2, 'kind': 'rollback_snapshot' if args.baseline else 'release',
                      'version': version, 'source_sha256': source_hash, 'git_commit': commit,
                      'env_file': str(env), 'env_sha256': env_hash, 'previous_version': previous,
                      'created_at_utc': dt.datetime.now(dt.timezone.utc).isoformat(),
                      'compose_files': [str(ROOT / version / 'compose.release.json')],
                      'validation': {'passed': True, 'evidence_file': 'validation-evidence.json',
                                     'schema_backward_compatible': not args.baseline,
                                     'rollback_compatible_from': [] if args.baseline else [previous]}}
            if args.baseline:
                record['baseline_of'] = previous
                write_json(candidate / 'historical-release.json', historical)
                record['images'] = {component: tag_snapshot(component, version, historical['images'][component]['id'])
                                    for component in ('backend', 'frontend')}
                old_bundle = ROOT / previous / 'source-history.bundle'
                if old_bundle.is_file() and not old_bundle.is_symlink():
                    shutil.copyfile(old_bundle, candidate / 'source-history.bundle')
            else:
                copy_verification(Path(args.verification), candidate / 'verification')
                build = release.json_file(candidate / 'verification/build-manifest.json')
                release.require(build.get('sourceSHA256') == source_hash, 'verified build source differs from Git worktree source')
                release.require(release.digest(candidate / 'verification/omnirack-server') == build.get('artifacts', {}).get('omnirack-server'), 'backend artifact hash mismatch')
                archive = candidate / 'verification/omnirack-replica-src.tar.gz'
                package_map = archive_inventory(archive, 'omnirack-replica/')
                expected_source = dict(source['files'])
                release.require(set(package_map) == set(expected_source) | {'PACKAGE-MANIFEST.txt'}, 'source package file inventory differs')
                release.require(all(package_map[name] == digest for name, digest in expected_source.items()), 'source package content differs')
                expected_source['PACKAGE-MANIFEST.txt'] = package_map['PACKAGE-MANIFEST.txt']
                extract_verified(archive, candidate / 'source', expected_source, 'omnirack-replica/')
                frontend_files = {name.removeprefix('frontend/'): digest for name, digest in build['artifacts'].items() if name.startswith('frontend/')}
                release.require(frontend_files and 'build-info.json' in frontend_files, 'frontend artifact inventory missing')
                extract_verified(candidate / 'verification/frontend-dist.tar.gz', candidate / 'frontend', frontend_files)
                info = release.json_file(candidate / 'frontend/build-info.json')
                release.require(info.get('sourceSHA256') == source_hash, 'frontend build identity mismatch')
                backend = candidate / 'verification/omnirack-server'
                backend.chmod(0o755)
                binary_info = json.loads(release.run([str(backend), '--build-info']))
                release.require(binary_info.get('sourceSHA256') == source_hash, 'backend build identity mismatch')
                git('bundle', 'create', str(candidate / 'source-history.bundle'), '--all')
                record['images'] = {component: build_image(component, version, historical['images'][component]['id'], candidate, source_hash, commit)
                                    for component in ('backend', 'frontend')}
            postgres_id = release.run(['docker', 'inspect', release.PROJECT + '-postgres-1', '--format', '{{.Image}}'])
            # Pin database descriptor too, though activation never starts/recreates it.
            write_json(candidate / 'compose.release.json', compose_document(record, postgres_id))
            record['file_sha256'] = {path.relative_to(candidate).as_posix(): release.digest(path)
                                     for path in sorted(candidate.rglob('*')) if path.is_file()}
            write_json(candidate / 'RELEASE.json', record)
            # Metadata expects final absolute compose paths. Check content now, topology after rename.
            release.validate_metadata(candidate, {**record, 'compose_files': [str(candidate / 'compose.release.json')]})
            # Standalone config resolution can be checked before publication using candidate paths.
            temp_record = {**record, 'compose_files': [str(candidate / 'compose.release.json')]}
            config = json.loads(release.run([*release.compose_command(temp_record), 'config', '--format', 'json']))
            release.validate_topology(config, record)
            for item in candidate.rglob('*'):
                if item.is_file():
                    with item.open('rb') as stream:
                        os.fsync(stream.fileno())
            for item in sorted([candidate, *(p for p in candidate.rglob('*') if p.is_dir())], key=lambda p: len(p.parts), reverse=True):
                fd = os.open(item, os.O_DIRECTORY)
                try:
                    os.fsync(fd)
                finally:
                    os.close(fd)
            release.require(not (ROOT / version).exists(), 'release target appeared during staging')
            os.rename(candidate, ROOT / version)
            fd = os.open(ROOT, os.O_DIRECTORY)
            try:
                os.fsync(fd)
            finally:
                os.close(fd)
            print('STAGED', version, '(current and containers unchanged)')
        except Exception as exc:
            if candidate.exists():
                write_json(candidate / 'STAGING-FAILED.json', {'error_type': type(exc).__name__, 'time_utc': dt.datetime.now(dt.timezone.utc).isoformat(),
                                                             'note': 'No deployment/current change. Do not overwrite existing image tags; inspect before retry.'})
                print('Retained failed private staging directory:', candidate, file=sys.stderr)
            else:
                print('Release directory was published but durability confirmation failed; inspect before retry:', ROOT / version, file=sys.stderr)
            raise


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--version', required=True)
    parser.add_argument('--verification')
    parser.add_argument('--evidence', required=True)
    parser.add_argument('--baseline', action='store_true')
    args = parser.parse_args()
    try:
        stage_release(args)
    except (release.ReleaseError, OSError, ValueError, KeyError, TypeError) as exc:
        print('ERROR:', str(exc), file=sys.stderr)
        return 1
    return 0

if __name__ == '__main__':
    sys.exit(main())
