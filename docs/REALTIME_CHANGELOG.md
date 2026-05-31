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
