# API 契约维护

`openapi.yaml` 是实际源码生成的 OpenAPI 3.0.3，使用 JSON 文法保存（YAML 1.2 子集），可由 OpenAPI 客户端生成器和查看器直接读取。HTTP 路径已经包含 `/api/v1`；server URL 为 `/`，不要重复添加 `/api/v1`。

在仓库根目录执行：

```sh
python3 scripts/api-contract.py --check
```

更新接口后重新生成并审阅三个文件：

```sh
python3 scripts/api-contract.py
```

对应的可读材料为 `docs/api-接口清单.md`、`docs/api-错误码清单.md`、`docs/架构与集成规范.md`。这些文件与源码共同保存在 103 的工作区仓库，后续 AI 不需要访问上一个 AI 的电脑。

生成过程调用 `go run ./cmd/route-inventory --source-contract`，仅注册 Gin 路由并读取 Go AST，不初始化数据库、不启动服务。依赖 Python 标准库与现有 Go 依赖；可用 `GO_BIN` 指定 Go 程序路径。新的 handler 必须先在 `scripts/api-contract.py` 补齐经核实的响应类型，不能凭名称猜测。Go JSON 结构不代替服务端业务验证，也不代表数据库关联字段可由调用方修改。

`openapi.legacy.yaml.txt` 为历史原文，保留追溯用。它不再是受支持契约，不能作为 SDK 生成输入。
