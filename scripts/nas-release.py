#!/usr/bin/env python3
"""Fail-closed activation of validated OmniRack replica releases on NAS 103.

Historical schema-1 records are readable with status, never activatable here.
No prepare/build/database restore command is provided. Secrets stay out of output.
"""
import argparse
import datetime as dt
import fcntl
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import subprocess
import sys
import tempfile
import time
import urllib.request
import uuid

ROOT = Path('/home/tina/OmniRackReplica-releases')
SHARED = Path('/home/tina/OmniRackReplica-shared')
CURRENT = Path('/home/tina/OmniRackReplica-current')
PROJECT = 'omnirack-replica'
HTTP = urllib.request.build_opener(urllib.request.ProxyHandler({}))

class ReleaseError(RuntimeError):
    pass

def require(condition, message):
    if not condition:
        raise ReleaseError(message)

def version_id(value):
    require(isinstance(value, str) and re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._-]{0,95}', value)
            and '..' not in value, 'invalid release ID')
    return value

def regular(path):
    require(path.is_file() and not path.is_symlink(), 'expected regular file: ' + str(path))
    return path

def release_dir(version):
    version_id(version)
    path = ROOT / version
    require(ROOT.is_dir() and not ROOT.is_symlink(), 'release root is unavailable or symlinked')
    require(path.is_dir() and not path.is_symlink() and path.resolve().parent == ROOT.resolve(), 'release escapes root')
    return path

def member(directory, relative):
    require(isinstance(relative, str) and relative and '\\' not in relative and '//' not in relative, 'invalid release file path')
    parts = PurePosixPath(relative).parts
    require(not relative.startswith('/') and all(p not in ('', '.', '..') for p in relative.split('/')), 'invalid release file path')
    path = directory.joinpath(*parts)
    require(path.resolve().is_relative_to(directory.resolve()), 'release file escapes directory')
    cursor = path
    while cursor != directory:
        require(not cursor.is_symlink(), 'symlink in release file path')
        cursor = cursor.parent
    return regular(path)

def digest(path):
    h = hashlib.sha256()
    with path.open('rb') as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b''):
            h.update(chunk)
    return h.hexdigest()

def json_file(path):
    try:
        return json.loads(regular(path).read_text())
    except (ValueError, OSError) as exc:
        raise ReleaseError('invalid JSON file: ' + str(path)) from exc

def run(args):
    # Never echo compose's expanded configuration or errors: they may contain secrets.
    result = subprocess.run(args, capture_output=True, text=True)
    require(result.returncode == 0, 'external command failed: ' + args[0] + ' (details suppressed to protect secrets)')
    return result.stdout.strip()

def image_id(reference):
    return run(['docker', 'image', 'inspect', reference, '--format', '{{.Id}}'])

def compose_command(record):
    command = ['docker', 'compose', '--env-file', record['env_file'], '-p', PROJECT]
    for path in record['compose_files']:
        command += ['-f', path]
    return command

def load(version):
    directory = release_dir(version)
    record = json_file(directory / 'RELEASE.json')
    require(isinstance(record, dict) and record.get('version') == version, 'release identity mismatch')
    return directory, record

def validate_metadata(directory, record):
    version = version_id(record.get('version'))
    require(record.get('schema_version') == 2, 'schema-2 immutable release metadata required')
    require(re.fullmatch(r'[0-9a-f]{40,64}', record.get('git_commit', '')) is not None, 'invalid Git revision')
    require(re.fullmatch(r'[0-9a-f]{64}', record.get('source_sha256', '')) is not None, 'invalid source digest')
    env = Path(record.get('env_file', ''))
    expected_env = SHARED / 'env' / (version + '.env')
    require(env == expected_env and not SHARED.is_symlink() and not env.parent.is_symlink(), 'versioned environment snapshot required')
    regular(env)
    require(env.stat().st_mode & 0o077 == 0, 'environment snapshot must be private (0600)')
    require(digest(env) == record.get('env_sha256'), 'environment snapshot digest mismatch')
    files = record.get('file_sha256')
    require(isinstance(files, dict) and files, 'release file digest map required')
    for name, expected in files.items():
        require(isinstance(expected, str) and re.fullmatch(r'[0-9a-f]{64}', expected), 'invalid file digest')
        require(digest(member(directory, name)) == expected, 'file digest mismatch: ' + name)
    config = record.get('compose_files')
    require(isinstance(config, list) and config and len(config) <= 4, 'compose files required')
    for name in config:
        require(isinstance(name, str), 'invalid compose path')
        path = Path(name)
        try:
            relative = str(path.relative_to(directory))
        except ValueError as exc:
            raise ReleaseError('compose file outside release') from exc
        require(relative in files, 'compose file is not integrity protected')
        member(directory, relative)
    images = record.get('images', {})
    require(set(images) == {'backend', 'frontend'}, 'exactly two application image records required')
    for component, spec in images.items():
        require(spec.get('tag') == 'omnirack-replica-' + component + ':' + version, 'invalid image tag')
        require(re.fullmatch(r'sha256:[0-9a-f]{64}', spec.get('id', '')) is not None, 'invalid image ID')
    validation = record.get('validation', {})
    evidence_name = validation.get('evidence_file')
    require(evidence_name in files, 'validation evidence must be integrity protected')
    evidence = json_file(member(directory, evidence_name))
    require(validation.get('passed') is True and evidence.get('passed') is True, 'release checks have not passed')
    require(evidence.get('source_sha256') == record['source_sha256'] and evidence.get('git_commit') == record['git_commit'], 'evidence does not describe this source revision')
    checks = evidence.get('checks', {})
    kind = record.get('kind', 'release')
    require(kind in ('release', 'rollback_snapshot'), 'invalid release kind')
    required = ({'runtime', 'restore'} if kind == 'rollback_snapshot' else
                {'go', 'frontend', 'api_contract', 'integration', 'restore', 'schema_compatibility'})
    require(isinstance(checks, dict) and required.issubset(checks) and all(checks[k] == 'passed' for k in required), 'required release checks missing or failed')
    if kind == 'rollback_snapshot':
        version_id(record.get('baseline_of'))
    else:
        require(validation.get('schema_backward_compatible') is True, 'backward-compatible schema evidence required')
    require(isinstance(validation.get('rollback_compatible_from'), list), 'rollback compatibility list required')
    for previous in validation['rollback_compatible_from']:
        version_id(previous)
    return record

def validate_topology(config, record):
    require(config.get('name') == PROJECT, 'wrong compose project')
    services = config.get('services', {})
    require(set(services) == {'postgres', 'backend', 'frontend'}, 'unexpected compose services')
    networks = config.get('networks', {})
    require(set(networks) == {'default'} and networks['default'].get('name') == PROJECT + '_default'
            and not networks['default'].get('external') and networks['default'].get('driver', 'bridge') == 'bridge', 'unexpected network topology')
    volumes = config.get('volumes', {})
    require(set(volumes) == {'asset-postgres-data'} and volumes['asset-postgres-data'].get('name') == PROJECT + '_asset-postgres-data'
            and not volumes['asset-postgres-data'].get('driver_opts'), 'unexpected persistent volume')
    for name, service in services.items():
        require(not any(service.get(key) for key in ['container_name', 'privileged', 'network_mode', 'pid', 'ipc', 'devices', 'volumes_from', 'cap_add', 'external_links', 'links', 'extra_hosts']), 'unsafe service option: ' + name)
        require(set(service.get('networks', {'default': {}})) == {'default'}, 'service network escapes project')
        ports = service.get('ports', [])
        if name == 'frontend':
            require(len(ports) == 1 and str(ports[0].get('published')) == '15174'
                    and ports[0].get('target') == 80 and ports[0].get('protocol', 'tcp') == 'tcp'
                    and ports[0].get('host_ip', '0.0.0.0') in ('0.0.0.0', '127.0.0.1'), 'only frontend port 15174 may be published')
        else:
            require(not ports, 'backend/database ports may not be published')
        if name == 'frontend':
            require(not service.get('environment'), 'frontend must not receive runtime secrets')
        if name == 'postgres':
            require(set(service.get('environment', {})).issubset({'POSTGRES_DB', 'POSTGRES_USER', 'POSTGRES_PASSWORD'}), 'unexpected database environment')
        if name in record['images']:
            require(service.get('image') in (record['images'][name]['tag'], record['images'][name]['id']), 'compose application image differs from release')
            require(not service.get('volumes'), 'application mounts are prohibited')
            require(not service.get('command') and not service.get('entrypoint'), 'application startup override is prohibited')
        else:
            mounts = service.get('volumes', [])
            require(len(mounts) == 1 and mounts[0].get('type') == 'volume'
                    and mounts[0].get('source') == 'asset-postgres-data'
                    and mounts[0].get('target') == '/var/lib/postgresql/data', 'unexpected database mount')
    return config

def validate(version):
    directory, record = load(version)
    validate_metadata(directory, record)
    unresolved = json.loads(run([*compose_command(record), 'config', '--no-env-resolution', '--format', 'json']))
    for name, service in unresolved.get('services', {}).items():
        require(name == 'backend' or not service.get('env_file'), 'only backend may consume the private environment snapshot')
        for entry in service.get('env_file', []):
            path = entry.get('path') if isinstance(entry, dict) else entry
            require(path == record['env_file'], 'service env_file must use the recorded environment snapshot')
    config = json.loads(run([*compose_command(record), 'config', '--format', 'json']))
    validate_topology(config, record)
    for spec in record['images'].values():
        require(image_id(spec['tag']) == spec['id'], 'recorded image tag changed')
    # The database is deliberately never recreated, including during rollback.
    pg = json.loads(run(['docker', 'inspect', PROJECT + '-postgres-1']))[0]
    labels = pg['Config'].get('Labels', {})
    require(labels.get('com.docker.compose.project') == PROJECT and labels.get('com.docker.compose.service') == 'postgres', 'database belongs to another compose project')
    require(pg['Image'] == image_id(config['services']['postgres']['image']), 'configured database image differs from running database')
    require(any(m.get('Type') == 'volume' and m.get('Name') == PROJECT + '_asset-postgres-data' and m.get('Destination') == '/var/lib/postgresql/data' for m in pg.get('Mounts', [])), 'running database uses an unexpected volume')
    require(PROJECT + '_default' in pg.get('NetworkSettings', {}).get('Networks', {}), 'database is outside expected network')
    run(['docker', 'volume', 'inspect', PROJECT + '_asset-postgres-data'])
    return directory, record

def journal(event):
    event = {'time_utc': dt.datetime.now(dt.timezone.utc).isoformat(), **event}
    path = SHARED / 'release-manager-history.jsonl'
    with path.open('a', encoding='utf-8') as stream:
        stream.write(json.dumps(event, ensure_ascii=False) + '\n')
        stream.flush()
        os.fsync(stream.fileno())

def point_current(directory):
    temporary = CURRENT.with_name(CURRENT.name + '.' + uuid.uuid4().hex + '.next')
    try:
        temporary.symlink_to(directory)
        os.replace(temporary, CURRENT)
        fd = os.open(CURRENT.parent, os.O_DIRECTORY)
        try:
            os.fsync(fd)
        finally:
            os.close(fd)
    finally:
        temporary.unlink(missing_ok=True)

def current_version():
    require(CURRENT.is_symlink(), 'current release pointer must be a symlink')
    target = CURRENT.resolve(strict=True)
    require(target.parent == ROOT.resolve(), 'current pointer escapes release root')
    version_id(target.name)
    return target.name

def healthy(record, timeout=60):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        try:
            with HTTP.open('http://127.0.0.1:15174/health/ready', timeout=2) as response:
                require(response.status == 200, 'readiness failed')
            with HTTP.open('http://127.0.0.1:15174/build-info.json', timeout=2) as response:
                require(json.load(response).get('sourceSHA256') == record['source_sha256'], 'frontend identity mismatch')
            info = json.loads(run(['docker', 'exec', PROJECT + '-backend-1', '/app/omnirack-server', '--build-info']))
            require(info.get('sourceSHA256') == record['source_sha256'], 'backend identity mismatch')
            for component, spec in record['images'].items():
                require(run(['docker', 'inspect', PROJECT + '-' + component + '-1', '--format', '{{.Image}}']) == spec['id'], 'running image mismatch')
            return
        except (ReleaseError, OSError, ValueError):
            time.sleep(1)
    raise ReleaseError('readiness or build identity did not pass before deadline')

def perform(record):
    # Use immutable IDs even if a tag changes after validation; never pull/build.
    with tempfile.TemporaryDirectory(prefix='omnirack-release-', dir=SHARED) as temporary:
        pinned = Path(temporary) / 'images.json'
        pinned.write_text(json.dumps({'services': {k: {'image': v['id'], 'pull_policy': 'never'} for k, v in record['images'].items()}}))
        run([*compose_command(record), '-f', str(pinned), 'up', '-d', '--no-deps', '--no-build', '--pull', 'never', 'backend', 'frontend'])
    healthy(record)

def activate(version, rollback=False):
    os.umask(0o077)
    require(SHARED.is_dir() and not SHARED.is_symlink(), 'shared directory unavailable or symlinked')
    with (SHARED / 'activation.lock').open('a') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        # Read both current and target under the same deployment lock.
        previous = current_version()
        target, record = validate(version)
        require(rollback or record.get('kind', 'release') == 'release', 'rollback snapshots cannot be activated as upgrade targets')
        if previous == version:
            healthy(record)
            print('ACTIVE', version)
            return
        old_dir, old = validate(previous)
        if rollback:
            require(version in old['validation']['rollback_compatible_from'], 'requested rollback target is not approved by current release')
        else:
            require(previous in record['validation']['rollback_compatible_from'], 'current release is not an approved schema rollback source')
        require(record['env_sha256'] == old['env_sha256'], 'environment changed: separate secret/configuration migration required')
        operation = uuid.uuid4().hex
        journal({'event': 'activation_started', 'operation': operation, 'from': previous, 'to': version, 'mode': 'rollback' if rollback else 'upgrade'})
        try:
            perform(record)
        except Exception as exc:
            journal({'event': 'activation_failed', 'operation': operation, 'to': version, 'error_type': type(exc).__name__})
            try:
                perform(old)
                journal({'event': 'application_rollback_succeeded', 'operation': operation, 'active': previous})
            except Exception as rollback:
                journal({'event': 'application_rollback_failed', 'operation': operation, 'pointer': previous, 'running_state': 'unknown', 'error_type': type(rollback).__name__})
                raise ReleaseError('activation and application rollback failed; inspect incident journal, no database restore attempted') from rollback
            raise ReleaseError('activation failed; previous application restored, database was not restored') from exc
        # Journal failure after successful deployment must not trigger a misleading rollback.
        journal({'event': 'application_healthy', 'operation': operation, 'version': version})
        point_current(target)
        journal({'event': 'activation_complete', 'operation': operation, 'active': version})
        print('ACTIVE', version)

def adopt_baseline(version):
    """Record current unmodified containers as a strict, immutable rollback snapshot."""
    os.umask(0o077)
    require(SHARED.is_dir() and not SHARED.is_symlink(), 'shared directory unavailable or symlinked')
    with (SHARED / 'activation.lock').open('a') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        previous = current_version()
        target, snapshot = validate(version)
        require(snapshot.get('kind') == 'rollback_snapshot' and snapshot.get('baseline_of') == previous,
                'snapshot must explicitly describe the current historical release')
        _, historical = load(previous)
        require(snapshot['source_sha256'] == historical.get('source_sha256') and
                snapshot['git_commit'] == historical.get('git_commit'), 'historical source identity differs')
        require(set(historical.get('images', {})) == {'backend', 'frontend'}, 'historical image records incomplete')
        for component in ('backend', 'frontend'):
            require(snapshot['images'][component]['id'] == historical['images'][component]['id'], 'snapshot image differs from historical image')
        # Historical env paths are mutable; compare content now, without copying or printing it.
        require(digest(regular(Path(historical.get('env_file', '')))) == snapshot['env_sha256'], 'historical environment differs from snapshot')
        healthy(snapshot)
        operation = uuid.uuid4().hex
        journal({'event': 'baseline_adoption_started', 'operation': operation, 'from': previous, 'to': version,
                 'containers_changed': False})
        point_current(target)
        journal({'event': 'baseline_adoption_complete', 'operation': operation, 'active': version,
                 'containers_changed': False})
        print('ADOPTED', version, '(containers unchanged)')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest='command', required=True)
    sub.add_parser('status')
    for name in ('validate', 'activate', 'rollback', 'adopt-baseline'):
        sub.add_parser(name).add_argument('version')
    args = parser.parse_args()
    try:
        if args.command == 'status':
            version = current_version()
            _, record = load(version)
            print(json.dumps({'current': version, 'schema_version': record.get('schema_version', 1), 'source_sha256': record.get('source_sha256'), 'git_commit': record.get('git_commit'), 'managed_activation_supported': record.get('schema_version') == 2}, indent=2))
        elif args.command == 'validate':
            validate(args.version)
            print('VALID', args.version)
        elif args.command == 'adopt-baseline':
            adopt_baseline(args.version)
        elif args.command == 'rollback':
            activate(args.version, rollback=True)
        else:
            activate(args.version)
    except (ReleaseError, OSError, ValueError, TypeError) as exc:
        print('ERROR:', str(exc), file=sys.stderr)
        return 1
    return 0

if __name__ == '__main__':
    sys.exit(main())
