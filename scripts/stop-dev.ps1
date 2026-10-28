# 停止本地依赖服务
$ErrorActionPreference = 'Stop'
$project = Split-Path -Parent $PSScriptRoot
if (Test-Path (Join-Path $project '.env.dev')) {
  docker compose -p omnirack-replica-dev --env-file (Join-Path $project '.env.dev') -f (Join-Path $project 'docker-compose.yml') down
} else {
  docker compose -p omnirack-replica-dev -f (Join-Path $project 'docker-compose.yml') down
}
