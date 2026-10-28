# 本地启动脚本
$ErrorActionPreference = 'Stop'
$project = Split-Path -Parent $PSScriptRoot

if (-not (Test-Path (Join-Path $project '.env.dev'))) {
  Copy-Item (Join-Path $project '.env.example') (Join-Path $project '.env.dev')
  Write-Host '已生成 .env.dev，请按环境修改配置。'
}

docker compose -p omnirack-replica-dev --env-file (Join-Path $project '.env.dev') -f (Join-Path $project 'docker-compose.yml') up -d postgres
Write-Host 'PostgreSQL 已启动。'
Write-Host '后端：在 backend 目录执行 go run ./cmd/server'
Write-Host '前端：在 frontend 目录执行 npm install; npm run dev'

