"""Backup manifest and safe extraction. Python 3.9+, standard library only."""
import hashlib
import json
import shutil
import sys
import tarfile
from pathlib import Path, PurePosixPath


def sha256(path):
    digest = hashlib.sha256()
    with path.open('rb') as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b''):
            digest.update(chunk)
    return digest.hexdigest()


def create_manifest(root):
    root = Path(root).resolve()
    files = {}
    for path in root.rglob('*'):
        if path.is_symlink():
            raise ValueError('备份中不能包含符号链接')
        if path.is_file() and path.name != 'manifest.json':
            files[path.relative_to(root).as_posix()] = {'sha256': sha256(path), 'size': path.stat().st_size}
    if 'database.dump' not in files or 'config/.env' not in files:
        raise ValueError('缺少数据库或部署配置')
    manifest = {'format': 'cabinet-assets-backup-v1', 'files': files}
    (root / 'manifest.json').write_text(json.dumps(manifest, ensure_ascii=False, indent=2), encoding='utf-8')
    return manifest


def unpack(archive_path, output):
    output = Path(output).resolve()
    if output.exists() and any(output.iterdir()):
        raise ValueError('输出目录必须为空')
    output.mkdir(parents=True, exist_ok=True)
    with tarfile.open(archive_path, 'r:gz') as archive:
        members = archive.getmembers()
        seen = set()
        normalized = []
        for member in members:
            path = PurePosixPath(member.name)
            name = path.as_posix()
            if path.is_absolute() or '..' in path.parts or '\\' in member.name or ':' in member.name:
                raise ValueError('非法归档路径')
            if not member.isdir() and not member.isfile():
                raise ValueError('不允许链接或特殊文件')
            target = (output / name).resolve()
            if target != output and output not in target.parents:
                raise ValueError('归档路径越界')
            if name in seen:
                raise ValueError('归档中存在重复路径')
            seen.add(name)
            normalized.append((name, member, target))
        manifest_item = next((m for name, m, _ in normalized if name == 'manifest.json'), None)
        if not manifest_item or manifest_item.size > 10 * 1024 * 1024:
            raise ValueError('备份清单缺失或过大')
        manifest = json.load(archive.extractfile(manifest_item))
        if manifest.get('format') != 'cabinet-assets-backup-v1':
            raise ValueError('备份格式不支持')
        expected = set(manifest['files']) | {'manifest.json'}
        actual = {name for name, member, _ in normalized if member.isfile()}
        if actual != expected or not {'database.dump', 'config/.env'}.issubset(actual):
            raise ValueError('归档内容与备份清单不一致')
        for name, member, target in normalized:
            if member.isdir():
                target.mkdir(parents=True, exist_ok=True)
                continue
            if name != 'manifest.json' and member.size != manifest['files'][name]['size']:
                raise ValueError('归档文件长度校验失败')
            target.parent.mkdir(parents=True, exist_ok=True)
            with archive.extractfile(member) as src, target.open('wb') as dst:
                shutil.copyfileobj(src, dst, 1024 * 1024)
            target.chmod(0o600)
            if name != 'manifest.json' and sha256(target) != manifest['files'][name]['sha256']:
                raise ValueError('备份文件摘要校验失败')
    return manifest


if __name__ == '__main__':
    if len(sys.argv) == 3 and sys.argv[1] == 'manifest':
        create_manifest(sys.argv[2])
    elif len(sys.argv) == 4 and sys.argv[1] == 'unpack':
        result = unpack(sys.argv[2], sys.argv[3])
        print('清单校验通过：%d 个文件' % len(result['files']))
    else:
        raise SystemExit('用法：manifest DIRECTORY | unpack ARCHIVE EMPTY_OUTPUT_DIRECTORY')
