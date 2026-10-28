#!/usr/bin/env bash
# Online Linux deployment of the isolated IT asset extension.
set -Eeuo pipefail
umask 077
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"
export COMPOSE_PROJECT_NAME=omnirack-replica
export COMPOSE_FILE="$ROOT/compose.assets.yml"
fail() { printf '%s\n' "$*" >&2; exit 1; }
command -v docker >/dev/null || fail '请先安装 Docker Engine 和 Docker Compose v2。'
docker compose version >/dev/null || fail 'Docker Compose v2 不可用。'
docker info >/dev/null || fail 'Docker 服务不可用，请检查服务状态和当前用户权限。'
[[ -f compose.assets.yml ]] || fail '部署包不完整：缺少 compose.assets.yml。'

if [[ ! -e .env ]]; then
  # A database volume retains its initialization password. A fresh dotenv file
  # must never be generated for an existing deployment, even in a new folder.
  existing_volumes="$(docker volume ls --format '{{.Name}}')" || fail '无法检查已有数据卷，已停止初始化。'
  existing_postgres="$(docker ps -a --filter "label=com.docker.compose.project=$COMPOSE_PROJECT_NAME" --filter 'label=com.docker.compose.service=postgres' --format '{{.ID}}')" || fail '无法检查已有数据库容器，已停止初始化。'
  if printf '%s\n' "$existing_volumes" | grep -Fxq "${COMPOSE_PROJECT_NAME}_asset-postgres-data" || [[ -n "$existing_postgres" ]]; then
    fail '检测到此项目已有数据库，但当前目录缺少 .env。已停止，未生成新密码、未修改容器。升级前请将旧安装目录的 .env 以 600 权限复制到本目录，保留原数据库密码和 JWT_SECRET 后重试。不要删除数据卷。'
  fi
  # Hex secrets avoid shell, dotenv and database connection string escaping issues.
  random_secret() { od -An -N32 -tx1 /dev/urandom | tr -d ' \n'; }
  database_secret="$(random_secret)"
  jwt_secret="$(random_secret)"
  admin_secret="$(random_secret)"
  [[ ${#database_secret} == 64 && ${#jwt_secret} == 64 && ${#admin_secret} == 64 ]] || fail '生成随机密码失败。'
  temporary_env="$(mktemp "$ROOT/.env.init.XXXXXXXX")"
  trap '[[ ! -f "${temporary_env:-}" ]] || rm -f -- "$temporary_env"' EXIT
  cat > "$temporary_env" <<EOF
APP_ENV=production
POSTGRES_DB=omnirack_replica
POSTGRES_USER=omnirack_replica
POSTGRES_PASSWORD=$database_secret
JWT_SECRET=$jwt_secret
ADMIN_USERNAME=admin
ADMIN_PASSWORD=$admin_secret
JWT_EXPIRES_MINUTES=120
ASSET_HTTP_BIND=0.0.0.0
ASSET_HTTP_PORT=15174
EOF
  # Never overwrite a configuration created by another deployment process.
  ln "$temporary_env" .env || fail '.env 已存在；未覆盖，请重新执行。'
  rm -f -- "$temporary_env"
  trap - EXIT
  printf '已生成 .env（仅当前用户可读写），默认监听服务器 15174 端口。\n'
else
  [[ -f .env && ! -L .env ]] || fail '.env 必须是普通文件。'
  printf '复用已有 .env，保留数据库密码及登录配置。\n'
fi
chmod 600 .env
for key in POSTGRES_DB POSTGRES_USER POSTGRES_PASSWORD JWT_SECRET ADMIN_USERNAME ADMIN_PASSWORD; do
  grep -Eq "^${key}=.+" .env || fail ".env 缺少非空配置：$key"
done
if grep -Eq 'REPLACE_WITH_|ChangeMe123!' .env; then
  fail '.env 仍有示例密码，请设置随机数据库密码、JWT_SECRET 和管理员密码后重试。'
fi

# No source/eval of dotenv files, no old compose file, no volume deletion.
docker compose --env-file .env config --quiet
command -v python3 >/dev/null || fail '源码校验需要 Python 3。'
SOURCE_SHA256="$(python3 scripts/replica_manifest.py | python3 -c 'import json,sys; print(json.load(sys.stdin)["source_sha256"])')"
export SOURCE_SHA256
printf '开始拉取基础镜像并构建程序；首次运行需要联网，耗时取决于网络。\n'
# Cached/mirrored base images are required on some NAS installations. Pulling is
# opt-in, and the switch is deliberately not evaluated as shell code.
case "${PULL_BASE_IMAGES:-false}" in
  true) docker compose --env-file .env build --pull ;;
  false) docker compose --env-file .env build ;;
  *) fail 'PULL_BASE_IMAGES 必须为 true 或 false' ;;
esac
docker compose --env-file .env up -d
printf '检查数据库就绪状态和前端 HTTP 服务……\n'
ready=false
for ((attempt=0; attempt<60; attempt++)); do
  if docker compose --env-file .env exec -T backend sh -c \
    'wget -T 5 -q -O /dev/null http://127.0.0.1:8080/health/ready && wget -T 5 -q -O /dev/null http://frontend/' >/dev/null 2>&1; then
    ready=true
    break
  fi
  sleep 2
done
[[ "$ready" == true ]] || fail '服务未通过健康检查。配置和数据已保留，请查看：docker compose --env-file .env -p omnirack-replica -f compose.assets.yml logs --tail=100'
docker compose --env-file .env ps
printf '\n部署成功。默认访问：http://服务器IP:15174（如修改过 .env，以其中的端口为准）。\n'
printf '首次管理员信息在 .env：在服务器本机执行以下命令查看，请勿对外分享。\n'
printf "  grep -E '^ADMIN_(USERNAME|PASSWORD)=' .env\n"
printf '已有数据库的管理员密码不会因再次部署而重置。\n'
