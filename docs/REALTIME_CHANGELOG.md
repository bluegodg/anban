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

### 12:21 vision PresenceState 触发问候 RED 测试

- 文件：`server/internal/domains/vision/service_test.go`
- 内容：新增 vision presence 状态机 RED 测试，要求 `ObservePresence` 修剪 `deviceId`，记录 `someone/no_one` 状态，启动时 `unknown -> someone` 不触发，`someone -> no_one -> someone` 仅触发 1 次主动问候，并拒绝坏 presence。
- 文件：`server/internal/domains/vision/handler_test.go`
- 内容：新增 `POST /api/vision/presence` handler RED 测试，覆盖连续上报 `no_one`、`someone` 后触发问候，以及坏 JSON、缺少设备、非法 presence。
- 文件：`server/internal/childapi/vision_routes_test.go`
- 内容：新增 childapi RED 断言，要求注入 vision 依赖时 `/api/vision/presence` 可注册为真路由，未注入时返回 501 占位。
- 目的：对齐完整 PRD #7 “有人 → 无人 → 有人状态变化触发 1 次问候”，先建立 VLM 结果进入后端后的 PresenceState 最小闭环；VLM 云调用和周期采帧留到后续切片。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 `PresenceObservationRequest`、`ObservePresence`、`ProactiveGreetingResult`、`TriggerProactiveGreeting` 和 `/vision/presence` 路由尚未实现。
- 验证：已运行 `go test ./internal/domains/vision ./internal/childapi`，得到有效 RED。失败原因是 vision 包缺少 `ProactiveGreetingResult`、`ObservePresence`、`PresenceObservationRequest`、`PresenceSomeone` 等类型/方法，且 childapi 未注入 vision 依赖时 `/api/vision/presence` 当前返回 404 而非 501。

### 12:32 vision PresenceState 触发问候 GREEN 实现

- 文件：`server/pkg/types/types.go`
- 内容：新增 `ProactiveGreetingResult` 和 `ProactiveGreetingTrigger`，让 vision 能通过跨域接口触发问候，而不直接 import greeting 域。
- 文件：`server/internal/domains/greeting/service.go`
- 内容：新增 `TriggerProactiveGreeting`，复用现有 `Trigger`，使用 casual tone 生成“回来啦”风格问候，并返回跨域摘要。
- 文件：`server/internal/domains/vision/types.go`、`server/internal/domains/vision/service.go`
- 内容：新增 `Presence`、`PresenceObservationRequest`、`PresenceObservationResult` 与内存 PresenceState；`unknown -> someone` 不触发，`no_one -> someone` 调用 `ProactiveGreetingTrigger`，重复 `someone` 不重复问候。
- 文件：`server/internal/domains/vision/handler.go`
- 内容：新增 `POST /api/vision/presence`，接收 VLM/演示脚本给出的 `someone/no_one` 粗粒度结果并返回是否触发问候。
- 文件：`server/internal/childapi/server.go`
- 内容：vision 依赖未注入时为 `/api/vision/presence` 返回 501 占位，保持子女端/演示脚本可预期的 API 形状。
- 文件：`server/cmd/anban/main.go`
- 内容：启动时把 `greetingService` 作为跨域问候触发器注入 `visionService`，仍保持 domains 之间不互相 import。
- 目的：落实完整 PRD #7 的后端内部状态机最小闭环，为后续“采帧 → VLM 判定 → ObservePresence”打基础。
- 功能：设备或演示脚本连续上报 `no_one` 后再上报 `someone` 时，vision 会触发一句 greeting 域问候；主动语音 10 分钟共享频控仍由 greeting 内的 gate 负责。
- 验证：
  - `go test ./internal/domains/vision ./internal/childapi` 通过。
  - `go test -count=1 -cover ./internal/domains/vision ./internal/childapi` 通过，覆盖率分别为 vision 94.2%、childapi 97.5%。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `npm test --prefix web` 通过，25 个测试全绿。

### 12:45 vision 采帧结果桥接 PresenceState RED 测试

- 文件：`server/internal/domains/vision/service_test.go`
- 内容：新增 `CaptureAndObservePresence` RED 测试，要求调用 MCP 拍照后从 raw JSON 的 `presence` 字段读取 `no_one/someone`，喂给 PresenceState，并在 `no_one -> someone` 时触发问候；缺少 presence、非法 presence 或坏 JSON 返回 `ErrPresenceUnavailable`。
- 文件：`server/internal/domains/vision/handler_test.go`
- 内容：新增 `POST /api/vision/check-presence` handler RED 测试，覆盖采帧 + presence 观察 + 触发问候，以及缺少 presence 时返回 502。
- 文件：`server/internal/childapi/vision_routes_test.go`
- 内容：新增 childapi RED 断言，要求注入 vision 依赖时 `/api/vision/check-presence` 可注册为真路由，未注入时返回 501 占位。
- 目的：对齐完整 PRD #7 “摄像头采帧 + VLM 判定 + 触发问候”的中间桥接；当前先接受 MCP/FakeClient raw JSON 中的 presence 结果，后续真实 VLM 只需产出同一 presence 语义。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 `CaptureAndObservePresence`、`PresenceCheckResult`、`ErrPresenceUnavailable` 和 `/vision/check-presence` 路由尚未实现。
- 验证：已运行 `go test ./internal/domains/vision ./internal/childapi`，得到有效 RED。失败原因是 `PresenceCheckResult`、`CaptureAndObservePresence`、`ErrPresenceUnavailable` 尚未实现，且 childapi 未注入 vision 依赖时 `/api/vision/check-presence` 当前返回 404 而非 501。

### 12:56 vision 采帧结果桥接 PresenceState GREEN 实现

- 文件：`server/internal/domains/vision/types.go`
- 内容：新增 `ErrPresenceUnavailable` 和 `PresenceCheckResult`，表达一次“采帧 + presence 观察”的组合结果。
- 文件：`server/internal/domains/vision/service.go`
- 内容：新增 `CaptureAndObservePresence`，复用 `Capture` 调 MCP 拍照，从 raw JSON 顶层 `presence` 字段解析 `someone/no_one`，再调用 `ObservePresence` 进入状态机；缺失、非法或坏 JSON 都返回 `ErrPresenceUnavailable`。
- 文件：`server/internal/domains/vision/handler.go`
- 内容：新增 `POST /api/vision/check-presence`，用于演示/后续 VLM 链路把采帧结果直接桥接到 PresenceState。
- 文件：`server/internal/childapi/server.go`
- 内容：vision 依赖未注入时为 `/api/vision/check-presence` 返回 501 占位。
- 目的：向完整 PRD #7 的“摄像头采帧 + VLM 判定 + 触发问候”再靠近一步；当前保留 FakeClient/MCP raw presence 作为可测桥接点，后续真实 VLM 输出同一 presence 即可复用状态机。
- 功能：当 MCP/FakeClient 返回 `{"presence":"no_one"}` 后再次返回 `{"presence":"someone"}`，后端会自动触发一次 greeting 域问候。
- 验证：
  - `go test ./internal/domains/vision ./internal/childapi` 通过。
  - `go test -count=1 -cover ./internal/domains/vision ./internal/childapi` 通过，覆盖率分别为 vision 89.0%、childapi 97.6%。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `npm test --prefix web` 通过，25 个测试全绿。
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

### 16:56 reminder 确认/未应答状态 GREEN 实现

- 文件：`server/internal/domains/reminder/types.go`
- 内容：新增 `completed`、`unanswered` 状态，新增 `AckKind`、`AckRequest`、`AckJobID` 和 `AcknowledgedAt` 字段。
- 目的：让 reminder 域能区分“已播报等待确认”“已完成”“未应答”，贴合 PRD #6 状态语义。
- 功能：提醒记录可持久化确认结果和超时 job。
- 文件：`server/internal/domains/reminder/service.go`
- 内容：播报成功后注册 30 分钟确认超时 job；新增 `Acknowledge`，语音确认时取消超时 job 并转 `completed`；超时 job 触发时转 `unanswered`。
- 目的：补齐 PRD #6 的确认/未应答状态机，同时继续只通过 `xiaozhiclient.InjectSpeak` 下发语音，不改原版小智。
- 功能：提醒到点播报后不会直接等同“完成”，需要确认或超时进入最终状态。
- 文件：`web/app.js`
- 内容：调整提醒状态文案，`played` 显示“已播报”，`completed` 显示“已完成”，`unanswered` 显示“未应答”。
- 目的：让子女端列表能正确表达后端新增状态。
- 功能：子女端可见提醒完成/未应答结果。
- 验证：
  - `go test ./internal/domains/reminder` 通过。
  - `npm test --prefix web` 通过。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `go test -count=1 -cover ./internal/domains/reminder` 通过，reminder 包覆盖率 75.9%。
  - 临时 Node 静态服务访问 `http://127.0.0.1:5182/` 返回 200，随后已停止该临时 job；检查时使用 `-NoProxy` 避免本机代理影响 localhost。

### 17:01 reminder ACK 受控入口 RED 测试

- 文件：`server/internal/domains/reminder/handler_test.go`
- 内容：新增 RED 测试，创建提醒并触发播报后，要求 `POST /api/reminders/:id/ack` 接收 `{"ackKind":"voice"}` 并返回 `completed` 提醒。
- 目的：把上一切片的 `Service.Acknowledge` 状态机接到受控 HTTP 入口，为后续设备适配器或演示脚本处理 PRD #6 `reminder_ack` 留出稳定接缝。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 handler 尚未注册 ack 路由。
- 文件：`server/internal/childapi/reminder_routes_test.go`
- 内容：扩展 reminder 路由装配测试，要求注入 reminder 依赖时 ack 路由可用；未注入时 ack 路由返回 501 占位。
- 目的：保持 childapi 的路由形状稳定，继续遵守 childapi 只接入 domain handler 的边界。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期未注入 reminder 时 ack 路由当前为 404。
- 文件：`web/smoke.test.mjs`
- 内容：新增 API client RED 测试，要求 `ackReminder` 带访问码调用 `POST /api/reminders/:id/ack` 并提交 `ackKind`。
- 目的：给子女端/演示脚本共享 API client 补齐 reminder ack 调用缝，不新增可见“子女代确认”按钮。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期 `web/api/client.js` 尚无 `ackReminder`。
- 验证：
  - 已运行 `go test ./internal/domains/reminder ./internal/childapi`，按预期失败。失败原因是 domain ack 路由返回 404，childapi 未注入 reminder 时 ack 占位也返回 404。
  - 已运行 `npm test --prefix web`，按预期失败。失败原因是 `client.ackReminder` 不存在，`web/api/client.js` 也没有暴露 `ackReminder`。

### 17:04 reminder ACK 受控入口 GREEN 实现

- 文件：`server/internal/domains/reminder/handler.go`
- 内容：新增 `POST /api/reminders/:id/ack`，解析 `AckRequest` 并调用 `Service.Acknowledge`，覆盖坏 id、坏 JSON、找不到提醒和不可确认状态。
- 目的：给 PRD #6 `reminder_ack` 提供受控 HTTP 接缝，未来设备适配器或演示脚本可通过此入口驱动“已完成”状态。
- 功能：已播报提醒可经 HTTP ack 转为 `completed`。
- 文件：`server/internal/childapi/server.go`
- 内容：未注入 reminder 域时新增 `/api/reminders/:id/ack` 的 501 占位。
- 目的：保持 childapi 路由形状稳定，前端/API client 可先对着 URL 开发。
- 功能：缺省状态不会落到 404。
- 文件：`web/api/client.js`
- 内容：新增 `ackReminder(reminderId, payload)`，带访问码调用 `POST /api/reminders/:id/ack`。
- 目的：给子女端和演示脚本共享 API client 增加 reminder ack 调用能力。
- 功能：可提交 `{ackKind:"voice"}` 并接收 completed 提醒。
- 验证：
  - `go test ./internal/domains/reminder ./internal/childapi` 通过。
  - `npm test --prefix web` 通过。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `go test -count=1 -cover ./internal/domains/reminder` 通过，reminder 包覆盖率 73.1%。
  - 临时 Node 静态服务访问 `http://127.0.0.1:5183/` 返回 200，随后已停止该临时 job；检查时使用 `-NoProxy` 避免本机代理影响 localhost。

### 17:59 web 连接加载画像 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增 API client 测试，要求 `getProfile` 带访问码调用 `GET /api/profile?deviceId=`；新增子女端连接流程 RED 测试，要求页面连接后通过 `refreshProfile` 调用 `client().getProfile` 并用 `writeProfileForm` 回填画像表单。
- 目的：对齐完整 PRD #5 “家庭画像 ≥ 8 字段”“后端重启后画像不丢”“子女端 Web 能增删改画像字段”，把已保存画像从“只能提交”推进到“连接即读取并回填”。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 `web/app.js` 尚未在连接流程中读取画像，也没有画像表单回填函数。
- 验证：已运行 `npm test --prefix web`，按预期失败。结果为 20 个测试中 19 个通过，`child web loads saved profile on connect` 失败；失败原因是 `web/app.js` 尚未包含 `refreshProfile`。

### 18:01 web 连接加载画像 GREEN 实现

- 文件：`web/app.js`
- 内容：连接刷新流程在状态、留言、提醒和问候时段后调用 `refreshProfile`，通过 `client().getProfile({ deviceId })` 读取已保存画像；新增 `writeProfileForm` / `writeFormValue`，把后端返回的 8 个画像字段回填到表单；当画像接口仍为占位或设备暂无画像时，忽略 501/404，不阻断连接。
- 目的：让子女端进入页面后能看到后端 DB 中的家庭画像，避免后端已持久化但前端只显示静态默认值。
- 功能：已保存的姓名、称呼、子女、孙辈、喜好、作息、健康背景、忌口会在连接后同步到页面摘要和编辑表单。
- 验证：
  - `npm test --prefix web` 通过。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `go test -count=1 -cover ./internal/domains/profile` 通过，profile 包覆盖率 85.4%；首次在沙箱内因 Go build cache 目录权限失败，提升权限后同一命令通过。
  - 临时 Node 静态服务访问 `http://127.0.0.1:5184/` 返回 200，随后已停止该临时 job；检查时使用 `-NoProxy` 避免本机代理影响 localhost。

### 18:07 xiaozhi GetHistory RED 测试

- 文件：`server/internal/xiaozhiclient/http_client_test.go`
- 内容：新增 `GetHistory` HTTP client 测试，要求带 `X-API-Token` 调用 `/api/open/v1/history/messages?deviceId=&limit=`，并解析 manager 常见的 `data.messages`、直接数组、`content/text/message` 和 `created_at/at/timestamp` 字段；同时验证非法时间会返回错误。
- 目的：对齐完整文档中 status 域依赖 `xiaozhiclient.GetHistory` 的 Roadmap，把状态/历史所需的南向只读缝从占位推进到真实 manager OpenAPI 调用。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 `HTTPClient.GetHistory` 仍返回 `types.ErrNotImplemented`。
- 验证：已运行 `go test ./internal/xiaozhiclient`；首次在沙箱内因 Go build cache 权限失败，提升权限后得到有效 RED。失败原因为 `TestGetHistoryReadsManagerHistoryEndpoint` 和 `TestGetHistoryParsesDirectArrayPayload` 均收到 `GetHistory: anban: not implemented`。

### 18:14 xiaozhi GetHistory GREEN 实现

- 文件：`server/internal/xiaozhiclient/http_client.go`
- 内容：实现 `HTTPClient.GetHistory`，构造 `/api/open/v1/history/messages` 查询参数，带 `X-API-Token` 读取 manager 历史消息；新增 `decodeHistoryMessages`，兼容 `data.messages`、根对象 `messages`、直接数组，以及 `content/text/message`、`created_at/at/timestamp` 字段。
- 目的：补齐 status/深度查询依赖的南向只读接口，同时继续保持所有 xiaozhi HTTP 细节只存在于 `internal/xiaozhiclient`。
- 功能：后续状态域或子女端深度历史功能可通过统一 client 读取小智 manager 中的对话历史；非法时间会显式返回解析错误。
- 验证：
  - `go test ./internal/xiaozhiclient` 通过。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `go test -count=1 -cover ./internal/xiaozhiclient` 通过，xiaozhiclient 包覆盖率 85.8%；首次在沙箱内因 Go build cache 目录权限失败，提升权限后同一命令通过。
  - `npm test --prefix web` 通过。

### 20:01 status 最近互动 RED 测试

- 文件：`server/internal/domains/status/service_test.go`
- 内容：新增 status 服务层 RED 测试，要求 `Service.Get` 调用 `xiaozhiclient.GetHistory` 读取最近历史消息，并用最新历史消息时间设置 `lastInteractionAt`；同时要求当历史接口仍不可用（`ErrNotImplemented`）时回退到设备 `last_active_at`，不阻断状态接口。
- 目的：对齐完整 PRD #4 “老人最近一次和设备说话是什么时候”和“最近互动时间准确度 ≤ 1 分钟误差”，把上一片新增的小智历史读取能力接入状态聚合。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 status 服务尚未调用 `GetHistory`。
- 验证：已运行 `go test ./internal/domains/status`；首次在沙箱内因 Go build cache 权限失败，提升权限后得到有效 RED。失败原因为 `TestServiceGetUsesLatestHistoryForLastInteraction` 中 `history deviceID = ""`，说明 status 服务尚未调用 `GetHistory`。

### 20:03 status 最近互动 GREEN 实现

- 文件：`server/internal/domains/status/service.go`
- 内容：`Service.Get` 在读取设备在线状态和留言状态后调用 `xiaozhiclient.GetHistory`，用历史消息中最大的 `At` 时间设置 `lastInteractionAt`；新增 `latestHistoryAt` 辅助函数；当历史接口返回 `ErrNotImplemented` 或无历史时，继续回退到设备 `last_active_at`。
- 目的：让子女端状态面板中的“最近互动”更接近 PRD #4 所说的“老人最近一次和设备说话是什么时候”，而不是只显示设备活跃心跳时间。
- 功能：`GET /api/device/status?deviceId=` 的响应保持原结构不变，但 `lastInteractionAt` 会优先反映小智对话历史中的最新互动时间；历史能力暂不可用时仍保持原有状态接口可用。
- 验证：
  - `go test ./internal/domains/status` 通过。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `go test -count=1 -cover ./internal/domains/status` 通过，status 包覆盖率 85.7%；首次在沙箱内因 Go build cache 目录权限失败，提升权限后同一命令通过。
  - `npm test --prefix web` 通过。

### 21:34 xiaozhi MCP 调用 RED 测试

- 文件：`server/internal/xiaozhiclient/http_client_test.go`
- 内容：新增 `CallDeviceMCPTool` HTTP client RED 测试，要求通过 `POST /api/open/v1/devices/:id/mcp-call`、`X-API-Token` 和 `{"tool","args"}` 调用 manager；同时要求返回值解包 manager 的 `data` 字段，若响应本身是直接 payload 则原样返回。
- 目的：对齐完整小智架构图 §9 “调设备能力（含拍照）→ POST /api/open/v1/devices/:id/mcp-call”，为后续 PRD #7 视觉触发/拍照 MCP 工具打南向基础。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 `HTTPClient.CallDeviceMCPTool` 仍返回 `types.ErrNotImplemented`。
- 验证：已运行 `go test ./internal/xiaozhiclient`，按预期失败。失败原因为 `TestCallDeviceMCPToolSendsManagerRequest` 和 `TestCallDeviceMCPToolReturnsDirectPayload` 均收到 `CallDeviceMCPTool: anban: not implemented`。

### 21:37 xiaozhi MCP 调用 GREEN 实现

- 文件：`server/internal/xiaozhiclient/http_client.go`
- 内容：实现 `HTTPClient.CallDeviceMCPTool`，向 `/api/open/v1/devices/:id/mcp-call` 发送 `tool` 和 `args`，沿用 `X-API-Token` 鉴权；新增 `mcpCallReq` 和 `unwrapData`，将 manager 响应中的 `data` 解包返回，直接 payload 则原样返回。
- 目的：补齐 vision 域后续“设备拍照 MCP 工具 / 调设备能力”依赖的南向封装，同时继续守住只有 `xiaozhiclient` 懂 manager OpenAPI 的边界。
- 功能：业务域可通过统一 client 调用设备 MCP 工具并拿到原始 JSON 结果；具体视觉判断、触发问候等逻辑仍留给后续 vision 小切片。
- 验证：
  - `go test ./internal/xiaozhiclient` 通过。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `go test -count=1 -cover ./internal/xiaozhiclient` 通过，xiaozhiclient 包覆盖率 85.3%。
  - `npm test --prefix web` 通过。

### 21:41 vision 采帧入口 RED 测试

- 文件：`server/internal/domains/vision/service_test.go`
- 内容：新增 vision 服务层 RED 测试，要求 `Capture` 修剪 `deviceId`、默认使用 `camera.capture`、调用 `xiaozhiclient.CallDeviceMCPTool` 并返回原始 JSON；缺少 `deviceId` 时返回 `ErrInvalidInput`。
- 文件：`server/internal/domains/vision/handler_test.go`
- 内容：新增 `POST /api/vision/capture` handler RED 测试，覆盖成功、非法 JSON、缺少设备 ID、MCP 调用失败返回 502。
- 文件：`server/internal/childapi/vision_routes_test.go`
- 内容：新增 childapi 路由装配 RED 测试，要求注入 vision 依赖时 `/api/vision/capture` 可替换 501 占位，未注入时仍返回 501。
- 目的：对齐完整 PRD #7 “轻量视觉触发”和 Roadmap “vision 首个 task：采帧→VLM 判定→触发”中的第一小步：先把设备拍照 MCP 工具封装成安伴后端可调用的受控入口。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 `domains/vision`、`Deps.VisionRoutes` 和启动装配尚不存在。
- 验证：已运行 `go test ./internal/domains/vision ./internal/childapi`；首次在沙箱内因 Go build cache 权限失败，提升权限后得到有效编译级 RED。失败原因是 `Deps.VisionRoutes` 字段不存在，且 `NewService`、`CaptureRequest`、`CaptureResult`、`NewHandler` 等 vision 域类型尚未实现。

### 21:44 vision 采帧入口 GREEN 实现

- 文件：`server/internal/domains/vision/types.go`
- 内容：新增 `CaptureRequest`、`CaptureResult`、`DefaultCaptureTool` 和 `ErrInvalidInput`。
- 文件：`server/internal/domains/vision/service.go`
- 内容：新增 vision 服务，`Capture` 校验并修剪 `deviceId`，默认使用 `camera.capture`，调用 `xiaozhiclient.CallDeviceMCPTool` 并返回原始 JSON。
- 文件：`server/internal/domains/vision/handler.go`
- 内容：新增 `POST /api/vision/capture`，覆盖坏请求、缺少设备 ID 和 manager/MCP 调用失败。
- 文件：`server/internal/childapi/server.go`
- 内容：新增 `Deps.VisionRoutes`，未注入 vision 域时 `/api/vision/capture` 返回 501 占位。
- 文件：`server/cmd/anban/main.go`
- 内容：启动时装配 vision service/handler，并注入 childapi。
- 目的：先把 PRD #7 的设备拍照 MCP 能力接成安伴后端受控入口；VLM 判定、PresenceState 状态机和触发问候留给后续切片。
- 功能：子女端/演示脚本可通过访问码调用 `/api/vision/capture`，触发设备 MCP 拍照工具并获得原始 JSON 结果，不影响原版小智语音主链路。
- 验证：
  - `go test ./internal/domains/vision ./internal/childapi` 通过。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `go test -count=1 -cover ./internal/domains/vision` 通过，vision 包覆盖率 100.0%；首次在沙箱内因 Go build cache 目录权限失败，提升权限后同一命令通过。
  - `npm test --prefix web` 通过。

### 22:22 web 看一眼入口 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增 API client RED 测试，要求 `captureVision` 带访问码调用 `POST /api/vision/capture` 并提交 `deviceId/tool/args`；新增页面 RED 测试，要求子女端有 `visionButton`、`visionResult`，点击逻辑调用 `client().captureVision` 并展示“看一眼结果”。
- 目的：把上一切片新增的后端 `/api/vision/capture` 采帧入口显露到子女端骨架，贴合三周计划 0.4 中“看一眼”按钮和 PRD #7 视觉触发的演示前置能力。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 web API client 和页面尚未提供视觉采帧入口。
- 验证：已运行 `npm test --prefix web`，得到有效 RED：`client.captureVision is not a function`，且页面缺少 `visionButton`。

### 22:29 web 看一眼入口 GREEN 实现

- 文件：`web/api/client.js`
- 内容：新增 `captureVision`，带访问码向 `POST /api/vision/capture` 提交采帧请求。
- 文件：`web/index.html`
- 内容：在“主动陪伴”面板加入“看一眼”按钮和 `visionResult` 输出区。
- 文件：`web/app.js`
- 内容：新增“看一眼”点击处理，调用 `client().captureVision`，提交默认 `camera.capture` 工具参数，并展示“看一眼结果”。
- 文件：`web/styles.css`
- 内容：给 `visionResult` 增加轻量结果框样式，保持现有子女端骨架视觉风格。
- 目的：把后端 vision 采帧入口接到子女端演示界面，先提供可触发、可见结果的 PRD #7 前置能力。
- 功能：子女端连接后可点击“看一眼”调用后端采帧接口，返回原始 MCP 结果；不在前端伪造“有人/没人”判定，后续由 VLM/状态机切片补齐。
- 验证：
  - `npm test --prefix web` 通过，22 个测试全绿。
  - `go test -count=1 ./...` 通过；本次使用仓库内 `.gocache-go` 避开用户级 Go cache 权限问题。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - 本地静态服务冒烟通过：`http://127.0.0.1:4173/index.html` 返回 200，页面包含 `visionButton`。

## 2026-06-03

### 00:18 xiaozhi SetRolePrompt 真实 manager 链路 RED 测试

- 文件：`server/internal/xiaozhiclient/http_client_test.go`
- 内容：替换 `SetRolePrompt` HTTP client 测试，要求先通过 `GET /api/open/v1/devices` 找到 `device_name=deviceId` 的设备及其 `agent_id`，再 `GET /api/open/v1/agents/:id` 读取现有 agent 配置，最后 `PUT /api/open/v1/agents/:id` 只更新 `custom_prompt` 并保留 `name/voice/mcp_service_names` 等已有字段；同时要求三次请求都带 `X-API-Token`。
- 目的：对齐完整文档中“家庭画像→人设 = manager role/agent/switch-role API”和真实上游 OpenAPI 路由，避免继续调用不存在的 `/devices/:id/role-prompt` 端点。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 `HTTPClient.SetRolePrompt` 仍会打旧端点并失败。
- 验证：已运行 `go test ./internal/xiaozhiclient`，得到有效 RED。失败原因为 `TestSetRolePromptSendsManagerAgentRequest` 收到 `xiaozhi manager PUT /api/open/v1/devices/dev-001/role-prompt -> 404`，说明旧实现仍调用不存在的设备 role-prompt 端点。

### 00:26 xiaozhi SetRolePrompt 真实 manager 链路 GREEN 实现

- 文件：`server/internal/xiaozhiclient/http_client.go`
- 内容：`HTTPClient.SetRolePrompt` 改为通过 `GET /api/open/v1/devices` 查找目标设备绑定的 `agent_id`，再 `GET /api/open/v1/agents/:id` 读取现有 agent JSON，最后在原 agent payload 上只替换 `custom_prompt` 并 `PUT /api/open/v1/agents/:id` 写回；新增设备列表、agent 响应和数字/字符串 ID 的解析辅助函数。
- 文件：`server/internal/xiaozhiclient/http_client_test.go`
- 内容：补充设备未绑定 agent 的错误路径、`data.devices` 嵌套设备列表、字符串/数字 ID、嵌套 `data.agent` 响应的解析测试。
- 目的：把家庭画像 prompt 注入接到真实 manager agent API，同时避免 PUT 时清空 agent 现有模型、音色、MCP 服务等配置。
- 功能：profile 域调用 `SetRolePrompt` 时，会把安伴生成的家庭画像 prompt 写到小智 agent 的 `custom_prompt`，仍保持所有 manager HTTP 细节只在 `internal/xiaozhiclient` 内。
- 验证：
  - `go test ./internal/xiaozhiclient` 通过。
  - `go test -count=1 -cover ./internal/xiaozhiclient` 通过，xiaozhiclient 包覆盖率 85.0%。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `npm test --prefix web` 通过，22 个测试全绿。

### 11:04 xiaozhi GetDeviceStatus 真实设备列表 RED 测试

- 文件：`server/internal/xiaozhiclient/http_client_test.go`
- 内容：将 `GetDeviceStatus` HTTP client 测试从单设备 `/api/open/v1/devices/:id` 改为真实的 `GET /api/open/v1/devices` 列表读取：按 `device_name/device_id/id` 匹配目标设备，解析 `online/status` 与 `last_active_at/last_seen_at`；新增设备不存在的错误路径。
- 目的：对齐完整文档中 “设备在线/最近互动 → manager 设备 API 的 `last_active_at`” 的 status 真相源，同时复用上一切片已经建立的设备列表解析方向。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 `HTTPClient.GetDeviceStatus` 仍会打旧的 `/devices/:id` 端点并失败。
- 验证：已运行 `go test ./internal/xiaozhiclient`，得到有效 RED。失败原因为 `TestGetDeviceStatusReadsManagerDeviceList` 收到 `xiaozhi manager GET /api/open/v1/devices/dev-001 -> 404`，说明旧实现仍调用单设备端点；另外列表 payload 也无法被旧的单对象解析逻辑处理。

### 11:09 xiaozhi GetDeviceStatus 真实设备列表 GREEN 实现

- 文件：`server/internal/xiaozhiclient/http_client.go`
- 内容：`HTTPClient.GetDeviceStatus` 改为请求 `GET /api/open/v1/devices`，复用 `decodeManagerDevices` 后按 `device_id/device_name/id` 找目标设备；扩展 `managerDevicePayload` 解析 `online/status/last_active_at/last_seen_at/last_interaction_at`，新增单条设备记录转 `DeviceStatus` 的逻辑，并移除不再使用的旧单设备对象解析 helper。
- 目的：让 status 域读取小智 manager DB 中由 core WS 更新的设备在线/活跃时间真相源，同时避免继续依赖未确认的单设备 OpenAPI 路径。
- 功能：`GET /api/device/status?deviceId=` 通过 profile/status 现有链路获取设备在线、最近活跃时间时，会走真实 manager 设备列表；设备不存在时返回明确错误。
- 验证：
  - `go test ./internal/xiaozhiclient` 通过。
  - `go test -count=1 -cover ./internal/xiaozhiclient` 通过，xiaozhiclient 包覆盖率 85.9%。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `npm test --prefix web` 通过，22 个测试全绿。

### 11:21 web 留言 100 字提示 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增消息草稿规整 RED 测试，要求 `web/message-input.js` 提供 `normalizeMessageDraft`：提交前 trim、按 100 字截断，并在超长时返回“留言已按 100 字发送”的子女端提示；同时新增页面集成断言，要求 `app.js` 在发送前使用该规整结果。
- 目的：对齐完整 PRD #3 “留言文字长度 ≤ 100 字（超出截断 + 子女端提示）”，补齐当前页面只依赖 `textarea maxlength`、缺少提交前明确提示的缺口。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前前端尚未提供 `message-input.js` 规整模块，页面发送逻辑也尚未展示截断提示。
- 验证：已运行 `npm test --prefix web`，得到有效 RED：25 个测试中 3 个失败，失败原因为 `ERR_MODULE_NOT_FOUND: web/message-input.js`，以及 `app.js` 缺少 `normalizeMessageDraft` 接入。

### 11:23 web 留言 100 字提示 GREEN 实现

- 文件：`web/message-input.js`
- 内容：新增 `MESSAGE_TEXT_LIMIT=100` 与 `normalizeMessageDraft`，按 Unicode 字符计数，提交前 trim，并在超出 100 字时截断且返回“留言已按 100 字发送”提示。
- 文件：`web/app.js`
- 内容：留言表单提交前改用 `normalizeMessageDraft`，向后端发送截断后的文本；发送成功后优先展示截断提示，否则展示原“留言已发送”。
- 目的：把 PRD #3 的“超出截断 + 子女端提示”从浏览器输入限制补强为明确的提交前业务规则。
- 功能：子女端即使通过粘贴或脚本填入超过 100 字的留言，也会只提交前 100 字，并给子女明确反馈。
- 验证：
  - `npm test --prefix web` 通过，25 个测试全绿。
  - `node --test --experimental-test-coverage smoke.test.mjs` 通过，web 整体行覆盖率 86.49%，`message-input.js` 行/函数覆盖率 100.0%。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。

### 11:32 reminder 播报文本 30-60 字 RED 测试

- 文件：`server/internal/domains/reminder/service_test.go`
- 内容：将提醒播报断言从固定字符串调整为 PRD 验收口径：播报必须包含提醒内容，且文本长度必须在 30-60 字；新增 `TestReminderTextFitsPRDLength` 覆盖短药品提醒与超长自定义提醒。
- 目的：对齐完整 PRD #6 “播报文本长度 30-60 字（短于 30 干瘪、长于 60 老人记不住）”，避免当前短提醒太干瘪、长提醒原样播报过长。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 `reminderText` 尚未统一压到 30-60 字窗口。
- 验证：已运行 `go test ./internal/domains/reminder`，得到有效 RED：短药品提醒长度 27，超长自定义提醒长度 118，均不满足 30-60 字窗口。

### 11:35 reminder 播报文本 30-60 字 GREEN 实现

- 文件：`server/internal/domains/reminder/service.go`
- 内容：新增 `minReminderTextRunes/maxReminderTextRunes` 与 `buildReminderText/truncateRunes/runeLen`，药品提醒和自定义提醒统一通过 30-60 字窗口生成播报话术；短提醒补足自然关怀尾句，长内容按可用字数截断后保留固定结尾。
- 目的：满足 PRD #6 播报文本长度验收，让提醒既不干瘪也不拖长，适合老人听完后记住。
- 功能：`Create` 保存的 `Reminder.Text`、定时触发的 `InjectSpeak`、以及重启后恢复的提醒都会使用长度受控的播报文本；不改变 xiaozhi 调用边界，仍只通过 `xiaozhiclient.InjectSpeak`。
- 文件：`server/internal/domains/reminder/handler_test.go`
- 内容：补充撤销不存在提醒、ack 坏 ID、ack 坏 JSON、ack 不存在提醒、ack 尚未播报提醒的 HTTP 边界测试。
- 目的：提高 reminder 域对真实子女端错误路径的覆盖，补足 TDD 技能要求的覆盖率门槛。
- 验证：
  - `go test ./internal/domains/reminder` 通过。
  - `go test -count=1 -coverprofile=reminder.out ./internal/domains/reminder` 通过，reminder 包覆盖率 81.1%。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `npm test --prefix web` 通过，25 个测试全绿。

### 11:52 主动语音 10 分钟共享频控 RED 测试

- 文件：`server/internal/proactive/voice_gate_test.go`
- 内容：新增主动语音共享 gate 的 RED 测试，要求同一设备 10 分钟窗口内第二次主动语音返回 `ErrProactiveVoiceThrottled`，窗口后可再次放行，并支持失败尝试回滚。
- 文件：`server/internal/proactive/voice_gate_integration_test.go`
- 内容：新增跨域集成 RED 测试，要求 greeting 和 reminder 注入同一个 gate 后，同一设备先问候再触发提醒时，提醒落为 skipped 且不再调用 xiaozhi `InjectSpeak`。
- 文件：`server/internal/domains/greeting/service_test.go`
- 内容：新增 greeting service/handler RED 测试，要求主动语音配额已用时问候落库为 skipped、不注入 xiaozhi，HTTP 返回 429 并带 skipped payload。
- 文件：`server/internal/domains/reminder/service_test.go`
- 内容：新增 reminder RED 测试，要求提醒到点但配额已用时落为 skipped、清空 `jobId`、不创建 30 分钟 ack timeout、不调用 xiaozhi。
- 文件：`web/smoke.test.mjs`
- 内容：新增子女端 RED 断言，要求 reminder 的 `skipped` 状态展示为“已跳过”。
- 目的：对齐完整 PRD #2/#6/#7 “同一 10 分钟窗口至多 1 条主动语音输出/视觉触发，三者共配额”，先锁住公共规则与 greeting/reminder 的最小接入行为。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 `internal/proactive`、共享 gate 接口、`StatusSkipped` 和 service 注入方法尚未实现。
- 验证：
  - 已运行 `go test ./internal/proactive ./internal/domains/greeting ./internal/domains/reminder`，得到有效 RED。失败原因是 `NewVoiceGate`、`ErrProactiveVoiceThrottled`、`ProactiveVoiceLease`、`UseProactiveVoiceGate` 和 `StatusSkipped` 尚未实现。
  - 已运行 `npm test --prefix web`，得到有效 RED：25 个测试中 1 个失败，失败原因是 reminder `skipped` 状态尚未展示为“已跳过”。

### 12:03 主动语音 10 分钟共享频控 GREEN 实现

- 文件：`server/pkg/types/types.go`
- 内容：新增 `ErrProactiveVoiceThrottled`、`ProactiveVoiceGate` 和 `ProactiveVoiceLease`，作为 #2/#6/#7 共用主动语音配额的跨模块接口。
- 文件：`server/internal/proactive/voice_gate.go`
- 内容：新增内存 `VoiceGate`，按设备记录最近主动语音提交时间；`TryAcquireProactiveVoice` 先预占并阻止并发双播，成功后 `Commit`，xiaozhi 注入失败可 `Rollback`；提交/回滚是本地状态操作，不因请求 context 取消而丢失。
- 文件：`server/internal/proactive/voice_gate_test.go`
- 内容：补充 gate 输入校验和 context canceled 行为测试，覆盖 blank device、取消的 acquire，以及取消 context 下仍能提交/回滚 lease。
- 文件：`server/internal/domains/greeting/types.go`、`server/internal/domains/greeting/service.go`、`server/internal/domains/greeting/handler.go`
- 内容：greeting 新增 `skipped` 状态和 `UseProactiveVoiceGate` 注入点；触发问候前先走共享 gate，配额已用时落库 skipped、不调用 xiaozhi，HTTP 返回 429。
- 文件：`server/internal/domains/reminder/types.go`、`server/internal/domains/reminder/service.go`
- 内容：reminder 新增 `skipped` 状态和共享 gate 注入点；提醒到点但配额已用时落库 skipped、清空 `jobId`、不创建 ack timeout、不调用 xiaozhi。
- 文件：`server/cmd/anban/main.go`
- 内容：启动时创建一个 10 分钟 `VoiceGate`，注入 greeting 和 reminder；vision 目前只采帧不主动播报，后续触发问候切片再接同一 gate。
- 文件：`web/app.js`
- 内容：子女端 reminder 列表把 `skipped` 展示为“已跳过”，避免误显示成“待提醒”。
- 目的：落实完整 PRD #2/#6/#7 的同设备主动语音 10 分钟共享配额，先覆盖当前已经会主动播报的 greeting/reminder。
- 功能：同一设备 10 分钟内只能成功发出一次主动问候/提醒；被限流的提醒或问候会保留记录，方便子女端和后续状态页解释。
- 验证：
  - `go test ./internal/proactive ./internal/domains/greeting ./internal/domains/reminder` 通过。
  - `npm test --prefix web` 通过，25 个测试全绿。
  - `go test -count=1 -cover ./internal/proactive ./internal/domains/greeting ./internal/domains/reminder` 通过，覆盖率分别为 proactive 86.1%、greeting 89.1%、reminder 81.5%。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。

### 15:05 子女端视觉触发演示入口 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增 API client RED 测试，要求 `checkVisionPresence` 带 `X-Access-Code` 调用 `POST /api/vision/check-presence`，并提交 `deviceId/tool/args`；新增页面集成断言，要求页面提供 `visionPresenceButton`、`visionPresenceResult`，并由 `app.js` 调用 `client().checkVisionPresence` 展示“视觉触发结果”。
- 目的：对齐完整 PRD #7 “有人 -> 无人 -> 有人”视觉触发问候，以及三周计划中子女端 Web 每完成一个能力要接入联调的要求；把已完成的后端 `CaptureAndObservePresence` 能力暴露到子女端演示骨架。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 web 只有 `/api/vision/capture` 的“看一眼”入口，尚未封装 `/api/vision/check-presence`。
- 验证：已运行 `npm test --prefix web`，得到有效 RED：27 个测试中 2 个失败，失败原因分别是 `TypeError: client.checkVisionPresence is not a function`，以及 `index.html` 缺少 `visionPresenceButton`/`visionPresenceResult` 页面节点。

### 15:08 子女端视觉触发演示入口 GREEN 实现

- 文件：`web/api/client.js`
- 内容：新增 `checkVisionPresence(payload)`，封装 `POST /api/vision/check-presence`，沿用现有 `request` 逻辑自动带 `X-Access-Code` 和 JSON body。
- 文件：`web/index.html`
- 内容：在主动陪伴面板保留原“看一眼”按钮的同时，新增 `visionPresenceButton` “视觉触发”按钮和 `visionPresenceResult` 结果输出。
- 文件：`web/app.js`
- 内容：新增视觉触发点击处理，调用 `client().checkVisionPresence({ deviceId, tool: "camera.capture", args: { quality: "low" } })`，并把后端 observation 渲染成“视觉触发结果：有人/无人/未知 · 已触发问候/未触发问候”；触发后同步更新顶部状态和 notice。
- 目的：让子女端 Web 骨架能直接演示后端视觉采帧 + presence 判定 + 主动问候触发链路，区别于只返回原始截图/识别结果的“看一眼”加分入口。
- 功能：路演调试时可以从同一个主动陪伴面板分别验证 `/api/vision/capture` 和 `/api/vision/check-presence`；不改变后端 xiaozhi 调用边界。
- 验证：
  - `npm test --prefix web` 通过，27 个测试全绿。
  - `node --test --experimental-test-coverage smoke.test.mjs` 通过，web 整体行覆盖率 86.84%，函数覆盖率 83.33%。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。

### 15:16 profile 画像 prompt 注入长度保护 RED 测试

- 文件：`server/internal/domains/profile/service_test.go`
- 内容：新增 `TestBuildPromptKeepsPromptWithinPRDBudget`，用超长家庭画像字段生成 prompt，要求最终 prompt 不超过 1500 个 Unicode 字符，同时仍保留姓名、称呼、子女、孙辈、喜好等高价值字段。
- 目的：对齐完整 PRD #5 “单次 LLM 调用注入的记忆 token ≤ 1500”，避免子女端输入大量画像文本时把写入 xiaozhi agent 的 system prompt 撑爆。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 `BuildPrompt` 仍会原样拼接全部字段，超出保守预算。
- 验证：已运行 `go test ./internal/domains/profile`，得到有效 RED：`TestBuildPromptKeepsPromptWithinPRDBudget` 失败，当前 prompt 长度为 12728 个字符，超过 1500 字符预算。

### 15:19 profile 画像 prompt 注入长度保护 GREEN 实现

- 文件：`server/internal/domains/profile/service.go`
- 内容：新增 `maxProfilePromptRunes=1500` 和 `maxProfilePromptLineRunes=160`，`BuildPrompt` 追加家庭画像行时先做单字段截断，再按总预算截断；新增 `appendPromptLine`、`promptRuneLen`、`truncateRunes` 辅助函数。
- 目的：让家庭画像 prompt 的注入规模保持在 PRD #5 的保守预算内，同时避免某个超长字段挤掉姓名、称呼、子女、孙辈、喜好等高价值信息。
- 功能：子女端可以继续保存较长画像，但写入 xiaozhi agent 的 prompt 会自动收敛到 1500 字符以内，并优先保留前序核心字段。
- 验证：
  - `go test ./internal/domains/profile` 通过。
  - `go test -count=1 -cover ./internal/domains/profile` 通过，profile 包覆盖率 85.0%。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `npm test --prefix web` 通过，27 个测试全绿。

### 15:47 子女端状态 30 秒轮询 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增状态轮询 RED 测试，要求 `web/status-polling.js` 暴露 `STATUS_REFRESH_INTERVAL_MS=30000`、`startStatusPolling` 和 `stopStatusPolling`；新增页面集成断言，要求 `app.js` 在连接后通过 `restartStatusPolling` 启动 `refreshBackendStatus` 轮询。
- 目的：对齐完整 PRD #4 “设备掉线后子女端 ≤ 30 秒内显示离线”和三周计划 W2 “子女端 Web 切到真后端，留言/状态/触发问候/触发提醒/编辑画像五件全通”。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 web 只在连接时读取一次状态，没有 30 秒轮询模块。
- 验证：已运行 `npm test --prefix web`，得到有效 RED：29 个测试中 2 个失败，失败原因分别是 `ERR_MODULE_NOT_FOUND: web/status-polling.js`，以及 `app.js` 未包含 `startStatusPolling`/`restartStatusPolling`/`refreshBackendStatus`。

### 18:31 子女端状态 30 秒轮询 GREEN 实现

- 文件：`web/status-polling.js`
- 内容：新增 `STATUS_REFRESH_INTERVAL_MS=30000`、`startStatusPolling` 和 `stopStatusPolling`，将 30 秒轮询间隔封装成可测试模块。
- 文件：`web/app.js`
- 内容：连接成功后调用 `restartStatusPolling`，先停止旧轮询再启动 `refreshBackendStatus`；轮询只请求 `/api/device/status` 并更新顶部状态，501 占位静默忽略，其他错误显示“离线 / 状态刷新失败”。
- 目的：对齐 PRD #4 设备离线 ≤30 秒可见，同时不把消息、提醒、问候和画像的完整刷新压到 30 秒循环里。
- 功能：子女端保持连接后会每 30 秒刷新一次设备在线/最近互动状态；切换设备或后端地址后不会叠加多个轮询。
- 验证：
  - `npm test --prefix web` 通过，29 个测试全绿。
  - 在 `web/` 运行 `node --test --experimental-test-coverage smoke.test.mjs` 通过，整体 line 86.72%、function 85.00%。
  - 在 `server/` 运行 `go test -count=1 ./...` 通过。
  - 在 `server/` 运行 `go build ./...` 通过。
  - 在 `server/` 运行 `go vet ./...` 通过。

### 18:47 子女端留言状态 10 秒轮询 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增留言状态轮询 RED 测试，要求 `web/message-status-polling.js` 暴露 `MESSAGE_STATUS_REFRESH_INTERVAL_MS=10000`、`startMessageStatusPolling` 和 `stopMessageStatusPolling`；新增页面集成断言，要求 `app.js` 在连接后通过 `restartMessageStatusPolling` 启动 `refreshBackendMessages` 轮询。
- 目的：对齐完整 PRD #4 “留言状态从 pending -> played 延迟 ≤ 10 秒”，补上上一轮 30 秒设备状态轮询之外的留言列表状态刷新。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 web 只在连接时读取一次消息列表，没有 10 秒留言状态轮询模块。
- 验证：已运行 `npm test --prefix web`，得到有效 RED：31 个测试中 2 个失败，失败原因分别是 `ERR_MODULE_NOT_FOUND: web/message-status-polling.js`，以及 `app.js` 未包含 `startMessageStatusPolling`/`restartMessageStatusPolling`/`refreshBackendMessages`。

### 18:50 子女端留言状态 10 秒轮询 GREEN 实现

- 文件：`web/message-status-polling.js`
- 内容：新增 `MESSAGE_STATUS_REFRESH_INTERVAL_MS=10000`、`startMessageStatusPolling` 和 `stopMessageStatusPolling`，将留言状态刷新间隔封装成可测试模块。
- 文件：`web/app.js`
- 内容：连接成功后调用 `restartMessageStatusPolling`，先停止旧轮询再启动 `refreshBackendMessages`；轮询只请求 `/api/messages?deviceId=...` 并重绘留言列表，501 占位静默忽略，其他错误显示“留言状态刷新失败”。
- 目的：对齐 PRD #4 “留言状态从 pending -> played 延迟 ≤ 10 秒”，和 30 秒设备状态轮询分离，避免高频刷新拖动提醒、问候和画像。
- 功能：子女端保持连接后会每 10 秒刷新一次留言状态；切换设备或后端地址后不会叠加多个留言状态轮询。
- 验证：
  - `npm test --prefix web` 通过，31 个测试全绿。
  - 在 `web/` 运行 `node --test --experimental-test-coverage smoke.test.mjs` 通过，整体 line 86.62%、function 86.36%。
  - 在 `server/` 运行 `go test -count=1 ./...` 通过。
  - 在 `server/` 运行 `go build ./...` 通过。
  - 在 `server/` 运行 `go vet ./...` 通过。

### 18:58 子女端提醒状态 10 秒轮询 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增提醒状态轮询 RED 测试，要求 `web/reminder-status-polling.js` 暴露 `REMINDER_STATUS_REFRESH_INTERVAL_MS=10000`、`startReminderStatusPolling` 和 `stopReminderStatusPolling`；新增页面集成断言，要求 `app.js` 在连接后通过 `restartReminderStatusPolling` 启动 `refreshBackendReminders` 轮询。
- 目的：对齐完整 PRD #6 “老人语音回好/知道了/收到 -> 状态自动转已完成；30 分钟无应答 -> 转未应答且子女端可见”，让后端提醒状态变化持续同步到子女端。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 web 只在连接时读取一次提醒列表，没有提醒状态轮询模块。
- 验证：已运行 `npm test --prefix web`，得到有效 RED：33 个测试中 2 个失败，失败原因分别是 `ERR_MODULE_NOT_FOUND: web/reminder-status-polling.js`，以及 `app.js` 未包含 `startReminderStatusPolling`/`restartReminderStatusPolling`/`refreshBackendReminders`。

### 19:00 子女端提醒状态 10 秒轮询 GREEN 实现

- 文件：`web/reminder-status-polling.js`
- 内容：新增 `REMINDER_STATUS_REFRESH_INTERVAL_MS=10000`、`startReminderStatusPolling` 和 `stopReminderStatusPolling`，将提醒状态刷新间隔封装成可测试模块。
- 文件：`web/app.js`
- 内容：连接成功后调用 `restartReminderStatusPolling`，先停止旧轮询再启动 `refreshBackendReminders`；轮询复用现有 `refreshReminders` 拉取 `/api/reminders?deviceId=...` 并重绘提醒列表，失败时显示“提醒状态刷新失败”。
- 目的：对齐 PRD #6 提醒完成/未应答状态对子女端可见，和留言状态轮询同频 10 秒，保持路演时状态变化可感知。
- 功能：子女端保持连接后会每 10 秒刷新一次提醒状态；切换设备或后端地址后不会叠加多个提醒状态轮询。
- 验证：
  - `npm test --prefix web` 通过，33 个测试全绿。
  - 在 `web/` 运行 `node --test --experimental-test-coverage smoke.test.mjs` 通过，整体 line 86.54%、function 87.50%。
  - 在 `server/` 运行 `go test -count=1 ./...` 通过。
  - 在 `server/` 运行 `go build ./...` 通过。
  - 在 `server/` 运行 `go vet ./...` 通过。

### 19:06 子女端画像字段删除 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增画像表单 RED 测试，要求 `web/profile-form.js` 暴露 `writeProfileFormFields(form, fields)`，并在后端返回缺失字段时清空对应输入；同时更新页面集成断言，要求 `app.js` 使用 `writeProfileFormFields` 写回已保存画像。
- 目的：对齐完整 PRD #5 “子女端 Web 能增删改画像字段”，避免字段被删除后前端仍保留旧值或默认值，下一次保存又把旧字段带回后端。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 `app.js` 内部 `writeFormValue` 会跳过 `undefined/null`，缺失字段不会被清空。
- 验证：已运行 `npm test --prefix web`，得到有效 RED：34 个测试中 2 个失败，失败原因分别是 `ERR_MODULE_NOT_FOUND: web/profile-form.js`，以及 `app.js` 未包含 `writeProfileFormFields`。

### 19:12 子女端画像字段删除 GREEN 实现

- 文件：`web/profile-form.js`
- 内容：新增 `writeProfileFormFields(form, fields)`，统一按 PRD 8 个画像字段写回表单；缺失、`null` 或 `undefined` 字段会清空输入，数组字段用逗号连接。
- 文件：`web/app.js`
- 内容：加载已保存画像后改用 `writeProfileFormFields(els.profileForm, profile.fields || {})`，删除原本会跳过空值的本地 `writeFormValue`。
- 目的：让“删除画像字段”在子女端 Web 上可见，避免旧值残留后被下一次同步重新提交。
- 功能：当其他端或后端清空某个画像字段后，当前子女端重新连接/刷新画像会同步清空对应输入框。
- 验证：
  - `npm test --prefix web` 通过，34 个测试全绿。
  - 在 `web/` 运行 `node --test --experimental-test-coverage smoke.test.mjs` 通过，整体 line 88.20%、function 88.46%。
  - 在 `server/` 运行 `go test -count=1 ./...` 通过。
  - 在 `server/` 运行 `go build ./...` 通过。
  - 在 `server/` 运行 `go vet ./...` 通过。

### 20:31 status 历史接口失败降级 RED 测试

- 文件：`server/internal/domains/status/service_test.go`
- 内容：新增 `TestServiceGetKeepsDeviceStatusWhenHistoryReadFails`，要求 xiaozhi history API 临时失败时，`status.Service.Get` 仍返回已读到的设备在线状态和 `last_active_at` 兜底的最近互动时间。
- 目的：对齐完整 PRD #4 “子女端能看到设备在线/最近互动”和“设备掉线后 ≤30 秒显示离线”，避免可选的历史消息读取失败拖垮核心状态接口。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前实现会把非 `ErrNotImplemented` 的 history 错误直接返回，导致 `/api/device/status` 失败。
- 验证：已运行 `go test ./internal/domains/status`，得到有效 RED：`TestServiceGetKeepsDeviceStatusWhenHistoryReadFails` 失败，错误为 `Get: history api timeout`，说明当前实现仍会把 history 临时失败冒泡为整个状态接口失败。

### 20:35 status 历史接口失败降级 GREEN 实现

- 文件：`server/internal/domains/status/service.go`
- 内容：调整 `Service.Get` 中的 history 读取逻辑：只有 `GetHistory` 成功时才用历史消息更新时间；history 失败时保留 `GetDeviceStatus` 返回的 `last_active_at` 作为 `lastInteractionAt` 兜底。
- 目的：让 PRD #4 的核心状态能力优先可用，避免可选历史接口抖动导致子女端看不到在线/离线和最近在线时间。
- 功能：`/api/device/status` 在 manager 设备状态可读、history 临时失败时仍返回 200 和设备快照；历史成功时仍使用最新历史时间提高“最近互动”准确度。
- 验证：
  - `go test ./internal/domains/status` 通过。
  - `go test -count=1 -cover ./internal/domains/status` 通过，status 包覆盖率 87.2%。
  - `npm test --prefix web` 通过，34 个测试全绿。
  - 在 `web/` 运行 `node --test --experimental-test-coverage smoke.test.mjs` 通过，整体 line 88.20%、function 88.46%。
  - 在 `server/` 运行 `go test -count=1 ./...` 通过。
  - 在 `server/` 运行 `go build ./...` 通过。
  - 在 `server/` 运行 `go vet ./...` 通过。

### 20:44 子女端留言失败状态可见 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增留言状态 RED 测试，要求 `web/message-state.js` 暴露 `upsertMessage` 与 `upsertMessageFromSendError`，能把后端 502 响应体里的 `payload.message` 合并到留言列表顶部；同时要求 `app.js` 在发送失败 catch 分支使用 `upsertMessageFromSendError` 并重新渲染留言列表。
- 目的：对齐完整 PRD #3 “单条留言失败不会卡死后续留言”和 PRD #4 “子女端能看到自己刚发的留言播了没”，避免 manager 注入失败时子女端只看到错误提示、看不到该条失败状态。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 Web 没有 `message-state.js`，且 `app.js` 发送失败分支只调用 `handleApiError`。
- 验证：已运行 `npm test --prefix web`，得到有效 RED：36 个测试中 2 个失败，失败原因分别是 `ERR_MODULE_NOT_FOUND: web/message-state.js`，以及 `app.js` 未包含 `upsertMessageFromSendError`。

### 20:47 子女端留言失败状态可见 GREEN 实现

- 文件：`web/message-state.js`
- 内容：新增 `upsertMessage(messages, message)` 与 `upsertMessageFromSendError(messages, error)`，按 `messageId` 去重并把最新状态放到列表顶部；发送失败时可从 `ApiError.payload.message` 提取后端已持久化的失败留言。
- 文件：`web/app.js`
- 内容：发送成功路径改用 `upsertMessage`；发送失败 catch 分支在 `ApiError` 携带 `payload.message` 时立即合并失败留言并 `renderMessages()`，再保留原有错误提示。
- 文件：`web/smoke.test.mjs`
- 内容：在 RED 测试基础上补充无效 message 与无 `payload.message` 的边界断言，覆盖新 helper 的静默不变分支。
- 目的：对齐完整 PRD #3 “单条留言失败不会卡死后续留言”和 PRD #4 “子女端能看到自己刚发的留言播了没”。
- 功能：当 xiaozhi manager 注入失败但后端已记录失败留言时，子女端马上显示该条留言为“失败”，不用等下一轮轮询。
- 验证：
  - `npm test --prefix web` 通过，36 个测试全绿。
  - 在 `web/` 运行 `node --test --experimental-test-coverage smoke.test.mjs` 通过，整体 line 89.23%、function 89.66%；`web/message-state.js` line/function/branch 均 100%。
  - 在 `server/` 运行 `go test -count=1 ./...` 通过。
  - 在 `server/` 运行 `go build ./...` 通过。
  - 在 `server/` 运行 `go vet ./...` 通过。

### 20:54 reminder 播报后应答超时恢复 RED 测试

- 文件：`server/internal/domains/reminder/service_test.go`
- 内容：新增 `TestServiceRestoreScheduledRehydratesPlayedAckTimeouts`，模拟后端重启时 DB 里已有一条 `played` 且等待老人回应的提醒，要求 `RestoreScheduled` 重新安排 `playedAt + 30 分钟` 的未应答超时任务，并更新新的 `AckJobID`。
- 目的：对齐完整 PRD #6 “30 分钟无应答 → 转未应答且子女端可见”和“后端重启后已排入提醒不丢”，避免已播报但未回应的提醒在重启后永远停留在 `played`。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 `RestoreScheduled` 只恢复 `scheduled` 提醒，不会恢复 `played` 提醒的应答超时任务。
- 验证：已运行 `go test ./internal/domains/reminder`，得到有效 RED：`TestServiceRestoreScheduledRehydratesPlayedAckTimeouts` 失败，`restored count = 0, want 1 played ack timeout`。

### 20:58 reminder 播报后应答超时恢复 GREEN 实现

- 文件：`server/internal/domains/reminder/service.go`
- 内容：扩展 `RestoreScheduled`，在恢复 `scheduled` 提醒播放任务后，再扫描 `played` 且有 `PlayedAt` 的提醒，按 `PlayedAt + defaultAckTimeout` 重新安排未应答超时任务，并更新新的 `AckJobID`。
- 目的：对齐完整 PRD #6 “30 分钟无应答 → 转未应答且子女端可见”和“后端重启后已排入提醒不丢”，让已经播报、等待老人回应的提醒在重启后继续生命周期。
- 功能：后端重启后，已播报但未完成/未应答的提醒会重新进入 30 分钟超时跟踪；若超时时间已过，底层一次性调度会立即触发并转为 `unanswered`。
- 验证：
  - `go test ./internal/domains/reminder` 通过。
  - `go test -count=1 -cover ./internal/domains/reminder` 通过，reminder 包覆盖率 80.3%。
  - `npm test --prefix web` 通过，36 个测试全绿。
  - 在 `web/` 运行 `node --test --experimental-test-coverage smoke.test.mjs` 通过，整体 line 89.23%、function 89.66%。
  - 在 `server/` 运行 `go test -count=1 ./...` 通过。
  - 在 `server/` 运行 `go build ./...` 通过。
  - 在 `server/` 运行 `go vet ./...` 通过。

## 2026-06-04

### 00:02 子女端画像保存后表单归一化 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增 `child web writes backend-normalized profile back after save`，要求画像保存成功后，`app.js` 在调用 `updateProfile` 并 `renderProfile(profile)` 后继续调用 `writeProfileForm(profile)`，把后端返回的归一化字段写回表单。
- 目的：对齐完整 PRD #5 “子女端 Web 能增删改画像字段”，避免子女端清空/删除画像字段后，后端已删除但当前输入框仍残留旧值，下一次保存又把旧值提交回去。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前实现只更新画像摘要，不会写回表单。
- 验证：
  - 初次运行 `npm test --prefix web` 意外通过，发现测试过宽，会匹配到刷新画像路径中的 `writeProfileForm(profile)`。
  - 收窄测试范围到画像提交 handler 的成功分支后，重新运行 `npm test --prefix web` 得到有效 RED：37 个测试中 1 个失败，失败项为 `child web writes backend-normalized profile back after save`，原因是提交成功分支缺少 `writeProfileForm(profile)`。

### 00:07 子女端画像保存后表单归一化 GREEN 实现

- 文件：`web/app.js`
- 内容：在画像保存成功后，`renderProfile(profile)` 之后调用 `writeProfileForm(profile)`，复用已有 `writeProfileFormFields` 把后端返回的画像字段写回表单。
- 目的：让子女端画像编辑形成完整增删改闭环，尤其是后端归一化或清空字段后，当前页面输入框立即反映真实保存结果。
- 功能：用户保存画像后，摘要和表单都以服务端返回值为准；被删除的子女、孙辈、喜好、忌口等列表字段不会继续残留在当前表单里。
- 验证：
  - `npm test --prefix web` 通过，37 个测试全绿。
  - 在 `web/` 运行 `node --test --experimental-test-coverage smoke.test.mjs` 通过，整体 line 89.23%、function 89.66%。
  - 首次并行运行后端 `go test/build/vet` 因 `C:\Users\12520\AppData\Local\Temp` 空间不足失败，错误为 `There is not enough space on the disk`，不是代码编译或测试断言失败。
  - 将 `GOTMPDIR` 切到 `D:\Program\Project\anban-code\.gotmp-go` 后，在 `server/` 运行 `go test -count=1 ./...` 通过。
  - 在同一 D 盘临时目录下运行 `go build ./...` 通过。
  - 在同一 D 盘临时目录下运行 `go vet ./...` 通过。
  - 已运行 `go clean -cache`，并删除 `.gocache-go/README` 与 `.gocache-go/trim.txt` 缓存残留。

### 00:17 子女端提醒完成联调 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增 `child web can mark played reminders completed for demo ack flow`，要求子女端对 `played` 提醒渲染 `data-reminder-action="complete"` 操作，并在点击后调用 `client().ackReminder(..., { ackKind: 'voice' })`，成功后提示“提醒已完成”。
- 目的：对齐完整 PRD #6 “老人答‘好的’→ 子女端那条提醒变已完成”，为路演/联调提供可手动触发的完成状态闭环，复用后端已有 `/api/reminders/:id/ack`。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前页面只支持撤销 scheduled 提醒，不支持 played 提醒的完成确认。
- 验证：已运行 `npm test --prefix web`，得到有效 RED：38 个测试中 1 个失败，失败项为 `child web can mark played reminders completed for demo ack flow`，原因是 `app.js` 未针对 `played` 提醒渲染完成操作，也没有在提醒列表点击分支调用 `ackReminder`。

### 00:20 子女端提醒完成联调 GREEN 实现

- 文件：`web/app.js`
- 内容：提醒列表点击处理器新增 `data-reminder-action` 分支：`cancel` 继续调用 `deleteReminder`，`complete` 调用 `ackReminder(reminderId, { ackKind: 'voice' })`；渲染提醒列表时，对 `scheduled` 显示“撤销”，对 `played` 显示“完成”。
- 目的：让子女端能在路演/联调中直接把已播报提醒标记为已完成，补齐 PRD #6 “已播报 → 老人应答 → 子女端完成态可见”的基础状态闭环。
- 功能：后端返回完成后的提醒后，前端原地替换列表项并提示“提醒已完成”；撤销提醒保持原行为。
- 验证：
  - `npm test --prefix web` 通过，38 个测试全绿。
  - 在 `web/` 运行 `node --test --experimental-test-coverage smoke.test.mjs` 通过，整体 line 89.23%、function 89.66%。
  - 在 `server/` 使用 D 盘 `GOTMPDIR` 运行 `go test -count=1 ./...` 通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR` 运行 `go build ./...` 通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR` 运行 `go vet ./...` 通过。
  - 已运行 `go clean -cache`，并删除 `.gocache-go/README` 与 `.gocache-go/trim.txt` 缓存残留。
