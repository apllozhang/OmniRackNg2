# API 错误码定义清单

来自 handler / middleware / router 中 `response.Error` 调用的静态定义；不表示每个端点会返回每个错误。
动态错误信息标为“动态”；Gin Recovery 等框架异常和前置代理错误不保证返回业务信封。
以 HTTP 状态与 code 组合判断；requestId 用于关联日志，message 不适合作为程序分支条件。

| code | HTTP | 默认消息 | 定义位置 |
|---|---|---|---|
| `APPROVAL_STATE_CONFLICT` | 409 | 动态：来自业务错误 | `backend/internal/handler/resource_handler.go:413` |
| `AUTH_UNAVAILABLE` | 503 | 认证服务暂时不可用，请稍后重试 | `backend/internal/handler/auth_handler.go:71` |
| `AUTH_UNAVAILABLE` | 503 | 认证服务暂时不可用，请稍后重试 | `backend/internal/handler/auth_handler.go:91` |
| `AUTH_UNAVAILABLE` | 503 | 认证服务暂时不可用，请稍后重试 | `backend/internal/middleware/auth.go:93` |
| `DEVICE_NOT_POSITIONED` | 409 | 设备必须先上架后才能建立供电连接 | `backend/internal/handler/pdu_handler.go:30` |
| `DEVICE_NOT_POSITIONED` | 409 | 动态：来自业务错误 | `backend/internal/handler/resource_handler.go:409` |
| `DEVICE_POSITIONED` | 409 | 动态：来自业务错误 | `backend/internal/handler/resource_handler.go:407` |
| `DEVICE_TYPE_IN_USE` | 409 | 动态：来自业务错误 | `backend/internal/handler/resource_handler.go:405` |
| `FORBIDDEN` | 403 | 审计角色不能修改资源 | `backend/internal/handler/asset_operations_handler.go:55` |
| `FORBIDDEN` | 403 | 需要资产管理或审计权限 | `backend/internal/handler/asset_operations_handler.go:74` |
| `FORBIDDEN` | 403 | 当前用户没有访问权限 | `backend/internal/middleware/roles.go:17` |
| `FORBIDDEN` | 403 | 当前用户没有访问权限 | `backend/internal/middleware/roles.go:23` |
| `FORBIDDEN` | 403 | 当前用户没有访问权限 | `backend/internal/middleware/roles.go:33` |
| `INTERNAL_ERROR` | 500 | 系统内部错误 | `backend/internal/handler/pdu_handler.go:30` |
| `INTERNAL_ERROR` | 500 | 系统内部错误 | `backend/internal/handler/rack_template_handler.go:21` |
| `INTERNAL_ERROR` | 500 | 系统内部错误 | `backend/internal/handler/resource_handler.go:427` |
| `INVALID_CREDENTIALS` | 401 | 用户名或密码错误 | `backend/internal/handler/auth_handler.go:73` |
| `INVALID_DEVICE_TYPE_ID` | 400 | 设备类型 ID 格式无效 | `backend/internal/handler/device_handler.go:93` |
| `INVALID_LDAP_CONFIG` | 400 | 动态：来自业务错误 | `backend/internal/handler/admin_handler.go:301` |
| `INVALID_RACK_ID` | 400 | 机柜 ID 格式无效 | `backend/internal/handler/device_handler.go:101` |
| `INVALID_REQUEST` | 400 | enabled 参数格式无效 | `backend/internal/handler/admin_handler.go:31` |
| `INVALID_REQUEST` | 400 | 用户名和密码不能为空 | `backend/internal/handler/auth_handler.go:57` |
| `INVALID_REQUEST` | 400 | PDU 参数格式无效 | `backend/internal/handler/pdu_handler.go:18` |
| `INVALID_REQUEST` | 400 | PDU 参数格式无效 | `backend/internal/handler/pdu_handler.go:20` |
| `INVALID_REQUEST` | 400 | 插座参数格式无效 | `backend/internal/handler/pdu_handler.go:23` |
| `INVALID_REQUEST` | 400 | 插座参数格式无效 | `backend/internal/handler/pdu_handler.go:24` |
| `INVALID_REQUEST` | 400 | 供电连接参数格式无效 | `backend/internal/handler/pdu_handler.go:27` |
| `INVALID_REQUEST` | 400 | 连接版本不能为空 | `backend/internal/handler/pdu_handler.go:28` |
| `INVALID_REQUEST` | 400 | 请求参数格式无效 | `backend/internal/handler/resource_handler.go:380` |
| `INVALID_RESOURCE_ID` | 400 | 资源 ID 格式无效 | `backend/internal/handler/pdu_handler.go:29` |
| `INVALID_RESOURCE_ID` | 400 | 资源 ID 格式无效 | `backend/internal/handler/resource_handler.go:355` |
| `INVALID_RESOURCE` | 400 | page 不能超过 10000 | `backend/internal/handler/asset_operations_handler.go:565` |
| `INVALID_RESOURCE` | 400 | 动态：来自业务错误 | `backend/internal/handler/pdu_handler.go:30` |
| `INVALID_RESOURCE` | 400 | 动态：来自业务错误 | `backend/internal/handler/resource_handler.go:403` |
| `INVALID_USER` | 400 | 动态：来自业务错误 | `backend/internal/handler/admin_handler.go:297` |
| `INVALID_VERSION` | 400 | version 必须是大于 0 的整数 | `backend/internal/handler/resource_handler.go:365` |
| `LAST_ADMIN_PROTECTED` | 409 | 系统至少需要保留一个启用的管理员 | `backend/internal/handler/admin_handler.go:295` |
| `LDAP_AUTH_FAILED` | 400 | LDAP 服务账号或用户认证失败 | `backend/internal/handler/admin_handler.go:305` |
| `LDAP_UNAVAILABLE` | 400 | 动态：来自业务错误 | `backend/internal/handler/admin_handler.go:307` |
| `LDAP_VERSION_CONFLICT` | 409 | LDAP 配置已被其他管理员修改，请刷新后重试 | `backend/internal/handler/admin_handler.go:303` |
| `LOGIN_RATE_LIMITED` | 429 | 登录尝试过于频繁，请稍后重试 | `backend/internal/handler/auth_handler.go:83` |
| `LOGOUT_FAILED` | 503 | 注销失败，请重试 | `backend/internal/handler/auth_handler.go:102` |
| `NOTIFICATION_FAILED` | 502 | 动态：来自业务错误 | `backend/internal/handler/notification_handler.go:116` |
| `NOTIFICATION_FAILED` | 502 | 动态：来自业务错误 | `backend/internal/handler/notification_handler.go:75` |
| `NOT_FOUND` | 404 | 接口不存在 | `backend/internal/router/router.go:109` |
| `NOT_READY` | 503 | 数据库尚未就绪 | `backend/internal/handler/health_handler.go:26` |
| `PARENT_RESOURCE_DISABLED` | 409 | 动态：来自业务错误 | `backend/internal/handler/resource_handler.go:397` |
| `PDU_DEVICE_RACK_MISMATCH` | 409 | 设备必须已上架在当前 PDU 所属机柜 | `backend/internal/handler/pdu_handler.go:30` |
| `PDU_HAS_CHILDREN` | 409 | 请先删除插座和供电连接后再删除 PDU | `backend/internal/handler/pdu_handler.go:30` |
| `PDU_SOCKET_CONNECTED` | 409 | 插座已有供电连接 | `backend/internal/handler/pdu_handler.go:30` |
| `PDU_SOCKET_UNAVAILABLE` | 409 | 插座不可用 | `backend/internal/handler/pdu_handler.go:30` |
| `RACK_DIAGRAM_IMPORT_DECISION_REQUIRED` | 400 | 动态：来自业务错误 | `backend/internal/handler/resource_handler.go:419` |
| `RACK_DIAGRAM_IMPORT_DRAFT_EXPIRED` | 410 | 导入校验已过期，请重新选择文件校验 | `backend/internal/handler/resource_handler.go:415` |
| `RACK_DIAGRAM_IMPORT_STALE` | 409 | 校验后机柜或设备数据发生变化，请重新校验 | `backend/internal/handler/resource_handler.go:417` |
| `RACK_TEMPLATE_DISABLED` | 409 | 动态：来自业务错误 | `backend/internal/handler/resource_handler.go:395` |
| `RACK_UNAVAILABLE` | 409 | 动态：来自业务错误 | `backend/internal/handler/resource_handler.go:411` |
| `RACK_U_CONFLICT` | 409 | 动态：来自业务错误 | `backend/internal/handler/resource_handler.go:421` |
| `REQUEST_TOO_LARGE` | 413 | 登录请求过大 | `backend/internal/handler/auth_handler.go:54` |
| `RESOURCE_CODE_DUPLICATE` | 409 | PDU 编码已存在 | `backend/internal/handler/pdu_handler.go:30` |
| `RESOURCE_CODE_DUPLICATE` | 409 | 同级资源编码已存在 | `backend/internal/handler/resource_handler.go:389` |
| `RESOURCE_HAS_CHILDREN` | 409 | 动态：来自业务错误 | `backend/internal/handler/resource_handler.go:401` |
| `RESOURCE_LIMIT_EXCEEDED` | 409 | 动态：来自业务错误 | `backend/internal/handler/resource_handler.go:399` |
| `RESOURCE_NOT_FOUND` | 404 | 资源不存在或已删除 | `backend/internal/handler/pdu_handler.go:30` |
| `RESOURCE_NOT_FOUND` | 404 | 资源不存在或已删除 | `backend/internal/handler/resource_handler.go:387` |
| `RESOURCE_VERSION_CONFLICT` | 409 | 数据已被其他用户修改，请刷新后重试 | `backend/internal/handler/pdu_handler.go:30` |
| `RESOURCE_VERSION_CONFLICT` | 409 | 数据已被其他用户修改，请刷新后重试 | `backend/internal/handler/resource_handler.go:391` |
| `ROLE_NOT_FOUND` | 400 | 选择的角色不存在 | `backend/internal/handler/admin_handler.go:299` |
| `SERVICE_UNAVAILABLE` | 503 | 认证服务不可用 | `backend/internal/handler/auth_handler.go:42` |
| `SYSTEM_TEMPLATE_PROTECTED` | 409 | 动态：来自业务错误 | `backend/internal/handler/resource_handler.go:393` |
| `UNAUTHORIZED` | 401 | 登录身份无效 | `backend/internal/handler/asset_operations_handler.go:27` |
| `UNAUTHORIZED` | 401 | 账号已停用或身份无法验证 | `backend/internal/handler/asset_operations_handler.go:33` |
| `UNAUTHORIZED` | 401 | 用户身份无效 | `backend/internal/handler/auth_handler.go:94` |
| `UNAUTHORIZED` | 401 | 未登录或登录已过期 | `backend/internal/middleware/auth.go:62` |
| `UNAUTHORIZED` | 401 | 登录凭证无效 | `backend/internal/middleware/auth.go:73` |
| `UNAUTHORIZED` | 401 | 登录凭证无效 | `backend/internal/middleware/auth.go:79` |
| `UNAUTHORIZED` | 401 | 登录凭证无效 | `backend/internal/middleware/auth.go:86` |
| `UNAUTHORIZED` | 401 | 登录凭证已失效，请重新登录 | `backend/internal/middleware/auth.go:97` |
| `USER_CONFLICT` | 409 | 用户名已存在 | `backend/internal/handler/admin_handler.go:293` |
| `USER_DISABLED` | 403 | 用户已停用 | `backend/internal/handler/auth_handler.go:69` |
| `USER_NOT_FOUND` | 404 | 用户不存在 | `backend/internal/handler/admin_handler.go:291` |
