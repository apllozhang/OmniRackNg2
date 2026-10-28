# 本地启动脚本
$ErrorActionPreference = 'Stop'
$project = Split-Path -Parent $PSScriptRoot

if (-not (Test-Path (Join-Path $project '.env'))) {
  Copy-Item (Join-Path $project '.env.example') (Join-Path $project '.env')
  Write-Host '已生成 .env，请按环境修改配置。'
}

docker compose --env-file (Join-Path $project '.env') -f (Join-Path $project 'docker-compose.yml') up -d postgres
Write-Host 'PostgreSQL 已启动。'
Write-Host '后端：在 backend 目录执行 go run ./cmd/server'
Write-Host '前端：在 frontend 目录执行 npm install; npm run dev'

