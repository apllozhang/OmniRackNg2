# 停止本地依赖服务
$ErrorActionPreference = 'Stop'
$project = Split-Path -Parent $PSScriptRoot
if (Test-Path (Join-Path $project '.env')) {
  docker compose --env-file (Join-Path $project '.env') -f (Join-Path $project 'docker-compose.yml') down
} else {
  docker compose -f (Join-Path $project 'docker-compose.yml') down
}
