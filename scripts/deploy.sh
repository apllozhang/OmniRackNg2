#!/usr/bin/env bash
# 一键部署脚本：支持通过 --env KEY=VALUE 注入 Compose 和应用配置。
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"
ENV_FILE="${PROJECT_ROOT}/.env"
ENV_FILE_SOURCE=""
IMAGE_PACKAGE=""
VERSION="${DEPLOY_VERSION:-0.1.0}"
WITH_LDAP=0
DO_BUILD=1
DO_HEALTHCHECK=1
HEALTH_TIMEOUT=90
BACKEND_URL="${BACKEND_URL:-http://127.0.0.1:8080}"
FRONTEND_URL="${FRONTEND_URL:-http://127.0.0.1:5173}"

declare -a ENV_ARGS=()
declare -A OVERRIDES=()

# 首次部署使用这些兼容性默认值；写入 .env 后会在后续重启/升级中复用。
DEFAULT_POSTGRES_PASSWORD=omnirack_replica
DEFAULT_ADMIN_USERNAME="${DEFAULT_ADMIN_USERNAME:-admin}"
DEFAULT_ADMIN_PASSWORD="${DEFAULT_ADMIN_PASSWORD:-ChangeMe123!}"

usage() {
  cat <<'USAGE'
用法：
  ./scripts/deploy.sh [选项]

常用选项：
  --env KEY=VALUE       设置或覆盖 .env 中的变量，可重复使用
  --env-file FILE       以指定文件作为 .env 基础，再应用所有 --env 参数
  --image-package FILE 加载已打包的 Docker 镜像 tar/tar.gz，跳过源码构建
  --version VERSION     镜像版本，默认读取 DEPLOY_VERSION，否则为 0.1.0
  --with-ldap           同时启动 ldap profile
  --no-build            不构建镜像，直接使用本机已有镜像
  --no-healthcheck      部署完成后不执行 HTTP 健康检查
  --health-timeout SEC  健康检查最大等待秒数，默认 90
  --backend-url URL     后端健康检查地址，默认 http://127.0.0.1:8080
  --frontend-url URL    前端检查地址，默认 http://127.0.0.1:5173
  -h, --help            显示帮助

示例：
  ./scripts/deploy.sh

  # 生产环境建议覆盖默认凭据；JWT_SECRET 不传时会首次自动生成并持久化。
  ./scripts/deploy.sh \
    --env APP_ENV=production \
    --env POSTGRES_PASSWORD='replace-with-a-strong-db-password' \
    --env ADMIN_USERNAME=admin \
    --env ADMIN_PASSWORD='replace-with-a-strong-admin-password'

  ./scripts/deploy.sh --image-package ./omnirack-replica-images-0.1.0.tar.gz \
    --env APP_ENV=production

说明：
  - 脚本默认在项目根目录运行 Docker Compose，不会执行 down -v，不会删除 PostgreSQL 数据卷。
  - --env 参数不会在日志中输出值；不要把含密码的命令写入公共历史记录。
  - 使用 --env-file 时，脚本会备份当前 .env，然后用指定文件替换 .env。
USAGE
}

fail() {
  echo "[ERROR] $*" >&2
  exit 1
}

log() {
  echo "[deploy] $*"
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "缺少命令：$1"
}

validate_assignment() {
  local assignment="$1"
  [[ "$assignment" == *=* ]] || fail "--env 必须使用 KEY=VALUE 格式：${assignment%%=*}"
  local key="${assignment%%=*}"
  [[ "$key" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] || fail "环境变量名不合法：${key}"
  [[ "$assignment" != *$'\n'* && "$assignment" != *$'\r'* ]] || fail "环境变量值不能包含换行：${key}"
}

quote_dotenv_value() {
  local value="$1"
  # 使用单引号保存空格、#、$ 等字符；单引号本身按 dotenv 规则转义。
  value="${value//\'/\'\\\'\'}"
  printf "'%s'" "$value"
}

has_dotenv_value() {
  local file="$1" key="$2" line value
  [[ -f "$file" ]] || return 1
  while IFS= read -r line || [[ -n "$line" ]]; do
    [[ "$line" =~ ^${key}= ]] || continue
    value="${line#*=}"
    value="${value#"${value%%[![:space:]]*}"}"
    value="${value%"${value##*[![:space:]]}"}"
    [[ -n "$value" && "$value" != "''" && "$value" != '""' ]] && return 0
  done < "$file"
  return 1
}

generate_jwt_secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 32
    return
  fi
  if command -v od >/dev/null 2>&1; then
    od -An -N32 -tx1 /dev/urandom | tr -d ' \n'
    return
  fi
  fail "无法自动生成 JWT_SECRET：服务器缺少 openssl 或 od"
}

ensure_default_env() {
  local key="$1" value="$2" source_file="$3"
  [[ -v "OVERRIDES[$key]" ]] && return 0
  has_dotenv_value "$source_file" "$key" && return 0
  OVERRIDES["$key"]="$value"
}

write_env_file() {
  local source_file="$1"
  local tmp_file
  tmp_file="$(mktemp "${ENV_FILE}.tmp.XXXXXX")"
  chmod 600 "$tmp_file"

  if [[ -f "$source_file" ]]; then
    while IFS= read -r line || [[ -n "$line" ]]; do
      if [[ "$line" =~ ^([A-Za-z_][A-Za-z0-9_]*)= ]]; then
        local key="${BASH_REMATCH[1]}"
        if [[ -v "OVERRIDES[$key]" ]]; then
          printf '%s=%s\n' "$key" "$(quote_dotenv_value "${OVERRIDES[$key]}")" >> "$tmp_file"
          unset 'OVERRIDES[$key]'
        else
          printf '%s\n' "$line" >> "$tmp_file"
        fi
      else
        printf '%s\n' "$line" >> "$tmp_file"
      fi
    done < "$source_file"
  fi

  local key
  for key in "${!OVERRIDES[@]}"; do
    printf '%s=%s\n' "$key" "$(quote_dotenv_value "${OVERRIDES[$key]}")" >> "$tmp_file"
  done

  mv -f "$tmp_file" "$ENV_FILE"
  chmod 600 "$ENV_FILE"
}

backup_env_file() {
  if [[ -f "$ENV_FILE" ]]; then
    local stamp
    stamp="$(date +%Y%m%d-%H%M%S)"
    cp -p "$ENV_FILE" "${ENV_FILE}.backup.${stamp}"
    chmod 600 "${ENV_FILE}.backup.${stamp}"
    log "已备份现有 .env（文件内容不会输出）"
  fi
}

wait_http() {
  local url="$1"
  local name="$2"
  local deadline=$((SECONDS + HEALTH_TIMEOUT))
  while (( SECONDS < deadline )); do
    if curl -fsS --max-time 5 "$url" >/dev/null 2>&1; then
      log "${name} 健康检查通过：${url}"
      return 0
    fi
    sleep 2
  done
  echo "[ERROR] ${name} 健康检查超时：${url}" >&2
  return 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --env)
      [[ $# -ge 2 ]] || fail "--env 缺少 KEY=VALUE 参数"
      validate_assignment "$2"
      ENV_ARGS+=("$2")
      OVERRIDES["${2%%=*}"]="${2#*=}"
      shift 2
      ;;
    --env-file)
      [[ $# -ge 2 ]] || fail "--env-file 缺少文件路径"
      ENV_FILE_SOURCE="$2"
      shift 2
      ;;
    --image-package)
      [[ $# -ge 2 ]] || fail "--image-package 缺少文件路径"
      IMAGE_PACKAGE="$2"
      shift 2
      ;;
    --version)
      [[ $# -ge 2 ]] || fail "--version 缺少版本号"
      VERSION="$2"
      shift 2
      ;;
    --with-ldap)
      WITH_LDAP=1
      shift
      ;;
    --no-build)
      DO_BUILD=0
      shift
      ;;
    --no-healthcheck)
      DO_HEALTHCHECK=0
      shift
      ;;
    --health-timeout)
      [[ $# -ge 2 ]] || fail "--health-timeout 缺少秒数"
      [[ "$2" =~ ^[0-9]+$ && "$2" -gt 0 ]] || fail "健康检查超时时间必须是正整数"
      HEALTH_TIMEOUT="$2"
      shift 2
      ;;
    --backend-url)
      [[ $# -ge 2 ]] || fail "--backend-url 缺少 URL"
      BACKEND_URL="$2"
      shift 2
      ;;
    --frontend-url)
      [[ $# -ge 2 ]] || fail "--frontend-url 缺少 URL"
      FRONTEND_URL="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "未知参数：$1（使用 --help 查看用法）"
      ;;
  esac
done

require_command docker
require_command curl

docker compose version >/dev/null 2>&1 || fail "当前 Docker 未提供 Docker Compose 插件"
[[ -n "$VERSION" ]] || fail "版本号不能为空"

if [[ -n "$ENV_FILE_SOURCE" && "$ENV_FILE_SOURCE" != /* ]]; then
  ENV_FILE_SOURCE="$(pwd)/${ENV_FILE_SOURCE}"
fi
if [[ -n "$IMAGE_PACKAGE" && "$IMAGE_PACKAGE" != /* ]]; then
  IMAGE_PACKAGE="$(pwd)/${IMAGE_PACKAGE}"
fi

cd "$PROJECT_ROOT"
[[ -f docker-compose.yml ]] || fail "未找到 docker-compose.yml：${PROJECT_ROOT}"

# 没有显式传参时也可以直接部署：数据库和管理员使用兼容性默认值，
# JWT_SECRET 只在首次缺失时生成，并持久化到 .env，避免每次重启导致 Token 失效。
CONFIG_SOURCE="$ENV_FILE"
if [[ -n "$ENV_FILE_SOURCE" ]]; then
  CONFIG_SOURCE="$ENV_FILE_SOURCE"
fi
ensure_default_env "POSTGRES_PASSWORD" "$DEFAULT_POSTGRES_PASSWORD" "$CONFIG_SOURCE"
ensure_default_env "ADMIN_USERNAME" "$DEFAULT_ADMIN_USERNAME" "$CONFIG_SOURCE"
ensure_default_env "ADMIN_PASSWORD" "$DEFAULT_ADMIN_PASSWORD" "$CONFIG_SOURCE"
if ! [[ -v "OVERRIDES[JWT_SECRET]" ]] && ! has_dotenv_value "$CONFIG_SOURCE" "JWT_SECRET"; then
  OVERRIDES["JWT_SECRET"]="$(generate_jwt_secret)"
  log "首次部署已自动生成 JWT_SECRET，并将持久化到 .env（密钥内容不会输出）"
fi

if [[ -n "$ENV_FILE_SOURCE" ]]; then
  [[ -f "$ENV_FILE_SOURCE" ]] || fail "指定的 env 文件不存在：${ENV_FILE_SOURCE}"
  backup_env_file
  write_env_file "$ENV_FILE_SOURCE"
elif [[ ${#OVERRIDES[@]} -gt 0 ]]; then
  backup_env_file
  write_env_file "$ENV_FILE"
elif [[ ! -f "$ENV_FILE" ]]; then
  : > "$ENV_FILE"
  chmod 600 "$ENV_FILE"
  log "未找到 .env，已创建空配置文件；可使用 --env 注入配置"
fi

if (( WITH_LDAP )); then
  has_dotenv_value "$ENV_FILE" "LDAP_ADMIN_PASSWORD" || fail "启用 LDAP 时必须提供 LDAP_ADMIN_PASSWORD，可使用 --env LDAP_ADMIN_PASSWORD='...'"
fi

if [[ -n "$IMAGE_PACKAGE" ]]; then
  [[ -f "$IMAGE_PACKAGE" ]] || fail "镜像包不存在：${IMAGE_PACKAGE}"
  log "加载镜像包（不输出镜像层内容）"
  docker load -i "$IMAGE_PACKAGE"
  docker tag "omnirack-replica-backend:${VERSION}" omnirack-replica-backend:latest
  docker tag "omnirack-replica-frontend:${VERSION}" omnirack-replica-frontend:latest
  docker tag "omnirack-replica-postgres:${VERSION}" postgres:16-alpine
  DO_BUILD=0
fi

if (( DO_BUILD )); then
  log "构建 backend/frontend 镜像，版本：${VERSION}"
  docker compose build --pull=false backend frontend
  docker tag omnirack-replica-backend:latest "omnirack-replica-backend:${VERSION}"
  docker tag omnirack-replica-frontend:latest "omnirack-replica-frontend:${VERSION}"
  if ! docker image inspect "omnirack-replica-postgres:${VERSION}" >/dev/null 2>&1; then
    docker image inspect postgres:16-alpine >/dev/null 2>&1 || docker compose pull postgres
    docker tag postgres:16-alpine "omnirack-replica-postgres:${VERSION}"
  fi
fi

services=(postgres backend frontend)
compose_args=()
if (( WITH_LDAP )); then
  compose_args+=(--profile ldap)
  services+=(ldap)
fi

log "启动服务：${services[*]}"
docker compose "${compose_args[@]}" up -d --no-build --remove-orphans "${services[@]}"

if (( DO_HEALTHCHECK )); then
  wait_http "${BACKEND_URL}/health/live" "backend liveness"
  wait_http "${BACKEND_URL}/health/ready" "backend readiness"
  wait_http "${FRONTEND_URL}/" "frontend"
fi

log "部署完成，版本：${VERSION}"
docker compose "${compose_args[@]}" ps

