# 实际 HTTP API 清单

此表从 Gin 实际注册表生成，不依赖 PostgreSQL、账号或运行实例。
完整机器契约：`backend/api/openapi.yaml`（JSON 格式，兼容 YAML 1.2 / OpenAPI 3.0.3）。
复核：在仓库根目录执行 `python3 scripts/api-contract.py --check`；修改接口后运行不带 `--check` 的生成命令。

共 **122 个 method/path**；路径参数必须使用真实资源 UUID。所有成功信封 `code` 为 `SUCCESS`，创建使用 HTTP 201；
资产导出例外，返回原始 JSON 文件，并写入审计日志。权限策略详见 `docs/架构与集成规范.md`，不能将角色名称理解为资源范围隔离。

| 方法 | 路径 | Handler | 权限策略 | 请求体 | query | 源码位置 |
|---|---|---|---|---|---|---|
| GET | `/api/v1/admin/approval-policy` | `AdminHandler.GetPolicy` | `system_admin` | — | — | `backend/internal/handler/admin_handler.go:138` |
| PUT | `/api/v1/admin/approval-policy` | `AdminHandler.UpdatePolicy` | `system_admin` | JSON body | — | `backend/internal/handler/admin_handler.go:146` |
| GET | `/api/v1/admin/approvals` | `AdminHandler.ListApprovals` | `system_admin` | — | status | `backend/internal/handler/admin_handler.go:159` |
| POST | `/api/v1/admin/approvals/:id/approve` | `AdminHandler.Approve` | `system_admin` | JSON body | — | `backend/internal/handler/admin_handler.go:167` |
| POST | `/api/v1/admin/approvals/:id/reject` | `AdminHandler.Reject` | `system_admin` | JSON body | — | `backend/internal/handler/admin_handler.go:168` |
| GET | `/api/v1/admin/ldap` | `AdminHandler.GetLDAP` | `system_admin` | — | — | `backend/internal/handler/admin_handler.go:237` |
| PUT | `/api/v1/admin/ldap` | `AdminHandler.UpdateLDAP` | `system_admin` | JSON body | — | `backend/internal/handler/admin_handler.go:248` |
| POST | `/api/v1/admin/ldap/test` | `AdminHandler.TestLDAP` | `system_admin` | JSON body | — | `backend/internal/handler/admin_handler.go:269` |
| GET | `/api/v1/admin/roles` | `AdminHandler.ListRoles` | `system_admin` | — | — | `backend/internal/handler/admin_handler.go:43` |
| GET | `/api/v1/admin/users` | `AdminHandler.ListUsers` | `system_admin` | — | authSource, enabled, search | `backend/internal/handler/admin_handler.go:26` |
| POST | `/api/v1/admin/users` | `AdminHandler.CreateUser` | `system_admin` | JSON body | — | `backend/internal/handler/admin_handler.go:65` |
| DELETE | `/api/v1/admin/users/:id` | `AdminHandler.DeleteUser` | `system_admin` | — | version | `backend/internal/handler/admin_handler.go:122` |
| PUT | `/api/v1/admin/users/:id` | `AdminHandler.UpdateUser` | `system_admin` | JSON body | version | `backend/internal/handler/admin_handler.go:78` |
| POST | `/api/v1/admin/users/:id/reset-password` | `AdminHandler.ResetPassword` | `system_admin` | JSON body | version | `backend/internal/handler/admin_handler.go:99` |
| GET | `/api/v1/approvals` | `AdminHandler.ListApprovals` | `system_admin` | — | status | `backend/internal/handler/admin_handler.go:159` |
| POST | `/api/v1/approvals/:id/approve` | `AdminHandler.Approve` | `system_admin` | JSON body | — | `backend/internal/handler/admin_handler.go:167` |
| POST | `/api/v1/approvals/:id/reject` | `AdminHandler.Reject` | `system_admin` | JSON body | — | `backend/internal/handler/admin_handler.go:168` |
| GET | `/api/v1/asset-ops/access/settings` | `AssetOperationsHandler.ScanAccess` | `asset_role_read_or_manager_write` | — | — | `backend/internal/handler/scan_access_handler.go:9` |
| PUT | `/api/v1/asset-ops/access/settings` | `AssetOperationsHandler.SaveScanAccess` | `system_admin` | JSON body | — | `backend/internal/handler/scan_access_handler.go:17` |
| GET | `/api/v1/asset-ops/audit` | `AssetOperationsHandler.Audit` | `asset_role_read_or_manager_write` | — | page | `backend/internal/handler/asset_operations_handler.go:559` |
| GET | `/api/v1/asset-ops/checks` | `AssetOperationsHandler.Checks` | `asset_role_read_or_manager_write` | — | — | `backend/internal/handler/asset_operations_handler.go:291` |
| POST | `/api/v1/asset-ops/checks` | `AssetOperationsHandler.CreateCheck` | `asset_role_read_or_manager_write` | JSON body | — | `backend/internal/handler/asset_operations_handler.go:299` |
| POST | `/api/v1/asset-ops/checks/:id/close` | `AssetOperationsHandler.Close` | `asset_role_read_or_manager_write` | — | — | `backend/internal/handler/asset_operations_handler.go:362` |
| GET | `/api/v1/asset-ops/checks/:id/discoveries` | `AssetOperationsHandler.Discoveries` | `asset_role_read_or_manager_write` | — | — | `backend/internal/handler/asset_operations_handler.go:373` |
| POST | `/api/v1/asset-ops/checks/:id/discoveries` | `AssetOperationsHandler.Discover` | `asset_role_read_or_manager_write` | JSON body | — | `backend/internal/handler/asset_operations_handler.go:385` |
| POST | `/api/v1/asset-ops/checks/:id/discoveries/:discoveryId/resolve` | `AssetOperationsHandler.ResolveDiscovery` | `asset_role_read_or_manager_write` | JSON body | — | `backend/internal/handler/asset_operations_handler.go:416` |
| GET | `/api/v1/asset-ops/checks/:id/lines` | `AssetOperationsHandler.Lines` | `asset_role_read_or_manager_write` | — | — | `backend/internal/handler/asset_operations_handler.go:312` |
| POST | `/api/v1/asset-ops/checks/:id/lines/:lineId/resolve` | `AssetOperationsHandler.Resolve` | `asset_role_read_or_manager_write` | JSON body | — | `backend/internal/handler/asset_operations_handler.go:341` |
| POST | `/api/v1/asset-ops/checks/:id/scan` | `AssetOperationsHandler.Scan` | `asset_role_read_or_manager_write` | JSON body | — | `backend/internal/handler/asset_operations_handler.go:324` |
| GET | `/api/v1/asset-ops/devices` | `AssetOperationsHandler.Devices` | `asset_role_read_or_manager_write` | — | — | `backend/internal/handler/asset_operations_handler.go:104` |
| POST | `/api/v1/asset-ops/devices/:id/coverages` | `AssetOperationsHandler.Coverage` | `asset_role_read_or_manager_write` | JSON body | — | `backend/internal/handler/asset_operations_handler.go:129` |
| POST | `/api/v1/asset-ops/devices/:id/pc-handover` | `AssetOperationsHandler.ConfirmPCHandover` | `asset_role_read_or_manager_write` | JSON body | — | `backend/internal/handler/pc_handover.go:41` |
| GET | `/api/v1/asset-ops/devices/:id/pc-history` | `AssetOperationsHandler.PCHistory` | `asset_role_read_or_manager_write` | — | page | `backend/internal/handler/pc_handover.go:59` |
| PUT | `/api/v1/asset-ops/devices/:id/profile` | `AssetOperationsHandler.Profile` | `asset_role_read_or_manager_write` | JSON body | — | `backend/internal/handler/asset_operations_handler.go:112` |
| GET | `/api/v1/asset-ops/devices/:id/terminal-state` | `AssetOperationsHandler.TerminalState` | `asset_role_read_or_manager_write` | — | — | `backend/internal/handler/pc_handover.go:28` |
| POST | `/api/v1/asset-ops/export` | `AssetOperationsHandler.Export` | `asset_role_read_or_manager_write` | — | — | `backend/internal/handler/asset_operations_handler.go:531` |
| GET | `/api/v1/asset-ops/minimums` | `AssetOperationsHandler.Minimums` | `asset_role_read_or_manager_write` | — | — | `backend/internal/handler/asset_operations_handler.go:246` |
| POST | `/api/v1/asset-ops/minimums` | `AssetOperationsHandler.SaveMinimum` | `asset_role_read_or_manager_write` | JSON body | — | `backend/internal/handler/asset_operations_handler.go:254` |
| DELETE | `/api/v1/asset-ops/minimums/:id` | `AssetOperationsHandler.DeleteCatalog` | `asset_role_read_or_manager_write` | — | version | `backend/internal/handler/asset_catalog_handler.go:81` |
| PUT | `/api/v1/asset-ops/minimums/:id` | `AssetOperationsHandler.UpdateMinimum` | `asset_role_read_or_manager_write` | JSON body | — | `backend/internal/handler/asset_catalog_handler.go:61` |
| GET | `/api/v1/asset-ops/movements` | `AssetOperationsHandler.Movements` | `asset_role_read_or_manager_write` | — | stockId | `backend/internal/handler/asset_operations_handler.go:229` |
| GET | `/api/v1/asset-ops/notifications/approval-settings` | `NotificationHandler.ApprovalSettings` | `system_admin` | — | — | `backend/internal/handler/notification_handler.go:12` |
| PUT | `/api/v1/asset-ops/notifications/approval-settings` | `NotificationHandler.SaveApprovalSettings` | `system_admin` | JSON body | — | `backend/internal/handler/notification_handler.go:21` |
| POST | `/api/v1/asset-ops/notifications/approval-test` | `NotificationHandler.TestApproval` | `system_admin` | JSON body | — | `backend/internal/handler/notification_handler.go:102` |
| GET | `/api/v1/asset-ops/notifications/deliveries` | `NotificationHandler.Deliveries` | `system_admin` | — | page | `backend/internal/handler/notification_handler.go:80` |
| GET | `/api/v1/asset-ops/notifications/settings` | `NotificationHandler.Settings` | `system_admin` | — | — | `backend/internal/handler/notification_handler.go:39` |
| PUT | `/api/v1/asset-ops/notifications/settings` | `NotificationHandler.Save` | `system_admin` | JSON body | — | `backend/internal/handler/notification_handler.go:47` |
| POST | `/api/v1/asset-ops/notifications/test` | `NotificationHandler.Test` | `system_admin` | JSON body | — | `backend/internal/handler/notification_handler.go:60` |
| GET | `/api/v1/asset-ops/operators` | `AssetOperationsHandler.Operators` | `asset_role_read_or_manager_write` | — | — | `backend/internal/handler/asset_operations_handler.go:508` |
| GET | `/api/v1/asset-ops/reminders` | `AssetOperationsHandler.Reminders` | `asset_role_read_or_manager_write` | — | — | `backend/internal/handler/asset_operations_handler.go:457` |
| POST | `/api/v1/asset-ops/reminders/:id/acknowledge` | `AssetOperationsHandler.Acknowledge` | `asset_role_read_or_manager_write` | JSON body | — | `backend/internal/handler/asset_operations_handler.go:472` |
| POST | `/api/v1/asset-ops/reminders/refresh` | `AssetOperationsHandler.Refresh` | `asset_role_read_or_manager_write` | — | — | `backend/internal/handler/asset_operations_handler.go:465` |
| GET | `/api/v1/asset-ops/sites` | `AssetOperationsHandler.Sites` | `asset_role_read_or_manager_write` | — | — | `backend/internal/handler/asset_operations_handler.go:83` |
| POST | `/api/v1/asset-ops/sites` | `AssetOperationsHandler.CreateSite` | `asset_role_read_or_manager_write` | JSON body | — | `backend/internal/handler/asset_operations_handler.go:91` |
| DELETE | `/api/v1/asset-ops/sites/:id` | `AssetOperationsHandler.DeleteCatalog` | `asset_role_read_or_manager_write` | — | version | `backend/internal/handler/asset_catalog_handler.go:81` |
| PUT | `/api/v1/asset-ops/sites/:id` | `AssetOperationsHandler.UpdateSite` | `asset_role_read_or_manager_write` | JSON body | — | `backend/internal/handler/asset_catalog_handler.go:10` |
| GET | `/api/v1/asset-ops/specs` | `AssetOperationsHandler.Specs` | `asset_role_read_or_manager_write` | — | — | `backend/internal/handler/asset_operations_handler.go:147` |
| POST | `/api/v1/asset-ops/specs` | `AssetOperationsHandler.CreateSpec` | `asset_role_read_or_manager_write` | JSON body | — | `backend/internal/handler/asset_operations_handler.go:155` |
| DELETE | `/api/v1/asset-ops/specs/:id` | `AssetOperationsHandler.DeleteCatalog` | `asset_role_read_or_manager_write` | — | version | `backend/internal/handler/asset_catalog_handler.go:81` |
| PUT | `/api/v1/asset-ops/specs/:id` | `AssetOperationsHandler.UpdateSpec` | `asset_role_read_or_manager_write` | JSON body | — | `backend/internal/handler/asset_catalog_handler.go:27` |
| POST | `/api/v1/asset-ops/stock-receipts` | `AssetOperationsHandler.ReceiveSpare` | `asset_role_read_or_manager_write` | JSON body | — | `backend/internal/handler/asset_operations_handler.go:199` |
| GET | `/api/v1/asset-ops/stocks` | `AssetOperationsHandler.Stocks` | `asset_role_read_or_manager_write` | — | siteId | `backend/internal/handler/asset_operations_handler.go:168` |
| POST | `/api/v1/asset-ops/stocks` | `AssetOperationsHandler.CreateStock` | `asset_role_read_or_manager_write` | JSON body | — | `backend/internal/handler/asset_operations_handler.go:185` |
| DELETE | `/api/v1/asset-ops/stocks/:id` | `AssetOperationsHandler.DeleteCatalog` | `asset_role_read_or_manager_write` | — | version | `backend/internal/handler/asset_catalog_handler.go:81` |
| PUT | `/api/v1/asset-ops/stocks/:id` | `AssetOperationsHandler.UpdateStock` | `asset_role_read_or_manager_write` | JSON body | — | `backend/internal/handler/asset_catalog_handler.go:44` |
| POST | `/api/v1/asset-ops/stocks/:id/move` | `AssetOperationsHandler.MoveStock` | `asset_role_read_or_manager_write` | JSON body | — | `backend/internal/handler/asset_operations_handler.go:212` |
| POST | `/api/v1/asset-ops/terminal-devices` | `AssetOperationsHandler.RegisterTerminal` | `asset_role_read_or_manager_write` | JSON body | — | `backend/internal/handler/pc_handover.go:10` |
| POST | `/api/v1/auth/login` | `AuthHandler.Login` | `public` | JSON body | — | `backend/internal/handler/auth_handler.go:40` |
| POST | `/api/v1/auth/logout` | `AuthHandler.Logout` | `authenticated_legacy_policy` | — | — | `backend/internal/handler/auth_handler.go:100` |
| GET | `/api/v1/auth/me` | `AuthHandler.Me` | `authenticated_legacy_policy` | — | — | `backend/internal/handler/auth_handler.go:86` |
| POST | `/api/v1/data-centers` | `ResourceHandler.CreateDataCenter` | `authenticated_legacy_policy` | JSON body | — | `backend/internal/handler/resource_handler.go:125` |
| POST | `/api/v1/data-centers/:dataCenterId/copy` | `ResourceHandler.CopyDataCenter` | `authenticated_legacy_policy` | JSON body | — | `backend/internal/handler/resource_handler.go:281` |
| POST | `/api/v1/data-centers/:dataCenterId/rooms` | `ResourceHandler.CreateRoom` | `authenticated_legacy_policy` | JSON body | — | `backend/internal/handler/resource_handler.go:175` |
| DELETE | `/api/v1/data-centers/:id` | `ResourceHandler.DeleteDataCenter` | `authenticated_legacy_policy` | — | version | `backend/internal/handler/resource_handler.go:159` |
| PUT | `/api/v1/data-centers/:id` | `ResourceHandler.UpdateDataCenter` | `authenticated_legacy_policy` | JSON body | — | `backend/internal/handler/resource_handler.go:141` |
| GET | `/api/v1/device-types` | `DeviceHandler.ListDeviceTypes` | `authenticated_legacy_policy` | — | — | `backend/internal/handler/device_handler.go:25` |
| POST | `/api/v1/device-types` | `DeviceHandler.CreateDeviceType` | `authenticated_legacy_policy` | JSON body | — | `backend/internal/handler/device_handler.go:34` |
| DELETE | `/api/v1/device-types/:id` | `DeviceHandler.DeleteDeviceType` | `authenticated_legacy_policy` | — | version | `backend/internal/handler/device_handler.go:66` |
| PUT | `/api/v1/device-types/:id` | `DeviceHandler.UpdateDeviceType` | `authenticated_legacy_policy` | JSON body | — | `backend/internal/handler/device_handler.go:48` |
| GET | `/api/v1/devices` | `DeviceHandler.ListDevices` | `authenticated_legacy_policy` | — | lifecycleStatus, page, pageSize, rackId, scope, search, typeId | `backend/internal/handler/device_handler.go:82` |
| POST | `/api/v1/devices` | `DeviceHandler.CreateDevice` | `authenticated_legacy_policy` | JSON body | — | `backend/internal/handler/device_handler.go:132` |
| DELETE | `/api/v1/devices/:id` | `DeviceHandler.DeleteDevice` | `authenticated_legacy_policy` | — | version | `backend/internal/handler/device_handler.go:166` |
| GET | `/api/v1/devices/:id` | `DeviceHandler.GetDevice` | `authenticated_legacy_policy` | — | — | `backend/internal/handler/device_handler.go:114` |
| PUT | `/api/v1/devices/:id` | `DeviceHandler.UpdateDevice` | `authenticated_legacy_policy` | JSON body | — | `backend/internal/handler/device_handler.go:148` |
| POST | `/api/v1/devices/:id/assign` | `DeviceHandler.Assign` | `authenticated_legacy_policy` | JSON body | — | `backend/internal/handler/device_handler.go:182` |
| POST | `/api/v1/devices/:id/decommission` | `DeviceHandler.Decommission` | `authenticated_legacy_policy` | JSON body | — | `backend/internal/handler/device_handler.go:214` |
| GET | `/api/v1/devices/:id/history` | `DeviceHandler.History` | `authenticated_legacy_policy` | — | — | `backend/internal/handler/device_handler.go:232` |
| POST | `/api/v1/devices/:id/move` | `DeviceHandler.Move` | `authenticated_legacy_policy` | JSON body | — | `backend/internal/handler/device_handler.go:186` |
| DELETE | `/api/v1/pdu-connections/:id` | `PDUHandler.Disconnect` | `authenticated_legacy_policy` | JSON body | — | `backend/internal/handler/pdu_handler.go:28` |
| DELETE | `/api/v1/pdu-sockets/:id` | `PDUHandler.DeleteSocket` | `authenticated_legacy_policy` | — | version | `backend/internal/handler/pdu_handler.go:25` |
| PUT | `/api/v1/pdu-sockets/:id` | `PDUHandler.UpdateSocket` | `authenticated_legacy_policy` | JSON body | — | `backend/internal/handler/pdu_handler.go:24` |
| POST | `/api/v1/pdu-sockets/:id/connection` | `PDUHandler.Connect` | `authenticated_legacy_policy` | JSON body | — | `backend/internal/handler/pdu_handler.go:27` |
| DELETE | `/api/v1/pdus/:id` | `PDUHandler.Delete` | `authenticated_legacy_policy` | — | version | `backend/internal/handler/pdu_handler.go:21` |
| GET | `/api/v1/pdus/:id` | `PDUHandler.Get` | `authenticated_legacy_policy` | — | — | `backend/internal/handler/pdu_handler.go:19` |
| PUT | `/api/v1/pdus/:id` | `PDUHandler.Update` | `authenticated_legacy_policy` | JSON body | — | `backend/internal/handler/pdu_handler.go:20` |
| GET | `/api/v1/pdus/:id/sockets` | `PDUHandler.ListSockets` | `authenticated_legacy_policy` | — | — | `backend/internal/handler/pdu_handler.go:22` |
| POST | `/api/v1/pdus/:id/sockets` | `PDUHandler.CreateSocket` | `authenticated_legacy_policy` | JSON body | — | `backend/internal/handler/pdu_handler.go:23` |
| GET | `/api/v1/rack-templates` | `RackTemplateHandler.List` | `authenticated_legacy_policy` | — | — | `backend/internal/handler/rack_template_handler.go:18` |
| POST | `/api/v1/rack-templates` | `RackTemplateHandler.Create` | `authenticated_legacy_policy` | JSON body | — | `backend/internal/handler/rack_template_handler.go:27` |
| DELETE | `/api/v1/rack-templates/:id` | `RackTemplateHandler.Delete` | `authenticated_legacy_policy` | — | version | `backend/internal/handler/rack_template_handler.go:81` |
| PUT | `/api/v1/rack-templates/:id` | `RackTemplateHandler.Update` | `authenticated_legacy_policy` | JSON body | — | `backend/internal/handler/rack_template_handler.go:41` |
| POST | `/api/v1/rack-templates/:id/versions` | `RackTemplateHandler.CreateVersion` | `authenticated_legacy_policy` | JSON body | version | `backend/internal/handler/rack_template_handler.go:59` |
| DELETE | `/api/v1/racks/:id` | `ResourceHandler.DeleteRack` | `authenticated_legacy_policy` | — | version | `backend/internal/handler/resource_handler.go:265` |
| PUT | `/api/v1/racks/:id` | `ResourceHandler.UpdateRack` | `authenticated_legacy_policy` | JSON body | — | `backend/internal/handler/resource_handler.go:247` |
| POST | `/api/v1/racks/:rackId/copy` | `ResourceHandler.CopyRack` | `authenticated_legacy_policy` | JSON body | — | `backend/internal/handler/resource_handler.go:283` |
| POST | `/api/v1/racks/:rackId/move` | `ResourceHandler.MoveRack` | `authenticated_legacy_policy` | JSON body | — | `backend/internal/handler/resource_handler.go:313` |
| GET | `/api/v1/racks/:rackId/pdu-connections` | `PDUHandler.ListConnections` | `authenticated_legacy_policy` | — | — | `backend/internal/handler/pdu_handler.go:26` |
| GET | `/api/v1/racks/:rackId/pdus` | `PDUHandler.List` | `authenticated_legacy_policy` | — | — | `backend/internal/handler/pdu_handler.go:17` |
| POST | `/api/v1/racks/:rackId/pdus` | `PDUHandler.Create` | `authenticated_legacy_policy` | JSON body | — | `backend/internal/handler/pdu_handler.go:18` |
| GET | `/api/v1/racks/:rackId/u-layout` | `DeviceHandler.RackLayout` | `authenticated_legacy_policy` | — | — | `backend/internal/handler/device_handler.go:245` |
| GET | `/api/v1/resource-tree` | `ResourceHandler.Tree` | `authenticated_legacy_policy` | — | — | `backend/internal/handler/resource_handler.go:116` |
| DELETE | `/api/v1/rooms/:id` | `ResourceHandler.DeleteRoom` | `authenticated_legacy_policy` | — | version | `backend/internal/handler/resource_handler.go:211` |
| PUT | `/api/v1/rooms/:id` | `ResourceHandler.UpdateRoom` | `authenticated_legacy_policy` | JSON body | — | `backend/internal/handler/resource_handler.go:193` |
| POST | `/api/v1/rooms/:roomId/copy` | `ResourceHandler.CopyRoom` | `authenticated_legacy_policy` | JSON body | — | `backend/internal/handler/resource_handler.go:282` |
| POST | `/api/v1/rooms/:roomId/move` | `ResourceHandler.MoveRoom` | `authenticated_legacy_policy` | JSON body | — | `backend/internal/handler/resource_handler.go:312` |
| POST | `/api/v1/rooms/:roomId/rack-diagram-import/commit` | `DeviceHandler.CommitRackDiagramImport` | `authenticated_legacy_policy` | JSON body | — | `backend/internal/handler/device_handler.go:277` |
| POST | `/api/v1/rooms/:roomId/rack-diagram-import/validate` | `DeviceHandler.ValidateRackDiagramImport` | `authenticated_legacy_policy` | JSON body | — | `backend/internal/handler/device_handler.go:258` |
| POST | `/api/v1/rooms/:roomId/racks` | `ResourceHandler.CreateRack` | `authenticated_legacy_policy` | JSON body | — | `backend/internal/handler/resource_handler.go:227` |
| GET | `/api/v1/settings/operation-policy` | `AdminHandler.GetPolicy` | `system_admin` | — | — | `backend/internal/handler/admin_handler.go:138` |
| PUT | `/api/v1/settings/operation-policy` | `AdminHandler.UpdatePolicy` | `system_admin` | JSON body | — | `backend/internal/handler/admin_handler.go:146` |
| GET | `/health/live` | `HealthHandler.Live` | `public` | — | — | `backend/internal/handler/health_handler.go:17` |
| GET | `/health/ready` | `HealthHandler.Ready` | `public` | — | — | `backend/internal/handler/health_handler.go:21` |

生成脚本遇到未评审的新 handler 会失败，必须先补齐响应映射。`--check` 仅读取并比较，不改写契约文件。
