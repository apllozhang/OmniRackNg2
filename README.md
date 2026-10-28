协作入口：[贡献约定](CONTRIBUTING.md) · [多AI协作](docs/GitHub多AI协作.md) · [来源/许可](NOTICE.md)。运行与交接证据保留103；GitHub合并不自动部署。

# IT 基础设施管理平台

> 当前目录含固定 IT 资产扩展预览版 **0.2.0-preview.1**。原产品版本及功能保留。新增 PC 配置、区域位置、机房备件、保修维保、二维码盘点和加密备份，使用前请阅读 [资产扩展说明](docs/资产扩展说明.md)。独立测试部署使用 `compose.assets.yml`，请勿直接覆盖生产实例。

> 数据中心、机房、机柜、设备及统一 U 位可视化管理系统。
>
> **产品版本：0.1.0**
>
> 说明：此前的 0.7.0 属于开发阶段内部版本，不作为当前正式产品版本。

## 项目简介

IT 基础设施管理平台用于管理数据中心基础设施资源，围绕以下层级组织数据：

```text
数据中心 → 机房 → 机柜 → U 位 → 设备
```

系统同时提供资源台账、机房大屏、设备位置管理、批量导入导出、容量分析、PDU 供电、用户权限和 LDAP 联调能力。

## 主要功能

### 数据中心、机房和机柜

- 数据中心、机房、机柜增删改查。
- 数据中心、机房和机柜自动生成或手工维护编码。
- 数据中心、机房、机柜复制。
- 跨数据中心移动机房。
- 跨机房或跨数据中心移动机柜。
- 资源状态、软删除、版本控制和操作审计。
- 支持在大屏中新增、编辑、删除和修改机柜状态。

### 机房大屏

- 按机房配置控制每排展示的机柜数量。
- 机柜固定格位展示，禁止相互重叠。
- 支持拖动机柜调整位置和交换格位。
- 支持机柜动态扩展和自动换行。
- U 位统一使用左键点击操作菜单。
- 空 U 位支持上架设备。
- 有设备的 U 位支持查看详情、编辑、迁移和下架。
- 支持通过设备名搜索并定位到机柜和 U 位。
- 机柜详情支持机柜图、规格参数和利用率分析。
- 全系统不再区分设备正面和背面。

### 设备管理

- 设备类型和设备台账。
- 设备新增、编辑、详情、筛选和分页。
- 统一设备编辑表单，设备台账和机房大屏使用同一套字段。
- 设备上架、迁移、下架和位置履历。
- 支持设备编号自动生成。
- 支持管理 IP、业务 IP、SN、功耗、规格、责任人和备注等信息。
- 一个设备只能有一个活动位置。
- 同一机柜内设备 U 位不能重叠。

### 机柜模板

- 内置标准 42U 模板。
- 支持自定义机柜模板和模板版本。
- 支持调整 U 位、宽度、深度、高度、承重、电源和 PDU 参数。
- 新增机柜时可使用模板默认值，也可以调整机柜实例参数。
- 模板版本发布后不会修改已有机柜的配置。

### 批量导入和导出

- 机柜批量导入及模板下载。
- 设备批量导入及模板下载。
- 模板标识必填字段、选填字段、字段类型、单位和示例。
- 普通机柜和设备批量导入不要求填写编码。
- 编码为空时由后端自动生成。
- 支持导出设备信息，修改管理 IP、业务 IP 等字段后重新导入更新。
- 支持导出机柜图，并使用导出的文件回导。
- 机柜图回导支持新增、改名、替换、下架和位置冲突验证。
- 导入验证阶段不直接写入正式数据，确认后通过事务提交。

### PDU 和容量分析

- PDU、插座和设备供电连接管理。
- 插座类型支持国标、欧标、10A、16A 等类型。
- 支持 U 位、功耗、承重和供电相关利用率分析。
- PDU 和供电信息为可选配置，不是设备或机柜的强制参数。
- 计算条件不足时显示“无法计算”及具体原因，不将缺失值当作 0。

### 用户、权限和 LDAP

- 本地账号密码登录。
- JWT 会话认证和 bcrypt 密码存储。
- RBAC 角色权限控制。
- `system_admin` 管理员角色。
- 管理员用户管理、角色分配、启停、删除和密码重置。
- 管理员审批策略和审批记录。
- LDAP 用户登录、用户搜索、组查询和组到角色映射。
- 管理员接口由后端权限校验保护，前端隐藏菜单不是安全边界。

## 业务规则

- 数据中心、机房和机柜不设置固定数量上限，实际容量取决于数据库、服务器和浏览器承载能力。
- `racksPerRow` 是大屏布局参数，不是资源数量上限，当前有效范围为 `1..20`。
- 必须区分字段未提供、显式填写 `0` 和其他有效数值，不能用默认值覆盖用户明确填写的值。
- 新增数据中心、机房、机柜、设备和模板时支持自动生成编码。
- 编辑已有资源不会因为修改名称、IP、SN 或位置而自动更换编码。
- 设备位置必须通过后端上架、迁移和下架业务流程修改。
- PostgreSQL 约束和事务用于防止活动设备位置重复及 U 位区间重叠。
- `ARCHIVED` 表示完整归档流程，不是简单隐藏数据。

## 技术栈

- 后端：Go、Gin、GORM。
- 数据库：PostgreSQL 16。
- 前端：Vue 3、TypeScript、Element Plus、Pinia、Vue Router。
- 构建：Vite。
- Excel：`xlsx-js-style`。
- 测试：Go Test、Go Vet、Vitest、Vue Test Utils、Playwright。
- 部署：Docker Compose、Nginx。
- LDAP：OpenLDAP 和 `go-ldap/ldap/v3`。

## 项目结构

```text
project-current/
├── backend/
│   ├── cmd/server/                 # 服务入口
│   ├── internal/config/            # 配置
│   ├── internal/router/            # 路由和中间件
│   ├── internal/handler/           # HTTP 接口处理
│   ├── internal/service/           # 业务逻辑和事务
│   ├── internal/repository/        # 数据访问
│   ├── internal/model/             # 数据模型和 DTO
│   └── api/openapi.yaml            # REST API 契约
├── frontend/
│   ├── src/api/                    # 接口请求
│   ├── src/components/             # 共享组件和表单
│   ├── src/features/               # 领域功能模块
│   ├── src/router/                 # 路由和懒加载
│   ├── src/types/                  # 前端类型
│   ├── src/utils/                  # 工具、映射和校验
│   ├── src/views/                  # 页面
│   └── e2e/                        # Playwright 测试
├── deploy/                         # 部署和 LDAP 联调说明
├── docs/                           # 项目文档
├── scripts/                        # 启停脚本
├── docker-compose.yml              # Docker Compose 配置
└── README.md
```

## 文档入口

- [项目维护文档](docs/项目维护文档-超详细版-20260904.md)：项目架构、数据模型、部署、测试、备份、排障和交接说明。
- [OpenAPI 契约](backend/api/openapi.yaml)：当前 REST API 定义。
- [部署与开发环境说明](deploy/README.md)：Docker Compose、Go 开发容器和 LDAP 联调说明。

## 一键部署

NAS/正式复刻使用根脚本 `bash deploy.sh`，项目名 `omnirack-replica`，配置为 `.env`，仅发布 15174；首次随机生成数据库、管理员和 JWT 密钥，已有配置保持原样。

本地开发另用 `bash scripts/deploy.sh`，项目名 `omnirack-replica-dev`，独立配置 `.env.dev`。首次缺省密码随机生成，可用 `DEFAULT_POSTGRES_PASSWORD`、`DEFAULT_ADMIN_PASSWORD` 或显式 `--env` 配置。备用脚本需要 Bash 4+。此脚本的开发服务会发布下面列出的端口，不用于替代 NAS 的根部署脚本。

开发启动参数：`--env-file FILE`、`--with-ldap`、`--no-build`、`--no-healthcheck`。不会删除 PostgreSQL 数据卷。开发 `.env.dev` 与正式 `.env` 分开保管。

## 运行环境

Docker Compose 当前包含以下服务：

| 服务 | 容器名 | 默认端口 | 说明 |
|---|---|---:|---|
| PostgreSQL | `omnirack-replica-postgres` | `15435` | 业务数据库 |
| Backend | `omnirack-replica-backend` | `18085` | Go REST API |
| Frontend | `omnirack-replica-frontend` | `5174` | 前端页面和 API 代理 |
| LDAP（可选） | `omnirack-replica-ldap` | `11389` | LDAP 联调服务 |
| Go Dev（可选） | `omnirack-replica-go-dev` | `2222/8081` | Go 开发容器 |

## 快速启动

### 完整容器启动

开发环境在项目根目录创建未提交的 `.env.dev` 文件，然后执行：

```powershell
Copy-Item .env.example .env.dev
# 根据环境修改管理员密码、JWT 密钥和数据库密码
docker compose --env-file .env.dev up -d --build
```

查看服务状态和日志：

```powershell
docker compose ps
docker compose logs --tail=200 backend
docker compose logs --tail=200 frontend
```

### 本地前后端开发

启动数据库：

```powershell
.\scripts\start-dev.ps1
```

启动后端：

```powershell
cd backend
go mod download
# 设置与 .env.dev 一致的凭据；Go 不会自动读取 dotenv 文件
$env:HTTP_ADDR = '127.0.0.1:18085'
$env:DATABASE_URL = 'host=127.0.0.1 user=omnirack_replica password=替换为开发库密码 dbname=omnirack_replica port=15435 sslmode=disable'
go run ./cmd/server
```

另开终端启动前端：

```powershell
cd frontend
pnpm install
pnpm dev
```

访问地址：

- 前端：<http://localhost:5174>
- 后端：<http://localhost:18085>
- 存活检查：<http://localhost:18085/health/live>
- 就绪检查：<http://localhost:18085/health/ready>

停止服务：

```powershell
docker compose stop
```

停止并删除容器但保留数据卷：

```powershell
docker compose down
```

不要在未确认备份的情况下执行 `docker compose down -v`，该命令会删除数据库数据卷。

## LDAP 联调

LDAP 使用 Compose profile，默认不启动：

```powershell
docker compose --profile ldap up -d ldap
docker compose ps ldap
docker compose logs --tail=200 ldap
```

详细的测试用户、测试组、连接配置和组到角色映射说明见：

```text
deploy/ldap/README.md
```

## 验证与测试

### 后端

```powershell
cd backend
go test -count=1 ./...
go vet ./...
```

### 前端

```powershell
cd frontend
pnpm typecheck
pnpm test
pnpm build
```

### 已登录 UI 回归

```powershell
cd frontend
pnpm test:e2e
```

更新视觉截图基线前，必须先确认界面变化是预期设计：

```powershell
pnpm test:e2e:update
```

## Docker 部署

服务器项目目录：

```text
/home/agic/workspace/omnirack-replica
```

部署前确认测试、构建和数据库备份均已完成：

```bash
cd /home/agic/workspace/omnirack-replica
docker compose ps
docker compose up -d postgres
docker compose up -d backend frontend
docker compose ps
```

部署后检查：

```bash
docker compose logs --tail=200 backend
curl -i http://127.0.0.1:18085/health/live
curl -i http://127.0.0.1:18085/health/ready
curl -I http://127.0.0.1:5174/
```

然后登录系统，检查资源树、大屏布局、U 位操作、设备编辑、Excel 导入导出和管理员权限。

## 配置与安全

配置通过未提交的 `.env` 注入，常用配置包括：

- `DATABASE_URL`、`POSTGRES_DB`、`POSTGRES_USER`、`POSTGRES_PASSWORD`。
- `JWT_SECRET`。
- `ADMIN_USERNAME`、`ADMIN_PASSWORD`。
- `GOPROXY`、`NPM_REGISTRY`。
- `LDAP_*`。
- `DEV_SSH_*`。

注意：

- 不要提交 `.env`、密码、Token、Cookie、数据库备份或生产 Excel 文件。
- 生产环境必须修改默认管理员密码、JWT 密钥和开发 SSH 密码。
- `SeedAdmin` 只在管理员不存在时初始化，不会因为修改 `.env` 自动重置已有密码。
- LDAP Bind 密码不会通过接口回显。
- 管理员权限必须由后端校验。
- 生产数据库变更前必须备份。

## 版本与本地 Git

项目使用本地 Git 仓库管理，不连接 GitHub 或其他远程仓库。提交前检查：

```powershell
git status
git log --oneline -10
git tag
```

产品版本以 README 顶部的版本号为准；开发阶段内部标识不作为正式产品版本。功能、API、数据库结构、测试流程或部署方式变化时，应同步更新 README、OpenAPI 和维护文档。

---

**最后更新：2026-09-04**

## 复刻完整度验证

源码清单、REA 原包对照、构建溯源、临时 PostgreSQL 真实回归和完整源码打包见 [复刻完整度验证](docs/复刻完整度验证.md)。NAS 运行中的三个实例不作为自动写入测试目标。
