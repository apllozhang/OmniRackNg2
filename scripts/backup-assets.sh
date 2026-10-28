#!/usr/bin/env bash
# Linux deployment host: encrypted full backup; no passwords in command arguments.
set -Eeuo pipefail
umask 077
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
export COMPOSE_FILE="${COMPOSE_FILE:-compose.assets.yml}"
export COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-omnirack-replica}"
for tool in docker gpg python3 tar flock; do command -v "$tool" >/dev/null || { echo "缺少依赖：$tool" >&2; exit 1; }; done
: "${BACKUP_PASSPHRASE_FILE:?请指定独立保存、权限为 600 或 400 的口令文件}"
[[ -f "$BACKUP_PASSPHRASE_FILE" && -s "$BACKUP_PASSPHRASE_FILE" ]] || { echo '备份口令文件不存在或为空' >&2; exit 1; }
case "$(stat -c '%a' "$BACKUP_PASSPHRASE_FILE")" in 600|400) ;; *) echo '口令文件权限必须为 600 或 400' >&2; exit 1;; esac
[[ -f .env ]] || { echo '.env 不存在，不能生成完整配置备份' >&2; exit 1; }
BACKUP_DIR="${BACKUP_DIR:-$ROOT/backups}"
mkdir -p -- "$BACKUP_DIR"
BACKUP_DIR="$(cd -- "$BACKUP_DIR" && pwd)"
exec 9> "$ROOT/.asset-maintenance.lock"
flock -n 9 || { echo '已有备份或恢复操作运行中' >&2; exit 1; }
WORK="$(mktemp -d "${TMPDIR:-/tmp}/cabinet-backup.XXXXXXXX")"
RUNNING=()
for service in backend frontend; do
  if docker compose ps --services --status running | grep -qx "$service"; then RUNNING+=("$service"); fi
done
restore_services() {
  local result=$?
  trap - EXIT
  if ((${#RUNNING[@]})); then docker compose start "${RUNNING[@]}" >/dev/null || result=1; fi
  case "$WORK" in */cabinet-backup.*) [[ -d "$WORK" ]] && rm -rf -- "$WORK" ;; esac
  exit "$result"
}
trap restore_services EXIT
if ((${#RUNNING[@]})); then docker compose stop "${RUNNING[@]}"; fi
mkdir -p "$WORK/payload/config"
docker compose exec -T postgres sh -c 'exec pg_dump -Fc --no-owner --no-privileges -U "$POSTGRES_USER" "$POSTGRES_DB"' > "$WORK/payload/database.dump"
[[ -s "$WORK/payload/database.dump" ]] || { echo '数据库备份为空' >&2; exit 1; }
cp -- .env docker-compose.yml compose.assets.yml "$WORK/payload/config/"
for file in .deploy-version .asset-extension-version; do [[ ! -f "$file" ]] || cp -- "$file" "$WORK/payload/config/"; done
# The original release has no attachment module. Include deployment-local business files if present.
if [[ -d files ]]; then
  [[ -z "$(find files -type l -print -quit)" ]] || { echo 'files 目录存在符号链接，请先核查备份范围' >&2; exit 1; }
  cp -a -- files "$WORK/payload/files"
fi
python3 scripts/verify-backup-archive.py manifest "$WORK/payload"
tar -czf "$WORK/plain.tar.gz" -C "$WORK/payload" .
NAME="cabinet-assets-$(date -u +%Y%m%dT%H%M%SZ)-$$.tar.gpg"
gpg --batch --yes --pinentry-mode loopback --passphrase-fd 3 --symmetric --cipher-algo AES256 --output "$BACKUP_DIR/$NAME.partial" "$WORK/plain.tar.gz" 3< "$BACKUP_PASSPHRASE_FILE"
mv -- "$BACKUP_DIR/$NAME.partial" "$BACKUP_DIR/$NAME"
sha256sum "$BACKUP_DIR/$NAME" > "$BACKUP_DIR/$NAME.sha256"
printf '加密备份完成：%s\n' "$BACKUP_DIR/$NAME"
printf '请复制至独立备份存储，并执行恢复验证；口令不要与备份放在一起。\n'
