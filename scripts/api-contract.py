#!/usr/bin/env python3
"""Generate/validate the actual HTTP contract; no DB, worker, socket or credential.

Only the Python standard library and the backend's existing Go modules are needed.
JSON is a YAML 1.2 subset; openapi.yaml is emitted as JSON for deterministic parsing.
"""
import argparse
import copy
import json
import os
from pathlib import Path
import re
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]
SPEC = ROOT / "backend/api/openapi.yaml"
INVENTORY = ROOT / "docs/api-接口清单.md"
ERROR_CATALOG = ROOT / "docs/api-错误码清单.md"


def ref(name):
    return {"$ref": "#/components/schemas/" + name.replace(".", "_")}


def obj(**properties):
    return {"type": "object", "properties": properties}


def array(name):
    return {"type": "array", "items": ref(name)}


def listing(name, page_size=None, variable_size=False):
    result = obj(items=array(name))
    if page_size:
        result["properties"].update(total={"type": "integer"}, page={"type": "integer"},
                                    pageSize=({"type": "integer", "minimum": 1, "maximum": 100, "default": page_size} if variable_size else {"type": "integer", "enum": [page_size]}))
    return result


BOOL = {"type": "boolean"}
TEXT = {"type": "string"}
DELETED = obj(deleted=BOOL, id={"type": "string", "format": "uuid"})


def response_contracts():
    """Reviewed handler result types; fail closed for new/renamed handlers."""
    result = {}

    def put(owner, names, shape):
        for name in names.split():
            result[owner + "." + name] = shape

    put("HealthHandler", "Live Ready", obj(status=TEXT))
    put("AuthHandler", "Login", ref("service.LoginResult"))
    put("AuthHandler", "Me", ref("model.User"))
    put("AuthHandler", "Logout", obj(loggedOut=BOOL))
    put("ResourceHandler", "Tree", listing("model.DataCenter"))
    for model in ("DataCenter", "Room", "Rack"):
        put("ResourceHandler", "Create" + model + " Update" + model, ref("model." + model))
        put("ResourceHandler", "Delete" + model, DELETED)
        put("ResourceHandler", "Copy" + model, ref("service.ResourceCopyResult"))
    put("ResourceHandler", "MoveRoom", ref("model.Room"))
    put("ResourceHandler", "MoveRack", ref("model.Rack"))
    put("DeviceHandler", "ListDeviceTypes", listing("model.DeviceType"))
    put("DeviceHandler", "CreateDeviceType UpdateDeviceType", ref("model.DeviceType"))
    put("DeviceHandler", "DeleteDeviceType DeleteDevice", DELETED)
    put("DeviceHandler", "ListDevices", ref("service.DeviceListResult"))
    put("DeviceHandler", "GetDevice CreateDevice UpdateDevice", ref("model.Device"))
    put("DeviceHandler", "Assign Move Decommission", ref("service.OperationResult"))
    put("DeviceHandler", "History", listing("model.DevicePositionHistory"))
    put("DeviceHandler", "RackLayout", ref("service.RackULayout"))
    put("DeviceHandler", "ValidateRackDiagramImport", ref("service.RackDiagramImportValidation"))
    put("DeviceHandler", "CommitRackDiagramImport", ref("service.RackDiagramImportCommitResult"))
    put("RackTemplateHandler", "List", listing("model.RackTemplate"))
    put("RackTemplateHandler", "Create Update", ref("model.RackTemplate"))
    put("RackTemplateHandler", "CreateVersion", ref("model.RackTemplateVersion"))
    put("RackTemplateHandler", "Delete", DELETED)
    put("PDUHandler", "List", listing("model.PDU"))
    put("PDUHandler", "Get Create Update", ref("model.PDU"))
    put("PDUHandler", "ListSockets", listing("model.PDUSocket"))
    put("PDUHandler", "CreateSocket UpdateSocket", ref("model.PDUSocket"))
    put("PDUHandler", "ListConnections", listing("model.PDUConnection"))
    put("PDUHandler", "Connect", ref("model.PDUConnection"))
    put("PDUHandler", "Delete DeleteSocket Disconnect", DELETED)
    put("AdminHandler", "ListUsers", listing("model.User"))
    put("AdminHandler", "CreateUser UpdateUser ResetPassword", ref("model.User"))
    put("AdminHandler", "DeleteUser", DELETED)
    put("AdminHandler", "ListRoles", listing("model.Role"))
    put("AdminHandler", "GetPolicy UpdatePolicy", ref("model.OperationPolicy"))
    put("AdminHandler", "ListApprovals", listing("model.ApprovalRecord"))
    put("AdminHandler", "Approve Reject", ref("model.ApprovalRecord"))
    put("AdminHandler", "GetLDAP UpdateLDAP", ref("model.LDAPConfigView"))
    put("AdminHandler", "TestLDAP", obj(connected=BOOL, message=TEXT))
    for plural, singular in [("Sites", "AssetSite"), ("Specs", "SpareSpec"), ("Stocks", "SpareStock"),
                             ("Minimums", "SpareMinimum"), ("Checks", "AssetCheckTask"),
                             ("Lines", "AssetCheckLine"), ("Discoveries", "AssetCheckDiscovery"),
                             ("Movements", "SpareMovement"), ("Reminders", "AssetReminder")]:
        put("AssetOperationsHandler", plural, listing("model." + singular))
    for names, model in [("CreateSite UpdateSite", "AssetSite"), ("CreateSpec UpdateSpec", "SpareSpec"),
                         ("CreateStock UpdateStock ReceiveSpare", "SpareStock"),
                         ("SaveMinimum UpdateMinimum", "SpareMinimum"), ("CreateCheck", "AssetCheckTask"),
                         ("Profile", "AssetProfile"), ("Coverage", "AssetCoverage"),
                         ("MoveStock", "SpareMovement"), ("Resolve", "AssetCheckLine"),
                         ("Discover", "AssetCheckDiscovery"), ("ConfirmPCHandover", "PCHandover"),
                         ("ScanAccess SaveScanAccess", "ScanAccessSettings"), ("RegisterTerminal", "Device")]:
        put("AssetOperationsHandler", names, ref("model." + model))
    put("AssetOperationsHandler", "Devices", listing("service.AssetOverview"))
    put("AssetOperationsHandler", "TerminalState", ref("service.TerminalRegistration"))
    put("AssetOperationsHandler", "PCHistory", listing("model.PCHandover", 50))
    put("AssetOperationsHandler", "Audit", listing("model.AuditLog", 50, True))
    put("AssetOperationsHandler", "Operators", obj(items={"type": "array", "items": obj(
        id={"type": "string", "format": "uuid"}, name=TEXT, canCheck=BOOL)}))
    put("AssetOperationsHandler", "Scan", obj(line=ref("model.AssetCheckLine"), duplicate=BOOL))
    for name, field in [("Close", "closed"), ("ResolveDiscovery", "resolved"),
                        ("Refresh", "refreshed"), ("Acknowledge", "acknowledged")]:
        put("AssetOperationsHandler", name, obj(**{field: BOOL}))
    put("AssetOperationsHandler", "DeleteCatalog", DELETED)
    put("NotificationHandler", "Settings Save", ref("service.NotificationView"))
    put("NotificationHandler", "ApprovalSettings SaveApprovalSettings", ref("service.ApprovalEmailSettings"))
    put("NotificationHandler", "Test TestApproval", ref("model.NotificationDelivery"))
    put("NotificationHandler", "Deliveries", listing("model.NotificationDelivery", 20, True))
    export = obj(format={"type": "string", "enum": ["cabinet-asset-export-v1"]},
                 generatedAt={"type": "string", "format": "date-time"},
                 devices=array("service.AssetOverview"))
    for field, model in {"pcHandovers": "PCHandover", "sites": "AssetSite", "specs": "SpareSpec",
                         "stocks": "SpareStock", "movements": "SpareMovement", "minimums": "SpareMinimum",
                         "checks": "AssetCheckTask", "checkLines": "AssetCheckLine",
                         "discoveries": "AssetCheckDiscovery", "reminders": "AssetReminder"}.items():
        export["properties"][field] = array("model." + model)
    put("AssetOperationsHandler", "Export", export)
    return result


def handler_key(raw):
    found = re.search(r"\(\*(\w+)\)\.(\w+)", raw)
    if not found:
        raise ValueError("unknown Gin handler: " + raw)
    return ".".join(found.groups())


def handler_metadata(key, handlers, seen=None):
    seen = set() if seen is None else seen
    if key in seen:
        return {}
    seen.add(key)
    own = copy.deepcopy(handlers[key])
    for child in own.get("calls", []):
        if child not in handlers:
            continue
        inherited = handler_metadata(child, handlers, seen)
        for field in ("requestSchema", "query"):
            if field not in own and field in inherited:
                own[field] = inherited[field]
    return own


def envelope(data):
    return {"allOf": [ref("SuccessEnvelope"), obj(data=data)]}


def admin_only(path, method):
    return (path.startswith("/api/v1/admin/") or path.startswith("/api/v1/approvals") or
            path == "/api/v1/settings/operation-policy" or
            path.startswith("/api/v1/asset-ops/notifications/") or
            (method == "PUT" and path == "/api/v1/asset-ops/access/settings"))


def tag(path):
    if path.startswith("/health/"): return "Health"
    if "/auth/" in path: return "Authentication"
    if "/notifications/" in path: return "Notifications"
    if "/asset-ops/" in path: return "AssetOperations"
    if path.startswith("/api/v1/admin/") or "/approvals" in path or "/settings/" in path: return "Administration"
    if "pdu" in path: return "Power"
    if "rack-templates" in path: return "RackTemplates"
    if "device" in path or "u-layout" in path or "rack-diagram-import" in path: return "Devices"
    return "Resources"


def generate(source):
    response_shapes = response_contracts()
    schemas = source["schemas"]
    schemas.update({
        "SuccessEnvelope": {"type": "object", "required": ["code", "message"], "properties": {
            "code": {"type": "string", "enum": ["SUCCESS"]}, "message": TEXT,
            "requestId": TEXT}},
        "ErrorEnvelope": {"type": "object", "required": ["code", "message"], "properties": {
            "code": TEXT, "message": TEXT, "details": {}, "requestId": TEXT}},
    })
    paths = {}
    rows = []
    for route in source["routes"]:
        method, raw_path = route["method"], route["path"]
        path = re.sub(r":(\w+)", r"{\1}", raw_path)
        key = handler_key(route["handler"])
        if key not in response_shapes:
            raise ValueError("Review/add response mapping before publishing new API: " + key)
        meta = handler_metadata(key, source["handlers"])
        public = raw_path.startswith("/health/") or raw_path == "/api/v1/auth/login"
        admin = admin_only(raw_path, method)
        policy = "public" if public else ("system_admin" if admin else (
            "asset_role_read_or_manager_write" if "/asset-ops/" in raw_path else "authenticated_legacy_policy"))
        export = key == "AssetOperationsHandler.Export"
        created = (key.split(".")[1].startswith("Create") or key in {
            "PDUHandler.Connect", "ResourceHandler.CopyDataCenter", "ResourceHandler.CopyRoom",
            "ResourceHandler.CopyRack", "AssetOperationsHandler.Coverage", "AssetOperationsHandler.Discover",
            "AssetOperationsHandler.ConfirmPCHandover", "AssetOperationsHandler.RegisterTerminal"})
        status = "201" if created else "200"
        operation = {"tags": [tag(raw_path)], "summary": key,
            "operationId": method.lower() + "_" + re.sub(r"[^a-zA-Z0-9]+", "_", raw_path).strip("_"),
            "security": [] if public else [{"bearerAuth": []}],
            "x-handler": key, "x-source": {"path": meta["source"], "line": meta["line"]},
            "x-access-policy": policy, "x-contract-evidence": "Gin registry + Go AST + reviewed handler result types",
            "responses": {status: {"description": "成功；业务执行结果仍需读取 data.executed 等字段。" if key in {
                "DeviceHandler.Assign", "DeviceHandler.Move", "DeviceHandler.Decommission"} else "成功",
                "headers": {"X-Request-ID": {"$ref": "#/components/headers/RequestID"}},
                "content": {"application/json": {"schema": response_shapes[key] if export else envelope(response_shapes[key])}}},
                "default": {"$ref": "#/components/responses/Error"}}}
        if not public:
            operation["responses"]["401"] = {"$ref": "#/components/responses/Unauthorized"}
            operation["responses"]["403"] = {"$ref": "#/components/responses/Forbidden"}
            operation["responses"]["503"] = {"description": "认证存储暂不可用（AUTH_UNAVAILABLE）或服务暂不可用；保留凭证并按退避策略重试。",
                "headers": {"X-Request-ID": {"$ref": "#/components/headers/RequestID"}},
                "content": {"application/json": {"schema": ref("ErrorEnvelope")}}}
        if key == "HealthHandler.Ready":
            operation["responses"]["503"] = {"$ref": "#/components/responses/Error"}
        if key == "AuthHandler.Logout":
            operation["responses"]["503"] = {"$ref": "#/components/responses/Error"}
        if key == "AuthHandler.Login":
            operation["responses"].update({"400": {"$ref": "#/components/responses/Error"},
                "401": {"$ref": "#/components/responses/Unauthorized"}, "403": {"$ref": "#/components/responses/Forbidden"},
                "413": {"$ref": "#/components/responses/Error"}, "429": {"description": "登录频率受限",
                    "headers": {"Retry-After": {"description": "建议等待秒数", "schema": {"type": "string"}}},
                    "content": {"application/json": {"schema": ref("ErrorEnvelope")}}},
                "503": {"$ref": "#/components/responses/Error"}})
        parameters = [{"name": name, "in": "path", "required": True, "schema": {"type": "string", "format": "uuid"}}
                      for name in re.findall(r"{(\w+)}", path)]
        for name, shape in sorted(meta.get("query", {}).items()):
            shape = copy.deepcopy(shape)
            required = shape.pop("x-required", False)
            if name.endswith("Id"): shape = {"type": "string", "format": "uuid"}
            if name == "enabled": shape = {"type": "boolean"}
            parameter = {"name": name, "in": "query", "required": required, "schema": shape}
            if key == "DeviceHandler.ListDevices" and name == "pageSize":
                parameter["description"] = "默认 20；小于等于 0 回退为默认值，大于 100 按 100 处理。"
            if key == "DeviceHandler.ListDevices" and name == "scope":
                parameter["description"] = "空、PC 或 ROOM；业务筛选，不是权限范围。"
            parameters.append(parameter)
        if parameters: operation["parameters"] = parameters
        request = meta.get("requestSchema")
        if request:
            operation["requestBody"] = {"required": True, "description":
                "来自 ShouldBindJSON 的可解码字段。服务仍执行额外业务验证；模型中的关联对象、ID、时间等字段不代表可写权限。",
                "content": {"application/json": {"schema": request}}}
        notes = []
        if "/asset-ops/" in raw_path:
            notes.append("CurrentAccess 将请求体限制为 1 MiB；超限绑定错误通常返回 400，不保证 413。")
        if method not in ("GET", "HEAD"):
            notes.append("没有通用 Idempotency-Key 协议；超时后的写入结果不明确时先查询，不得盲目重试。")
        if meta.get("query", {}).get("version"):
            notes.append("version 位于 query，必须是大于 0 的整数。")
        if key in {"AssetOperationsHandler.MoveStock", "AssetOperationsHandler.ReceiveSpare"}:
            notes.append("submissionId 是此库存操作的专用去重键；不等价于所有接口支持幂等请求。")
        if export:
            notes.append("返回原始 JSON 文件，无 SUCCESS 信封；写入 EXPORT 审计记录，因此不是无副作用的纯读取。无跨表一致快照保证。")
            operation["responses"][status]["headers"]["Content-Disposition"] = {"schema": TEXT,
                "description": "attachment; filename=asset-operations-YYYYMMDD.json"}
        if key in {"DeviceHandler.ValidateRackDiagramImport", "DeviceHandler.CommitRackDiagramImport"}:
            notes.append("校验草稿保存在当前进程内存，30 分钟有效、单次提交，并绑定用户和机房；重启或跨副本请求会丢失草稿。")
        if key == "AuthHandler.Login":
            notes.append("登录请求体上限 16 KiB；当前进程限制每 IP 每分钟 120 次、每账号每 5 分钟 10 次，账号登录成功后清零。限制不跨进程共享。")
        if key == "AuthHandler.Logout":
            notes.append("注销递增用户 version，撤销该账号全部会话；客户端应清理令牌。修改用户或重置密码也会使旧令牌失效。")
        if raw_path.startswith("/api/v1/approvals") or raw_path == "/api/v1/settings/operation-policy":
            operation["deprecated"] = True
            notes.append("兼容别名；新集成使用 /api/v1/admin 下的对应路径，权限仍要求 system_admin。")
        if notes: operation["description"] = "\n\n".join(notes)
        paths.setdefault(path, {})[method.lower()] = operation
        rows.append((method, raw_path, key, policy, meta["source"], str(meta["line"]),
                     "JSON body" if request else "—", ", ".join(sorted(meta.get("query", {}))) or "—"))
    # Include only transitively referenced data contracts; unbound Go DTOs and
    # database/service structs are not externally supported API contracts.
    used = set()

    def collect(value):
        if isinstance(value, dict):
            if "$ref" in value and value["$ref"].startswith("#/components/schemas/"):
                name = value["$ref"].rsplit("/", 1)[-1]
                if name not in schemas: raise ValueError("unresolved schema: " + name)
                if name not in used:
                    used.add(name)
                    collect(schemas[name])
            for item in value.values(): collect(item)
        elif isinstance(value, list):
            for item in value: collect(item)

    collect(paths)
    collect(ref("ErrorEnvelope"))
    error = {"description": "失败；code 是稳定机器码，message 用于展示。具体业务失败由 handler/service 决定。",
             "headers": {"X-Request-ID": {"$ref": "#/components/headers/RequestID"}},
             "content": {"application/json": {"schema": ref("ErrorEnvelope")}}}
    spec = {"openapi": "3.0.3", "info": {"title": "OmniRack Replica — 实际 HTTP 契约",
        "version": "v1-source-contract-2026-10-28", "description":
        "由实际 Gin 路由注册表生成，含健康检查、认证、机柜/设备/供电、管理及资产运营全部接口。"
        "请求字段来自 Go JSON 绑定类型；响应数据类型经 handler 审核。不是完整的业务规则验证器，"
        "未把数据库约束当成自动参数校验。发布版本以 Git tag 与发布清单为准。"},
        "servers": [{"url": "/", "description": "当前 HTTP origin；路径已完整包含 /api/v1"}],
        "paths": paths, "components": {"securitySchemes": {"bearerAuth": {
            "type": "http", "scheme": "bearer", "bearerFormat": "JWT"}},
            "headers": {"RequestID": {"description": "请求关联 ID；响应信封也会携带 requestId。", "schema": TEXT}},
            "schemas": {name: schemas[name] for name in sorted(used)},
            "responses": {"Error": error, "Unauthorized": dict(error, description="凭证缺失、无效、过期或当前用户不可用。"),
                          "Forbidden": dict(error, description="凭证有效但服务端权限检查不允许此操作。")}},
        "x-generation": {"command": "python3 scripts/api-contract.py", "runtimeRouteCount": len(rows),
                         "checkCommand": "python3 scripts/api-contract.py --check",
                         "sideEffects": "无应用、实例或数据库操作；生成模式只写契约文件，check 模式不写契约文件"}}
    text = ["# 实际 HTTP API 清单", "", "此表从 Gin 实际注册表生成，不依赖 PostgreSQL、账号或运行实例。",
        "完整机器契约：`backend/api/openapi.yaml`（JSON 格式，兼容 YAML 1.2 / OpenAPI 3.0.3）。",
        "复核：在仓库根目录执行 `python3 scripts/api-contract.py --check`；修改接口后运行不带 `--check` 的生成命令。", "",
        f"共 **{len(rows)} 个 method/path**；路径参数必须使用真实资源 UUID。所有成功信封 `code` 为 `SUCCESS`，创建使用 HTTP 201；",
        "资产导出例外，返回原始 JSON 文件，并写入审计日志。权限策略详见 `docs/架构与集成规范.md`，不能将角色名称理解为资源范围隔离。", "",
        "| 方法 | 路径 | Handler | 权限策略 | 请求体 | query | 源码位置 |",
        "|---|---|---|---|---|---|---|"]
    text += [f"| {m} | `{p}` | `{h}` | `{a}` | {b} | {q} | `{s}:{line}` |" for m, p, h, a, s, line, b, q in rows]
    text += ["", "生成脚本遇到未评审的新 handler 会失败，必须先补齐响应映射。`--check` 仅读取并比较，不改写契约文件。", ""]
    errors = ["# API 错误码定义清单", "",
        "来自 handler / middleware / router 中 `response.Error` 调用的静态定义；不表示每个端点会返回每个错误。",
        "动态错误信息标为“动态”；Gin Recovery 等框架异常和前置代理错误不保证返回业务信封。",
        "以 HTTP 状态与 code 组合判断；requestId 用于关联日志，message 不适合作为程序分支条件。", "",
        "| code | HTTP | 默认消息 | 定义位置 |", "|---|---|---|---|"]
    for item in source["handlerErrorDefinitions"]:
        message = (item["message"] or "动态：来自业务错误").replace("|", "\\|").replace("\n", " ")
        errors.append(f"| `{item['code']}` | {item['status'] or '动态'} | {message} | `{item['source']}:{item['line']}` |")
    errors.append("")
    return (json.dumps(spec, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
            "\n".join(text), "\n".join(errors))


def validate(spec):
    operations = []
    for path, methods in spec["paths"].items():
        for method, op in methods.items():
            assert method in {"get", "post", "put", "delete", "patch", "head", "options"}
            assert op["responses"] and op.get("operationId")
            expected = set(re.findall(r"{(\w+)}", path))
            actual = {p["name"] for p in op.get("parameters", []) if p["in"] == "path" and p["required"]}
            assert expected == actual, (path, expected, actual)
            operations.append(op["operationId"])
    assert len(operations) == len(set(operations)), "duplicate operationId"
    assert len(operations) == spec["x-generation"]["runtimeRouteCount"]

    def check_ref(value):
        if isinstance(value, dict):
            if "$ref" in value:
                pointer = value["$ref"]
                assert pointer.startswith("#/"), pointer
                target = spec
                for part in pointer[2:].split("/"): target = target[part]
            for child in value.values(): check_ref(child)
        elif isinstance(value, list):
            for child in value: check_ref(child)
    check_ref(spec)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true", help="compare without writing")
    args = parser.parse_args()
    go = os.environ.get("GO_BIN", "go")
    completed = subprocess.run([go, "run", "./cmd/route-inventory", "--source-contract"],
                               cwd=ROOT / "backend", check=True, capture_output=True, text=True)
    source = json.loads(completed.stdout)
    expected_spec, expected_table, expected_errors = generate(source)
    validate(json.loads(expected_spec))
    changed = []
    for path, content in [(SPEC, expected_spec), (INVENTORY, expected_table), (ERROR_CATALOG, expected_errors)]:
        if args.check:
            if not path.exists() or path.read_text() != content: changed.append(str(path.relative_to(ROOT)))
        else:
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(content)
    if changed:
        print("API contract drift: " + ", ".join(changed), file=sys.stderr)
        print("Review the implementation and run python3 scripts/api-contract.py to regenerate.", file=sys.stderr)
        return 1
    mode = "checked" if args.check else "generated"
    print(f"API contract {mode}: {len(source['routes'])} actual routes, "
          f"{len(json.loads(expected_spec)['components']['schemas'])} referenced schemas; no application DB/network/listener.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
