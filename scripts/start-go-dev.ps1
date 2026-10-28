# 启动 Go SSH 开发容器
$ErrorActionPreference = 'Stop'
$project = Split-Path -Parent $PSScriptRoot
$composeFile = Join-Path $project 'docker-compose.yml'
$envFile = Join-Path $project '.env.dev'

if (-not (Test-Path $envFile)) {
  Copy-Item (Join-Path $project '.env.example') $envFile
  Write-Host '已生成 .env.dev。开发环境默认 SSH 密码为 ChangeMe123!，请按需修改。'
}

docker compose -p omnirack-replica-dev --env-file $envFile -f $composeFile --profile dev up -d go-dev
Write-Host 'Go SSH 开发容器已启动。'
Write-Host '连接：ssh developer@localhost -p 2222'
Write-Host '容器工作目录：/workspace'