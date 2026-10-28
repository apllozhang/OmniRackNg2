#!/usr/bin/env python3
"""NAS-local task isolation and verifiable AI handoff. Python standard library + Git."""
import argparse
import contextlib
from datetime import datetime, timezone
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import stat
import subprocess
import sys
import tarfile

DEFAULT_ROOT = Path('/home/tina/OmniRackWorkspace')
SAFE_ID = re.compile(r'[a-z][a-z0-9-]{2,63}\Z')


def require_id(value):
    if not SAFE_ID.fullmatch(value):
        raise ValueError('task ID must be 3–64 lowercase letters/digits/hyphens, starting with a letter')
    return value


def now():
    return datetime.now(timezone.utc).isoformat()


def git(directory, *args, binary=False):
    result = subprocess.run(['git', '-C', str(directory), *args], check=True,
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    return result.stdout if binary else result.stdout.decode().rstrip('\n')


def atomic_json(path, value):
    temporary = path.with_name('.' + path.name + '.' + secrets.token_hex(6))
    descriptor = os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(descriptor, 'w') as stream:
        json.dump(value, stream, ensure_ascii=False, indent=2)
        stream.write('\n')
        stream.flush()
        os.fsync(stream.fileno())
    os.replace(temporary, path)
    fsync_dir(path.parent)


def fsync_dir(path):
    descriptor = os.open(path, os.O_RDONLY)
    try:
        os.fsync(descriptor)
    finally:
        os.close(descriptor)


@contextlib.contextmanager
def lock(root):
    (root / 'locks').mkdir(mode=0o700, parents=True, exist_ok=True)
    with (root / 'locks/workspace.lock').open('a') as stream:
        os.chmod(stream.name, 0o600)
        fcntl.flock(stream.fileno(), fcntl.LOCK_EX)
        yield


def secure_path(root, child):
    candidate = root / child
    if candidate.is_symlink() or not candidate.resolve().is_relative_to(root.resolve()):
        raise ValueError('workspace path must be contained and must not be a symlink: ' + str(child))
    return candidate


def task_record(root, task_id):
    require_id(task_id)
    directory = secure_path(root, 'tasks/' + task_id)
    record = json.loads((directory / 'TASK.json').read_text())
    worktree = secure_path(root, 'worktrees/' + task_id)
    if record.get('id') != task_id or record.get('worktree') != str(worktree):
        raise ValueError('task record identity/path mismatch; inspect without overwriting')
    return directory, record, worktree


def sensitive(path):
    parts = Path(path).parts
    name = parts[-1].lower() if parts else ''
    return any(part.lower() in ('secrets', '.ssh', 'private') for part in parts) or name == '.env' or name.startswith('.env.') or name.endswith(('.pem', '.key', '.p12', '.pfx', '.jks')) or name in ('id_rsa', 'id_ed25519', 'credentials.json')


def file_names(raw):
    return [os.fsdecode(item) for item in raw.split(b'\0') if item]


def fingerprint(worktree, paths):
    values = {}
    for name in paths:
        path = worktree / name
        try:
            metadata = path.lstat()
        except FileNotFoundError:
            values[name] = 'deleted'
            continue
        if stat.S_ISLNK(metadata.st_mode):
            digest = hashlib.sha256(os.fsencode(os.readlink(path))).hexdigest()
        elif stat.S_ISREG(metadata.st_mode):
            digest = hashlib.sha256(path.read_bytes()).hexdigest()
        else:
            digest = 'special'
        values[name] = (metadata.st_mode, digest)
    return values


def sync_file(path):
    with path.open('rb') as stream:
        os.fsync(stream.fileno())


def task_new(root, args):
    task_id = require_id(args.task_id)
    for name in ('tasks', 'worktrees', 'handoffs', 'evidence'):
        secure_path(root, name).mkdir(mode=0o700, parents=True, exist_ok=True)
    with lock(root):
        task = secure_path(root, 'tasks/' + task_id)
        worktree = secure_path(root, 'worktrees/' + task_id)
        if task.exists() or worktree.exists():
            raise ValueError('task/worktree already exists; inspect or hand it off, never overwrite it')
        repo = secure_path(root, 'repo')
        commit = git(repo, 'rev-parse', '--verify', 'HEAD^{commit}')
        branch = 'work/' + task_id
        task.mkdir(mode=0o700)
        record = {'schema_version': 1, 'id': task_id, 'actor': args.actor,
                  'goal': args.goal, 'state': 'provisioning', 'created_at': now(),
                  'base_commit': commit, 'branch': branch, 'worktree': str(worktree)}
        atomic_json(task / 'TASK.json', record)
        try:
            git(repo, 'worktree', 'add', '-b', branch, str(worktree), commit)
        except Exception:
            record.update(state='provisioning-failed', updated_at=now())
            atomic_json(task / 'TASK.json', record)
            raise
        record.update(state='active', updated_at=now())
        atomic_json(task / 'TASK.json', record)
        return record


def handoff(root, args):
    with lock(root):
        task, record, worktree = task_record(root, args.task_id)
        if record['state'] == 'provisioning-failed':
            raise ValueError('repair/inspect provisioning failure before handing off')
        branch = git(worktree, 'branch', '--show-current')
        if branch != record['branch']:
            raise ValueError('unexpected branch; reconcile TASK.json explicitly before handoff')
        evidence = []
        for raw in args.evidence:
            path = Path(raw)
            if not path.is_absolute():
                path = root / path
            if path.is_symlink() or not path.resolve().is_relative_to(root.resolve()) or not path.is_file():
                raise ValueError('evidence must be an existing regular file inside this workspace')
            if sensitive(str(path.relative_to(root))):
                raise ValueError('secret paths cannot be evidence')
            evidence.append({'path': str(path), 'sha256': hashlib.sha256(path.read_bytes()).hexdigest()})
        initial_head = git(worktree, 'rev-parse', 'HEAD')
        initial_status = git(worktree, 'status', '--porcelain=v1', '-z', binary=True)
        tracked = file_names(git(worktree, 'diff', '--name-only', '-z', 'HEAD', binary=True))
        untracked = file_names(git(worktree, 'ls-files', '--others', '--exclude-standard', '-z', binary=True))
        excluded = [p for p in tracked + untracked if sensitive(p)]
        if args.status == 'complete' and excluded:
            raise ValueError('secret-path changes are not snapshotted; complete handoff requires separate secure preservation and reconciliation')
        clean_tracked = [p for p in tracked if not sensitive(p)]
        clean_untracked = [p for p in untracked if not sensitive(p)]
        for item in clean_untracked:
            path = worktree / item
            if path.is_symlink() or not path.is_file() or not path.resolve().is_relative_to(worktree.resolve()):
                raise ValueError('untracked snapshot refuses symlinks/special files; reconcile: ' + item)
            if path.stat().st_size > 32 * 1024 * 1024:
                raise ValueError('untracked file exceeds 32 MiB; move artifact to workspace evidence and record it: ' + item)
        before = fingerprint(worktree, clean_tracked + clean_untracked)
        handoff_id = datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%S') + '-' + args.task_id + '-' + secrets.token_hex(4)
        directory = secure_path(root, 'handoffs/' + handoff_id)
        directory.mkdir(mode=0o700)
        patch = directory / 'working-tree.patch'
        diff = git(worktree, 'diff', '--no-ext-diff', '--binary', 'HEAD', '--', *clean_tracked, binary=True) if clean_tracked else b''
        patch.write_bytes(diff)
        os.chmod(patch, 0o600)
        archive = directory / 'untracked.tar.gz'
        with tarfile.open(archive, 'w:gz', dereference=False) as tar:
            for item in clean_untracked:
                tar.add(worktree / item, arcname=item, recursive=False)
        os.chmod(archive, 0o600)
        hashes = {path.name: hashlib.sha256(path.read_bytes()).hexdigest() for path in (patch, archive)}
        if before != fingerprint(worktree, clean_tracked + clean_untracked):
            raise ValueError('file contents changed during snapshot; stop edits and retry; partial snapshot remains unpublished')
        if initial_head != git(worktree, 'rev-parse', 'HEAD') or initial_status != git(worktree, 'status', '--porcelain=v1', '-z', binary=True):
            raise ValueError('worktree changed during snapshot; stop edits and retry; partial snapshot remains unpublished')
        snapshot = {'schema_version': 1, 'id': handoff_id, 'created_at': now(),
                    'task_id': args.task_id, 'actor': record['actor'], 'goal': record['goal'],
                    'state': args.status, 'summary': args.summary, 'next_steps': args.next_steps,
                    'risks': args.risk, 'commands_and_results': args.check,
                    'branch': branch, 'commit': initial_head,
                    'worktree': str(worktree), 'git_status': git(worktree, 'status', '--short'),
                    'tracked_changes': tracked, 'untracked_files': untracked,
                    'excluded_secret_paths': excluded, 'snapshot_sha256': hashes,
                    'evidence': evidence, 'verification': 'Successor must independently rerun checks; statements are not execution proof.'}
        atomic_json(directory / 'HANDOFF.json', snapshot)
        note = directory / 'README.md'
        note.write_text('# AI 交接\n\n任务：' + args.task_id + '\n\n目标：' + record['goal'] + '\n\n' + args.summary + '\n\n下一步：' + args.next_steps + '\n\n以 HANDOFF.json 中实际 Git 状态、校验和及证据为准；接班方需重新核实。\n')
        os.chmod(note, 0o600)
        for path in (patch, archive, note):
            sync_file(path)
        fsync_dir(directory)
        record.update(state=args.status, updated_at=now(), latest_handoff=str(directory), head_commit=snapshot['commit'])
        atomic_json(task / 'TASK.json', record)
        temporary = root / 'handoffs' / ('.latest-' + secrets.token_hex(6))
        os.symlink(directory.name, temporary)
        os.replace(temporary, root / 'handoffs/latest')
        fsync_dir(root / 'handoffs')
        return snapshot


def status(root):
    repo = secure_path(root, 'repo')
    output = {'workspace': str(root), 'observed_at': now(), 'tasks': []}
    if repo.exists():
        output['canonical_commit'] = git(repo, 'rev-parse', 'HEAD')
        output['canonical_status'] = git(repo, 'status', '--short')
    for file in sorted((root / 'tasks').glob('*/TASK.json')):
        if file.is_symlink():
            continue
        value = json.loads(file.read_text())
        _, _, worktree = task_record(root, value['id'])
        if worktree.exists():
            value['observed_commit'] = git(worktree, 'rev-parse', 'HEAD')
            value['observed_status'] = git(worktree, 'status', '--short')
        output['tasks'].append(value)
    current = root.parent / 'OmniRackReplica-current'
    if current.is_symlink():
        output['deployed_release_pointer'] = str(current.resolve())
    return output


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=Path, default=DEFAULT_ROOT)
    commands = parser.add_subparsers(dest='command', required=True)
    commands.add_parser('status')
    new = commands.add_parser('task-new')
    new.add_argument('task_id')
    new.add_argument('--actor', required=True)
    new.add_argument('--goal', required=True)
    transfer = commands.add_parser('handoff')
    transfer.add_argument('task_id')
    transfer.add_argument('--summary', required=True)
    transfer.add_argument('--next', dest='next_steps', required=True)
    transfer.add_argument('--status', choices=('handoff', 'complete', 'needs-input'), default='handoff')
    transfer.add_argument('--check', action='append', default=[], help='command and observed result; never include secrets')
    transfer.add_argument('--evidence', action='append', default=[], help='existing file under workspace, absolute or relative')
    transfer.add_argument('--risk', action='append', default=[])
    args = parser.parse_args()
    root = args.root.absolute()
    os.umask(0o077)
    try:
        if args.command == 'task-new':
            result = task_new(root, args)
        elif args.command == 'handoff':
            result = handoff(root, args)
        else:
            result = status(root)
        print(json.dumps(result, ensure_ascii=False, indent=2))
    except (ValueError, OSError, subprocess.CalledProcessError, json.JSONDecodeError) as error:
        print('ERROR: ' + str(error), file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
