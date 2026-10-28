# 停止 Go SSH 开发容器
$ErrorActionPreference = 'Stop'
$project = Split-Path -Parent $PSScriptRoot
$composeFile = Join-Path $project 'docker-compose.yml'
$envFile = Join-Path $project '.env.dev'

if (Test-Path $envFile) {
  docker compose -p omnirack-replica-dev --env-file $envFile -f $composeFile --profile dev stop go-dev
} else {
  docker compose -p omnirack-replica-dev -f $composeFile --profile dev stop go-dev
}