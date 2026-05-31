# 实时修改记录

> 目的：记录本轮代码编写中每一批改动的文件、内容、目的、功能和验证方式。后续每次代码改动都要同步更新本文件。

## 2026-05-31

### 23:40 初始化实时修改记录

- 文件：`docs/REALTIME_CHANGELOG.md`
- 内容：新增实时修改记录文档。
- 目的：满足“每次更改代码时实时更新修改文档”的协作要求。
- 功能影响：无运行时影响。
- 验证：文档文件已创建。

### 23:42 后端 message 域 RED 测试

- 文件：`server/internal/domains/message/service_test.go`
- 内容：新增 message 服务层测试，覆盖发送留言后调用 `InjectSpeak`、持久化已播报状态、输入校验、100 字截断、注入失败时落库为 failed。
- 目的：按 PRD #3 “子女端留言 → 设备播报”的最小闭环先写测试，锁定业务行为。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期测试失败。
- 文件：`server/internal/childapi/message_routes_test.go`
- 内容：新增 childapi 路由装配测试，验证注入 message 路由后 `/api/messages` 可替换 501 占位，同时未注入时保持占位。
- 目的：保证北向边界仍只通过可注入路由进入业务域，不直接碰 xiaozhi/store。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期测试失败。
- 验证：已运行 `go test ./internal/domains/message ./internal/childapi`，按预期失败。失败原因是 `Service` / `NewStore` / `NewService` / `SendRequest` / `StatusPlayed` / `ListFilter` / `ErrInvalidInput` 未实现，以及 `childapi.Deps.MessageRoutes` 字段尚不存在。

### 23:45 后端 message 域 GREEN 实现

- 文件：`server/internal/domains/message/types.go`
- 内容：新增留言状态、请求、过滤器和 GORM/JSON 共用的 `Message` 数据结构，限制单条留言最长 100 个 rune。
- 目的：承接 PRD #3 的 `messageId/status/queuedAt/playedAt` 数据契约。
- 功能：支持 pending/played/failed 三种状态，为后续状态页聚合留下字段。
- 文件：`server/internal/domains/message/store.go`
- 内容：新增 message 域数据访问层，负责本域表迁移、创建、更新、按 deviceId/status 查询。
- 目的：遵守“域内 store 只管本域表，共享 store 只管 DB 连接”的边界。
- 功能：留言可持久化，后端重启后消息记录不丢。
- 文件：`server/internal/domains/message/service.go`
- 内容：新增发送留言业务逻辑，校验 deviceId/text、截断 100 字、持久化、通过 `xiaozhiclient.InjectSpeak` 主动播报、根据结果更新 played/failed。
- 目的：实现 PRD #3 的最小端到端链路：子女发文字，设备通过 xiaozhi manager 注入消息播报。
- 功能：播报成功时记录 played 和 playedAt；播报失败时记录 failed，不阻塞后续留言。
- 文件：`server/internal/domains/message/handler.go`
- 内容：新增 Gin handler，注册 `POST /api/messages` 与 `GET /api/messages`。
- 目的：把 childapi 的 message 占位替换为真业务入口。
- 功能：子女端可创建留言、按设备或状态查询留言列表。
- 文件：`server/internal/childapi/server.go`
- 内容：新增 `RouteRegistrar` 接口和 `Deps.MessageRoutes`，有 message 依赖时注册真路由，缺省时保留 501 占位。
- 目的：保持 childapi 只接入 domain handler，不直接触碰 xiaozhi 或数据库。
- 功能：后续其他域可沿用同一注入方式逐步替换占位。
- 文件：`server/internal/childapi/message_routes_test.go`
- 内容：清理 message 路由 stub 测试里的无意义 context 占位代码。
- 目的：保持测试意图清晰，只验证路由注入行为。
- 功能：无生产功能影响。
- 文件：`server/cmd/anban/main.go`
- 内容：启动时装配 message store/service/handler，并执行 message 表迁移。
- 目的：把 message 域接进现有地基，同时不改冻结的 xiaozhi 上游。
- 功能：服务启动后 `/api/messages` 真可用，其它域仍保持占位。
- 验证：已运行 `go test -count=1 ./internal/domains/message ./internal/childapi`，通过。
