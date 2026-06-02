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

### 00:27 web 问候响应消费 GREEN 实现

- 文件：`web/app.js`
- 内容：问候按钮触发成功后读取后端返回的 `greeting.text`，更新状态面板并显示“问候已触发：<问候文本>”。
- 目的：让子女端从真实 `/api/greetings/trigger` 响应中获得可见反馈，证明按钮不再只是占位交互。
- 功能：点击“触发问候”后，页面会展示设备实际要播报的问候文本。
- 验证：已运行 `npm test --prefix web`，通过。

### 00:32 greeting 切片总体验证

- 文件：无代码文件变化；本条记录验证结果。
- 内容：完成新增 greeting 手动触发切片后的全量后端测试、构建、vet、greeting 覆盖率检查、web smoke test 和静态页面访问检查。
- 目的：确认新增主动问候 API 与 web 反馈没有破坏 message 域、childapi、xiaozhiclient 原有适配器和现有地基。
- 功能影响：无生产功能变化。
- 验证：
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `go test -count=1 -cover ./internal/domains/greeting` 通过，greeting 包覆盖率 92.7%。
  - `npm test --prefix web` 通过。
  - `http://127.0.0.1:5173/` 本地 HTTP 检查返回 200。

### 00:42 reminder 创建与调度 RED 测试

- 文件：`server/internal/domains/reminder/service_test.go`
- 内容：新增 reminder 服务层测试，覆盖创建提醒后注册一次性调度、到点调用 `InjectSpeak`、更新 played 状态、输入校验、category 归一化、播报失败记录 failed。
- 目的：按 PRD #6 “子女端创建提醒 → 设备落到本地调度 → 到点主动播报”的最小闭环先写测试。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期测试失败。
- 文件：`server/internal/domains/reminder/handler_test.go`
- 内容：新增 HTTP handler 测试，覆盖 `POST /api/reminders` 创建提醒、`GET /api/reminders` 查询列表、非法 JSON/缺少字段返回 400。
- 目的：锁定子女端提醒表单要调用的后端接口行为。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期测试失败。
- 文件：`server/internal/childapi/reminder_routes_test.go`
- 内容：新增 childapi 路由装配测试，验证注入 reminder 路由后 `/api/reminders` 可替换 501 占位，同时未注入时保持占位。
- 目的：继续沿用北向边界注入模式，保持 childapi 不直接碰 xiaozhi/store/scheduler。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期测试失败。
- 验证：已运行 `go test ./internal/domains/reminder ./internal/childapi`，按预期失败。失败原因是 `Service` / `NewStore` / `NewService` / `CreateRequest` / `NewHandler` / `Reminder` / `StatusScheduled` 未实现，以及 `childapi.Deps.ReminderRoutes` 字段尚不存在。

### 00:50 reminder 创建与调度 GREEN 实现

- 文件：`server/internal/domains/reminder/types.go`
- 内容：新增提醒记录、创建请求、分类、状态和列表过滤类型。
- 目的：承接 PRD #6 `POST /api/reminders` / `GET /api/reminders` 的最小数据契约。
- 功能：记录提醒设备、计划时间、内容、分类、调度任务 ID、播报状态和播报时间。
- 文件：`server/internal/domains/reminder/store.go`
- 内容：新增 reminder 域数据访问层，负责提醒表迁移、创建、更新、按 deviceId/status 查询。
- 目的：保持域内 store 只管理本域表。
- 功能：提醒记录可持久化，后端重启后记录不丢。
- 文件：`server/internal/domains/reminder/service.go`
- 内容：新增提醒创建业务逻辑和一次性调度回调，到点通过 `xiaozhiclient.InjectSpeak` 播报并更新 played/failed。
- 目的：实现 PRD #6 的最小链路：子女端创建提醒 → 本地调度 → 到点主动播报。
- 功能：当前进程内提醒会到点播报；语音确认和 30 分钟无应答留给后续切片。
- 文件：`server/internal/domains/reminder/handler.go`
- 内容：新增 Gin handler，注册 `POST /api/reminders` 和 `GET /api/reminders`。
- 目的：把 childapi 的 reminder 占位替换为真业务入口。
- 功能：子女端可创建提醒、查询提醒列表。
- 文件：`server/internal/childapi/server.go`
- 内容：新增 `Deps.ReminderRoutes`，有 reminder 依赖时注册真路由，缺省时保留 501 占位。
- 目的：继续保持 childapi 只接入 domain handler，不直接触碰 xiaozhi/store/scheduler。
- 功能：提醒路由可按域独立接入，画像/状态仍保持占位。
- 文件：`server/cmd/anban/main.go`
- 内容：启动时装配 reminder store/service/handler，并执行 reminder 表迁移，把已有 scheduler 注入 reminder 服务。
- 目的：把 reminder 域接进现有地基，同时不改冻结的 xiaozhi 上游。
- 功能：服务启动后 `/api/reminders` 真可用。
- 验证：已运行 `go test -count=1 ./internal/domains/reminder ./internal/childapi`，通过。

### 00:56 web 提醒接口接入 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增 smoke test，要求 API client 提供 `createReminder` 并带访问码调用 `POST /api/reminders`；要求前端提交提醒后展示后端返回的 `reminder.content`。
- 目的：把子女端提醒表单从“草稿”推进到真实后端接口调用。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 `web/api/client.js` 和 `web/app.js` 未实现提醒接入而失败。
- 验证：已运行 `npm test --prefix web`，按预期失败。失败原因是 `client.createReminder` 不存在，以及 `web/app.js` 仍只显示“提醒草稿已记录”。

### 01:00 web 提醒接口接入 GREEN 实现

- 文件：`web/api/client.js`
- 内容：新增 `createReminder` 和 `listReminders`，分别封装 `POST /api/reminders` 与 `GET /api/reminders`。
- 目的：建立子女端提醒功能的前后端 API 对接缝。
- 功能：前端可带访问码创建提醒并查询提醒列表。
- 文件：`web/app.js`
- 内容：提醒表单改为调用 `client.createReminder`，把 `datetime-local` 转成 ISO 时间，成功后展示“提醒已创建：<内容>”并更新状态面板。
- 目的：让提醒从本地草稿变成真实后端调度任务。
- 功能：子女端提交提醒后，后端会落库并注册一次性定时播报。
- 文件：`web/index.html`
- 内容：提醒按钮文案从“保存提醒草稿”改为“创建提醒”。
- 目的：让界面文案与真实后端功能一致。
- 功能影响：仅文案变化。
- 验证：已运行 `npm test --prefix web`，通过。

### 01:06 reminder 切片总体验证

- 文件：无代码文件变化；本条记录验证结果。
- 内容：完成新增 reminder 创建/调度切片后的全量后端测试、构建、vet、reminder 覆盖率检查、web smoke test 和静态页面访问检查。
- 目的：确认新增主动提醒 API、一次性调度、web 提醒提交没有破坏 message/greeting、childapi、xiaozhiclient 原有适配器和现有地基。
- 功能影响：无生产功能变化。
- 验证：
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `go test -count=1 -cover ./internal/domains/reminder` 通过，reminder 包覆盖率 83.5%。
  - `npm test --prefix web` 通过。
  - `http://127.0.0.1:5173/` 本地 HTTP 检查返回 200。

### 09:00 status 设备状态 RED 测试

- 文件：`server/internal/xiaozhiclient/http_client_test.go`
- 内容：新增 `GetDeviceStatus` HTTP client 测试，要求读取 manager `GET /api/open/v1/devices/{deviceId}`，携带 `X-API-Token`，并解析 `online` 与 `last_active_at`。
- 目的：先锁住 status 域依赖的唯一 xiaozhi 读取入口，保持只有 `xiaozhiclient` 懂 manager API。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 `GetDeviceStatus` 仍返回未实现。
- 文件：`server/internal/domains/status/service_test.go`
- 内容：新增 status 服务测试，覆盖 deviceId 清洗、设备在线状态、最近可见时间和最近互动时间映射，以及缺少 deviceId 的输入校验。
- 目的：按 PRD #4 先实现“设备在线 + 最近互动”的最小状态快照，同时不跨域读取 message。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期 status 域尚未实现而失败。
- 文件：`server/internal/domains/status/handler_test.go`
- 内容：新增 `GET /api/status?deviceId=` handler 测试，覆盖成功返回状态快照和缺少 deviceId 返回 400。
- 目的：替换 childapi 中 status 占位路由的最小北向接口。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期 handler 尚未实现而失败。
- 文件：`server/internal/childapi/status_routes_test.go`
- 内容：新增 childapi 路由装配测试，验证注入 status 路由后 `/api/status` 可替换 501 占位，同时未注入时仍保持占位。
- 目的：延续域 handler 注入模式，保持 childapi 只做北向边界。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期 `Deps.StatusRoutes` 尚不存在而失败。
- 文件：`web/smoke.test.mjs`
- 内容：新增 web smoke test，要求 API client 提供 `getStatus` 并带访问码调用 `/api/status?deviceId=`，要求前端刷新后端状态再读取留言。
- 目的：让子女端顶部状态从本地文案接入真实后端状态 API。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期 `getStatus` 与状态渲染函数尚未实现而失败。
- 验证：已运行 `go test ./internal/xiaozhiclient ./internal/domains/status ./internal/childapi` 和 `npm test --prefix web`，按预期失败。失败原因是 `GetDeviceStatus` 返回 `anban: not implemented`、status 域和 `Deps.StatusRoutes` 尚未实现、web `client.getStatus` 和 `renderBackendStatus` 尚不存在。

### 09:04 status 设备状态 GREEN 实现

- 文件：`server/internal/xiaozhiclient/http_client.go`
- 内容：实现 `GetDeviceStatus`，调用 manager `GET /api/open/v1/devices/{deviceId}`，解析直接响应或 `{data: ...}` 包裹响应中的 `device_id`、`online`、`last_active_at` 等字段；当 manager 不显式返回 online 时，按 `status` 字段或 30 秒内 `last_active_at` 做轻量推断。
- 目的：补齐 status 域读取 xiaozhi 设备在线/最近活跃的唯一南向入口。
- 功能：安伴可通过冻结的 manager OpenAPI 读取设备在线态和最近活跃时间。
- 文件：`server/internal/domains/status/types.go`
- 内容：新增 `GetRequest`、`Snapshot` 和 `ErrInvalidInput`。
- 目的：定义 status 域最小北向响应契约。
- 功能：返回 `deviceId`、`online`、`lastSeenAt`、`lastInteractionAt`。
- 文件：`server/internal/domains/status/service.go`
- 内容：新增 status 服务，清洗 deviceId，调用 `xiaozhiclient.GetDeviceStatus`，把 `last_active_at` 映射成最近可见和最近互动时间。
- 目的：先完成 PRD #4 中“设备在线 + 最近互动”的独立快照，不跨域 import message。
- 功能：可在不读取安伴其他业务域的前提下得到设备状态。
- 文件：`server/internal/domains/status/handler.go`
- 内容：新增 Gin handler，注册 `GET /api/status?deviceId=`，缺少 deviceId 返回 400，xiaozhi 读取失败返回 502。
- 目的：把 childapi status 占位替换为真业务入口。
- 功能：子女端可查询设备在线状态。
- 文件：`server/internal/childapi/server.go`
- 内容：新增 `Deps.StatusRoutes`，有 status 依赖时注册真路由，缺省时保留 501 占位。
- 目的：沿用业务域 handler 注入模式，保持 childapi 不直接碰 xiaozhi。
- 功能：status 路由可按域独立接入。
- 文件：`server/cmd/anban/main.go`
- 内容：启动时装配 status service/handler，并注入 childapi。
- 目的：让服务启动后 `/api/status` 可用。
- 功能：子女端状态面板具备后端数据来源。
- 文件：`web/api/client.js`
- 内容：新增 `getStatus({deviceId})`，封装 `GET /api/status?deviceId=` 并携带访问码。
- 目的：建立子女端状态面板到后端 status API 的接缝。
- 功能：前端可读取设备状态。
- 文件：`web/app.js`
- 内容：连接后先调用 `client().getStatus`，再读取留言列表；新增 `renderBackendStatus` 和日期时间格式化。
- 目的：让顶部状态面板从静态/本地文案变成真实后端状态展示。
- 功能：展示在线/离线与最近互动时间；留言状态仍通过已有 `/api/messages` 列表显示。
- 验证：已运行 `go test ./internal/xiaozhiclient ./internal/domains/status ./internal/childapi` 和 `npm test --prefix web`，通过。

### 09:04 status 切片总体验证

- 文件：无代码文件变化；本条记录验证结果。
- 内容：完成新增 status 设备状态切片后的全量后端测试、构建、vet、status 覆盖率检查、web smoke test 和静态页面访问检查。
- 目的：确认 `xiaozhiclient.GetDeviceStatus`、status 域、childapi 路由注入和子女端状态面板接入没有破坏 message/greeting/reminder 等既有能力。
- 功能影响：无生产功能变化。
- 验证：
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `go test -count=1 -cover ./internal/domains/status` 通过，status 包覆盖率 80.0%。
  - `npm test --prefix web` 通过。
  - `http://127.0.0.1:5173/` 本地 HTTP 检查返回 200。

### 09:13 status 聚合留言状态 RED 测试

- 文件：`server/internal/domains/status/service_test.go`
- 内容：新增 status 服务测试，要求 `Snapshot` 包含最近留言状态摘要，并通过注入的 message reader 读取 `deviceId` 对应的 10 条状态。
- 目的：对齐 PRD #4 中 `messages: [{ messageId, status, queuedAt, playedAt? }]` 的状态聚合要求，同时保持 status 域不直接 import message 域。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期共享类型、message reader 注入和 `Snapshot.Messages` 尚不存在而失败。
- 文件：`server/internal/domains/status/handler_test.go`
- 内容：新增 handler 测试，要求同时支持完整 PRD 中的 `GET /api/device/status?deviceId=` 路由。
- 目的：兼容完整文档里的接口轮廓，同时保留已有 `/api/status` 骨架路由。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期 `/api/device/status` 尚未注册而失败。
- 文件：`server/internal/domains/message/service_test.go`
- 内容：新增 message 服务测试，要求输出最近留言状态摘要，按 deviceId 过滤并支持 limit。
- 目的：让 status 通过共享接口读取 message 状态，而不是跨域 import message。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期 `ListMessageStatusSummaries` 尚未实现而失败。
- 文件：`web/smoke.test.mjs`
- 内容：将 status API client 的 smoke 期望路径改为 `/api/device/status?deviceId=`。
- 目的：让子女端逐步贴近 PRD 接口轮廓。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 web client 仍调用 `/api/status` 而失败。
- 验证：已运行 `go test ./internal/domains/status ./internal/domains/message` 和 `npm test --prefix web`，按预期失败。失败原因是 `MessageStatusSummary`、`Snapshot.Messages`、status 的 message reader 注入、message 服务 `ListMessageStatusSummaries` 尚未实现，以及 web client 仍调用旧 `/api/status` 路径。

### 09:13 status 聚合留言状态 GREEN 实现

- 文件：`server/pkg/types/types.go`
- 内容：新增 `MessageStatusSummary` 和 `MessageStatusReader`。
- 目的：给 status 与 message 之间提供一个共享小接口，避免 status 域直接 import message 域。
- 功能：status 可读取留言状态摘要，同时继续遵守域间依赖纪律。
- 文件：`server/internal/domains/message/service.go`
- 内容：新增 `ListMessageStatusSummaries`，按 deviceId 查询留言、按已有列表顺序返回最近状态摘要，并支持 limit。
- 目的：让 message 域向 status 暴露最小必要的播放状态视图。
- 功能：输出 `{messageId,status,queuedAt,playedAt?}`。
- 文件：`server/internal/domains/status/types.go`
- 内容：`Snapshot` 新增 `messages` 字段。
- 目的：对齐 PRD #4 的 status 响应轮廓。
- 功能：状态接口可携带最近留言播放状态。
- 文件：`server/internal/domains/status/service.go`
- 内容：`NewService` 支持注入 `MessageStatusReader`，`Get` 默认读取 10 条最近留言状态并放入 `Snapshot.Messages`。
- 目的：实现“设备在线/最近互动/留言状态”的最小聚合。
- 功能：status 响应同时包含 xiaozhi 在线态和安伴本地留言状态。
- 文件：`server/internal/domains/status/handler.go`
- 内容：新增注册 `/api/device/status`，保留 `/api/status`。
- 目的：兼容完整 PRD 接口路径，同时不打断已有 web 骨架调用历史。
- 功能：两个路径均可读取同一 status 快照。
- 文件：`server/internal/childapi/server.go`
- 内容：缺省占位时也为 `/api/device/status` 返回 501。
- 目的：让未注入 status 域时的 PRD 路径也保持清晰占位语义。
- 功能：childapi 骨架路径更完整。
- 文件：`server/cmd/anban/main.go`
- 内容：装配 status 服务时注入 `messageService`。
- 目的：由启动层编排跨域协作，避免业务域互相 import。
- 功能：运行态 `/api/device/status` 会返回最近留言状态。
- 文件：`web/api/client.js`
- 内容：`getStatus` 改为调用 `/api/device/status`。
- 目的：子女端 API client 向完整 PRD 路径靠拢。
- 功能：前端状态刷新使用 PRD 路径。
- 验证：已运行 `go test ./internal/domains/status ./internal/domains/message` 和 `npm test --prefix web`，通过。

### 09:24 status 聚合留言状态总体验证

- 文件：无代码文件变化；本条记录验证结果。
- 内容：完成 status 聚合留言状态与 PRD 路径切片后的全量后端测试、构建、vet、相关包覆盖率检查、web smoke test 和静态页面访问检查。
- 目的：确认新增共享接口、status 聚合、message 状态摘要和 web PRD 路径切换没有破坏既有 message/greeting/reminder/xiaozhiclient 能力。
- 功能影响：无生产功能变化。
- 验证：
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `go test -count=1 -cover ./internal/domains/status ./internal/domains/message` 通过，status 包覆盖率 85.3%，message 包覆盖率 90.7%。
  - `npm test --prefix web` 通过。
  - `http://127.0.0.1:5173/` 本地 HTTP 检查返回 200。

### 09:27 profile 家庭画像 RED 测试

- 文件：`server/internal/xiaozhiclient/http_client_test.go`
- 内容：新增 `SetRolePrompt` HTTP client 测试，要求通过 manager OpenAPI 写入设备人设 prompt，并携带 `X-API-Token`。
- 目的：先锁住 profile 域同步 xiaozhi 的唯一南向入口，保持不直接改 xiaozhi。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 `SetRolePrompt` 仍返回未实现。
- 文件：`server/internal/domains/profile/service_test.go`
- 内容：新增 profile 服务测试，要求保存 8 个家庭画像字段，生成包含姓名/孙辈/喜好/健康背景的 prompt，并调用 `SetRolePrompt`。
- 目的：对齐 PRD #5 “家庭画像 ≥ 8 字段 + 注入 LLM 系统提示词”的最小闭环。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期 profile 域尚未实现而失败。
- 文件：`server/internal/domains/profile/handler_test.go`
- 内容：新增 `PUT /api/profile` 与 `GET /api/profile?deviceId=` 测试。
- 目的：锁定子女端画像编辑需要的北向接口。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期 profile handler 尚未实现而失败。
- 文件：`server/internal/childapi/profile_routes_test.go`
- 内容：新增 childapi 路由装配测试，验证注入 profile 路由后可替换 501 占位。
- 目的：沿用业务域 handler 注入模式，保持 childapi 不直接碰 xiaozhi/store。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期 `Deps.ProfileRoutes` 尚不存在而失败。
- 文件：`web/smoke.test.mjs`
- 内容：新增 web smoke test，要求 API client 提供 `updateProfile` 并带访问码调用 `PUT /api/profile`，要求前端提交画像时调用真实后端。
- 目的：把子女端家庭画像从本地草稿推进到后端持久化与 prompt 同步。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期 `updateProfile` 与真实提交逻辑尚未实现而失败。
- 验证：已运行 `go test ./internal/xiaozhiclient ./internal/domains/profile ./internal/childapi` 和 `npm test --prefix web`，按预期失败。失败原因是 `SetRolePrompt` 仍返回 `anban: not implemented`、profile 域服务/存储/handler 尚不存在、`Deps.ProfileRoutes` 尚未实现，以及 web `client.updateProfile` 与真实提交逻辑尚不存在。

### 09:38 profile 家庭画像 GREEN 实现

- 文件：`server/internal/xiaozhiclient/http_client.go`
- 内容：实现 `SetRolePrompt`，调用 manager `PUT /api/open/v1/devices/{deviceId}/role-prompt`，请求体为 `{prompt}`。
- 目的：补齐 profile 域把家庭画像同步为 xiaozhi 人设 prompt 的南向入口。
- 功能：画像更新后可通过 manager OpenAPI 写入设备人设。
- 文件：`server/internal/xiaozhiclient/fake.go`
- 内容：`FakeClient` 新增 `RolePromptCalls` 记录。
- 目的：让 profile 域能对着 fake client 并行开发和单测。
- 功能：测试可断言 prompt 同步内容。
- 文件：`server/internal/domains/profile/types.go`
- 内容：新增 `Profile`、`Fields`、`UpdateRequest`、错误类型，覆盖 name/nickname/children/grandchildren/hobbies/schedule/health/taboos 8 个字段。
- 目的：承接 PRD #5 家庭画像字段定义。
- 功能：画像数据具备持久化模型和 JSON 响应契约。
- 文件：`server/internal/domains/profile/store.go`
- 内容：新增 profile 域数据访问层，负责画像表迁移、按 deviceId upsert 和读取。
- 目的：保持画像编辑源在安伴自有 DB。
- 功能：后端重启后家庭画像不丢。
- 文件：`server/internal/domains/profile/service.go`
- 内容：新增画像更新/读取业务逻辑、字段清洗和 prompt 生成，更新时调用 `SetRolePrompt`。
- 目的：实现子女编辑画像 → 安伴存储 → xiaozhi 人设同步的最小闭环。
- 功能：生成包含老人姓名、称呼、亲属、喜好、作息、健康背景、忌口的 system prompt。
- 文件：`server/internal/domains/profile/handler.go`
- 内容：新增 Gin handler，注册 `GET /api/profile`、`PUT /api/profile`、`POST /api/profile`。
- 目的：把 childapi 的 profile 占位替换为真业务入口。
- 功能：子女端可保存和读取家庭画像。
- 文件：`server/internal/childapi/server.go`
- 内容：新增 `Deps.ProfileRoutes`，有 profile 依赖时注册真路由，缺省时保留 501 占位。
- 目的：继续保持 childapi 只接入 domain handler。
- 功能：profile 路由可按域独立接入。
- 文件：`server/cmd/anban/main.go`
- 内容：启动时迁移并装配 profile store/service/handler，注入 childapi。
- 目的：让服务启动后 `/api/profile` 真可用。
- 功能：画像更新可落库并同步 prompt。
- 文件：`web/api/client.js`
- 内容：新增 `getProfile` 和 `updateProfile`。
- 目的：建立子女端画像表单与后端 profile API 的接缝。
- 功能：前端可带访问码读取/更新画像。
- 文件：`web/index.html`
- 内容：家庭画像表单扩展为 8 个 PRD 字段，并把按钮文案改为“同步画像”。
- 目的：让子女端骨架覆盖 PRD #5 的必填演示字段。
- 功能：页面可编辑姓名、称呼、子女、孙辈、喜好、作息、健康背景、忌口。
- 文件：`web/app.js`
- 内容：画像表单提交改为调用 `client().updateProfile`，新增 profile 渲染与列表字段解析。
- 目的：把本地画像草稿推进到真实后端持久化与 prompt 同步。
- 功能：提交成功后显示“画像已同步”并刷新摘要。
- 文件：`server/internal/xiaozhiclient/http_client_test.go`
- 内容：补充 `SetRolePrompt` 非 2xx 错误测试，以及尚未实现的 `GetHistory`/`CallDeviceMCPTool` 返回错误测试。
- 目的：覆盖新增 xiaozhi client 分支，明确后续 status 深度/vision 仍未实现。
- 功能影响：无生产功能变化。
- 文件：`server/internal/domains/profile/service_test.go`
- 内容：补充 profile 读取缺少 deviceId、prompt 同步失败但画像已持久化的测试。
- 目的：提高 profile 域错误分支覆盖率，并锁定同步失败时不丢画像草稿的行为。
- 功能影响：无生产功能变化。
- 文件：`server/internal/domains/profile/handler_test.go`
- 内容：补充画像不存在返回 404、prompt 同步失败返回 502 且带画像 payload 的测试。
- 目的：提高 profile HTTP 分支覆盖率，让前端能区分未建档与同步失败。
- 功能影响：无生产功能变化。
- 文件：`server/internal/xiaozhiclient/fake_test.go`
- 内容：新增 `FakeClient` 调用记录与默认返回测试，覆盖 `InjectSpeak`、`SetRolePrompt`、`GetDeviceStatus`、`GetHistory`、`CallDeviceMCPTool`。
- 目的：提高 xiaozhi 适配器测试覆盖率，并锁定各域并行开发依赖的 fake 行为。
- 功能影响：无生产功能变化。
- 文件：`server/internal/xiaozhiclient/http_client_test.go`
- 内容：补充 `GetDeviceStatus` 直接响应体解析、`status:"active"` 推断在线、非法时间返回错误的测试。
- 目的：覆盖 manager 设备状态响应的兼容解析分支。
- 功能影响：无生产功能变化。
- 验证：已运行 `go test ./internal/xiaozhiclient ./internal/domains/profile ./internal/childapi` 和 `npm test --prefix web`，通过。

### 09:54 profile 家庭画像总体验证

- 文件：无代码文件变化；本条记录验证结果。
- 内容：完成 profile 家庭画像与 prompt 同步切片后的全量后端测试、构建、vet、相关包覆盖率检查、web smoke test 和静态页面访问检查。
- 目的：确认新增 profile 域、`SetRolePrompt`、childapi 路由注入、子女端画像提交没有破坏 message/greeting/reminder/status/xiaozhi 适配器既有能力。
- 功能影响：无生产功能变化。
- 验证：
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `go test -count=1 -cover ./internal/domains/profile ./internal/xiaozhiclient` 通过，profile 包覆盖率 85.4%，xiaozhiclient 包覆盖率 86.1%。
  - `npm test --prefix web` 通过。
  - `http://127.0.0.1:5173/` 本地 HTTP 检查返回 200。

### 14:08 reminder 重启恢复 RED 测试

- 文件：`server/internal/domains/reminder/service_test.go`
- 内容：新增 reminder 服务测试，模拟 DB 中已有 `scheduled` 提醒但进程重启后内存定时器为空，要求 `RestoreScheduled` 只恢复 pending 提醒、跳过已 played 提醒、刷新 jobId，并能重新触发播报。
- 目的：对齐 PRD #6 “后端重启后已排入提醒不丢”的验收缺口。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 reminder 服务尚无 `RestoreScheduled` 而失败。
- 验证：已运行 `go test ./internal/domains/reminder`，按预期失败。失败原因是 `svc.RestoreScheduled` 尚未实现。

### 14:14 reminder 重启恢复 GREEN 实现

- 文件：`server/internal/domains/reminder/service.go`
- 内容：新增 `RestoreScheduled`，从 DB 读取 `scheduled` 提醒，逐条重新注册 `ScheduleAt`，刷新 `jobId`；调度注册失败时把提醒标记为 `failed`。
- 目的：让后端进程重启后重新恢复内存中的一次性提醒定时器。
- 功能：已排入但未播报的提醒在启动恢复后仍会到点播报；过期的 scheduled 提醒会由 scheduler 立即补触发。
- 文件：`server/cmd/anban/main.go`
- 内容：启动装配 reminder 后调用 `RestoreScheduled(context.Background())`，恢复成功时记录恢复数量，失败时终止启动。
- 目的：把 reminder 恢复逻辑接入真实服务启动流程。
- 功能：服务重启后会自动恢复 DB 中的 scheduled 提醒。
- 验证：已运行 `go test ./internal/domains/reminder`，通过。

### 14:14 reminder 重启恢复总体验证

- 文件：无代码文件变化；本条记录验证结果。
- 内容：完成 reminder 重启恢复切片后的全量后端测试、构建、vet、reminder 覆盖率检查、web smoke test 和临时静态页面访问检查。
- 目的：确认 `RestoreScheduled` 和启动恢复装配没有破坏 message/greeting/profile/status/xiaozhiclient 既有能力。
- 功能影响：无生产功能变化。
- 验证：
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `go test -count=1 -cover ./internal/domains/reminder` 通过，reminder 包覆盖率 80.4%。
  - `npm test --prefix web` 通过。
  - 临时 Python 静态服务访问 `http://127.0.0.1:5176/` 返回 200，随后已停止该临时 job。

### 14:16 reminder 撤销 RED 测试

- 文件：`server/internal/domains/reminder/service_test.go`
- 内容：新增 reminder 服务测试，要求 `Cancel` 能取消 scheduled 提醒、调用 scheduler cancel、清空 jobId，并把状态持久化为 `canceled`。
- 目的：对齐 PRD #6 `DELETE /api/reminders/:id` 撤销接口，避免已撤销提醒仍然播报。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 reminder 服务尚无取消能力而失败。
- 文件：`server/internal/domains/reminder/handler_test.go`
- 内容：扩展 HTTP handler 测试，要求 `DELETE /api/reminders/:id` 返回 canceled 提醒，并校验非法 id 返回 400。
- 目的：锁定子女端撤销提醒的北向接口。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 handler 尚未注册 DELETE 路由而失败。
- 文件：`web/smoke.test.mjs`
- 内容：新增 web smoke test，要求 API client 提供 `deleteReminder` 并带访问码调用 `DELETE /api/reminders/:id`。
- 目的：为子女端接入提醒撤销能力建立 API 缝。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 web client 尚未实现 `deleteReminder` 而失败。
- 验证：已运行 `go test ./internal/domains/reminder` 和 `npm test --prefix web`，按预期失败。失败原因是 `StatusCanceled`、`Service.Cancel` 和 web `client.deleteReminder` 尚未实现。

### 14:24 reminder 撤销 GREEN 实现

- 文件：`server/internal/domains/reminder/types.go`
- 内容：新增 `StatusCanceled` 和 `ErrNotFound`。
- 目的：用持久状态表达“已撤销”，避免直接删除记录导致子女端看不到历史。
- 功能：提醒可进入 `canceled` 状态并支持列表筛选。
- 文件：`server/internal/domains/reminder/store.go`
- 内容：新增 `Get(ctx,id)`，按主键读取提醒，找不到返回 `ErrNotFound`。
- 目的：支持撤销单条提醒。
- 功能：service 可按 ID 获取待撤销提醒。
- 文件：`server/internal/domains/reminder/service.go`
- 内容：`OneShotScheduler` 增加 `Cancel`，新增 `Cancel(ctx,id)`；仅 scheduled 提醒会取消内存 job、清空 jobId 并标记 canceled，其他终态保持幂等返回；定时回调播报前会重新读取状态，已撤销提醒不会误播。
- 目的：实现 PRD #6 的撤销语义，防止已撤销提醒到点播报。
- 功能：撤销后 DB 和内存定时器同步更新。
- 文件：`server/internal/domains/reminder/handler.go`
- 内容：新增 `DELETE /api/reminders/:id`，非法 id 返回 400，找不到返回 404。
- 目的：提供子女端撤销提醒的北向接口。
- 功能：HTTP API 可撤销提醒。
- 文件：`web/api/client.js`
- 内容：新增 `deleteReminder(reminderId)`。
- 目的：为子女端后续 UI 接入撤销提醒提供 API client 能力。
- 功能：前端可带访问码调用 `DELETE /api/reminders/:id`。

### 14:26 reminder 撤销总体验证

- 文件：无代码文件变化；本条记录验证结果。
- 内容：完成 reminder 撤销切片后的局部测试、全量后端测试、构建、vet、reminder 覆盖率检查、web smoke test 和临时静态页面访问检查。
- 目的：确认 `DELETE /api/reminders/:id`、scheduler cancel、canceled 状态持久化和 web API client 不破坏 message/greeting/profile/status/xiaozhiclient 既有能力。
- 功能影响：无生产功能变化。
- 验证：
  - `go test ./internal/domains/reminder` 通过。
  - `npm test --prefix web` 通过。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `go test -count=1 -cover ./internal/domains/reminder` 通过，reminder 包覆盖率 76.5%。
  - 临时 Node 静态服务访问 `http://127.0.0.1:5177/` 返回 200，随后已停止该临时进程；检查时使用 `-NoProxy` 避免本机代理影响 localhost。

## 2026-06-02

### 12:35 greeting 定时配置 RED 测试

- 文件：`server/internal/domains/greeting/service_test.go`
- 内容：新增 greeting 服务层 RED 测试，要求 `GetSchedule` 在设备未配置时返回早/午/晚 3 个默认问候时段，`UpdateSchedule` 可保存 3 个 slot 并校验 deviceId、slot、`HH:MM` 时间格式。
- 目的：对齐完整 PRD #2 的“每天预设 3 个时间段，可在子女端配置”，先锁定持久配置能力。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 schedule 类型和服务方法尚不存在。
- 文件：`server/internal/domains/greeting/handler_test.go`
- 内容：新增 `GET /api/greetings/schedule?deviceId=` 与 `PUT /api/greetings/schedule` 的 HTTP RED 测试，并覆盖坏请求返回 400。
- 目的：锁定子女端配置问候时段的北向接口轮廓。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 handler 尚未注册 schedule 路由。
- 文件：`server/internal/childapi/greeting_routes_test.go`
- 内容：扩展 greeting 路由装配测试，要求注入依赖时 schedule 路由可替换占位，未注入时 schedule 路由返回 501。
- 目的：保持 childapi 只接入 domain handler，同时让 PRD 路径在缺省状态下也有明确占位。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前缺省 childapi 没有 schedule 占位。
- 文件：`web/smoke.test.mjs`
- 内容：新增 web RED 测试，要求 API client 提供 `getGreetingSchedule` / `updateGreetingSchedule`，并要求页面具备 `greetingScheduleForm` 且提交时调用真实后端。
- 目的：让子女端主动问候从“只能点一次按钮”推进到“可配置早/午/晚问候时段”的基础骨架。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 web client 和页面尚未实现。
- 验证：
  - 已运行 `go test ./internal/domains/greeting ./internal/childapi`，按预期失败。失败原因是 `GreetingSchedule` / `ScheduleRequest` / `ScheduleSlot` / `Service.GetSchedule` / `Service.UpdateSchedule` 尚未实现，且 childapi 缺省 `GET /api/greetings/schedule` 仍为 404 而非 501。
  - 已运行 `npm test --prefix web`，按预期失败。失败原因是 `client.updateGreetingSchedule` / `client.getGreetingSchedule` 不存在，页面也缺少 `greetingScheduleForm`。

### 12:42 greeting 定时配置 GREEN 实现

- 文件：`server/internal/domains/greeting/types.go`
- 内容：新增 `ScheduleSlot`、`GreetingSchedule`、`ScheduleRequest` 和 `ErrNotFound`。
- 目的：承接 PRD #2 `GET/PUT /api/greetings/schedule` 的设备问候时段数据契约。
- 功能：一个设备可拥有早/午/晚等多个问候 slot，每个 slot 记录标签、`HH:MM` 时间、启用状态和口吻。
- 文件：`server/internal/domains/greeting/store.go`
- 内容：`AutoMigrate` 增加 `GreetingSchedule` 表，新增 `UpsertSchedule` 和 `GetSchedule`。
- 目的：把问候时段作为 greeting 域自有数据持久化，不放进 xiaozhi。
- 功能：后端重启后子女端配置的问候时段不丢。
- 文件：`server/internal/domains/greeting/service.go`
- 内容：新增 `GetSchedule` / `UpdateSchedule`，未配置设备返回默认早/午/晚 3 时段；保存时校验 deviceId、slot 列表和 `HH:MM` 时间，并规范口吻默认值。
- 目的：先实现可配置时段的基础层，自动到点触发留给后续 scheduler 切片。
- 功能：子女端可读取默认问候计划并保存自定义问候计划。
- 文件：`server/internal/domains/greeting/handler.go`
- 内容：注册 `GET /api/greetings/schedule` 和 `PUT /api/greetings/schedule`，坏请求返回 400。
- 目的：补齐 PRD #2 的北向接口轮廓。
- 功能：HTTP API 可读写一台设备的问候时段配置。
- 文件：`server/internal/childapi/server.go`
- 内容：未注入 greeting 域时新增 schedule GET/PUT 的 501 占位。
- 目的：保持 childapi 路由形状完整，且仍只接入 domain handler。
- 功能：前端可对着稳定 URL 开发；缺省状态不会落到 404。
- 文件：`web/api/client.js`
- 内容：新增 `getGreetingSchedule` 和 `updateGreetingSchedule`。
- 目的：建立子女端问候时段配置与后端 API 的调用缝。
- 功能：前端可带 `X-Access-Code` 读取/保存问候时段。
- 文件：`web/index.html`
- 内容：主动陪伴面板新增 `greetingScheduleForm`，包含早/午/晚三个时间、启用开关和口吻选择。
- 目的：让子女端不只支持手动问候，也能配置 PRD 要求的 3 个预设时间段。
- 功能：路演操作者可在同一页面保存问候时段。
- 文件：`web/app.js`
- 内容：连接后读取问候时段并渲染表单，提交表单调用 `client().updateGreetingSchedule`，成功提示“问候时段已保存”。
- 目的：把页面控件接到真实后端，而不是本地草稿。
- 功能：问候时段保存后可立即看到表单按后端返回值刷新。
- 文件：`web/styles.css`
- 内容：新增 schedule 表单、时段行、启用 checkbox 和 select 的样式。
- 目的：让新增控件在桌面和手机宽度下稳定排列，不挤压已有提醒表单。
- 功能：主动陪伴面板可同时容纳问候配置和提醒创建。
- 验证：已运行 `go test ./internal/domains/greeting ./internal/childapi` 和 `npm test --prefix web`，通过。

### 12:43 greeting 定时配置总体验证

- 文件：无代码文件变化；本条记录验证结果。
- 内容：完成 greeting 定时配置切片后的全量后端测试、构建、vet、greeting 覆盖率检查、web smoke test 和临时静态页面访问检查。
- 目的：确认新增问候时段持久化、GET/PUT API、childapi 占位和子女端表单没有破坏 message/reminder/profile/status/xiaozhiclient 既有能力。
- 功能影响：无生产功能变化。
- 验证：
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `go test -count=1 -cover ./internal/domains/greeting` 通过，greeting 包覆盖率 86.2%。
  - `npm test --prefix web` 通过。
  - 临时 Node 静态服务访问 `http://127.0.0.1:5178/` 返回 200，随后已停止该临时进程；检查时使用 `-NoProxy` 避免本机代理影响 localhost。

### 12:47 greeting 定时触发 RED 测试

- 文件：`server/internal/domains/greeting/service_test.go`
- 内容：新增 greeting 服务层 RED 测试，要求保存问候 schedule 时只为 enabled slot 注册 cron，disabled slot 跳过；cron 触发时仍通过 `InjectSpeak` 主动播报；再次更新 schedule 时取消旧 cron；`RestoreSchedules` 能在启动时恢复 DB 中已保存的 enabled slot。
- 目的：对齐完整 PRD #2 “定时问候到点触发误差 ≤ 30 秒”和“三个预设时段可配置”，把上一切片的持久配置推进到真实调度。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 greeting 服务尚未接入 scheduler。
- 文件：`server/internal/scheduler/scheduler_test.go`
- 内容：新增 cron job 取消 RED 测试，要求 `Scheduler.Cancel` 可移除 `RegisterCron` 创建的 `cron-*` job。
- 目的：让 greeting schedule 更新时能撤掉旧定时任务，避免重复问候。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 `Cancel` 只取消一次性 timer，不取消 cron。
- 验证：已运行 `go test ./internal/domains/greeting ./internal/scheduler`，按预期失败。失败原因是 `NewService` 尚不接受 scheduler、`Service.RestoreSchedules` 尚不存在，且 `Scheduler.Cancel` 对 `cron-*` job 不生效。

### 12:51 greeting 定时触发 GREEN 实现

- 文件：`server/internal/scheduler/scheduler.go`
- 内容：扩展 `Cancel`，当 jobId 为 `cron-*` 时解析 cron entry id 并调用 `cron.Remove`；一次性 `once-*` timer 取消逻辑保持不变。
- 目的：支持业务域更新计划时撤销旧 cron 任务，避免重复主动问候。
- 功能：scheduler 同时可取消一次性提醒和 cron 问候任务。
- 文件：`server/internal/domains/greeting/store.go`
- 内容：新增 `ListSchedules(ctx)`，按设备列出已保存的问候时段配置。
- 目的：服务启动时可恢复 DB 中已有的 schedule。
- 功能：问候时段配置可在进程重启后重新注册到 scheduler。
- 文件：`server/internal/domains/greeting/service.go`
- 内容：`NewService` 支持可选 `CronScheduler`；`UpdateSchedule` 保存后注册 enabled slot；新增 `RestoreSchedules` 恢复所有已保存 schedule；新增 cron spec 生成、旧 job 取消和定时触发逻辑。
- 目的：实现 PRD #2 的“定时问候到点触发”，并保持小智边界不变。
- 功能：enabled 早/午/晚 slot 会被注册为每日 cron，到点复用 `Trigger`，最终仍通过 `xiaozhiclient.InjectSpeak` 主动播报；disabled slot 不注册；再次保存会撤销旧 cron。
- 文件：`server/cmd/anban/main.go`
- 内容：提前创建并启动共享 scheduler，将其注入 greeting 服务，启动时调用 `greetingService.RestoreSchedules(context.Background())`。
- 目的：让真实服务进程启动后恢复已保存的问候定时任务。
- 功能：后端重启后，子女端保存过的问候时段会恢复为可触发 cron。
- 验证：已运行 `go test ./internal/domains/greeting ./internal/scheduler`，通过。

### 12:52 greeting 定时触发总体验证

- 文件：无代码文件变化；本条记录验证结果。
- 内容：完成 greeting 定时触发切片后的全量后端测试、构建、vet、greeting/scheduler 覆盖率检查和 web smoke test。
- 目的：确认新增 cron 取消、问候时段注册、启动恢复和定时触发没有破坏 message/reminder/profile/status/xiaozhiclient 既有能力，也不影响子女端页面 API 骨架。
- 功能影响：无生产功能变化。
- 验证：
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `go test -count=1 -cover ./internal/domains/greeting ./internal/scheduler` 通过，greeting 包覆盖率 88.6%，scheduler 包覆盖率 86.3%。
  - `npm test --prefix web` 通过。

### 16:32 web 提醒列表撤销 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增 smoke test，要求 API client 可带访问码筛选查询 `GET /api/reminders`，并要求子女端页面具备 `reminderList`、连接后调用 `client().listReminders` 渲染列表、点击撤销时调用 `client().deleteReminder` 并显示“提醒已撤销”。
- 目的：对齐完整 PRD #6 `GET /api/reminders?deviceId=&status=` 列表和 `DELETE /api/reminders/:id` 撤销接口，把已实现的后端/API client 能力显露到子女端页面。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前页面只有创建提醒，没有提醒列表和撤销交互。
- 验证：已运行 `npm test --prefix web`，按预期失败。失败原因是 `web/index.html` 缺少 `reminderList`，页面还没有提醒列表与撤销交互。

### 16:34 web 提醒列表撤销 GREEN 实现

- 文件：`web/index.html`
- 内容：在主动陪伴面板的提醒表单下新增 `reminderList` 列表容器。
- 目的：承接 PRD #6 的提醒列表/撤销入口，让子女端能看到后端已排入的提醒。
- 功能：页面有稳定位置展示提醒记录。
- 文件：`web/app.js`
- 内容：新增 `state.reminders`、`refreshReminders`、`renderReminders` 和提醒列表点击撤销逻辑；连接后读取后端提醒列表，创建提醒后立即插入列表，点击“撤销”调用 `client().deleteReminder` 并显示“提醒已撤销”。
- 目的：把后端已有 `GET /api/reminders` 和 `DELETE /api/reminders/:id` 能力接到子女端页面。
- 功能：子女端可查看提醒状态并撤销 scheduled 提醒，撤销后页面状态更新为“已撤销”。
- 文件：`web/styles.css`
- 内容：复用留言列表样式并新增提醒列表、撤销按钮和移动端单列布局。
- 目的：让提醒列表在桌面/手机都保持可扫读、按钮不挤压内容。
- 功能：提醒内容、状态、时间和撤销按钮稳定排列。
- 验证：
  - `npm test --prefix web` 通过。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `go test -count=1 -cover ./internal/domains/reminder` 通过，reminder 包覆盖率 76.5%。
  - 临时 Node 静态服务访问 `http://127.0.0.1:5181/` 返回 200，随后已停止该临时 job；检查时使用 `-NoProxy` 避免本机代理影响 localhost。

### 16:53 reminder 确认/未应答状态 RED 测试

- 文件：`server/internal/domains/reminder/service_test.go`
- 内容：新增 RED 测试，要求提醒播报成功后注册 30 分钟确认超时 job，超时触发时状态转为 `unanswered`；老人语音确认对应的 `Acknowledge` 会把 `played` 提醒转为 `completed`、记录 `ackKind`/确认时间并取消超时 job。
- 目的：对齐完整 PRD #6 “老人语音回好/知道了/收到 → 已完成；30 分钟无应答 → 未应答且子女端可见”，先把 reminder 域状态机补齐。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 reminder 只有 scheduled/played/failed/canceled，没有确认和未应答状态。
- 文件：`web/smoke.test.mjs`
- 内容：新增 RED 测试，要求子女端能把 `played` 显示为“已播报”、`completed` 显示为“已完成”、`unanswered` 显示为“未应答”。
- 目的：确保后端状态扩展后，子女端列表能表达 PRD #6 的完成/未应答结果。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前页面仍把 `played` 当成“已完成”，且没有 completed/unanswered 文案。
- 验证：
  - 已运行 `go test ./internal/domains/reminder`，按预期失败。失败原因是 `AckJobID` 字段、`StatusUnanswered`、`StatusCompleted`、`AckKindTimeout`、`AckKindVoice`、`AckRequest` 和 `Service.Acknowledge` 尚未实现。
  - 已运行 `npm test --prefix web`，按预期失败。失败原因是前端 `reminderStatusLabel` 尚未包含 `completed`/`unanswered` 文案，且 `played` 仍被当成“已完成”。
