# 机柜管理工具开发环境

## 一键部署脚本

在 Linux 服务器项目根目录执行：

```bash
chmod +x scripts/deploy.sh
./scripts/deploy.sh
```

不传参数时，脚本会自动创建或补全 `.env`：数据库密码默认使用 `cabinet`，管理员账号默认使用 `admin`，管理员密码默认使用 `ChangeMe123!`；首次没有 `JWT_SECRET` 时自动生成 32 字节随机签名密钥并持久化到 `.env`。后续重启、重新执行部署脚本或更新镜像时会复用该密钥，不会因为每次启动重新生成而导致登录 Token 失效。

生产环境建议覆盖默认数据库和管理员密码：

```bash
./scripts/deploy.sh \
  --env APP_ENV=production \
  --env POSTGRES_PASSWORD='replace-with-a-strong-db-password' \
  --env ADMIN_USERNAME=admin \
  --env ADMIN_PASSWORD='replace-with-a-strong-admin-password'
```

`--env KEY=VALUE` 可重复传入，用于设置或覆盖 `.env`；`--env-file FILE` 可使用指定文件作为配置基础。`JWT_SECRET` 可以省略，也可以通过 `--env JWT_SECRET='...'` 显式指定。若已有镜像包，可使用 `--image-package FILE` 加载并跳过构建；部署完成后脚本会检查后端存活、就绪和前端首页。脚本不会删除 PostgreSQL 数据卷。使用 `--with-ldap` 可同时启动 LDAP profile。

## 前置环境

- Go 1.24 或更高版本
- Node.js 22 或更高版本
- Docker Desktop（用于 PostgreSQL 和容器化启动）

## 推荐启动方式

1. 复制 `.env.example` 为 `.env`，修改本地配置。
2. 执行 `./scripts/start-dev.ps1` 启动 PostgreSQL。
3. 在 `backend` 目录执行 `go mod tidy`，然后执行 `go run ./cmd/server`。
4. 在 `frontend` 目录执行 `npm install`，然后执行 `npm run dev`。
5. 浏览器打开 `http://localhost:5173`，默认管理员账号为 `.env` 中的 `ADMIN_USERNAME` 和 `ADMIN_PASSWORD`。

后端启动时会使用 GORM AutoMigrate 创建当前基础表，并自动初始化管理员账号。生产环境应将 AutoMigrate 替换为经过评审的正式迁移流程，并修改默认密钥和管理员密码。

## Go SSH 开发容器

项目提供 `backend/Dockerfile.dev`，用于构造带 Go 工具链和 SSH 服务的本地开发容器。它适合在宿主机没有安装 Go，或希望通过远程终端进入统一开发环境时使用。容器挂载 `backend` 目录到 `/workspace`，代码在宿主机和容器之间实时同步；Go 模块缓存和编译缓存使用独立 Docker volume 保存。

> 这是开发环境能力，不是生产部署方案。生产环境不要暴露 SSH 端口，不要使用默认密码，优先使用 SSH 公钥认证。

### 启动

在项目根目录执行：

```powershell
Copy-Item .env.example .env
# 编辑 .env，至少修改 DEV_SSH_PASSWORD；也可以配置 DEV_SSH_PUBLIC_KEY
.\scripts\start-go-dev.ps1
```

也可以直接使用 Compose：

```powershell
docker compose --profile dev up -d go-dev
```

开发容器仅依赖 PostgreSQL；脚本会使用 Compose 的健康检查等待数据库服务可用。

### SSH 连接

- 地址：`localhost`
- 端口：`2222`
- 用户名：`developer`
- 密码：`.env` 中的 `DEV_SSH_PASSWORD`
- 容器工作目录：`/workspace`

```powershell
ssh developer@localhost -p 2222
```

如果使用公钥登录，将本机公钥内容写入 `.env` 的 `DEV_SSH_PUBLIC_KEY`（保持为一行 `authorized_keys` 格式），然后重建或重启开发容器：

```powershell
docker compose --profile dev up -d --build go-dev
```

容器内可以执行：

```bash
cd /workspace
go version
go test ./...
go run ./cmd/server
```

容器内运行的 Go 服务端口为 `8080`，宿主机通过 `http://localhost:8081` 访问，避免与普通 `backend` 服务的 `8080` 端口冲突。

### 停止与清理

```powershell
.\scripts\stop-go-dev.ps1
# 如需同时删除开发容器：
docker compose --profile dev rm -f go-dev
```

### SSH 安全约束

- 镜像禁止 root 通过 SSH 登录，只允许 `developer` 用户。
- SSH 密码由运行时环境变量注入，Dockerfile 中不写入真实密码。
- `DEV_SSH_PASSWORD` 仅用于本地开发；不要将 `.env` 提交到版本库。
- 配置 `DEV_SSH_PUBLIC_KEY` 后可以使用公钥认证；仅公钥场景下会锁定密码登录。
- `2222:22` 仅为宿主机端口映射，生产环境不应把 SSH 暴露到公网。

## 基础接口

```text
GET  http://localhost:8080/health/live
GET  http://localhost:8080/health/ready
POST http://localhost:8080/api/v1/auth/login
GET  http://localhost:8080/api/v1/auth/me
```

当前阶段已完成工程基础、认证骨架和前端登录/概览页面；下一阶段开发数据中心、机房、机柜和设备资源模块。
