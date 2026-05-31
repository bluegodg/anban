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

### 23:50 子女端 web 骨架 RED 测试

- 文件：`web/package.json`
- 内容：新增子女端静态骨架的 Node 测试脚本入口。
- 目的：不引入前端构建依赖，先用 Node 自带测试保证最小页面和 API 封装存在。
- 功能影响：暂无运行时页面；这是 TDD RED 阶段。
- 文件：`web/smoke.test.mjs`
- 内容：新增 smoke test，验证页面包含访问码、设备 ID、状态、留言、问候、提醒、画像核心控件，并验证 API client 会带 `X-Access-Code` 调用 `POST /api/messages`。
- 目的：对齐三周计划任务 0.4 的“子女端 Web 骨架（连假数据）”和 PRD #3/#4 的最小控件需求。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期因 `web/index.html` 和 `web/api/client.js` 未实现而失败。
- 验证：已运行 `npm test --prefix web`，按预期失败。失败原因是 `web/index.html` 不存在，以及 `web/api/client.js` 模块不存在。

### 23:55 子女端 web 骨架 GREEN 实现

- 文件：`web/api/client.js`
- 内容：新增无框架 API client，统一设置 `X-Access-Code`，封装 `sendMessage`、`listMessages`、`triggerGreeting`。
- 目的：建立前端 `api/` 对接缝，先把已实现的 `/api/messages` 接成可调用接口。
- 功能：子女端可带访问码发送留言、拉取留言列表；问候按钮可调用现有占位路由并显示结果。
- 文件：`web/app.js`
- 内容：新增页面交互逻辑，保存访问码/设备 ID/后端地址，发送留言，刷新留言状态，处理问候占位、提醒草稿、画像草稿。
- 目的：覆盖三周计划任务 0.4 的最小交互控件，先让前端可在假数据/占位模式下跑起来。
- 功能：留言接真后端；状态、提醒、画像先作为骨架行为，不影响 xiaozhi 原版语音主链路。
- 文件：`web/index.html`
- 内容：新增子女端主界面，包含访问码、设备 ID、状态面板、留言表单、主动问候按钮、提醒表单、画像表单。
- 目的：给子女端提供第一屏可用操作面，而不是 landing page。
- 功能：手机/桌面浏览器可打开并进行基础操作。
- 文件：`web/styles.css`
- 内容：新增安伴子女端的响应式工作台样式，偏安静、实用、适合重复操作。
- 目的：让骨架具备路演早期可演示的视觉完整度，同时避免营销页式布局。
- 功能：桌面双列、移动端单列；控件尺寸稳定。
- 文件：`web/README.md`
- 内容：新增静态页面本地启动与测试说明。
- 目的：团队成员能快速打开子女端骨架。
- 功能影响：无运行时影响。
- 验证：已运行 `npm test --prefix web`，通过。

### 23:58 message handler 覆盖率补强

- 文件：`server/internal/domains/message/handler_test.go`
- 内容：新增 handler 测试，覆盖创建留言、查询留言、非法 JSON、缺少必填字段、xiaozhi 注入失败返回 502。
- 目的：补齐 message 包中此前未覆盖的 HTTP handler 行为，避免只测 service 导致接口层薄弱。
- 功能影响：无生产功能变化。
- 验证：已运行 `go test -count=1 -cover ./internal/domains/message`，通过，message 包覆盖率 92.1%。

### 00:05 总体验证与本地 web 服务

- 文件：无代码文件变化；本条记录验证结果。
- 内容：完成后端全量测试、构建、vet、message 覆盖率检查和 web smoke test。
- 目的：确认新增 message 域、childapi 路由注入、web 骨架没有破坏原有地基与 xiaozhi 适配器测试。
- 功能影响：无生产功能变化。
- 验证：
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `go test -count=1 -cover ./internal/domains/message` 通过，message 覆盖率 92.1%。
  - `npm test --prefix web` 通过。
  - 已启动静态 web 服务：`http://127.0.0.1:5173/`，本地 HTTP 检查返回 200。

## 2026-06-01

### 00:12 greeting 手动触发 RED 测试

- 文件：`server/internal/domains/greeting/service_test.go`
- 内容：新增 greeting 服务层测试，覆盖手动触发问候后调用 `InjectSpeak`、持久化 played 状态、默认 tonePreset、输入校验、注入失败记录 failed。
- 目的：按 PRD #2 “子女端按钮立即触发主动问候”的最小闭环先写测试，只做手动触发，不展开定时配置。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期测试失败。
- 文件：`server/internal/domains/greeting/handler_test.go`
- 内容：新增 HTTP handler 测试，覆盖 `POST /api/greetings/trigger` 的成功、坏请求、xiaozhi 注入失败返回 502。
- 目的：确保子女端问候按钮对应的后端入口行为明确。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期测试失败。
- 文件：`server/internal/childapi/greeting_routes_test.go`
- 内容：新增 childapi 路由装配测试，验证注入 greeting 路由后 `/api/greetings/trigger` 可替换 501 占位，同时未注入时保持占位。
- 目的：继续沿用北向边界注入模式，保持 childapi 不直接碰 xiaozhi/store。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期测试失败。
- 验证：已运行 `go test ./internal/domains/greeting ./internal/childapi`，按预期失败。失败原因是 `Service` / `NewStore` / `NewService` / `TriggerRequest` / `NewHandler` / `Greeting` / `StatusPlayed` 未实现，以及 `childapi.Deps.GreetingRoutes` 字段尚不存在。

### 00:18 greeting 手动触发 GREEN 实现

- 文件：`server/internal/domains/greeting/types.go`
- 内容：新增问候记录、触发请求、tonePreset、状态和查询过滤类型。
- 目的：承接 PRD #2 `POST /api/greetings/trigger` 的最小数据契约。
- 功能：记录每次问候触发的设备、口吻、文本、状态、触发时间和播报结果。
- 文件：`server/internal/domains/greeting/store.go`
- 内容：新增 greeting 域数据访问层，负责问候表迁移、创建、更新、按 deviceId/status 查询。
- 目的：保持域内 store 只管理本域表，沿用 message 域已验证的边界模式。
- 功能：问候触发记录可持久化，后续可给状态页或审计使用。
- 文件：`server/internal/domains/greeting/service.go`
- 内容：新增手动触发问候业务逻辑，校验 deviceId、规范 tonePreset、生成演示问候文本、通过 `xiaozhiclient.InjectSpeak` 播报并更新 played/failed。
- 目的：实现 PRD #2 的子女端按钮立即触发问候，不展开定时问候。
- 功能：子女端点击问候按钮后，设备可经 manager OpenAPI 主动开口。
- 文件：`server/internal/domains/greeting/handler.go`
- 内容：新增 Gin handler，注册 `POST /api/greetings/trigger`。
- 目的：把 childapi 的 greeting 占位替换为真业务入口。
- 功能：前端可调用问候触发接口，失败时返回 502 和 failed 记录。
- 文件：`server/internal/childapi/server.go`
- 内容：新增 `Deps.GreetingRoutes`，有 greeting 依赖时注册真路由，缺省时保留 501 占位。
- 目的：继续保持 childapi 只接入 domain handler，不直接触碰 xiaozhi 或数据库。
- 功能：问候路由可按域独立接入，未接入的提醒/画像/状态仍保持占位。
- 文件：`server/cmd/anban/main.go`
- 内容：启动时装配 greeting store/service/handler，并执行 greeting 表迁移。
- 目的：把 greeting 域接进现有地基，同时不改冻结的 xiaozhi 上游。
- 功能：服务启动后 `/api/greetings/trigger` 真可用。
- 验证：已运行 `go test -count=1 ./internal/domains/greeting ./internal/childapi`，通过。

### 00:24 web 问候响应消费 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增 smoke test，要求前端在触发问候成功后展示后端返回的 `greeting.text`。
- 目的：让子女端按钮真正消费 `/api/greetings/trigger` 的业务响应，而不是只显示固定成功文案。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 `web/app.js` 未展示响应文本而失败。
- 验证：已运行 `npm test --prefix web`，按预期失败。失败原因是 `web/app.js` 仍只显示固定的“问候已触发”，未使用 `greeting.text`。
