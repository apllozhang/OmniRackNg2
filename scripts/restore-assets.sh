#!/usr/bin/env bash
# Default is verification only. Production restore requires an explicit destructive-operation flag.
set -Eeuo pipefail
umask 077
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
export COMPOSE_FILE="${COMPOSE_FILE:-compose.assets.yml}"
export COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-omnirack-replica}"
[[ $# -ge 1 ]] || { echo '用法：restore-assets.sh BACKUP [--verify-only | --restore --confirm-overwrite]' >&2; exit 1; }
ARCHIVE="$(realpath -- "$1")"; shift
MODE=verify
if [[ $# -gt 0 ]]; then
  case "$*" in --verify-only) ;; '--restore --confirm-overwrite') MODE=restore ;; *) echo '参数无效，恢复须同时指定 --restore --confirm-overwrite' >&2; exit 1;; esac
fi
for tool in docker gpg python3 flock; do command -v "$tool" >/dev/null || { echo "缺少依赖：$tool" >&2; exit 1; }; done
: "${BACKUP_PASSPHRASE_FILE:?请指定独立保管的备份口令文件}"
[[ -f "$ARCHIVE" && -s "$BACKUP_PASSPHRASE_FILE" ]] || { echo '备份或口令文件无效' >&2; exit 1; }
case "$(stat -c '%a' "$BACKUP_PASSPHRASE_FILE")" in 600|400) ;; *) echo '口令文件权限必须为 600 或 400' >&2; exit 1;; esac
exec 9> "$ROOT/.asset-maintenance.lock"
flock -n 9 || { echo '已有备份或恢复操作运行中' >&2; exit 1; }
WORK="$(mktemp -d "${TMPDIR:-/tmp}/cabinet-restore.XXXXXXXX")"
cleanup() { case "$WORK" in */cabinet-restore.*) [[ -d "$WORK" ]] && rm -rf -- "$WORK" ;; esac; }
trap cleanup EXIT
gpg --batch --yes --pinentry-mode loopback --passphrase-fd 3 --output "$WORK/plain.tar.gz" --decrypt "$ARCHIVE" 3< "$BACKUP_PASSPHRASE_FILE"
python3 scripts/verify-backup-archive.py unpack "$WORK/plain.tar.gz" "$WORK/verified"
if [[ "$MODE" == verify ]]; then echo '解密及清单完整性校验通过。尚未执行数据库恢复，不能替代隔离环境恢复演练。'; exit 0; fi
if [[ -f .asset-extension-version && -f "$WORK/verified/config/.asset-extension-version" ]]; then
  cmp -s .asset-extension-version "$WORK/verified/config/.asset-extension-version" || { echo '应用扩展版本与备份不一致，请先在匹配版本的隔离环境恢复' >&2; exit 1; }
fi
[[ -f .env ]] || { echo '请先配置目标环境 .env；不自动覆盖当前数据库连接和 JWT 配置' >&2; exit 1; }
docker compose stop frontend backend
# On failure keep application stopped: serving a partially restored database would be unsafe.
if ! docker compose exec -T postgres sh -c 'exec pg_restore --exit-on-error --single-transaction --clean --if-exists --no-owner --no-privileges -U "$POSTGRES_USER" -d "$POSTGRES_DB"' < "$WORK/verified/database.dump"; then
  echo '数据库恢复失败，应用保持停止。请排查并重新恢复，勿直接开放访问。' >&2; exit 1
fi
if [[ -d "$WORK/verified/files" ]]; then
  [[ ! -e files ]] || { echo '数据库已恢复；现有 files 目录需人工核对及移走后再恢复附件。应用保持停止。' >&2; exit 1; }
  cp -a -- "$WORK/verified/files" "$ROOT/files"
fi
echo '数据库恢复完成。原配置副本仍在加密备份中；请根据部署手册恢复相匹配的 JWT/LDAP 配置，并核对数据后手工启动 backend/frontend。'
