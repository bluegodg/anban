# 实时修改记录

> 目的：记录本轮代码编写中每一批改动的文件、内容、目的、功能和验证方式。后续每次代码改动都要同步更新本文件。

## 2026-06-15

### PRD #6 提醒下发 60 秒边界 RED 测试

- 文件：`server/internal/domains/reminder/service_test.go`
- 内容：新增 `TestServiceFireBoundsInjectForPRDDeliveryWindow`，要求提醒到点播放时调用 `InjectSpeak` 的 context 带有不超过 60 秒的 deadline。
- 目的：对齐 PRD #6 “子女端创建提醒 → 设备落到本地调度，端到端 ≤ 60 秒”的路演稳定性，避免 manager 慢请求把提醒播放、30 分钟确认超时和语音确认轮询卡住。
- 边界：仅约束 reminder 下发调用耗时；不改变提醒文案、分类、主动语音配额、确认/未应答状态机、调度恢复或 `xiaozhiclient` 契约。
- 验证：切片前完整基线全绿；RED 阶段运行 `go test -count=1 ./internal/domains/reminder`，按预期失败于 `InjectSpeak context has no deadline`。

### PRD #5 当前会话承接提示词 RED 测试

- 文件：`server/internal/domains/profile/service_test.go`
- 内容：新增 `TestBuildPromptGuidesCurrentConversationContinuity`，要求 `BuildPrompt` 明确包含“当前会话”“老人刚说过的事”“后续回答要自然承接”等指令。
- 目的：对齐 PRD #5 “当前会话沉淀”的路演验收口径，在不新建记忆系统、不改 xiaozhi 上游的前提下，先让写入 xiaozhi agent 的 prompt 明确要求同一轮对话内自然承接老人刚说过的内容。
- 边界：仅增强 profile prompt；不改变画像字段、1500 字符预算、`SetRolePrompt` 契约、对话历史接口或 xiaozhi 源码。
- 验证：切片前完整基线全绿；RED 阶段运行 `go test -count=1 ./internal/domains/profile`，按预期失败于缺少 `当前会话` 指令。

### PRD #5 当前会话承接提示词 GREEN 实现

- 文件：`server/internal/domains/profile/service.go`、`server/internal/domains/profile/service_test.go`
- 内容：`BuildPrompt` 静态提示词新增“当前会话中老人刚说过的事也要当作短期上下文，后续回答要自然承接，不要像第一次听到一样重复追问。”
- 目的：在 V0.1 不新增完整记忆系统的前提下，增强 xiaozhi agent 对同一轮对话上下文的承接能力，支撑 PRD #5 当前会话沉淀的路演表现。
- 边界：不改变画像持久化、1500 字符预算、`SetRolePrompt` 调用方式、历史记录读取、xiaozhi 上游或任何设备设置工具行为。
- 验证：`go test -count=1 ./internal/domains/profile`、`go build ./...`、`go vet ./...`、`go test -count=1 ./...`、`GOOS=linux GOARCH=amd64 go build ./cmd/anban`、`npm test --prefix web` 均通过。

### PRD #3 留言下发 60 秒边界 RED 测试

- 文件：`server/internal/domains/message/service_test.go`
- 内容：新增 `TestServiceSendBoundsInjectForPRDDeliveryWindow`，要求 message 域调用 `InjectSpeak` 时 context 带有不超过 60 秒的 deadline。
- 目的：对齐 PRD #3 “子女端发送到设备播报完毕端到端 ≤ 60 秒”，在服务层锁住留言下发调用的最长等待窗口，同时保留留言点对点必达、不走主动语音配额的硬约束。
- 边界：仅约束 message 下发调用耗时；不改变留言状态机、AutoListen、长度截断、失败落库、问候/提醒/视觉主动配额或 `xiaozhiclient` 契约。
- 验证：切片前完整基线全绿；RED 阶段运行 `go test -count=1 ./internal/domains/message`，按预期失败于 `InjectSpeak context has no deadline`。

### PRD #3 留言下发 60 秒边界 GREEN 实现

- 文件：`server/internal/domains/message/service.go`、`server/internal/domains/message/service_test.go`
- 内容：message `play` 调用 `xiaozhiclient.InjectSpeak` 前套 60 秒 timeout；若外层请求已有更短 deadline，则沿用外层 deadline。
- 目的：对齐 PRD #3 “子女端发送到设备播报完毕端到端 ≤ 60 秒”，避免 manager 慢请求拖住留言发送，同时不把留言放回主动语音配额。
- 边界：不改变留言点对点必达语义、状态机、AutoListen、长度截断、失败落库、`xiaozhiclient` 接口或问候/提醒/视觉配额逻辑。
- 验证：`go test -count=1 ./internal/domains/message`、`go build ./...`、`go vet ./...`、`go test -count=1 ./...`、`GOOS=linux GOARCH=amd64 go build ./cmd/anban`、`npm test --prefix web` 均通过。

### PRD #3 留言绕过主动语音配额 RED 测试

- 文件：`server/internal/domains/message/service_test.go`
- 内容：将 message 域的配额测试改为 `TestServiceBypassesProactiveVoiceQuota`，要求即使误给留言服务挂上主动语音 gate 且 gate 返回 throttled，子女留言也仍直接调用 `InjectSpeak` 播报，不进入主动语音重试队列。
- 目的：落实真机交接“留言不走主动语音配额”的硬约束，防止未来把 `main.go` 已经绕开的 10 分钟主动配额重新接回留言链路，破坏已验证的子女留言 → 设备开口闭环。
- 边界：仅锁住 message 域对主动语音配额的免疫；不改变问候/提醒/视觉共享配额，不改变留言长度、状态摘要或 `xiaozhiclient` 契约。
- 验证：切片前完整基线全绿；RED 阶段运行 `go test -count=1 ./internal/domains/message`，按预期失败于 `Status = "pending", want "played"`。

### PRD #3 留言绕过主动语音配额 GREEN 实现

- 文件：`server/internal/domains/message/service.go`、`server/internal/domains/message/service_test.go`、`server/internal/domains/message/handler_test.go`
- 内容：message 域保留兼容的 `UseProactiveVoiceGate` 入口但实现为 no-op；留言播放路径不再尝试获取主动语音 lease，也不再因主动配额 throttled 进入 pending/retry。handler 测试同步要求误挂 gate 时仍返回已播报的 `201/played`。
- 目的：把真机已验证的“子女留言点对点必达，不走主动语音 10 分钟配额”从 `main.go` 装配约束下沉为 message 域防回归语义。
- 边界：不改变问候/提醒/视觉的主动配额；不改变留言失败落 `failed`、AutoListen、长度截断、状态摘要或 xiaozhi manager 调用契约。
- 验证：`go test -count=1 ./internal/domains/message`、`go build ./...`、`go vet ./...`、`go test -count=1 ./...`、`GOOS=linux GOARCH=amd64 go build ./cmd/anban`、`npm test --prefix web` 均通过。

### PRD #2 问候触发 5 秒下发边界 RED 测试

- 文件：`server/internal/domains/greeting/service_test.go`
- 内容：新增 `TestServiceTriggerBoundsInjectForPRDClickLatency`，要求子女端触发问候时，greeting 域调用 `InjectSpeak` 的 context 带有不超过 5 秒的 deadline。
- 目的：对齐 PRD #2 “子女端点击按钮到设备开口播报 ≤ 5 秒”，避免 manager/OpenAPI 慢请求把问候按钮长时间拖住。
- 边界：仅约束问候下发调用；不改变问候文案、主动语音配额、留言必达链路或 xiaozhi manager 契约。
- 验证：切片前完整基线全绿；RED 阶段运行 `go test -count=1 ./internal/domains/greeting`，按预期失败于 `InjectSpeak context has no deadline`。

### PRD #2 问候触发 5 秒下发边界 GREEN 实现

- 文件：`server/internal/domains/greeting/service.go`、`server/internal/domains/greeting/service_test.go`
- 内容：`play` 调用 `xiaozhiclient.InjectSpeak` 前套 5 秒 timeout；若外层请求已有更短 deadline，则沿用外层 deadline。
- 目的：对齐 PRD #2 “子女端点击按钮到设备开口播报 ≤ 5 秒”，避免 manager 慢请求拖住问候。
- 边界：不改问候文案、排队/配额逻辑、`xiaozhiclient` 接口或留言链路。
- 验证：`go test -count=1 ./internal/domains/greeting`、`go build ./...`、`go vet ./...`、`go test -count=1 ./...`、`GOOS=linux GOARCH=amd64 go build ./cmd/anban`、`npm test --prefix web` 均通过。

### PRD #7 视觉 MCP 调用 8 秒边界 RED 测试

- 文件：`server/internal/domains/vision/service_test.go`
- 内容：新增 `TestServiceCaptureBoundsMCPCallForPRDLatency`，要求 vision 域调用设备 MCP 拍照/Presence 工具时给下游 context 设置不超过 8 秒的 deadline。
- 目的：对齐 PRD #7 “VLM 调用延迟 + 触发延迟 ≤ 8 秒”，避免视觉触发链路在 manager/MCP 卡住时长时间拖住子女端请求。
- 边界：仅覆盖 vision 可降级链路；不改 xiaozhi、不影响原版语音、留言、问候、提醒主链路。
- 验证：切片前完整基线全绿；RED 阶段运行 `go test -count=1 ./internal/domains/vision`，按预期失败于 `MCP call context has no deadline`。

### PRD #7 视觉 MCP 调用 8 秒边界 GREEN 实现

- 文件：`server/internal/domains/vision/service.go`、`server/internal/domains/vision/service_test.go`
- 内容：`Capture` 调用 `xiaozhiclient.CallDeviceMCPTool` 前统一套 8 秒 timeout；若外层请求已有更短 deadline，则保留外层 deadline。
- 目的：让“看一眼 / 视觉 Presence 触发”这条可降级链路具备明确超时边界，避免路演时视觉卡顿拖住子女端操作。
- 边界：不改变 MCP 工具名、请求参数、返回形状或 `xiaozhiclient` 接口；不触碰原版小智语音和已通真机留言链路。
- 验证：`go test -count=1 ./internal/domains/vision` 已转绿；随后 `go build ./...`、`go vet ./...`、`go test -count=1 ./...`、Linux amd64 交叉编译、`npm test --prefix web` 77/77 全部通过。

### 方案 C 仓库边界与部署入口

- 文件：`docs/deployment/方案C仓库边界与部署总纲.md`、`README.md`、`docs/README.md`
- 内容：新增一份面向当前阶段的方案 C 总纲，集中说明 `anban-code` 仓库定位、`xiaozhi-esp32-server-golang + anban` 两服务拓扑、设备到手后的 Gate A/B/C/D 部署顺序、服务器部署口径和真实设备联调坑位。
- 目的：把“先跑原版小智，再接安伴增强”的可插拔边界写成入口文档，避免继续向大产品扩散或误把本仓当成 xiaozhi fork。
- 边界：只补文档入口和 README 指针；不改代码、不改部署脚本、不写任何 token/key。
- 验证：`git diff --check` 仅有既有 Windows 换行提示；`go build ./...`、`go vet ./...`、`go test -count=1 ./...`、Linux amd64 交叉编译、`npm test --prefix web` 77/77 全部通过。

### PRD #4 对话记录缺失时间展示

- 文件：`web/history-view.js`、`web/app.js`、`web/smoke.test.mjs`
- 内容：新增对话记录展示 helper，统一处理角色、文本和时间；当 manager 历史记录缺少时间或时间非法时，前端显示“时间未知”，不再把 `undefined`/异常时间交给日期格式化器。
- 目的：避免路演页对话记录出现 `Invalid Date` 这类技术泄漏，让开发期完整对话记录在真实 manager 数据不完整时仍保持可读。
- 边界：仅影响子女端展示；不改 childapi 契约、不补写时间、不缓存 manager 历史。
- 验证：切片前服务端 build/vet/test、Linux 交叉编译和 76 个 Web smoke tests 全绿；RED 阶段新增测试因 `history-view.js` 不存在且 `app.js` 未接入 helper 按预期失败；实现后 `go build ./...`、`go vet ./...`、`go test -count=1 ./...`、Linux amd64 交叉编译、`npm test --prefix web` 77/77 全部通过。

### PRD #4 对话记录时间顺序

- 文件：`server/internal/domains/status/service.go`、`server/internal/domains/status/service_test.go`
- 内容：status 对外历史响应在过滤为老人↔设备文本后，按时间稳定排序为旧到新；时间缺失的记录保留在末尾。
- 依据：冻结上游 `ChatHistoryController.GetMessages` 明确按 `created_at DESC` 返回，且注释要求前端反转；安伴 childapi 作为北向边界应给子女端可顺读的对话流。
- 目的：让“对话记录”在路演页按“老人提问 → 安伴回答”的自然顺序展示，而不是 manager 的最新优先分页顺序。
- 验证：RED 阶段倒序输入下第一条变成 assistant，`go test -count=1 ./internal/domains/status -run TestServiceGetHistoryReturnsConversationMessages` 按预期失败；实现后定向 status 测试转绿；`go build ./...`、`go vet ./...`、`go test -count=1 ./...` 全部通过（含架构守护测试），Linux 交叉编译通过，`npm test --prefix web` 76/76 通过。

### PRD #4 对话历史读取失败降级

- 文件：`web/history-refresh.js`、`web/app.js`、`web/smoke.test.mjs`
- 内容：新增可测试的对话历史加载器；初次连接或切换连接时若历史读取失败，清空旧设备历史但继续加载状态/留言等核心数据；后续状态轮询中历史读取短暂失败时，保留最后一次成功记录。
- 目的：完整对话记录是开发期附加能力，不应因 manager history 暂时不可用而阻断子女端 Gate C 主链路，也不应让路演页面上已展示的对话突然消失。
- 边界：不缓存或复制 manager 历史到 anban DB；成功读取时仍以 manager 为真相源，失败仅保留当前浏览器内已有展示数据。
- 验证：切片前服务端 build/vet/test、Linux 交叉编译与 73 个 Web smoke tests 全绿；RED 阶段新增 3 个测试按预期失败，原因是历史加载器不存在且 `refreshHistory` 仍会抛错；实现后 `npm test --prefix web` 76/76 通过；`go build ./...`、`go vet ./...`、`go test -count=1 ./...` 全部通过（含架构守护测试），Linux 交叉编译通过。

### PRD #4 对话记录内容边界

- 文件：`server/internal/domains/status/service.go`、`server/internal/domains/status/service_test.go`
- 内容：status 对外历史接口只返回 `user` 与 `assistant` 两类老人↔设备文本，过滤 manager 中的 `system` / `tool` 内部记录；设备“最近互动”时间也忽略这些内部事件。
- 目的：让子女端看到的确实是 PRD #4 开发期完整对话，而不是家庭画像提示词或 MCP 工具执行细节，并避免工具调用把最近互动时间误报得更晚。
- 架构影响：过滤发生在 status 域的北向展示模型中；`xiaozhiclient.GetHistory` 仍保留 manager 原始历史语义，reminder 等其他使用方契约不变。
- 验证：RED 阶段 status 测试显示 tool 记录把最近互动从 08:45 推到 09:45，且 API 返回 4 条含 system/tool 的记录；实现后 status 定向测试转绿；`go build ./...`、`go vet ./...`、`go test -count=1 ./...` 全部通过（含 reminder 与架构守护测试），Linux 交叉编译通过，`npm test --prefix web` 73/73 通过。

### PRD #4 manager 历史查询参数映射修复

- 文件：`server/internal/xiaozhiclient/http_client.go`、`server/internal/xiaozhiclient/http_client_test.go`
- 内容：`GetHistory(deviceID, limit)` 保持既有契约不变，内部调用 manager `GET /api/open/v1/history/messages` 时改用上游真实参数 `device_id` 和 `page_size`，并用测试禁止旧的 `deviceId` / `limit` 参数回归。
- 依据：完整文档仓的 manager 深读说明 OpenAPI history 复用 `ChatHistoryController.GetMessages`；本地冻结上游源码中该控制器读取 `device_id` 与 `page_size`，并按 `created_at DESC` 返回 `data`。
- 目的：确保 PRD #4 对话记录只读取当前设备且遵守条数上限，避免同一 manager 用户下其他设备的历史被混入子女端，同时不改变 xiaozhi 上游或跨越 `xiaozhiclient` 边界。
- 验证：切片前服务端 build/vet/test、Linux 交叉编译和 73 个 Web smoke tests 全绿；RED 测试显示 manager 收到的 `device_id` / `page_size` 为空；修复后定向 xiaozhiclient 测试转绿；`go build ./...`、`go vet ./...`、`go test -count=1 ./...` 全部通过（含架构守护测试），Linux 交叉编译通过，`npm test --prefix web` 73/73 通过。

### PRD #4 最近互动相对时间展示

- 文件：`web/status-summary.js`、`web/app.js`、`web/smoke.test.mjs`
- 内容：状态卡默认把最近互动时间展示为“刚刚 / N 分钟前 / N 小时前”，超过 24 小时回退为原日期时间；最新留言状态拼接和显式格式器兼容行为保持不变。
- 目的：补齐 PRD #4 路演展示中的“最近互动：3 分钟前”，让子女端状态信息更易扫读，不改变后端接口或轮询频率。
- 验证：TDD RED 阶段确认缺少相对时间函数且页面仍强制绝对日期；实现后 `npm test --prefix web` 通过，73 个 smoke tests 全绿；`server/` 下 `go build ./...`、`go vet ./...`、`go test ./...` 全部通过（含架构守护测试）；`GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/anban` 通过；`git diff --check` 通过，仅有既有 Windows LF/CRLF 提示。

### PRD #6 语音确认轮询 RED 测试

- 文件：`server/internal/domains/reminder/service_test.go`
- 内容：新增语音确认历史轮询测试，要求提醒播报后同时保留 30 分钟超时任务和 10 秒历史轮询任务；历史中播报后的老人“好的”应自动完成提醒，旧确认词和“我不好”不得误判，无命中时继续轮询，重启恢复 `played` 提醒时也恢复轮询。
- 内容：新增确认词表测试，覆盖 PRD 明确的“好 / 知道了 / 收到”及常见“好的”，并拒绝“不好 / 我不知道 / 没收到”等反例。
- 目的：补齐 PRD #6 “老人语音回复后状态自动转已完成”的关键闭环，同时遵守方案 C 单向数据纪律，以 anban 主动轮询现有 `xiaozhiclient.GetHistory` 实现，不要求 xiaozhi 推送事件。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `go test ./internal/domains/reminder`，得到有效编译期 RED：缺少 `voiceAckPollInterval`、`voiceAckHistoryLimit` 及对应轮询行为。

### PRD #6 语音确认轮询 GREEN 实现

- 文件：`server/internal/domains/reminder/service.go`
- 内容：提醒成功播报后每 10 秒经 `xiaozhiclient.GetHistory(deviceID, 20)` 只读轮询对话历史；发现播报时间之后的老人确认短句时，复用现有 `acknowledge` 状态机转为 `completed/voice` 并取消 30 分钟超时任务。
- 内容：确认词按去空白/标点后的短语白名单识别，支持“好、好的、知道了、收到”等，避免把“不好、没收到”等否定表达误判；历史读取失败或未命中时继续轮询，达到 30 分钟仍由既有超时任务转 `unanswered`。
- 内容：`RestoreScheduled` 恢复尚未超时的 `played` 提醒时，除恢复超时任务外立即执行一次语音确认轮询，避免临近 30 分钟超时时先被标记未应答；轮询任务不新增 DB 字段，`AckJobID` 继续只保存可取消的超时任务。
- 文件：`server/internal/domains/reminder/service_test.go`
- 内容：保留自动完成、反例防误判、继续轮询、确认词表、重启恢复及原有手动确认/超时回归测试。
- 目的：让路演中“提醒播报 → 老人说好的 → 子女端状态变已完成”具备真实后端闭环，不改 xiaozhi 上游、不复制 manager 历史、不新增跨域依赖。
- 功能影响：仅为 `played` 提醒增加 manager 历史只读轮询；不改变留言配额、主动语音 gate、提醒下发参数、childapi 契约或 Web 轮询周期。
- 验证：
  - 切片前 `server/` 下 `go build ./... && go vet ./... && go test ./...` 通过，架构测试全绿。
  - 切片前 `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/anban` 通过。
  - 切片前 `npm test --prefix web` 通过，71 个 smoke tests 全绿。
  - RED 后 `go test ./internal/domains/reminder` 得到有效编译失败：缺少语音确认轮询常量与行为。
  - GREEN 与 `gofmt` 后 `go test ./internal/domains/reminder` 通过，RED 用例已转绿。
  - 恢复边界 RED：`TestServiceRestoreScheduledRehydratesPlayedAckTimeouts` 显示轮询被排到启动后 10 秒；改为启动时立即轮询后该用例转绿。
  - 切片后 `server/` 下 `go build ./... && go vet ./... && go test ./...` 通过，含 `internal/architecture` 守护测试。
  - 切片后 `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/anban` 通过。
  - 切片后 `npm test --prefix web` 通过，71 个 smoke tests 全绿。
  - `git diff --check` 通过；仅出现既有 Windows LF/CRLF 换行提示。
  - `git diff --check` 通过；仅出现既有 Windows LF/CRLF 换行提示。

### PRD #4 掉线显示轮询余量 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：把状态轮询测试改为要求 20 秒刷新一次，为 PRD #4“设备掉线后 ≤30 秒显示离线”留出 HTTP 请求耗时与浏览器调度余量。
- 目的：避免轮询间隔本身已占满 30 秒，导致真实离线展示必然可能越过验收上限。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `npm test --prefix web`，得到有效 RED：71 个 smoke tests 中仅新增约束失败，实际值 `30000`、期望值 `20000`。

### PRD #4 掉线显示轮询余量 GREEN 实现

- 文件：`web/status-polling.js`
- 内容：把 `STATUS_REFRESH_INTERVAL_MS` 从 30 秒调整为 20 秒；首次连接仍立即刷新，后续只增加既有 `/api/device/status` 的刷新频率。
- 文件：`web/smoke.test.mjs`
- 内容：保留 20 秒状态轮询守护测试，并明确这是为 30 秒离线目标预留请求延迟余量。
- 目的：让子女端对设备掉线的感知更稳定地落在 PRD #4 的 30 秒窗口内。
- 功能影响：仅调整子女端状态/对话历史刷新频率；不改变后端在线判定、固件保活、留言状态 10 秒轮询或任何 xiaozhi manager 契约。
- 验证：
  - 切片前 `server/` 下 `go build ./... && go vet ./... && go test ./...` 通过，架构测试全绿。
  - 切片前 `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/anban` 通过。
  - 切片前 `npm test --prefix web` 通过，71 个 smoke tests 全绿。
  - RED 后 `npm test --prefix web` 得到有效失败：状态轮询实际值 `30000`、期望值 `20000`。
  - GREEN 后 `npm test --prefix web` 通过，71 个 smoke tests 全绿，RED 用例已转绿。
  - 切片后 `server/` 下 `go build ./... && go vet ./... && go test ./...` 通过，含 `internal/architecture` 守护测试。
  - 切片后 `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/anban` 通过。
  - 切片后 `npm test --prefix web` 通过，71 个 smoke tests 全绿。

## 2026-06-14

### PRD #4 对话记录后端 RED 测试

- 文件：`server/internal/domains/status/service_test.go`
- 内容：新增 `TestServiceGetHistoryReturnsConversationMessages`、`TestServiceGetHistoryDefaultsAndCapsLimit`、`TestServiceGetHistoryRejectsMissingDeviceID`，要求 status 域把 `xiaozhiclient.GetHistory` 包装成子女端可读的只读对话历史响应，修剪 deviceId，默认 limit=50，最大 limit=100。
- 文件：`server/internal/domains/status/handler_test.go`
- 内容：新增 `TestHandlerGetHistory` 和 `TestHandlerGetHistoryRejectsInvalidLimit`，要求 `GET /api/device/history?deviceId=&limit=` 返回对话记录，并拒绝非法 limit。
- 文件：`server/internal/childapi/status_routes_test.go`
- 内容：要求 childapi 在 status 依赖缺失时也保留 `/api/device/history` 的 501 占位，在依赖提供时由 status handler 接管。
- 目的：落实 PRD #4/开发期完整对话记录，只读展示 xiaozhi 已有历史，不复制到 anban DB，不改变 `xiaozhiclient.GetHistory` 契约。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `go test ./internal/domains/status ./internal/childapi`（`server/`，含 `GOPROXY=https://goproxy.cn,direct`、`GOSUMDB=off`、`CGO_ENABLED=0`），得到有效 RED：`status` 包因缺少 `HistoryRequest`、`HistoryResponse` 和 `Service.GetHistory` 编译失败；`childapi` 的 `/api/device/history` 在缺少 status 依赖时返回 404 而非 501。

### PRD #4 对话记录后端 GREEN 实现

- 文件：`server/internal/domains/status/types.go`
- 内容：新增 `HistoryRequest`、`HistoryResponse`、`HistoryEntry`，用 `role/text/at` 小写 JSON 字段给子女端展示对话历史。
- 文件：`server/internal/domains/status/service.go`
- 内容：新增 `Service.GetHistory`，修剪 deviceId，经 `xiaozhiclient.GetHistory` 只读拉取历史，并把时间转为 UTC；接口默认 limit=50，最大 limit=100。
- 文件：`server/internal/domains/status/handler.go`
- 内容：注册 `GET /api/device/history`，支持 `deviceId` 和可选 `limit` 查询参数，非法 `deviceId/limit` 返回 400，manager/history 读取失败返回 502。
- 文件：`server/internal/childapi/server.go`
- 内容：在 status 依赖缺失时为 `/api/device/history` 保留 501 地基占位。
- 目的：让子女端可以通过 anban 只读查看 xiaozhi 对话历史，同时保持 `xiaozhiclient` 仍是唯一懂 manager OpenAPI 的边界。
- 功能影响：新增只读 childapi 路由；不写 anban DB，不改变设备播报、留言配额、profile prompt 或 xiaozhi manager 客户端契约。
- 验证：已重跑 `go test ./internal/domains/status ./internal/childapi`（`server/`，同上 Go 环境）通过，后端 RED 用例已转绿。

### PRD #4 对话记录前端 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增 `API client fetches device conversation history with access code`，要求 `createAnbanClient().getHistory({deviceId, limit})` 调用 `/api/device/history?deviceId=&limit=` 并携带 `X-Access-Code`。
- 文件：`web/smoke.test.mjs`
- 内容：新增 `child web shows backend conversation history on connect`，要求页面包含“对话记录”和 `historyList`，应用状态包含 `history`，连接刷新流程调用 `refreshHistory()`，并有 `renderHistory()` 渲染函数。
- 目的：把 PRD #4 开发期完整对话记录落实到子女端可见 UI，而不是只停留在后端接口。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `npm test --prefix web`，得到有效 RED：68 个 smoke tests 中 2 个失败，分别为 `client.getHistory is not a function` 和页面缺少“对话记录”/`historyList`。

### PRD #4 对话记录前端 GREEN 实现

- 文件：`web/api/client.js`
- 内容：新增 `getHistory({deviceId, limit})`，请求 `/api/device/history` 并复用访问码鉴权与查询参数清洗。
- 文件：`web/app.js`
- 内容：新增 `state.history`、`refreshHistory()`、`renderHistory()` 和角色标签映射；连接成功刷新时拉取最近 50 条对话记录，501/502 时清空历史但不阻断状态/留言主链路。
- 文件：`web/index.html`
- 内容：新增“对话记录”面板和 `historyList` 列表。
- 文件：`web/styles.css`
- 内容：复用列表样式并为 history 列表增加角色、正文、时间三列布局。
- 目的：让子女端在开发期能直接查看老人和设备的文字对话历史，支撑 PRD #4 的状态安心感与 §8 的开发期记录策略。
- 功能影响：新增只读 UI 展示；不改变留言发送、主动问候、提醒、画像或视觉链路。
- 验证：
  - `npm test --prefix web` 通过，68 个 smoke tests 全绿，前端 RED 用例已转绿。
  - 切片后 `go build ./... && go vet ./... && go test ./...`（`server/`，含 `GOPROXY=https://goproxy.cn,direct`、`GOSUMDB=off`、`CGO_ENABLED=0`）通过，含 `internal/architecture` 守护测试。
  - `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/anban`（`server/`，同上 GOPROXY/GOSUMDB）通过。
  - 切片后 `npm test --prefix web` 通过，68 个 smoke tests 全绿。
  - `git diff --check` 通过；仅出现既有 Windows LF/CRLF 换行提示。

### 子女端视觉工具名对齐真机 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增 `child web uses real ESP32 camera MCP tool for vision actions`，要求子女端视觉按钮集中使用 `VISION_CAPTURE_TOOL = 'self.camera.take_photo'`，且不再向后端显式传旧的 `camera.capture`。
- 目的：对齐 2026-06-14 真机交接中“vision 默认工具名 `self.camera.take_photo` 已修，勿回退”的约束，避免 web 覆盖后端默认工具名导致真机拍照 MCP 调用失败。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `npm test --prefix web`，得到有效 RED：69 个 smoke tests 中新增用例失败，失败点显示 `app.js` 缺少 `VISION_CAPTURE_TOOL = 'self.camera.take_photo'` 且仍传 `tool: 'camera.capture'`。

### 子女端视觉工具名对齐真机 GREEN 实现

- 文件：`web/app.js`
- 内容：新增 `VISION_CAPTURE_TOOL = 'self.camera.take_photo'` 常量，并让“看一眼”和“视觉触发”两个按钮都使用该真实 ESP32 拍照 MCP 工具名。
- 文件：`web/smoke.test.mjs`
- 内容：保留前端工具名守护测试，防止后续把 web 侧回退到旧的 `camera.capture`。
- 目的：避免子女端显式传旧工具名覆盖后端 vision 域的真机默认值，保护已验证的 `self.camera.take_photo` 链路。
- 功能影响：仅调整子女端视觉请求 payload 的 tool 字段；不改变后端 vision 默认、不改 xiaozhi client、不影响留言/问候/提醒主链路。
- 验证：
  - `npm test --prefix web` 通过，69 个 smoke tests 全绿，前端 RED 用例已转绿。
  - 切片后 `go build ./... && go vet ./... && go test ./...`（`server/`，含 `GOPROXY=https://goproxy.cn,direct`、`GOSUMDB=off`、`CGO_ENABLED=0`）通过，含 `internal/architecture` 守护测试。
  - `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/anban`（`server/`，同上 GOPROXY/GOSUMDB）通过。
  - 切片后 `npm test --prefix web` 通过，69 个 smoke tests 全绿。
  - `git diff --check` 通过；仅出现既有 Windows LF/CRLF 换行提示。

### 对话记录随状态轮询刷新 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增 `child web status polling refreshes conversation history`，要求 `refreshBackendStatus()` 成功刷新设备状态后也调用 `refreshHistory()`。
- 目的：补齐 PRD #4 “设备状态/最近互动/开发期对话记录”在子女端持续可见的细节，避免对话记录只在连接时拉取一次，老人后续对话需要重新连接才显示。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `npm test --prefix web`，得到有效 RED：70 个 smoke tests 中新增用例失败，失败点显示 `refreshBackendStatus()` 只调用 `updateStatusSnapshot(snapshot)`，未调用 `refreshHistory()`。

### 对话记录随状态轮询刷新 GREEN 实现

- 文件：`web/app.js`
- 内容：`refreshBackendStatus()` 在成功刷新设备状态后调用 `await refreshHistory()`，让 30 秒状态轮询同时更新开发期对话记录。
- 文件：`web/smoke.test.mjs`
- 内容：保留状态轮询刷新对话记录的 smoke 守护测试。
- 目的：让子女端打开后能持续看到新的老人 ↔ 设备对话记录，支撑 PRD #4 的“最近互动/对话记录”安心感。
- 功能影响：仅新增只读 history 刷新；不改变留言、问候、提醒、画像、视觉或 xiaozhi manager 客户端契约。
- 验证：
  - 切片后 `server/` 下 `go build ./... && go vet ./... && go test ./...` 通过，架构测试全绿。
  - 切片后 `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/anban` 通过。
  - 切片后 `npm test --prefix web` 通过，70 个 smoke tests 全绿。
  - `git diff --check` 通过；仅出现既有 Windows LF/CRLF 换行提示。

### PRD #6 提醒分类选择 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增 `child web lets children choose PRD reminder categories`，要求子女端提醒表单暴露 `med`、`birthday`、`festival`、`custom` 四类，并在创建提醒时从表单读取 category。
- 目的：补齐 PRD #6 “用药 / 生日 / 节日”提醒在子女端的演示入口，避免 Web 永远把提醒写死成 `med`。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `npm test --prefix web`，得到有效 RED：71 个 smoke tests 中新增用例失败，失败点显示页面缺少 `#reminderCategory`。

### PRD #6 提醒分类选择 GREEN 实现

- 文件：`web/index.html`
- 内容：在提醒表单增加“提醒类型”下拉，支持用药、生日、节日、自定义，对应后端已有 `med`、`birthday`、`festival`、`custom`。
- 文件：`web/app.js`
- 内容：新增 `els.reminderCategory`，创建提醒时使用 `category: els.reminderCategory.value`，不再把子女端提醒固定为 `med`。
- 文件：`web/smoke.test.mjs`
- 内容：保留提醒分类选择的 smoke 守护测试。
- 目的：让 #6 主动提醒能在 Web 演示中覆盖用药、生日、节日三种 PRD 场景，同时复用现有 reminder 后端契约。
- 功能影响：仅改变子女端创建提醒 payload 的 category 来源；不改变提醒调度、主动语音配额、设备下发或 xiaozhi manager 客户端。
- 验证：
  - 切片前 `server/` 下 `go build ./... && go vet ./... && go test ./...` 通过，架构测试全绿。
  - 切片前 `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/anban` 通过。
  - 切片前 `npm test --prefix web` 通过，70 个 smoke tests 全绿。
  - GREEN 后 `npm test --prefix web` 通过，71 个 smoke tests 全绿，前端 RED 用例已转绿。
  - 切片后 `server/` 下 `go build ./... && go vet ./... && go test ./...` 通过，架构测试全绿。
  - 切片后 `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/anban` 通过。
  - 切片后 `npm test --prefix web` 通过，71 个 smoke tests 全绿。
  - `git diff --check` 通过；仅出现既有 Windows LF/CRLF 换行提示。
  - `git diff --check` 通过；仅出现既有 Windows LF/CRLF 换行提示。

### PRD #6 分类提醒文案 RED 测试

- 文件：`server/internal/domains/reminder/service_test.go`
- 内容：扩展 `TestReminderTextFitsPRDLength`，要求 `birthday` 分类的播报文案含“生日”、`festival` 分类含“节日”，同时继续满足 PRD 30–60 字长度和老人称谓。
- 目的：让 Web 已能选择的生日/节日提醒在设备播报时也有明确场景感，不退回通用“提醒您”。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `go test ./internal/domains/reminder`，得到有效 RED：生日、节日子用例失败，当前文案分别缺少“生日”和“节日”分类提示。

### PRD #6 分类提醒文案 GREEN 实现

- 文件：`server/internal/domains/reminder/service.go`
- 内容：`reminderText` 为 `CategoryBirthday` 和 `CategoryFestival` 增加专门文案分支；用药和自定义分支保持原有行为，仍经 `buildReminderText` 控制 30–60 字。
- 文件：`server/internal/domains/reminder/service_test.go`
- 内容：保留生日/节日分类文案守护测试。
- 目的：让 #6 主动提醒在用药、生日、节日三个 PRD 场景里都有可听懂的播报语境，同时不改变调度、ack、主动语音配额或 manager 客户端契约。
- 功能影响：仅影响提醒播报文本生成；不改变提醒状态机、DB schema、childapi 路由、Web 请求或 xiaozhi 下发参数。
- 验证：
  - 切片前 `server/` 下 `go build ./... && go vet ./... && go test ./...` 通过，架构测试全绿。
  - 切片前 `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/anban` 通过。
  - 切片前 `npm test --prefix web` 通过，71 个 smoke tests 全绿。
  - RED 后 `go test ./internal/domains/reminder` 得到有效失败：生日、节日文案缺少分类提示。
  - GREEN 后 `go test ./internal/domains/reminder` 通过，RED 用例已转绿。
  - 切片后 `server/` 下 `go build ./... && go vet ./... && go test ./...` 通过，架构测试全绿。
  - 切片后 `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/anban` 通过。
  - 切片后 `npm test --prefix web` 通过，71 个 smoke tests 全绿。

### 真机后阶段对齐文档

- 文件：`docs/plans/2026-06-14-真机后阶段对齐与方案C部署说明.md`
- 内容：新增真机验证后的阶段对齐说明，明确方案 C 两进程可插拔架构、`anban-code` 仓库边界、当前服务器与真机事实、Gate A/B/C/D 部署顺序、PRD 基础闭环优先级，以及“基础框架和基本功能目标”何时才算实现。
- 文件：`README.md`、`docs/README.md`
- 内容：把仓库入口和文档索引更新到 2026-06-14 真机后对齐文档，同时保留 2026-06-12 设备到手前对齐文档作为历史入口。
- 目的：回应“设备到了后按方案 C 怎么部署、这个仓库是什么、现在是否仍按 PRD 基础目标推进”的对齐需求，防止继续向大产品范围扩张。
- 功能影响：仅文档变更，不改变后端、前端、部署脚本或 xiaozhi 上游。
- 验证：
  - `git diff --check` 通过；仅出现既有 Windows LF/CRLF 换行提示。
  - 已检查 `README.md` 和 `docs/README.md` 均指向新文档，并保留 2026-06-12 对齐文档入口。

## 2026-06-12

### profile BuildPrompt 设备设置约束 RED 测试

- 文件：`server/internal/domains/profile/service_test.go`
- 内容：新增 `TestBuildPromptGuardsDeviceSettingsUnlessElderAsks`，断言 `BuildPrompt` 包含“非老人明确要求，不要更改设备设置/音量/屏幕主题/字体”。
- 目的：落实 2026-06-14 真机交接里的人设约束，防止豆包在老人未明确要求时主动调用固件自带的音量、屏幕主题、字体等设备设置工具。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `go test ./internal/domains/profile`（`server/`，含 `GOPROXY=https://goproxy.cn,direct`、`GOSUMDB=off`、`CGO_ENABLED=0`），得到有效 RED：`TestBuildPromptGuardsDeviceSettingsUnlessElderAsks` 失败，当前 prompt 尚不包含设备设置保护语。

### profile BuildPrompt 设备设置约束 GREEN 实现

- 文件：`server/internal/domains/profile/service.go`
- 内容：`BuildPrompt` 固定系统指令新增“非老人明确要求，不要更改设备设置/音量/屏幕主题/字体；日常陪伴中不要主动调用设备设置工具。”。
- 文件：`server/internal/domains/profile/service_test.go`
- 内容：保留设备设置保护语断言，防止后续改 prompt 时回退。
- 目的：抑制真机上豆包在老人未明确要求时主动调用 `self.audio_speaker.set_volume`、`self.screen.set_theme` 等固件工具，保护已通的原版小智/设备体验。
- 功能影响：仅改变同步到 xiaozhi manager 的 profile role prompt；不改 manager OpenAPI 调用契约，不改留言/问候/提醒主动播报链路。
- 验证：
  - `go test ./internal/domains/profile`（`server/`，含 `GOPROXY=https://goproxy.cn,direct`、`GOSUMDB=off`、`CGO_ENABLED=0`）通过。
  - 切片后 `go build ./... && go vet ./... && go test ./...`（`server/`，同上环境）通过，含 `internal/architecture` 守护测试。
  - `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/anban`（`server/`，同上 GOPROXY/GOSUMDB）通过。
  - `npm test --prefix web` 通过，66 个 smoke tests 全绿。

### 子女端默认后端地址对齐本地 anban RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增 `child web defaults backend address to local anban server for Gate C`，要求 `state.apiBaseURL` 在没有 localStorage 覆盖时默认使用 `http://localhost:8090`。
- 目的：对齐首日执行单和 Gate C 静态部署方式，减少子女端首次打开后还需要手填后端地址的联调摩擦。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `npm test`（`web/`），得到有效 RED：66 个 smoke tests 中 1 个失败，失败点显示 `apiBaseURL` 仍为 `localStorage.getItem('anban.apiBaseURL') || ''`。

### 子女端默认后端地址对齐本地 anban GREEN 实现

- 文件：`web/app.js`
- 内容：`state.apiBaseURL` 在 localStorage 没有保存值时默认填入 `http://localhost:8090`。
- 文件：`web/smoke.test.mjs`
- 内容：保留默认后端地址测试，确保首屏连接表单和首日 Gate C 部署说明一致。
- 目的：让子女端首次打开时直接指向本地 anban 后端默认端口，降低设备到手联调的手工配置成本。
- 功能影响：仅调整子女端默认表单值；用户已有 localStorage 配置优先，不改变 API client、后端或 xiaozhi 原版链路。
- 验证：
  - `npm test`（`web/`）通过，66 个 smoke tests 全绿。
  - `node --test --experimental-test-coverage smoke.test.mjs`（`web/`）通过，66 个测试全绿；all files 行覆盖率 95.60%、函数覆盖率 97.78%。
  - `git diff --check` 通过；仅出现既有 Windows LF/CRLF 换行提示。

### 子女端后端地址占位提示对齐必填行为 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增 `child web backend address placeholder matches required static deployment`，要求 `apiBaseURL` 输入框占位提示为 `http://localhost:8090`，且页面不再出现“同源留空”。
- 目的：上一个切片已经让后端地址成为 Gate C 静态联调必填项；这里防止 UI 继续提示“可留空”，避免设备到手现场误配置。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `npm test`（`web/`），得到有效 RED：65 个 smoke tests 中 1 个失败，失败点显示 `index.html` 仍包含 `placeholder="同源留空"`，不符合后端地址必填行为。

### 子女端后端地址占位提示对齐必填行为 GREEN 实现

- 文件：`web/index.html`
- 内容：将后端地址输入框占位提示从“同源留空”改为 `http://localhost:8090`。
- 文件：`web/smoke.test.mjs`
- 内容：保留占位提示测试，确保页面提示和 Gate C 静态部署步骤一致。
- 目的：让子女端首屏直接给出本地 anban 后端默认地址，减少设备到手现场联调时的错误输入。
- 功能影响：仅调整子女端 UI 提示；不改变 API 调用路径、后端行为或 xiaozhi 原版能力。
- 验证：
  - `npm test`（`web/`）通过，65 个 smoke tests 全绿。
  - `node --test --experimental-test-coverage smoke.test.mjs`（`web/`）通过，65 个测试全绿；all files 行覆盖率 95.60%、函数覆盖率 97.78%。
  - `git diff --check` 通过；仅出现既有 Windows LF/CRLF 换行提示。

### 子女端连接要求填写后端地址 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：将连接设置校验测试改为要求 `state.apiBaseURL`、`state.accessCode`、`state.deviceId` 三项都在 `refreshMessages()` 前完成校验，并把无效连接提示调整为“后端地址、访问码和设备 ID 必填”。
- 目的：对齐设备到手后的 Gate C 联调方式：子女端静态页通常跑在 `http://127.0.0.1:5173`，后端跑在 `http://localhost:8090`，必须显式填写后端地址，避免误向静态页自身发 API 请求导致排查混乱。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `npm test`（`web/`），得到有效 RED：64 个 smoke tests 中 3 个失败，失败点均为当前 `app.js` 找不到“后端地址、访问码和设备 ID 必填”校验，说明生产代码仍只校验访问码和设备 ID。

### 子女端连接要求填写后端地址 GREEN 实现

- 文件：`web/app.js`
- 内容：连接表单提交时将 `state.apiBaseURL` 纳入必填校验；缺少后端地址、访问码或设备 ID 时停止轮询、清空旧数据，并显示“请填写后端地址、访问码和设备 ID”。
- 文件：`web/smoke.test.mjs`
- 内容：保留 RED 阶段新增的连接校验断言，确保后端地址校验发生在任何 `refreshMessages()` 调用前。
- 目的：让 Gate C 子女端联调失败更早、更清楚，避免静态页面在未填写后端地址时向自身 origin 发起 `/api/*` 请求。
- 功能影响：子女端静态页现在要求显式填写后端地址；对 anban 后端和 xiaozhi 原版语音链路无影响。
- 验证：
  - `npm test`（`web/`）通过，64 个 smoke tests 全绿。
  - `node --test --experimental-test-coverage smoke.test.mjs`（`web/`）通过，64 个测试全绿；all files 行覆盖率 95.60%、函数覆盖率 97.78%。
  - `git diff --check` 通过；仅出现既有 Windows LF/CRLF 换行提示。

### 新增设备到手方案 C 首日执行单

- 文件：`docs/deployment/设备到手方案C首日执行单.md`
- 内容：新增一份面向现场联调的短执行单，明确三个仓库边界、方案 C 两进程拓扑、Gate A/B/C/D 顺序、PowerShell 命令、可插拔验证和首日记录模板。
- 文件：`docs/README.md`
- 内容：把首日执行单加入文档索引。
- 文件：`README.md`
- 内容：在仓库入口补充首日执行单链接。
- 目的：回应“设备到了后按方案 C 怎么部署、这个仓库是什么”的对齐需求，避免继续向大产品发散，先围绕基础语音闭环和最小安伴增强做真设备验证。
- 功能影响：仅文档变更，不改变后端、前端、预检工具或 xiaozhi 上游。
- 验证：
  - `Test-Path docs\deployment\设备到手方案C首日执行单.md` 返回 `True`。
  - `Select-String -Path README.md,docs\README.md -Pattern '设备到手方案C首日执行单|设备到手：方案 C 首日执行单'` 命中文档入口链接。
  - `Select-String -Path docs\deployment\设备到手方案C首日执行单.md -Pattern 'Gate A|Gate B|Gate C|Gate D|xiaozhi-esp32-server-golang|anban-code|ANBAN_MANAGER_API_TOKEN|--xiaozhi-gate-passed'` 命中关键部署与架构约束。
  - `git diff --check` 通过；仅出现既有 Windows LF/CRLF 换行提示。

### anban-preflight 拒绝 manager token 占位符 RED 测试

- 文件：`server/cmd/anban-preflight/main_test.go`
- 内容：新增 `TestLoadPreflightConfigRejectsExampleManagerTokenPlaceholder`，要求 `anban-preflight` 的配置加载同样拒绝 `.env.example` 里的 `ANBAN_MANAGER_API_TOKEN=请填_manager签发的APIToken`。
- 目的：让预检工具和主服务在 Gate B 前置配置门禁上保持一致，避免复制 `.env.example` 后忘填真实 token 时，preflight 继续向 manager 发出无意义请求。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `go test -count=1 ./cmd/anban-preflight`，得到有效 RED：`TestLoadPreflightConfigRejectsExampleManagerTokenPlaceholder` 失败，说明当前 preflight 配置层仍接受 `.env.example` 占位 token。

### anban-preflight 拒绝 manager token 占位符 GREEN 实现

- 文件：`server/internal/config/config.go`
- 内容：将占位值判断 helper 导出为 `IsPlaceholderValue`，供主服务和 preflight 复用同一套示例占位判断。
- 文件：`server/cmd/anban-preflight/main.go`
- 内容：`loadPreflightConfig()` 增加 `config.IsPlaceholderValue(cfg.ManagerAPIToken)` 校验，遇到 `.env.example` 占位 token 时直接返回配置错误。
- 文件：`server/cmd/anban-preflight/main_test.go`
- 内容：保留 preflight 占位 token 拒绝测试。
- 目的：让 Gate B 预检在发起 manager 请求前就拦住“复制示例配置但未填写真实 token”的常见错误，并与主服务配置规则保持一致。
- 功能影响：`anban-preflight` 对明显占位 token 更严格；真实 manager token 不受影响。不改变 xiaozhi 上游，也不改变 anban 运行时业务逻辑。
- 验证：
  - `go test -count=1 ./cmd/anban-preflight ./internal/config` 通过。
  - `go test -count=1 ./...`（`server/`）通过。
  - `go build ./...`（`server/`）通过。
  - `go vet ./...`（`server/`）通过。
  - `npm test`（`web/`）通过，64 个 smoke tests 全绿。
  - `node --test --experimental-test-coverage smoke.test.mjs`（`web/`）通过，64 个测试全绿；all files 行覆盖率 95.60%、函数覆盖率 97.78%。
  - `git diff --check` 通过；仅出现既有 Windows LF/CRLF 换行提示。

### config 拒绝 manager token 占位符 RED 测试

- 文件：`server/internal/config/config_test.go`
- 内容：新增 `TestLoadRejectsExampleManagerTokenPlaceholder`，要求 `ANBAN_MANAGER_API_TOKEN=请填_manager签发的APIToken` 这类 `.env.example` 占位值不能通过配置加载。
- 目的：避免复制 `.env.example` 后忘记填写真实 xiaozhi manager token 时，`anban` 服务仍然启动，导致 Gate B 联调时才暴露 token 错误。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `go test -count=1 ./internal/config`，得到有效 RED：`TestLoadRejectsExampleManagerTokenPlaceholder` 失败，说明当前配置层仍接受 `.env.example` 占位 token。

### config 拒绝 manager token 占位符 GREEN 实现

- 文件：`server/internal/config/config.go`
- 内容：新增 `isPlaceholderValue` 校验，并在 `Load()` 中拒绝 `ANBAN_MANAGER_API_TOKEN` 使用包含 `请填` 或尖括号的示例占位值。
- 文件：`server/internal/config/config_test.go`
- 内容：保留占位 token 拒绝测试，确保复制 `.env.example` 后未填写真实 token 时服务启动配置会失败。
- 目的：把 xiaozhi manager token 的配置错误前移到启动阶段，减少 Gate B 设备联调时才发现 token 未填写的情况。
- 功能影响：真实 manager token 不受影响；明显占位 token 会导致 `config.Load()` 返回错误。
- 验证：
  - `go test -count=1 ./internal/config` 通过。
  - `go test -count=1 ./...`（`server/`）通过。
  - `go build ./...`（`server/`）通过；首次并行验证时遇到 Windows 沙箱 `apply deny-read ACLs` 工具层异常，单独重跑后通过，非 Go 编译错误。
  - `go vet ./...`（`server/`）通过。
  - `npm test`（`web/`）通过，64 个 smoke tests 全绿。
  - `node --test --experimental-test-coverage smoke.test.mjs`（`web/`）通过，64 个测试全绿；all files 行覆盖率 95.60%、函数覆盖率 97.78%。
  - `git diff --check` 通过；仅出现既有 Windows LF/CRLF 换行提示。

### anban-preflight 默认要求设备 ID RED 测试

- 文件：`server/cmd/anban-preflight/main_test.go`
- 内容：新增 `TestRunCommandRequiresDeviceIDByDefault`，要求预检 CLI 在未提供 `-device-id` / `ANBAN_PREFLIGHT_DEVICE_ID` 时默认返回非 0，并提示可用 `--allow-missing-device-id` 做 manager-only 排查；同时把原 manager-only 成功测试改为显式传入 `--allow-missing-device-id`。
- 目的：设备已到手后，预检默认应证明“manager 能看到这台真实设备”，避免只验证 manager token 可用就误以为方案 C 联调准备完成。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `go test -count=1 ./cmd/anban-preflight`，得到有效 RED：`--allow-missing-device-id` 尚未定义，且默认缺设备 ID 时 exit code 仍为 0。

### anban-preflight 默认要求设备 ID GREEN 实现

- 文件：`server/cmd/anban-preflight/main.go`
- 内容：新增 `--allow-missing-device-id` flag 和 `ANBAN_PREFLIGHT_ALLOW_MISSING_DEVICE_ID` 环境变量；预检在 Gate A 已确认但未提供设备 ID 时默认返回非 0，并提示提供 `-device-id`，只有显式允许时才作为 manager-only 检查通过。
- 文件：`server/cmd/anban-preflight/main_test.go`
- 内容：将原先只依赖 Gate A 环境变量的 manager-only 成功用例改为同时设置 `ANBAN_PREFLIGHT_ALLOW_MISSING_DEVICE_ID`，并覆盖带空格的环境变量解析；新增 `TestRunCommandPassesWithConfirmedGateAndOnlineDevice`，验证提供设备 ID 且 manager 返回在线设备时 CLI 正常退出 0。
- 文件：`docs/deployment/方案C部署与联调指南.md`
- 内容：同步更新 Gate B 判据和缺设备 ID 的排查命令，明确缺设备 ID 时必须显式加 `--allow-missing-device-id`，且该模式只代表 manager-only 网络/token 检查通过。
- 目的：把方案 C 的设备到手联调顺序落到 CLI 默认行为里，避免“只验证 manager token”被误读成“真实设备 manager 接入已通过”。
- 功能影响：`anban-preflight` 默认更严格；缺设备 ID 的临时网络/token 排查需要显式加 `--allow-missing-device-id`。不改变 xiaozhi 上游，也不影响 anban 服务运行时。
- 验证：
  - `go test -count=1 ./cmd/anban-preflight` 通过。
  - `go test -count=1 ./...`（`server/`）通过。
  - `go build ./...`（`server/`）通过。
  - `go vet ./...`（`server/`）通过。
  - `npm test`（`web/`）通过，64 个 smoke tests 全绿。
  - `node --test --experimental-test-coverage smoke.test.mjs`（`web/`）通过，64 个测试全绿；all files 行覆盖率 95.60%、函数覆盖率 97.78%。
  - `git diff --check` 通过；仅出现既有 Windows LF/CRLF 换行提示。

### greeting 三时段日程契约 RED 测试

- 文件：`server/internal/domains/greeting/service_test.go`
- 内容：扩展 `TestServiceGreetingScheduleValidatesInput`，要求主动问候日程必须保持 PRD 的早 / 午 / 晚 3 个预设时段；缺少 `noon`、重复 `morning`、未知 `bedtime` 或空 label 都应返回 `ErrInvalidInput`。
- 目的：对齐 PRD #2 “每天预设 3 个时间段（早 / 午 / 晚），可在子女端配置”，避免基础子女端骨架漂成任意复杂日程系统。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `go test -count=1 ./internal/domains/greeting`，得到有效 RED：`missing_noon_slot`、`duplicate_morning_slot`、`unknown_slot_label`、`blank_slot_label` 均返回 `nil`，说明当前实现仍接受任意 slot。

### greeting 三时段日程契约 GREEN 实现

- 文件：`server/internal/domains/greeting/service.go`
- 内容：新增 `requiredScheduleSlotLabels` 校验，`UpdateSchedule` 现在要求日程 slots 必须且只能包含 `morning`、`noon`、`evening` 三个 label，缺失、重复、额外或空 label 都返回 `ErrInvalidInput`；每个固定时段仍可配置时间、启用状态和语气。
- 目的：把 PRD #2 的早 / 午 / 晚三时段基础契约落到后端服务层，保持子女端骨架和路演脚本简单稳定。
- 功能影响：主动问候日程更新不再接受自定义 slot label；默认日程和现有早 / 午 / 晚配置不受影响，不改变 xiaozhi 调用路径。
- 验证：
  - `go test -count=1 ./internal/domains/greeting` 通过。
  - `go test -count=1 ./...`（`server/`）通过。
  - `go build ./...`（`server/`）通过。
  - `go vet ./...`（`server/`）通过。
  - `npm test`（`web/`）通过，64 个 smoke tests 全绿。
  - `node --test --experimental-test-coverage smoke.test.mjs`（`web/`）通过，64 个测试全绿；all files 行覆盖率 95.60%、函数覆盖率 97.78%。
  - `git diff --check` 通过；仅出现既有 Windows LF/CRLF 换行提示。

### 当前阶段对齐文档

- 文件：`docs/plans/2026-06-12-phase-alignment-scheme-c.md`
- 内容：新增当前阶段对齐说明，明确现在应回到“基础框架 + 基本功能 + 真设备联调”的阶段；解释方案 C 的两进程可插拔架构、`anban-code` 仓库边界、设备到手后的 Gate A/B/C/D 部署顺序，以及基础目标何时才算真正实现。
- 文件：`README.md`、`docs/README.md`
- 内容：新增该阶段对齐文档入口，避免只看部署命令时忽略“先纯 xiaozhi、再接安伴”的阶段纪律。
- 目的：回应“现在是什么阶段、是否按 PRD、这个仓库是什么、方案 C 怎么部署”的对齐要求，防止继续向大产品范围扩张。
- 功能影响：仅文档变更；不改变后端、前端、部署编排或 xiaozhi 上游。
- 验证：已运行 `git diff --check`，通过；仅出现既有 Windows LF/CRLF 换行提示。

### Compose anban Dockerfile 守护测试

- 文件：`server/internal/architecture/architecture_test.go`
- 内容：新增 `TestDockerComposeAnbanBuildContextHasDockerfile`，要求 `docker-compose.yml` 中 `anban` 服务从 `./server` 构建时，`server/Dockerfile` 必须存在，并包含 `go build`、`./cmd/anban` 和 `CGO_ENABLED=0`。
- 目的：守住方案 C 的 Compose 部署骨架，避免 `docker compose --profile anban up` 因后端 Dockerfile 缺失或构建入口漂移而失败。
- 功能影响：仅新增架构守护测试；不改变运行时代码，不影响只部署原版 xiaozhi。
- 验证：已运行 `go test -count=1 ./internal/architecture`，测试通过，确认当前 `server/Dockerfile` 已满足该守护条件。

### Compose anban 构建上下文清理 RED 测试

- 文件：`server/internal/architecture/architecture_test.go`
- 内容：新增 `TestAnbanDockerBuildContextIgnoresLocalArtifacts`，要求 `server/.dockerignore` 排除 `.gotmp-go/`、`.gocache-go/`、`*.db` 和 `anban.db`。
- 目的：对齐方案 C 的 Compose 部署骨架，避免 `docker compose --profile anban up` 构建安伴后端时把本地 Go 缓存或 Demo DB 一起复制进镜像上下文。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `go test -count=1 ./internal/architecture`，得到有效 RED：`server/.dockerignore` 不存在。

### Compose anban 构建上下文清理 GREEN 实现

- 文件：`server/.dockerignore`
- 内容：新增 Docker 构建上下文忽略规则，排除 `.gotmp-go/`、`.gocache-go/`、`tmp/`、本地 sqlite DB、覆盖率和测试二进制产物。
- 文件：`server/internal/architecture/architecture_test.go`
- 内容：保留 Dockerfile 和 `.dockerignore` 两个 Compose 构建守护测试；同时将 `mustServerRoot` 改为优先从测试工作目录向上查找 `go.mod`，再用 `runtime.Caller` 兜底，避免 Go/sandbox 源码路径映射到临时 cwd 时误找仓库根。
- 目的：让 `docker compose --profile anban up` 的安伴后端构建更接近干净部署环境，同时保持方案 C 可选增强服务的部署护栏稳定可跑。
- 功能影响：仅影响 Docker 构建上下文和架构测试；不改变运行时代码，不修改 xiaozhi 上游，也不影响只部署原版 xiaozhi。
- 验证：
  - `go test -count=1 ./internal/architecture` 通过。过程中发现 `runtime.Caller` 在当前沙箱下会指向 `C:\Users\CodexSandboxOffline\.codex\.sandbox\cwd`，已改为以测试工作目录定位真实 server root。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `npm test`（`web/`）通过，64 个 smoke tests 全绿。
  - `node --test --experimental-test-coverage smoke.test.mjs`（`web/`）通过，64 个测试全绿；all files 行覆盖率 95.60%、函数覆盖率 97.78%。
  - `git diff --check` 通过；仅出现既有 Windows LF/CRLF 换行提示。

### 子女端留言排队提示 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增 `message send result formatter distinguishes queued messages`，要求发送留言后根据后端返回的 `status=played|pending` 区分“留言已播报”和“留言已排队等待设备空闲”；新增 `child web uses message send result formatter`，要求 `app.js` 接入该格式化器并使用格式化后的通知。
- 目的：承接后端 message `pending/202` 语义，对齐 PRD #3“老人正在对话时排队，不强行打断”和“子女端可见留言状态”。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `npm test`（`web/`），得到有效 RED：`web/message-result.js` 尚未实现，且 `app.js` 尚未接入 `formatMessageSendResult`；64 个测试中 2 个按预期失败。

### 子女端留言排队提示 GREEN 实现

- 文件：`web/message-result.js`
- 内容：新增 `formatMessageSendResult`，根据后端留言状态输出子女端通知文案；`pending` 显示“留言已排队等待设备空闲”，`played` 显示“留言已播报”，并可合并 100 字截断提示。
- 文件：`web/app.js`
- 内容：发送留言成功后改用 `formatMessageSendResult(message, { draftNotice: draft.notice })` 渲染通知，不再把所有成功响应统一显示为“留言已发送”。
- 文件：`web/smoke.test.mjs`
- 内容：保留 played/pending 两类留言发送结果格式化测试，并验证 app 接入 `formatMessageSendResult`。
- 目的：承接后端 message `pending/202` 语义，让子女端基础骨架在路演联调时能明确提示“已排队等待设备空闲”。
- 功能影响：仅改变子女端发送留言后的提示文案；无后端 API 变更，无 xiaozhi 上游代码变化。
- 验证：
  - `npm test`（`web/`）通过，64 个 smoke tests 全绿。
  - `node --test --experimental-test-coverage smoke.test.mjs`（`web/`）通过，64 个测试全绿；all files 行覆盖率 95.60%、函数覆盖率 97.78%。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `git diff --check` 通过；仅出现既有 Windows LF/CRLF 换行提示。

### message 留言主动语音等待 RED 测试

- 文件：`server/internal/domains/message/service_test.go`
- 内容：新增 `TestServiceQueuesMessageWhenProactiveVoiceQuotaUsed`，要求子女留言遇到主动语音窗口冲突时保持 `pending`、注册一次性 retry job、不调用 xiaozhi 注入。
- 文件：`server/internal/domains/message/handler_test.go`
- 内容：新增 `TestHandlerCreateReturnsAcceptedWhenMessageIsQueued`，要求 HTTP 创建留言在排队时返回 `202 Accepted` 和 `pending` message payload。
- 目的：对齐 PRD #3“老人正在对话时排队，不强行打断”，让留言链路具备与问候/提醒一致的基础等待语义。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `go test -count=1 ./internal/domains/message`，得到有效 RED：`UseProactiveVoiceGate`、可选 scheduler 和 `messageRetryDelay` 尚未实现。

### message 留言主动语音等待 GREEN 实现

- 文件：`server/internal/domains/message/service.go`
- 内容：新增 `OneShotScheduler`、`messageRetryDelay`、主动语音 gate 接入、`queueRetry` 和 `retryQueuedMessage`；留言遇到主动语音窗口冲突时保持 `pending`，注册一次性重试任务，后续从数据库重新读取同一条留言再尝试播放。
- 文件：`server/internal/domains/message/store.go`
- 内容：新增 `Get(ctx, id)`，用于重试任务按留言 ID 重新读取持久化消息。
- 文件：`server/internal/domains/message/types.go`
- 内容：新增 `ErrNotFound`，为 message store 的未找到结果提供域内错误。
- 文件：`server/internal/domains/message/handler.go`
- 内容：创建留言成功但状态为 `pending` 时返回 `202 Accepted`，已播报仍返回 `201 Created`。
- 文件：`server/cmd/anban/main.go`
- 内容：把共享 scheduler 和主动语音 gate 注入 message service，使留言与问候/提醒共用主动语音等待语义。
- 目的：对齐 PRD #3“老人正在对话时排队，不强行打断”，并保持 xiaozhi 调用仍只通过 `xiaozhiclient`。
- 功能影响：子女留言在设备主动语音窗口忙时不再立即失败，而是进入 pending 并等待重试；未部署 anban 时不影响原始 xiaozhi 对话服务。
- 验证：
  - `go test -count=1 ./internal/domains/message` 通过，message 域测试全绿。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `npm test`（`web/`）通过，62 个 smoke tests 全绿。
  - `node --test --experimental-test-coverage smoke.test.mjs`（`web/`）通过，62 个测试全绿；all files 行覆盖率 96.88%、函数覆盖率 97.62%。
  - `git diff --check` 通过；仅出现既有 Windows LF/CRLF 换行提示。

### 子女端视觉触发排队提示 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增 `vision presence result formatter distinguishes queued greeting`，要求视觉触发结果根据 `observation.greeting.status` 区分“已触发问候”和“问候已排队”；同时要求 `app.js` 接入 `formatVisionPresenceResult`。
- 目的：承接主动问候 `pending/202` 排队语义，避免视觉触发复用问候服务时，把“问候已排队等待设备空闲”误显示成“视觉触发已完成”。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `npm test --prefix web`，得到有效 RED：`web/vision-presence-result.js` 尚未实现，且 `app.js` 尚未接入 `formatVisionPresenceResult`。

### 子女端视觉触发排队提示 GREEN 实现

- 文件：`web/vision-presence-result.js`
- 内容：新增 `formatVisionPresenceResult`，根据 `observation.presence`、`observation.triggeredGreeting` 和 `observation.greeting.status` 生成视觉触发结果、状态卡详情和通知文案；当问候为 `pending` 时显示“视觉触发已排队 / 问候已排队”。
- 文件：`web/app.js`
- 内容：视觉触发按钮点击后改用 `formatVisionPresenceResult(result)` 渲染结果输出、状态卡和通知；移除旧的本地 `presenceLabel` 死代码。
- 文件：`web/smoke.test.mjs`
- 内容：保留 played/pending 视觉触发格式化测试，并验证 app 接入 `formatVisionPresenceResult`。
- 目的：承接视觉触发复用主动问候服务后的 `pending` 排队语义，让子女端基础骨架诚实展示“视觉触发已排队等待设备空闲”。
- 功能影响：仅改变子女端视觉触发显示文案；无后端 API 变更，无 xiaozhi 上游代码变化。
- 验证：
  - `npm test --prefix web` 通过，62 个 smoke tests 全绿。
  - `node --test --experimental-test-coverage smoke.test.mjs` 通过，62 个测试全绿；all files 行覆盖率 96.88%、函数覆盖率 97.62%，`vision-presence-result.js` 行覆盖率 100%。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `git diff --check` 通过；仅出现既有 Windows LF/CRLF 换行提示。

### 子女端问候排队提示 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增 `greeting trigger result formatter distinguishes queued greetings` 和 `child web uses greeting trigger result formatter`，要求子女端区分后端返回的 `played` 与 `pending` 问候：已播报显示“问候已触发”，排队中显示“问候已排队”。
- 目的：承接后端 `pending/202` 排队语义，避免子女端把“主动问候已排队等待”误显示成“刚刚触发一次主动问候”。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `npm test --prefix web`，得到有效 RED：`web/greeting-result.js` 尚未实现，且 `app.js` 尚未接入 `formatGreetingTriggerResult`。

### 子女端问候排队提示 GREEN 实现

- 文件：`web/greeting-result.js`
- 内容：新增 `formatGreetingTriggerResult`，将后端问候结果规整为子女端状态卡标题、状态详情和通知文案；`status=pending` 显示“主动问候已排队 / 问候已排队”，其他成功结果显示“刚刚触发一次主动问候 / 问候已触发”。
- 文件：`web/app.js`
- 内容：问候按钮点击后改用 `formatGreetingTriggerResult(greeting)` 渲染状态卡和通知，不再把所有成功响应都写成“刚刚触发”。
- 文件：`web/smoke.test.mjs`
- 内容：保留 played/pending 两类问候结果格式化测试，并验证 app 接入 `formatGreetingTriggerResult`。
- 目的：承接后端 `pending/202` 排队语义，让子女端基础骨架在路演联调时能诚实展示“已排队等待设备空闲”，而不是误报已播报。
- 功能影响：仅改变子女端问候按钮的显示文案；无后端 API 变更，无 xiaozhi 上游代码变化。
- 验证：
  - `npm test --prefix web` 通过，61 个 smoke tests 全绿。
  - `node --test --experimental-test-coverage smoke.test.mjs` 通过，61 个测试全绿；all files 行覆盖率 96.52%、函数覆盖率 97.50%。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `git diff --check` 通过；仅出现既有 Windows LF/CRLF 换行提示。

### greeting 主动语音配额等待 RED 测试

- 文件：`server/internal/domains/greeting/service_test.go`
- 内容：将主动问候遇到主动语音配额冲突时的期望从 `skipped/429` 改为 `pending/202`，并要求注册一次性 retry job，不调用 xiaozhi 注入。
- 目的：对齐 PRD #2“老人正在和 AI 对话时，主动问候应排队等待，不强行打断”，避免子女端点击问候按钮时因为 10 分钟主动语音窗口已用而直接失败。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `go test -count=1 ./internal/domains/greeting`，得到有效 RED：`Trigger` 仍返回 `ErrProactiveVoiceThrottled`，handler 仍返回 429 且 payload 为 `skipped`。

### greeting 主动语音配额等待 GREEN 实现

- 文件：`server/internal/domains/greeting/service.go`
- 内容：新增 `OneShotScheduler`、`greetingRetryDelay`、`queueRetry` 和 `retryQueuedGreeting`；当主动语音 gate 返回 `ErrProactiveVoiceThrottled` 时，主动问候保持 `pending`，注册一次性重试任务，后续从数据库重新读取同一条问候再尝试播放。
- 文件：`server/internal/domains/greeting/store.go`
- 内容：新增 `Get(ctx, id)`，让重试闭包按 ID 读取最新问候状态，避免重复播放已终结的记录。
- 文件：`server/internal/domains/greeting/handler.go`
- 内容：当 `Trigger` 返回 `pending` 问候时，HTTP 返回 `202 Accepted`，让子女端知道问候已排队而不是失败。
- 文件：`server/internal/domains/greeting/service_test.go`
- 内容：保留 `TestServiceTriggerQueuesWhenProactiveVoiceQuotaUsed` 和 `TestHandlerTriggerGreetingReturnsAcceptedWhenQuotaUsed`，覆盖 service/HTTP 两层排队语义。
- 目的：补齐 PRD #2“主动问候排队等待，不强行打断”的基础能力；与 reminder 的等待语义保持一致。
- 功能影响：只改变主动问候遇到主动语音配额冲突时的状态流转；xiaozhi manager 注入失败仍然标记 `failed`；不修改 xiaozhi 上游代码。
- 验证：
  - `go test -count=1 ./internal/domains/greeting` 通过。
  - `go test -count=1 ./internal/proactive` 通过。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `npm test --prefix web` 通过，59 个 smoke tests 全绿。
  - `node --test --experimental-test-coverage smoke.test.mjs` 通过，59 个测试全绿；all files 行覆盖率 96.28%、函数覆盖率 97.37%。
  - `git diff --check` 通过；仅出现既有 Windows LF/CRLF 换行提示。

### reminder 主动语音配额等待 RED 测试

- 文件：`server/internal/domains/reminder/service_test.go`
- 内容：将主动语音配额被占用时的提醒行为从“标记 skipped”改为期望“保持 scheduled 并重新排队”；新增对 retry job、下一次尝试时间和不调用 xiaozhi 注入的断言。
- 文件：`server/internal/proactive/voice_gate_integration_test.go`
- 内容：同步跨域集成测试期望，要求 greeting 已占用同设备主动语音配额后，reminder 不丢弃，而是继续留在调度队列中等待下一次尝试。
- 目的：对齐 PRD #2/#6 的共同纪律：同一 10 分钟窗口至多 1 条主动语音输出，老人正在对话或主动语音配额暂不可用时应排队等待，不应直接跳过提醒。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `go test -count=1 ./internal/domains/reminder ./internal/proactive`，得到有效 RED：`proactiveRetryDelay` 尚未实现，且集成测试显示当前提醒未保持 `scheduled`。

### reminder 主动语音配额等待 GREEN 实现

- 文件：`server/internal/domains/reminder/service.go`
- 内容：新增 `proactiveRetryDelay` 和 `requeueProactiveVoice`；当主动语音 gate 返回 `ErrProactiveVoiceThrottled` 时，不再把提醒标为 `skipped`，而是保持 `scheduled`，写入下一次重试时间和 retry job，等待后续调度再次尝试。
- 文件：`server/internal/domains/reminder/service_test.go`
- 内容：保留 `TestServiceRequeuesReminderWhenProactiveVoiceQuotaUsed`，验证配额占用时不会调用 xiaozhi 注入、提醒仍在队列里、并产生 retry job。
- 文件：`server/internal/proactive/voice_gate_integration_test.go`
- 内容：保留跨域集成断言，验证 greeting 占用同设备主动语音窗口后，reminder 会重新排队而不是丢弃。
- 目的：让主动提醒符合 PRD “同一 10 分钟窗口至多 1 条主动语音输出，老人正在对话时排队等待”的路演基本纪律。
- 功能影响：只改变提醒遇到主动语音配额冲突时的状态流转；xiaozhi manager 注入失败仍然标记 `failed`，不掩盖真实设备/manager 故障；不修改 xiaozhi 上游代码。
- 验证：
  - `go test -count=1 ./internal/domains/reminder ./internal/proactive` 通过。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `npm test --prefix web` 通过，59 个 smoke tests 全绿。
  - `node --test --experimental-test-coverage smoke.test.mjs` 通过，59 个测试全绿；all files 行覆盖率 96.28%、函数覆盖率 97.37%。
  - `git diff --check` 通过；仅出现既有 Windows LF/CRLF 换行提示。

### docker-compose 可插拔部署 RED 架构测试

- 文件：`server/internal/architecture/architecture_test.go`
- 内容：新增 `TestDockerComposeKeepsAnbanOptionalByProfile`，要求 `docker-compose.yml` 中 `xiaozhi` 服务不能依赖 `anban`，且 `anban` 服务必须挂显式 profile，避免默认 `docker compose up` 把安伴当成必启服务。
- 目的：把方案 C“只部署 xiaozhi 也能正常对话，anban 是可选增强进程”的架构铁律落实到部署编排测试里。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `go test -count=1 ./internal/architecture`，得到有效 RED：`anban service must be behind an explicit profile`。

### docker-compose 可插拔部署 GREEN 实现

- 文件：`docker-compose.yml`
- 内容：为 `anban` 服务增加 `profiles: ["anban"]`，并补充注释说明默认 `docker compose up` 只启动 `xiaozhi`；需要联调安伴增强能力时显式运行 `docker compose --profile anban up`。
- 文件：`server/internal/architecture/architecture_test.go`
- 内容：保留 `TestDockerComposeKeepsAnbanOptionalByProfile`，并新增 `composeServiceBlock` 辅助函数，持续约束 `xiaozhi` 不依赖 `anban`、`anban` 必须挂显式 profile。
- 目的：把方案 C 的“两进程、可拔插”部署形态落到仓库默认编排，避免设备基础小智对话能力被安伴服务可用性绑定。
- 功能影响：默认 Compose 启动行为变为只启动 xiaozhi；安伴后端作为可选增强进程启动。未修改 xiaozhi 代码，未改变所有 xiaozhi 调用必须经过 `internal/xiaozhiclient` 的边界。
- 验证：
  - `go test -count=1 ./internal/architecture` 通过。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `npm test --prefix web` 通过，59 个 smoke tests 全绿。
  - `node --test --experimental-test-coverage smoke.test.mjs` 通过，59 个测试全绿；all files 行覆盖率 96.28%、函数覆盖率 97.37%。
  - `git diff --check` 通过；仅出现既有 Windows LF/CRLF 换行提示。

### status 快照缓存兜底 RED 测试

- 文件：`server/internal/domains/status/service_test.go`
- 内容：新增 `TestServiceGetFallsBackToCachedSnapshotWhenDeviceStatusFails`，要求 status 域先把一次成功读取到的设备状态/最近互动写入本地缓存；当 xiaozhi manager 后续不可用时，仍能返回同设备最近快照，并把在线状态标成离线，同时继续带上 message 域持久化的留言状态摘要。
- 目的：对齐 PRD #4“后端重启后状态信息不丢”和子女端状态兜底诉求，避免 manager 短暂不可用时子女端只看到 502。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `go test -count=1 ./internal/domains/status`，得到有效 RED：编译失败点为 `UseStore`、`Store`、`NewStore` 尚未实现。

### status 快照缓存兜底 GREEN 实现

- 文件：`server/internal/domains/status/types.go`
- 内容：新增 `SnapshotCache` 持久化模型和 `ErrNotFound`。
- 文件：`server/internal/domains/status/store.go`
- 内容：新增 status 域 Store，支持 `AutoMigrate`、按 `device_id` upsert 最近快照、读取最近快照。
- 文件：`server/internal/domains/status/service.go`
- 内容：`Service` 新增 `UseStore`；`Get` 成功读取 xiaozhi manager 状态后缓存 `lastSeenAt/lastInteractionAt`；当 `GetDeviceStatus` 失败且存在缓存时，返回缓存快照并将 `online=false`，同时继续读取 message 域留言状态摘要。
- 文件：`server/cmd/anban/main.go`
- 内容：启动时迁移 status 表，并把 status Store 注入 status Service。
- 文件：`server/internal/domains/status/service_test.go`
- 内容：保留 `TestServiceGetFallsBackToCachedSnapshotWhenDeviceStatusFails` 回归测试。
- 目的：让子女端状态在 manager 短暂不可用或 anban 重启后仍有最近一次可用状态，不把“妈还好”的信息链路退化成 502。
- 功能影响：不改变 xiaozhi 调用边界；不影响只部署 xiaozhi 的原始对话能力；anban 状态接口在有缓存时具备离线兜底。
- 验证：
  - `go test -count=1 ./internal/domains/status` 通过。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `npm test --prefix web` 通过，59 个 smoke tests 全绿。
  - `node --test --experimental-test-coverage smoke.test.mjs` 通过，59 个测试全绿；all files 行覆盖率 96.28%、函数覆盖率 97.37%。
  - `git diff --check` 通过；仅出现既有 Windows LF/CRLF 换行提示。

### 子女端提醒草稿未来时间校验 GREEN 实现

- 文件：`web/reminder-input.js`
- 内容：新增 `normalizeReminderDraft`，统一规整提醒内容和本地时间；空内容/空时间、无效时间、过去或当前时间都会返回不可提交状态和子女端提示。
- 目的：把“提醒只能创建在未来”这条规则前移到子女端，和后端 reminder 域的 `scheduledAt > now` 校验保持一致。
- 功能：子女端创建提醒时，过去/当前时间会先提示 `提醒时间要晚于现在`，不会调用后端；未来提醒会把时间转换为 ISO 字符串后提交。
- 文件：`web/app.js`
- 内容：提醒表单提交改为调用 `normalizeReminderDraft`，并使用规整后的 `draft.content` 和 `draft.scheduledAt` 创建提醒。
- 文件：`web/smoke.test.mjs`
- 内容：保留 `reminder draft normalizer rejects non-future times` 和 `reminder draft normalizer returns ISO time for future reminders` 回归测试；追加 `reminder draft normalizer rejects incomplete or invalid time input`，覆盖空内容/无效时间边界。
- 功能影响：仅增强子女端提醒创建输入校验；无后端 API 变更，无数据库结构变更，无 xiaozhi 上游代码变化。
- 验证：
  - `npm test --prefix web` 通过，59 个 smoke tests 全绿。
  - `node --test --experimental-test-coverage smoke.test.mjs` 通过，59 个测试全绿；all files 行覆盖率 96.28%、函数覆盖率 97.37%，`reminder-input.js` 行覆盖率 100%。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。

### 子女端提醒草稿未来时间校验 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增 `reminder draft normalizer rejects non-future times` 和 `reminder draft normalizer returns ISO time for future reminders`，要求 `normalizeReminderDraft` 拒绝过去/当前时间，并为未来提醒返回规整后的内容和 ISO 时间。
- 目的：后端已拒绝 `scheduledAt <= now`；子女端也应先拦住明显无效的提醒时间，避免现场创建提醒时只看到后端 400。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `npm test --prefix web`，得到有效 RED：两个新增测试失败，原因是 `web/reminder-input.js` 尚未实现。

### reminder 创建过去/当前时间 RED 测试

- 文件：`server/internal/domains/reminder/service_test.go`
- 内容：在 `TestServiceCreateValidatesAndNormalizesInput` 中新增 `past time` 和 `current time` 用例，要求 `scheduledAt <= now` 返回 `ErrInvalidInput`。
- 目的：`scheduler.ScheduleAt` 对过去时间会立即触发，`reminder.Create` 随后仍可能写回旧的 scheduled 状态；提醒应只接受未来时间，避免现场创建提醒时出现立即触发/状态回写竞态。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `go test -count=1 ./internal/domains/reminder`，得到有效 RED：新增 `past time` 和 `current time` 用例返回 `nil` 错误。

### reminder 创建未来时间校验 GREEN 实现

- 文件：`server/internal/domains/reminder/service.go`
- 内容：`Create` 新增 `scheduledAt := req.ScheduledAt.UTC()`，并要求 `scheduledAt` 非零且必须晚于 `s.now().UTC()`；保存时复用规整后的 UTC 时间。
- 目的：把“提醒是未来调度任务”的约束前移到 reminder 域入口，避免过去/当前时间进入 `scheduler.ScheduleAt` 后立即触发，造成创建流程与触发流程竞态。
- 功能：`scheduledAt <= now` 的提醒创建会返回 `ErrInvalidInput`，不会入库、不会创建一次性定时器、不会调用 xiaozhi 注入。
- 文件：`server/internal/domains/reminder/service_test.go`
- 内容：保留 `past time` 和 `current time` 回归测试。
- 功能影响：提醒创建现在只接受未来时间；无数据库结构变更，无 xiaozhi 上游代码变化。
- 验证：
  - `go test -count=1 ./internal/domains/reminder` 通过。
  - `go test -count=1 ./...` 通过。
  - `npm test --prefix web` 通过，56 个 smoke tests 全绿。
  - `node --test --experimental-test-coverage smoke.test.mjs` 通过，56 个测试全绿；all files 行覆盖率 95.75%、函数覆盖率 97.22%。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `git diff --check` 通过；仅出现既有 Windows LF/CRLF 换行提示。

### 子女端画像同步失败保留后端 profile RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增 `child web keeps backend-persisted profile when xiaozhi profile sync fails`，要求画像提交失败分支识别 `error.payload.profile`，并在显示错误前 `renderProfile` + `writeProfileForm`。
- 目的：profile 后端在 xiaozhi role prompt 同步失败时仍会返回已落库画像；子女端应展示这份已保存数据，避免 W2 编辑画像联调时误以为保存失败。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `npm test --prefix web`，得到有效 RED：新增测试失败，当前 catch 分支只调用 `handleApiError(error, '画像同步失败')`。

### 子女端画像同步失败保留后端 profile GREEN 实现

- 文件：`web/app.js`
- 内容：画像提交 catch 分支新增 `ApiError` payload 处理；当后端返回 `error.payload.profile` 时，先 `renderProfile(error.payload.profile)` 并 `writeProfileForm(error.payload.profile)`，再展示同步失败提示。
- 目的：让“编辑画像”在 xiaozhi manager role prompt 同步失败但 anban 已落库时仍有可见反馈，符合方案 C 的可插拔思路：anban 自身数据不因下游同步失败而在子女端消失。
- 功能：子女端同步画像遇到 `502 { error: "画像同步失败", profile: ... }` 时，页面会显示并回填后端已保存/规整后的画像，同时保留错误提示。
- 文件：`web/smoke.test.mjs`
- 内容：保留 `child web keeps backend-persisted profile when xiaozhi profile sync fails` 回归测试。
- 功能影响：仅增强子女端画像保存错误态体验；无后端 API 变更，无数据库结构变更，无 xiaozhi 上游代码变化。
- 验证：
  - `npm test --prefix web` 通过，56 个 smoke tests 全绿。
  - `node --test --experimental-test-coverage smoke.test.mjs` 通过，56 个测试全绿；all files 行覆盖率 95.75%、函数覆盖率 97.22%。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `git diff --check` 通过；仅出现既有 Windows LF/CRLF 换行提示。

### greeting 列表过滤规整 RED 测试

- 文件：`server/internal/domains/greeting/service_test.go`
- 内容：新增 `TestServiceListNormalizesFilters`，要求 `svc.List` 对 `ListFilter{DeviceID: "  dev-001  ", Status: "  played  "}` 仍能查到已触发并播报的问候。
- 目的：PRD #2 主动问候和 W2 子女端真后端联调会依赖问候触发结果；调试/后续列表入口不应因复制粘贴空白导致查空。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `go test -count=1 ./internal/domains/greeting`，得到有效 RED：新增测试失败，`List` 返回空列表。

### greeting 列表过滤规整 GREEN 实现

- 文件：`server/internal/domains/greeting/service.go`
- 内容：`Service.List` 在调用 store 前对 `ListFilter.DeviceID` 和 `ListFilter.Status` 执行 `strings.TrimSpace` 规整。
- 目的：让主动问候列表过滤和 message/reminder 域保持一致的输入容错，便于 W2 联调时排查某设备问候触发结果。
- 功能：后续若暴露问候历史列表或调试入口，即使 `deviceId/status` 带前后空白，也会按规整后的设备和状态查询。
- 文件：`server/internal/domains/greeting/service_test.go`
- 内容：保留 `TestServiceListNormalizesFilters` 回归测试。
- 功能影响：仅增强 greeting 域列表过滤容错；无数据库结构变更，无 xiaozhi 上游代码变化。
- 验证：
  - `go test -count=1 ./internal/domains/greeting` 通过。
  - `go test -count=1 ./...` 通过。
  - `npm test --prefix web` 通过，55 个 smoke tests 全绿。
  - `node --test --experimental-test-coverage smoke.test.mjs` 通过，55 个测试全绿；all files 行覆盖率 95.75%、函数覆盖率 97.22%。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `git diff --check` 通过；仅出现既有 Windows LF/CRLF 换行提示。

### reminder 列表过滤规整 RED 测试

- 文件：`server/internal/domains/reminder/service_test.go`
- 内容：新增 `TestServiceListNormalizesFilters`，要求 `svc.List` 对 `ListFilter{DeviceID: "  dev-001  ", Status: "  scheduled  "}` 仍能查到已创建的提醒。
- 目的：PRD #6 主动提醒和 W2 子女端联调都依赖提醒列表；现场复制粘贴设备 ID 或状态过滤值带空白时，后端不应返回空列表。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `go test -count=1 ./internal/domains/reminder`，得到有效 RED：新增测试失败，`List` 返回空列表。

### reminder 列表过滤规整 GREEN 实现

- 文件：`server/internal/domains/reminder/service.go`
- 内容：`Service.List` 在调用 store 前对 `ListFilter.DeviceID` 和 `ListFilter.Status` 执行 `strings.TrimSpace` 规整。
- 目的：让提醒列表过滤和 message/status/greeting 域保持一致的输入容错，避免子女端或手工 API 联调时因为复制粘贴空白看不到提醒。
- 功能：`GET /api/reminders?deviceId=dev-001&status=scheduled` 经 handler/service 时，即使 filter 值带前后空白，也会按 `dev-001` 和 `scheduled` 查询。
- 文件：`server/internal/domains/reminder/service_test.go`
- 内容：保留 `TestServiceListNormalizesFilters` 回归测试。
- 功能影响：仅增强 reminder 域列表过滤容错；无数据库结构变更，无 xiaozhi 上游代码变化。
- 验证：
  - `go test -count=1 ./internal/domains/reminder` 通过。
  - `npm test --prefix web` 通过，55 个 smoke tests 全绿。
  - `node --test --experimental-test-coverage smoke.test.mjs` 通过，55 个测试全绿；all files 行覆盖率 95.75%、函数覆盖率 97.22%。
  - 首次并行运行 `go test -count=1 ./...` 时，`internal/architecture` 曾因 sandbox 源码路径映射到 `C:\Users\CodexSandboxOffline\.codex\.sandbox\cwd\docker-compose.yml` 而无法读取 repo 根 `docker-compose.yml`；随后单独运行 `go test -count=1 .` 于 `server/internal/architecture` 通过，再次运行 `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `git diff --check` 通过；仅出现既有 Windows LF/CRLF 换行提示。

### 子女端 API client reminderId path 规整 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增 `API client normalizes pasted reminder id path values`，要求 `client.deleteReminder(" 9 ")` 请求 URL 为 `/api/reminders/9`。
- 目的：子女端提醒撤销/确认属于最小提醒闭环；path 参数如果带复制粘贴空白，会让后端收到错误 ID，基础容错应在 API client 层统一处理。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `npm test --prefix web`，得到有效 RED：新增测试失败，实际 URL 为 `http://anban.local/api/reminders/%209%20`。

### 子女端 API client reminderId path 规整 GREEN 实现

- 文件：`web/api/client.js`
- 内容：新增 `encodePathSegment(value)`，对 path 参数执行 `String(value ?? "").trim()` 后再 `encodeURIComponent`；`deleteReminder` 和 `ackReminder` 统一使用该 helper。
- 目的：让提醒撤销/确认接口在 API client 层具备复制粘贴容错，减少最小提醒闭环现场联调时的错误 ID 噪音。
- 功能：`client.deleteReminder(" 9 ")` 现在请求 `/api/reminders/9`；`ackReminder` 也会使用同样的 path 参数规整。
- 文件：`web/smoke.test.mjs`
- 内容：保留 `API client normalizes pasted reminder id path values` 回归测试。
- 功能影响：仅增强子女端 API client 输入容错；无后端 API 变更，无 xiaozhi 上游代码变化。
- 验证：
  - `npm test --prefix web` 通过，55 个 smoke tests 全绿。
  - `node --test --experimental-test-coverage smoke.test.mjs` 通过，55 个测试全绿；all files 行覆盖率 95.75%、函数覆盖率 97.22%，`api/client.js` 行覆盖率 95.19%。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `git diff --check` 通过；仅出现既有 Windows LF/CRLF 换行提示。

### 子女端 API client 设备 ID query 规整 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增 `API client normalizes pasted device id query values`，要求 `client.getStatus({ deviceId: "  dev-001  " })` 请求 URL 为 `/api/device/status?deviceId=dev-001`。
- 目的：PRD #3/#4/#6 的基础接口都依赖 `deviceId`；现场从 manager 复制设备 ID 时前后带空白，不应让子女端请求打到错误设备 ID。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `npm test --prefix web`，得到有效 RED：新增测试失败，实际 URL 为 `http://anban.local/api/device/status?deviceId=++dev-001++`。

### 子女端 API client query 参数规整 GREEN 实现

- 文件：`web/api/client.js`
- 内容：新增 `setQueryParam(params, name, value)`，对 query 参数执行 `String(value ?? "").trim()` 后再写入 `URLSearchParams`；`listMessages`、`getGreetingSchedule`、`listReminders`、`getStatus`、`getProfile` 统一使用该 helper。
- 目的：让 `deviceId/status` 等基础查询参数在 API client 层具备一致复制粘贴容错，减少子女端连接 manager 设备 ID 时的现场联调噪音。
- 功能：`client.getStatus({ deviceId: "  dev-001  " })` 现在请求 `/api/device/status?deviceId=dev-001`；空白 query 参数会被忽略。
- 文件：`web/smoke.test.mjs`
- 内容：保留 `API client normalizes pasted device id query values` 回归测试。
- 功能影响：仅增强子女端 API client 输入容错；无后端 API 变更，无 xiaozhi 上游代码变化。
- 验证：
  - `npm test --prefix web` 通过，54 个 smoke tests 全绿。
  - `node --test --experimental-test-coverage smoke.test.mjs` 通过，54 个测试全绿；all files 行覆盖率 95.69%、函数覆盖率 97.14%，`api/client.js` 行覆盖率 95.00%。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。

### 子女端 API client 访问码规整 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增 `API client normalizes pasted child access code`，要求 `createAnbanClient({ accessCode: "  demo-code  " })` 发起请求时，`X-Access-Code` 请求头为 `demo-code`。
- 目的：子女端最小联调常见输入来自复制粘贴；访问码前后带空白不应导致后端 401，基础容错应在 API client 层兜住。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `npm test --prefix web`，得到有效 RED：新增测试失败，实际请求头仍为 `  demo-code  `。

### 子女端 API client 访问码规整 GREEN 实现

- 文件：`web/api/client.js`
- 内容：`createAnbanClient` 初始化时新增 `childAccessCode = String(accessCode || "").trim()`，所有请求的 `X-Access-Code` 使用规整后的访问码。
- 目的：让子女端和后端 `childapi` 访问码 trim 行为保持一致，降低现场复制粘贴访问码导致 401 的基础联调风险。
- 功能：用户在子女端后端连接表单里粘贴 ` demo-code ` 时，API client 仍会发送 `X-Access-Code: demo-code`。
- 文件：`web/smoke.test.mjs`
- 内容：保留 `API client normalizes pasted child access code` 回归测试。
- 功能影响：仅增强子女端 API client 输入容错；无后端 API 变更，无 xiaozhi 上游代码变化。
- 验证：
  - `npm test --prefix web` 通过，53 个 smoke tests 全绿。
  - `node --test --experimental-test-coverage smoke.test.mjs` 通过，53 个测试全绿；all files 行覆盖率 95.60%、函数覆盖率 97.06%，`api/client.js` 行覆盖率 94.74%。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。

### preflight Gate D 可插拔性手工项 RED 测试

- 文件：`server/internal/preflight/preflight_test.go`
- 内容：新增/扩展 preflight 报告测试，要求 `Run` 输出 `anban optionality smoke` 手工检查项，并在格式化报告中提示 `Gate D`、停掉 `anban` 后原版小智仍应能对话。
- 目的：把方案 C 的“安伴是可选增强进程，停掉它不应影响原版 xiaozhi”从部署文档推进到可执行预检报告，避免只验证 Gate A 后忘记做可插拔性复核。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `go test -count=1 ./internal/preflight`，得到有效 RED：报告中缺少 `anban optionality smoke` 检查项。

### preflight Gate D 可插拔性手工项 GREEN 实现

- 文件：`server/internal/preflight/preflight.go`
- 内容：`Run` 的初始手工检查项中新增 `anban optionality smoke`，提示完成安伴最小联调后停掉 `anban`，再确认设备仍可继续原版小智对话。
- 目的：让预检报告同时提醒 Gate A 和 Gate D，守住方案 C 的可插拔边界：xiaozhi 独立可用，anban 只是可选增强。
- 功能：`anban-preflight` 输出会多一条 `[MANUAL] anban optionality smoke - Gate D: ...`；该项不调用、不修改 xiaozhi，也不改变命令退出码。
- 文件：`server/internal/preflight/preflight_test.go`
- 内容：保留 Gate D 手工项和格式化文案的回归测试。
- 功能影响：仅增强预检报告；无业务 API 变更，无 xiaozhi 上游代码变化。
- 验证：
  - `go test -count=1 ./internal/preflight` 通过。
  - `go test -count=1 -cover ./internal/preflight` 通过，覆盖率 97.0%。
  - `go test -count=1 ./cmd/anban-preflight` 通过。
  - `go test -count=1 ./...` 通过。
  - `npm test --prefix web` 通过，52 个 smoke tests 全绿。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `git diff --check` 无空白错误，仅提示既有 LF/CRLF 换行警告。

### preflight Gate A 环境变量空白容错 RED 测试

- 文件：`server/cmd/anban-preflight/main_test.go`
- 内容：新增 `TestRunCommandAcceptsTrimmedGateConfirmationFromEnv`，把 `ANBAN_PREFLIGHT_XIAOZHI_GATE_PASSED` 设置为 ` true `，要求预检命令仍视为 Gate A 已人工确认。
- 目的：方案 C 联调前必须先确认原版小智唤醒、回应、打断；现场 `.env` 复制粘贴带空白时，不应让已经确认的 Gate A 被误判为未确认。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `go test -count=1 ./cmd/anban-preflight`，得到有效 RED：新增测试失败，stderr 提示 Gate A not confirmed。

### preflight Gate A 环境变量空白容错 GREEN 实现

- 文件：`server/cmd/anban-preflight/main.go`
- 内容：`envBool` 读取 `ANBAN_PREFLIGHT_XIAOZHI_GATE_PASSED` 时先执行 `strings.TrimSpace`，再匹配 `true/yes/1` 等确认值。
- 目的：和已有 manager URL/token trim 行为保持一致，减少方案 C 现场预检时因 `.env` 空白字符造成的误失败。
- 功能：`ANBAN_PREFLIGHT_XIAOZHI_GATE_PASSED=" true "` 现在会被识别为 Gate A 已确认；原有 `true`、`yes`、`1` 行为不变。
- 文件：`server/cmd/anban-preflight/main_test.go`
- 内容：保留 `TestRunCommandAcceptsTrimmedGateConfirmationFromEnv` 回归测试。
- 功能影响：仅增强 anban 预检命令的配置容错；不调用、不修改 xiaozhi 原版语音链路。
- 验证：
  - `go test -count=1 ./cmd/anban-preflight` 通过。
  - `go test -count=1 -cover ./cmd/anban-preflight` 通过，覆盖率 86.2%。
  - `npm test --prefix web` 通过，52 个 smoke tests 全绿。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。

### 可选运行配置规整 RED 测试

- 文件：`server/internal/config/config_test.go`
- 内容：新增 `TestLoadTrimsOptionalEnvValues`，要求 `ANBAN_DB_DSN`、`ANBAN_LISTEN_ADDR`、`ANBAN_ALLOWED_ORIGINS` 这类可选配置在读取时规整首尾空白。
- 目的：降低现场 `.env` 复制粘贴导致监听地址或 sqlite 路径带空格的部署风险，服务于方案 C 本地/路演联调。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `go test -count=1 ./internal/config`，得到有效 RED：新增测试失败，原因是 `ANBAN_DB_DSN` 仍保留首尾空白。

### 可选运行配置规整 GREEN 实现

- 文件：`server/internal/config/config.go`
- 内容：`envOr` 读取可选环境变量时先执行 `strings.TrimSpace`；规整后为空仍返回默认值。
- 目的：让可选配置与必填配置的复制粘贴容错一致，避免 `.env` 里的空白字符变成真实监听地址或 sqlite 文件路径。
- 功能：`ANBAN_DB_DSN=" ./data/anban.db "` 会按 `./data/anban.db` 使用；`ANBAN_LISTEN_ADDR=" :8091 "` 会按 `:8091` 使用；纯空白仍走默认值。
- 文件：`server/internal/config/config_test.go`
- 内容：保留 `TestLoadTrimsOptionalEnvValues` 回归测试。
- 功能影响：仅增强 anban 自身配置读取容错；无 xiaozhi 上游代码变化，不影响只部署原版 xiaozhi 的独立对话能力。
- 验证：
  - `go test -count=1 ./internal/config` 通过。
  - `go test -count=1 -cover ./internal/config` 通过，覆盖率 90.0%。
  - `npm test --prefix web` 通过，52 个 smoke tests 全绿。
  - `node --test --experimental-test-coverage smoke.test.mjs` 通过，52 个测试全绿；all files 行覆盖率 95.58%、函数覆盖率 97.06%，`status-summary.js` 行覆盖率 100%。
  - `go test -count=1 ./...` 通过。
  - `go build ./...` 通过。
  - `go vet ./...` 通过。
  - `git diff --check` 无空白错误，仅提示既有 LF/CRLF 换行警告。

### 子女端无状态快照时本地留言构造状态卡片 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增 `status display can be built from local messages before backend status exists`，要求 `status-summary.js` 导出 `buildStatusSnapshotForDisplay`，在没有后端状态快照但已有本地留言结果时，也能生成可展示的状态快照。
- 目的：补齐子女端基础闭环的边角场景；如果用户未先刷新设备状态就发送留言，顶部状态卡片也应能立即显示“最新留言：已播报”。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `npm test --prefix web`，得到有效 RED：新增测试失败，原因是 `buildStatusSnapshotForDisplay` 还不存在。

### 子女端无状态快照时本地留言构造状态卡片 GREEN 实现

- 文件：`web/status-summary.js`
- 内容：新增 `buildStatusSnapshotForDisplay`，优先合并后端状态快照和本地消息列表；没有后端快照但有本地消息时，生成可展示的最小状态快照。
- 目的：把状态卡片的数据合成逻辑收口在可单测模块里，避免 `app.js` 各路径重复判断。
- 功能：未先拿到后端状态快照时，发送留言成功后也能显示“最新留言：已播报”；失败 message 会被合成为离线/失败状态。
- 文件：`web/app.js`
- 内容：`renderCurrentBackendStatus` 改为调用 `buildStatusSnapshotForDisplay`，统一处理有/无后端状态快照两类场景。
- 目的：让发送结果、消息轮询和设备状态轮询共用同一套状态卡片合成逻辑。
- 文件：`web/smoke.test.mjs`
- 内容：保留无状态快照时由本地消息构造状态卡片的回归测试，并补充有后端快照时本地消息覆盖摘要、无消息返回空状态的覆盖。
- 功能影响：仅增强子女端 web 状态展示；无后端 API 变更，无 xiaozhi 上游代码变化，不影响只部署原版 xiaozhi 的独立对话能力。
- 验证：
  - 已运行 `npm test --prefix web`，52 个 smoke 测试通过。
  - 已运行 `node --test --experimental-test-coverage smoke.test.mjs`，52 个 smoke 测试通过；all files line 95.58%，funcs 97.06%，`status-summary.js` line 100%。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go test -count=1 ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go build ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go vet ./...`，通过。
  - 已运行 `git diff --check`，无空白错误，仅提示 Windows 下 LF 后续会被 Git 转为 CRLF。

### 子女端发送结果立即刷新状态卡片 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增 `child web refreshes status card immediately after send result changes messages`，并限定检查 `messageForm` handler 内部，要求发送成功和后端返回失败 message payload 两条路径都在 `renderMessages()` 后调用 `renderCurrentBackendStatus()`。
- 目的：对齐 PRD #4 的留言状态可见性；子女点击发送后应立即看到顶部“最新留言：已播报/失败”，不必等待下一次留言轮询。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `npm test --prefix web`，得到有效 RED：新增测试失败，原因是发送分支仍使用临时状态文案且没有重绘当前后端状态卡片。

### 子女端发送结果立即刷新状态卡片 GREEN 实现

- 文件：`web/app.js`
- 内容：留言发送成功后、以及发送失败但后端返回失败 message payload 后，均在 `renderMessages()` 后调用 `renderCurrentBackendStatus()`。
- 目的：让子女端“发送留言”这条最核心操作立即同步顶部状态卡片，不依赖后续 10 秒轮询。
- 功能：发送成功时顶部状态卡片会立刻按当前消息列表显示“最新留言：已播报”；发送失败且后端返回失败 message 时会立刻显示“最新留言：失败”。
- 文件：`web/smoke.test.mjs`
- 内容：保留发送成功/失败 payload 后刷新状态卡片的回归测试。
- 功能影响：仅增强子女端 web 状态展示；无后端 API 变更，无 xiaozhi 上游代码变化，不影响只部署原版 xiaozhi 的独立对话能力。
- 验证：
  - 已运行 `npm test --prefix web`，50 个 smoke 测试通过。
  - 已运行 `node --test --experimental-test-coverage smoke.test.mjs`，50 个 smoke 测试通过；all files line 95.24%，funcs 96.97%，`status-summary.js` line 100%。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go test -count=1 ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go build ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go vet ./...`，通过。
  - 已运行 `git diff --check`，无空白错误，仅提示 Windows 下 LF 后续会被 Git 转为 CRLF。

### 子女端消息轮询刷新状态卡片 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增 `child web refreshes status card after message status polling updates`，要求 `app.js` 保存后端状态快照，并在 `refreshBackendMessages` 的 10 秒留言状态轮询更新消息列表后重渲染状态卡片。
- 目的：对齐 PRD #4“留言状态 pending → played 延迟 ≤ 10 秒”，避免顶部“最新留言”只跟随 30 秒设备状态轮询而滞后。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `npm test --prefix web`，得到有效 RED：新增测试失败，原因是 `app.js` 还没有 `statusSnapshot` 和 `renderCurrentBackendStatus`。

### 子女端消息轮询刷新状态卡片 GREEN 实现

- 文件：`web/app.js`
- 内容：新增 `state.statusSnapshot`、`updateStatusSnapshot` 和 `renderCurrentBackendStatus`；设备状态轮询更新快照，留言状态轮询更新 `state.messages` 后同步重渲染顶部状态卡片。
- 目的：让子女端顶部状态卡片里的“最新留言”跟随 10 秒留言状态轮询刷新，避免等待 30 秒设备状态轮询。
- 功能：留言列表从 `pending` 更新为 `played/failed` 后，状态卡片也会同步显示最新留言状态；连接信息失效时会清空缓存快照。
- 文件：`web/smoke.test.mjs`
- 内容：保留消息轮询刷新状态卡片的回归测试。
- 功能影响：仅增强子女端 web 状态刷新；无后端 API 变更，无 xiaozhi 上游代码变化，不影响只部署原版 xiaozhi 的独立对话能力。
- 验证：
  - 已运行 `npm test --prefix web`，49 个 smoke 测试通过。
  - 已运行 `node --test --experimental-test-coverage smoke.test.mjs`，49 个 smoke 测试通过；all files line 95.24%，funcs 96.97%，`status-summary.js` line 100%。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go test -count=1 ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go build ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go vet ./...`，通过。
  - 已运行 `git diff --check`，无空白错误，仅提示 Windows 下 LF 后续会被 Git 转为 CRLF。

### 子女端状态卡片展示最新留言状态 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增 `status detail surfaces latest message playback state`，要求状态详情格式化函数把后端 `snapshot.messages[0].status` 显示为“最新留言：已播报”。
- 目的：对齐 PRD #4“设备在线 + 最近互动 + 自己刚发的留言播了没”的子女端状态卡片基础体验，让连接后第一眼就能看到最新留言播报状态。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `npm test --prefix web`，得到有效 RED：新增测试失败，原因是 `web/status-summary.js` 尚不存在。

### 子女端状态卡片展示最新留言状态 GREEN 实现

- 文件：`web/status-summary.js`
- 内容：新增 `formatStatusDetail` 和 `messageStatusLabel`，把 `lastInteractionAt/lastSeenAt` 与最新留言状态组合成状态卡片详情。
- 目的：把 PRD #4 的状态摘要文案做成可独立测试的前端小模块，避免状态卡片逻辑散在 `app.js` 里。
- 功能：状态卡片现在会显示类似 `最近互动：06/01 08:30 · 最新留言：已播报`；没有留言时仍显示原来的最近互动/暂无最近互动。
- 文件：`web/app.js`
- 内容：`renderBackendStatus` 改为调用 `formatStatusDetail`，留言列表状态标签复用 `messageStatusLabel`。
- 目的：让顶部状态卡片和留言列表的状态文案保持一致。
- 文件：`web/smoke.test.mjs`
- 内容：保留最新留言状态展示回归测试，并补充无留言 fallback、失败/排队状态标签覆盖。
- 功能影响：仅增强子女端 web 状态展示；无后端 API 变更，无 xiaozhi 上游代码变化，不影响只部署原版 xiaozhi 的独立对话能力。
- 验证：
  - 已运行 `npm test --prefix web`，48 个 smoke 测试通过。
  - 已运行 `node --test --experimental-test-coverage smoke.test.mjs`，48 个 smoke 测试通过；all files line 95.24%，funcs 96.97%，`status-summary.js` line 100%。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go test -count=1 ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go build ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go vet ./...`，通过。
  - 已运行 `git diff --check`，无空白错误，仅提示 Windows 下 LF 后续会被 Git 转为 CRLF。

## 2026-06-10

### 留言列表 deviceId query 规整 RED 测试

- 文件：`server/internal/domains/message/handler_test.go`
- 内容：新增 `TestHandlerListMessagesTrimsDeviceIDQuery`，先创建一条 `dev-001` 留言，再用 `GET /api/messages?deviceId=%20dev-001%20` 查询，要求仍能查到该留言。
- 目的：现场联调和子女端手工输入设备 ID 时，复制粘贴首尾空白不应导致留言列表误判为空；这是 PRD #3/#4 的基础闭环稳定性。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `go test -count=1 ./internal/domains/message`，得到有效 RED：新增测试失败，原因是列表查询过滤条件未规整 `deviceId`。

### 留言列表 deviceId/status 规整 GREEN 实现

- 文件：`server/internal/domains/message/service.go`
- 内容：`Service.List` 在进入 store 查询前，对 `filter.DeviceID` 和 `filter.Status` 执行 `strings.TrimSpace`。
- 目的：把留言域的输入规整收口在 service 边界，保证 handler query、未来 childapi 编排或其他调用路径都得到一致行为。
- 功能：`GET /api/messages?deviceId=%20dev-001%20` 现在能返回 `dev-001` 留言；`status` 查询同样容忍首尾空白。
- 文件：`server/internal/domains/message/handler_test.go`
- 内容：保留 `TestHandlerListMessagesTrimsDeviceIDQuery` 回归测试。
- 功能影响：仅增强 anban 留言列表查询容错；无上游 xiaozhi 代码变化，不影响只部署原版 xiaozhi 的独立对话能力。
- 验证：
  - 已运行 `go test -count=1 ./internal/domains/message`，通过。
  - 已运行 `go test -count=1 -cover ./internal/domains/message`，message 包覆盖率 91.1%。
  - 已运行 `npm test --prefix web`，46 个 smoke 测试通过。
  - 已运行 `node --test --experimental-test-coverage smoke.test.mjs`，46 个 smoke 测试通过；all files line 94.63%，funcs 96.67%。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go test -count=1 ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go build ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go vet ./...`，通过。
  - 已运行 `git diff --check`，无空白错误，仅提示 Windows 下 LF 后续会被 Git 转为 CRLF。

### xiaozhi 设备状态时间规整 RED 测试

- 文件：`server/internal/xiaozhiclient/http_client_test.go`
- 内容：新增 `TestGetDeviceStatusTrimsManagerTimeFields`，模拟 manager 设备列表返回 `last_active_at` 带首尾空白，要求 `GetDeviceStatus` 仍能解析出最近活跃时间。
- 目的：设备联调时 manager 响应字段若带格式空白，不应让状态页整体失败；规整应收口在 `xiaozhiclient` 边界。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `go test ./internal/xiaozhiclient`，得到有效 RED：新增测试失败，原因是 `time.Parse` 未 trim 时间字符串。

### xiaozhi 设备状态时间规整 GREEN 实现

- 文件：`server/internal/xiaozhiclient/http_client.go`
- 内容：`parseOptionalTime` 在解析 RFC3339 时间前先执行 `strings.TrimSpace`。
- 目的：把 manager OpenAPI 返回字段的首尾空白容错收口在 xiaozhi 适配器边界，避免状态域和 childapi 处理格式细节。
- 功能：`GetDeviceStatus` 和 `GetHistory` 解析时间字段时可接受首尾空白；非法时间仍返回错误。
- 文件：`server/internal/xiaozhiclient/http_client_test.go`
- 内容：保留 `TestGetDeviceStatusTrimsManagerTimeFields` 回归测试。
- 功能影响：仅增强 anban 读取 xiaozhi manager 响应的容错；无上游 xiaozhi 代码变化，不影响只部署原版 xiaozhi 的独立对话能力。
- 验证：
  - 已运行 `go test -count=1 ./internal/xiaozhiclient`，通过。
  - 已运行 `go test -count=1 -cover ./internal/xiaozhiclient`，xiaozhiclient 包覆盖率 87.5%。
  - 已运行 `npm test --prefix web`，46 个 smoke 测试通过。
  - 已运行 `node --test --experimental-test-coverage smoke.test.mjs`，46 个 smoke 测试通过；all files line 94.63%，funcs 96.67%。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go test -count=1 ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go build ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go vet ./...`，通过。
  - 已运行 `git diff --check`，无空白错误，仅提示 Windows 下 LF 后续会被 Git 转为 CRLF。

### childapi 空访问码配置拒绝 RED 测试

- 文件：`server/internal/childapi/accesscode_test.go`
- 内容：新增 `TestAccessCodeMiddlewareRejectsEmptyConfiguredCode`，要求 `RequireAccessCode("")` 不因为请求头同样为空而放行 API。
- 目的：虽然主配置层已要求 `ANBAN_ACCESS_CODE` 必填，但 childapi 中间件自身也要守住空配置不放行的安全边界，避免测试或未来装配路径误传空访问码。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `go test ./internal/childapi`，得到有效 RED：新增测试失败，空配置下请求返回 200。

### childapi 空访问码配置拒绝 GREEN 实现

- 文件：`server/internal/childapi/accesscode.go`
- 内容：`RequireAccessCode` 初始化时 trim 配置访问码；如果配置访问码为空，所有受保护 API 均返回 401。
- 目的：让 childapi 中间件自身具备安全默认值，避免未来测试装配或替代启动路径误传空访问码导致 API 无保护。
- 功能：正常配置访问码不变；配置码带首尾空白会被规整；配置码为空时不再放行空 header。
- 文件：`server/internal/childapi/accesscode_test.go`
- 内容：新增空配置拒绝回归测试。
- 目的：防止访问码中间件后续重构时再次出现空配置放行。
- 功能影响：仅后端 childapi 访问码安全边界变化；无 xiaozhi 上游代码变化，不影响只部署原版 xiaozhi 的独立对话能力。
- 验证：
  - 已运行 `go test -count=1 ./internal/childapi`，通过。
  - 已运行 `go test -count=1 -cover ./internal/childapi`，childapi 包覆盖率 93.0%。
  - 已运行 `npm test --prefix web`，46 个 smoke 测试通过。
  - 已运行 `node --test --experimental-test-coverage smoke.test.mjs`，46 个 smoke 测试通过；all files line 94.63%，funcs 96.67%。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go test -count=1 ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go build ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go vet ./...`，通过。
  - 已运行 `git diff --check`，无空白错误，仅提示 Windows 下 LF 后续会被 Git 转为 CRLF。

## 2026-06-09

### childapi 访问码 header 规整 RED 测试

- 文件：`server/internal/childapi/accesscode_test.go`
- 内容：新增 `TestAccessCodeMiddlewareTrimsHeaderValue`，要求 `X-Access-Code` 请求头带首尾空白时也能通过访问码校验。
- 目的：与配置加载和子女端连接表单的 trim 行为保持一致，减少设备联调时复制粘贴访问码导致的 401。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `go test ./internal/childapi`，得到有效 RED：新增测试失败，返回 401。

### childapi 访问码 header 规整 GREEN 实现

- 文件：`server/internal/childapi/accesscode.go`
- 内容：`RequireAccessCode` 比较访问码前对 `X-Access-Code` 请求头执行 `strings.TrimSpace`。
- 目的：让后端访问码入口与配置加载、子女端表单 trim 行为一致，降低复制粘贴访问码造成的联调摩擦。
- 功能：`X-Access-Code: " secret "` 现在会按 `secret` 校验；错误访问码仍返回 401。
- 文件：`server/internal/childapi/accesscode_test.go`
- 内容：新增访问码 header 规整回归测试。
- 目的：防止后续中间件重构时重新对首尾空白敏感。
- 功能影响：仅后端 childapi 访问码容错变化；无 xiaozhi 上游代码变化，不影响只部署原版 xiaozhi 的独立对话能力。
- 验证：
  - 已运行 `go test -count=1 ./internal/childapi`，通过。
  - 已运行 `go test -count=1 -cover ./internal/childapi`，childapi 包覆盖率 92.9%。
  - 已运行 `npm test --prefix web`，46 个 smoke 测试通过。
  - 已运行 `node --test --experimental-test-coverage smoke.test.mjs`，46 个 smoke 测试通过；all files line 94.63%，funcs 96.67%。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go test -count=1 ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go build ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go vet ./...`，通过。
  - 已运行 `git diff --check`，无空白错误，仅提示 Windows 下 LF 后续会被 Git 转为 CRLF。

### 子女端连接成功后才启动轮询 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增 `child web starts polling only after a successful backend refresh`，要求连接表单只有在 `await refreshMessages()` 返回成功后才启动状态、留言、提醒轮询，并要求 `refreshMessages` 明确返回 `true/false`。
- 目的：访问码错误、后端不可达或业务域占位时，不应继续启动三类轮询反复请求后端；这会干扰设备现场联调。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `npm test --prefix web`，得到有效 RED：新增测试失败，失败点为连接流程仍无条件 `await refreshMessages()` 后启动轮询。

### 子女端连接成功后才启动轮询 GREEN 实现

- 文件：`web/app.js`
- 内容：`refreshMessages()` 成功完成状态、留言、提醒、问候时段和画像刷新后返回 `true`；业务域占位或连接失败时返回 `false`。连接表单提交后如果 `await refreshMessages()` 为 false，直接返回，不启动三类轮询。
- 目的：访问码错误、后端不可达或骨架占位时，让子女端停在明确失败状态，避免后台轮询持续打无效请求。
- 功能：连接成功才进入持续刷新；连接失败仍会显示原有错误提示，但不会开启状态/留言/提醒轮询。
- 文件：`web/smoke.test.mjs`
- 内容：新增连接成功后才启动轮询的回归测试。
- 目的：防止后续连接流程重构时重新在失败后启动轮询。
- 功能影响：仅子女端连接状态管理变化；无 xiaozhi 上游代码变化，不影响只部署原版 xiaozhi 的独立对话能力。
- 验证：
  - 已运行 `npm test --prefix web`，46 个 smoke 测试通过。
  - 已运行 `node --test --experimental-test-coverage smoke.test.mjs`，46 个 smoke 测试通过；all files line 94.63%，funcs 96.67%。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go test -count=1 ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go build ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go vet ./...`，通过。
  - 已运行 `git diff --check`，无空白错误，仅提示 Windows 下 LF 后续会被 Git 转为 CRLF。

### 子女端无效连接清旧数据 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增 `child web clears stale device data when connection settings become invalid`，要求无效连接分支调用 `clearConnectionData()`，并要求该 helper 清空 messages/reminders、重新渲染列表、清空画像。
- 目的：用户切换或清空设备 ID 后，页面不应一边显示“未连接”，一边保留上一台设备的留言、提醒或画像示例，避免现场联调误判。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `npm test --prefix web`，得到有效 RED：新增测试失败，失败点为无效连接分支缺少 `clearConnectionData()`。

### 子女端无效连接清旧数据 GREEN 实现

- 文件：`web/app.js`
- 内容：新增 `clearConnectionData()`，清空 `state.messages` 和 `state.reminders`，重新渲染留言/提醒空列表，并复用 `clearProfile()` 清空画像；无效连接分支在停止轮询后调用该 helper。
- 目的：用户清空访问码或设备 ID 后，页面立即清掉上一台设备的业务数据，避免“未连接”状态下仍展示旧留言、旧提醒或旧画像。
- 功能：有效连接流程不变；无效连接现在会同时停止轮询、清旧数据、显示未连接提示。
- 文件：`web/smoke.test.mjs`
- 内容：新增无效连接清旧数据回归测试。
- 目的：防止连接入口后续重构时保留上一台设备的数据。
- 功能影响：仅子女端本地显示状态变化；无 xiaozhi 上游代码变化，不影响只部署原版 xiaozhi 的独立对话能力。
- 验证：
  - 已运行 `npm test --prefix web`，45 个 smoke 测试通过。
  - 已运行 `node --test --experimental-test-coverage smoke.test.mjs`，45 个 smoke 测试通过；all files line 94.63%，funcs 96.67%。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go test -count=1 ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go build ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go vet ./...`，通过。
  - 已运行 `git diff --check`，无空白错误，仅提示 Windows 下 LF 后续会被 Git 转为 CRLF。

### 子女端无效连接停止轮询 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增 `child web stops existing polling when connection settings become invalid`，要求连接表单变成无效配置时，在提示“访问码和设备 ID 必填”前调用 `stopConnectionPolling()`，并存在统一停止轮询 helper。
- 目的：用户曾经连接成功后，如果现场把访问码或设备 ID 清空再点连接，旧轮询不应继续用空配置请求后端。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `npm test --prefix web`，得到有效 RED：新增测试失败，失败点为无效连接分支缺少 `stopConnectionPolling()`。

### 子女端无效连接停止轮询 GREEN 实现

- 文件：`web/app.js`
- 内容：新增 `stopConnectionPolling()`，统一停止设备状态、留言状态、提醒状态三个轮询，并把对应 timer ID 清为 `null`；连接表单无效配置分支在提示前调用该 helper。
- 目的：用户曾经连接成功后再清空访问码或设备 ID 时，前端立即停止旧轮询，避免继续产生无效后端请求。
- 功能：有效连接流程不变；无效连接会停掉旧轮询并停留在“未连接”状态。
- 文件：`web/smoke.test.mjs`
- 内容：新增无效连接停止轮询回归测试。
- 目的：防止后续重构连接入口时遗漏旧轮询清理。
- 功能影响：仅子女端本地连接状态管理变化；无 xiaozhi 上游代码变化，不影响只部署原版 xiaozhi 的独立对话能力。
- 验证：
  - 已运行 `npm test --prefix web`，44 个 smoke 测试通过。
  - 已运行 `node --test --experimental-test-coverage smoke.test.mjs`，44 个 smoke 测试通过；all files line 94.63%，funcs 96.67%。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go test -count=1 ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go build ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go vet ./...`，通过。
  - 已运行 `git diff --check`，无空白错误，仅提示 Windows 下 LF 后续会被 Git 转为 CRLF。

### 子女端连接必填校验 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增 `child web validates connection settings before backend calls`，要求连接表单在 `refreshMessages()` 前先检查 `state.accessCode` 和 `state.deviceId`，并显示“访问码和设备 ID 必填”。
- 目的：设备现场联调时，避免子女端把空访问码或空设备 ID 写入 localStorage 后再请求后端，导致一串 400/401 错误不易定位。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `npm test --prefix web`，得到有效 RED：新增测试失败，失败点为本地连接校验提示缺失。

### 子女端连接必填校验 GREEN 实现

- 文件：`web/app.js`
- 内容：连接表单提交时，在 trim 访问码、设备 ID、后端地址后，先检查访问码和设备 ID；缺失则显示“访问码和设备 ID 必填”，状态停在“未连接”，并提前返回。
- 目的：减少设备现场联调的入口错误，把缺访问码/设备 ID 这种本地问题挡在前端，不污染 localStorage，也不向后端发无效请求。
- 功能：访问码和设备 ID 都填写后，原来的保存配置、刷新状态、启动轮询流程不变；任一为空时不会继续连接流程。
- 文件：`web/smoke.test.mjs`
- 内容：新增连接必填校验回归测试，确认本地校验发生在后端请求前。
- 目的：防止后续改连接流程时重新把空配置发到后端。
- 功能影响：仅子女端本地连接体验变化；无 xiaozhi 上游代码变化，不影响只部署原版 xiaozhi 的独立对话能力。
- 验证：
  - 已运行 `npm test --prefix web`，43 个 smoke 测试通过。
  - 已运行 `node --test --experimental-test-coverage smoke.test.mjs`，43 个 smoke 测试通过；all files line 94.63%，funcs 96.67%。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go test -count=1 ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go build ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go vet ./...`，通过。
  - 已运行 `git diff --check`，无空白错误，仅提示 Windows 下 LF 后续会被 Git 转为 CRLF。

### 子女端无画像清空示例 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增 `child web clears sample profile when backend has no saved profile`，要求 `refreshProfile` 在后端返回画像 404 时调用 `clearProfile()`，清掉 HTML 里预置的示例画像。
- 目的：真实后端尚无家庭画像时，避免子女端仍显示“王阿姨 · 喜欢豫剧”等示例内容，误导现场联调判断。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `npm test --prefix web`，得到有效 RED：新增测试失败，失败点为 `clearProfile()` 未出现。

### 子女端无画像清空示例 GREEN 实现

- 文件：`web/app.js`
- 内容：新增 `clearProfile()`，在 `refreshProfile` 捕获画像 404 或 501 时把概要改成“暂无画像”，并用空 fields 写回画像表单。
- 目的：连接真实后端但尚未保存家庭画像时，清掉静态 HTML 里的演示样例，避免现场误判“画像已经同步”。
- 功能：后端无画像或 profile 域仍为占位时，子女端显示明确的空画像状态；保存画像成功后仍按后端返回值渲染并回填表单。
- 文件：`web/smoke.test.mjs`
- 内容：新增无画像清空示例回归测试。
- 目的：防止后续重构时重新把 404/501 静默吞掉并保留示例画像。
- 功能影响：仅子女端显示状态变化；无 xiaozhi 上游代码变化，不影响只部署原版 xiaozhi 的独立对话能力。
- 验证：
  - 已运行 `npm test --prefix web`，42 个 smoke 测试通过。
  - 已运行 `node --test --experimental-test-coverage smoke.test.mjs`，42 个 smoke 测试通过；all files line 94.63%，funcs 96.67%。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go test -count=1 ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go build ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go vet ./...`，通过。
  - 已运行 `git diff --check`，无空白错误，仅提示 Windows 下 LF 后续会被 Git 转为 CRLF。

### 方案 C 部署现场速查文档

- 文件：`docs/deployment/方案C部署与联调指南.md`
- 内容：在指南开头新增“现场速查”章节，按只跑 xiaozhi、签 manager token、启动 anban、打开子女端 web 四步整理设备到场后的最短执行路径，并补充 PowerShell 命令清单。
- 目的：把方案 C 的两进程边界和现场联调步骤写成可照做的 Runbook，避免把本仓库误当成 xiaozhi 仓库，也避免 Gate A 未过时继续堆安伴功能。
- 功能：新增 Gate A-D 速查表，明确纯 xiaozhi、manager 接入、子女端闭环、可插拔性的验收点和未通过时的排查方向。
- 功能影响：仅文档变更；无 xiaozhi 上游代码变化，不影响只部署原版 xiaozhi 的独立对话能力。
- 验证：已运行 `git diff --check -- docs/deployment/方案C部署与联调指南.md docs/REALTIME_CHANGELOG.md`，无空白错误，仅提示 Windows 下 LF 后续会被 Git 转为 CRLF。

### 子女端后端地址规整 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增 `API client normalizes pasted backend base URL`，要求 `createAnbanClient` 在 `baseURL` 带首尾空白和多个尾斜杠时，仍请求干净的 `http://anban.local/api/messages?...`。
- 目的：设备到场联调时子女端常会复制粘贴后端地址；该测试防止 `http://localhost:8090/ ` 这类输入拼出错误 API URL。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `npm test --prefix web`，得到有效 RED：新增测试失败，实际 URL 为 `'  http://anban.local///  /api/messages?deviceId=dev-001'`。

### 子女端后端地址规整 GREEN 实现

- 文件：`web/api/client.js`
- 内容：`createAnbanClient` 初始化 `root` 时改为 `baseURL.trim().replace(/\/+$/, '')`，同时处理首尾空白和多个尾斜杠。
- 目的：降低设备联调时子女端输入后端地址的摩擦，避免复制粘贴导致所有 API 请求 URL 拼错。
- 功能：后端地址 `http://localhost:8090/ `、`http://localhost:8090///` 都会规整成 `http://localhost:8090` 再拼 API 路径；空字符串仍保持相对路径模式。
- 文件：`web/smoke.test.mjs`
- 内容：新增后端地址规整回归测试。
- 目的：防止前端 API client 后续重构时退回只删除单个尾斜杠。
- 功能影响：无 xiaozhi 上游代码变化，不影响只部署原版 xiaozhi 的独立对话能力。
- 验证：已运行 `npm test --prefix web`，41 个 smoke 测试通过。

## 2026-06-08

### 00:00 childapi 留言列表路由 RED 测试

- 文件：`server/internal/childapi/message_routes_test.go`
- 内容：扩展 message 路由测试，要求 `MessageRoutes` 依赖提供时 `GET /api/messages` 能被注册路由接管；缺少依赖时该路径返回 501 占位。
- 目的：让子女端 Web 刷新留言状态实际调用的列表 API 与后端骨架测试保持一致，避免测试只覆盖发送留言的 POST。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前测试 stub 只注册 `/messages` POST。
- 验证：已运行 `go test ./internal/childapi`，得到有效 RED：`TestMessageRoutesAreRegisteredWhenDependencyProvided` 失败，`GET /api/messages` 返回 404 而不是 200。

### 00:01 childapi 留言列表路由 GREEN 测试契约

- 文件：`server/internal/childapi/message_routes_test.go`
- 内容：message 路由 stub 新增 `GET /messages`，并覆盖有依赖/无依赖两种场景：有依赖时返回 200 stub 列表，缺依赖时返回 501 占位。
- 目的：把 `childapi` 路由契约和真实 `message.Handler`、子女端 Web 的 `listMessages` 调用路径对齐，防止后续骨架测试遗漏留言列表。
- 功能：仅测试契约变更；生产 `message.Handler` 已经同时注册 `POST /messages` 和 `GET /messages`，运行时行为不变。
- 功能影响：无 xiaozhi 上游代码变化，不影响只部署原版 xiaozhi 的独立对话能力。
- 验证：
  - 已运行 `go test -count=1 ./internal/childapi`，通过。
  - 已运行 `go test -count=1 -cover ./internal/childapi`，childapi 包覆盖率 92.9%。
  - 已运行 `npm test --prefix web`，40 个 smoke 测试通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go test -count=1 ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go build ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go vet ./...`，通过。
  - 已运行 `git diff --check`，无空白错误，仅提示 Windows 下 LF 后续会被 Git 转为 CRLF。

### 11:41 childapi 设备状态 PRD 路径 RED 测试

- 文件：`server/internal/childapi/status_routes_test.go`
- 内容：扩展状态路由测试，要求 `StatusRoutes` 依赖提供时 `/api/device/status` 这个 PRD 路径也能被注册路由接管；缺少依赖时该路径返回 501 占位。
- 目的：让子女端 Web 实际调用的 `/api/device/status` 与后端骨架测试保持一致，避免测试只覆盖旧别名 `/api/status`。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前测试 stub 只注册 `/status`。
- 验证：已运行 `go test ./internal/childapi`，得到有效 RED：`TestStatusRoutesAreRegisteredWhenDependencyProvided` 失败，`GET /api/device/status` 返回 404 而不是 200。

### 11:42 childapi 设备状态 PRD 路径 GREEN 测试契约

- 文件：`server/internal/childapi/status_routes_test.go`
- 内容：状态路由 stub 新增 `GET /device/status`，并覆盖有依赖/无依赖两种场景：有依赖时返回 200 stub，缺依赖时返回 501 占位。
- 目的：把 `childapi` 路由契约和真实 `status.Handler`、子女端 Web 调用路径对齐，防止后续骨架测试遗漏 PRD 路径。
- 功能：仅测试契约变更；生产 `status.Handler` 已经同时注册 `/status` 和 `/device/status`，运行时行为不变。
- 功能影响：无 xiaozhi 上游代码变化，不影响只部署原版 xiaozhi 的独立对话能力。
- 验证：
  - 已运行 `go test -count=1 ./internal/childapi`，通过。
  - 已运行 `go test -count=1 -cover ./internal/childapi`，childapi 包覆盖率 92.9%。
  - 已运行 `npm test --prefix web`，40 个 smoke 测试通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go test -count=1 ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go build ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go vet ./...`，通过。
  - 已运行 `git diff --check`，无空白错误，仅提示 Windows 下 LF 后续会被 Git 转为 CRLF。

### 11:37 childapi 提醒撤销占位路由 RED 测试

- 文件：`server/internal/childapi/reminder_routes_test.go`
- 内容：扩展 reminder 路由测试，要求提供 `ReminderRoutes` 依赖时 `DELETE /api/reminders/:id` 能被注册路由接管；缺少依赖时，该路径也应返回 501 占位而不是 404。
- 目的：让子女端骨架里的撤销提醒 API 与后端占位路由形状一致，前端在域未接入时看到“未实现”而不是“路由不存在”。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 `childapi` 缺少 DELETE 提醒占位。
- 验证：已运行 `go test ./internal/childapi`，得到有效 RED：`TestReminderRoutesStayPlaceholderWhenDependencyMissing` 失败，`DELETE /api/reminders/:id` 返回 404 而不是 501。

### 11:39 childapi 提醒撤销占位路由 GREEN 实现

- 文件：`server/internal/childapi/server.go`
- 内容：在缺少 `ReminderRoutes` 依赖的占位分支中新增 `api.DELETE("/reminders/:id", notImpl)`。
- 目的：补齐子女端提醒撤销 API 的后端骨架路由，让 reminder 域未装配时仍返回统一的 501 占位响应。
- 功能：真实 reminder handler 已装配时行为不变；缺依赖的骨架模式下，`DELETE /api/reminders/:id` 不再是 404。
- 文件：`server/internal/childapi/reminder_routes_test.go`
- 内容：新增有依赖/无依赖两种 DELETE 路由覆盖。
- 目的：防止子女端骨架和后端占位路由再次不一致。
- 功能影响：无 xiaozhi 上游代码变化，不影响只部署原版 xiaozhi 的独立对话能力。
- 验证：
  - 已运行 `go test -count=1 ./internal/childapi`，通过。
  - 已运行 `go test -count=1 -cover ./internal/childapi`，childapi 包覆盖率 92.9%。
  - 已运行 `npm test --prefix web`，40 个 smoke 测试通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go test -count=1 ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go build ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go vet ./...`，通过。
  - 已运行 `git diff --check`，无空白错误，仅提示 Windows 下 LF 后续会被 Git 转为 CRLF。

### 11:32 xiaozhi 历史畸形对象 RED 测试

- 文件：`server/internal/xiaozhiclient/http_client_test.go`
- 内容：新增 `TestGetHistoryRejectsMalformedObjectWithoutList`，模拟 manager 历史接口返回 `{"data":{"unexpected":true}}`，要求 `GetHistory` 返回错误。
- 目的：避免状态页依赖的历史读取把 manager 契约不匹配静默解释为“没有历史”，让联调时能区分“无互动”和“历史接口返回形状不支持”。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前实现会返回空历史且无错误。
- 验证：已运行 `go test ./internal/xiaozhiclient`，得到有效 RED：`TestGetHistoryRejectsMalformedObjectWithoutList` 失败，错误为 expected malformed history response error, got nil。

### 11:34 xiaozhi 历史畸形对象 GREEN 实现

- 文件：`server/internal/xiaozhiclient/http_client.go`
- 内容：`decodeHistoryMessages` 解析对象响应时增加 `foundList` 标记；如果没有找到 `messages/items/list/records/rows` 任一历史列表字段，返回 `xiaozhi manager history response does not contain a message list` 错误。
- 目的：把 manager 历史接口契约不匹配留在 `xiaozhiclient` 边界内暴露，`status` 域仍可按已有降级逻辑回退到 `last_active_at`。
- 功能：空数组仍是合法空历史；畸形对象不再静默成功，便于设备到场联调时定位 manager 历史接口形状问题。
- 文件：`server/internal/xiaozhiclient/http_client_test.go`
- 内容：新增畸形对象回归测试。
- 目的：防止历史解析再次把未知对象当成空历史。
- 功能影响：无 xiaozhi 上游代码变化，不影响只部署原版 xiaozhi 的独立对话能力。
- 验证：
  - 已运行 `go test -count=1 ./internal/xiaozhiclient`，通过。
  - 已运行 `go test -count=1 -cover ./internal/xiaozhiclient`，xiaozhiclient 包覆盖率 86.9%。
  - 已运行 `npm test --prefix web`，40 个 smoke 测试通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go test -count=1 ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go build ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go vet ./...`，通过。
  - 已运行 `git diff --check`，无空白错误，仅提示 Windows 下 LF 后续会被 Git 转为 CRLF。

### 11:28 xiaozhi 历史 records 返回形状 RED 测试

- 文件：`server/internal/xiaozhiclient/http_client_test.go`
- 内容：新增 `TestGetHistoryParsesNestedRecordsPayload`，模拟 xiaozhi manager `/api/open/v1/history/messages` 返回 `{"data":{"records":[...]}}`，要求 `GetHistory` 能解析出角色、文本和时间。
- 目的：增强 PRD #4 “最近互动时间”和 #5 “当前会话沉淀”所依赖的只读历史读取容错性，避免 manager 使用常见分页字段 `records` 时状态页退化为空历史。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前实现只支持直接数组或 `messages` 字段。
- 验证：已运行 `go test ./internal/xiaozhiclient`，得到有效 RED：`TestGetHistoryParsesNestedRecordsPayload` 失败，`history = []`。

### 11:30 xiaozhi 历史列表字段 GREEN 实现

- 文件：`server/internal/xiaozhiclient/http_client.go`
- 内容：`decodeHistoryMessages` 在历史响应是对象时，按顺序识别 `messages`、`items`、`list`、`records`、`rows` 这些常见列表字段，并解析为统一的 `HistoryMessage`。
- 目的：让安伴状态域只依赖 `xiaozhiclient.GetHistory` 的稳定接口，不把 manager 分页字段差异泄露到 `status` 或 `childapi`。
- 功能：`GET /api/device/status` 通过 `GetHistory` 计算最近互动时，能兼容更多 manager 历史接口返回形状；直接数组和原有 `messages` 形状保持不变。
- 文件：`server/internal/xiaozhiclient/http_client_test.go`
- 内容：新增 `data.records` 回归测试。
- 目的：防止后续重构把历史列表解析重新收窄。
- 功能影响：无 xiaozhi 上游代码变化，不影响只部署原版 xiaozhi 的独立对话能力。
- 验证：
  - 已运行 `go test -count=1 ./internal/xiaozhiclient`，通过。
  - 已运行 `go test -count=1 -cover ./internal/xiaozhiclient`，xiaozhiclient 包覆盖率 86.7%。
  - 已运行 `npm test --prefix web`，40 个 smoke 测试通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go test -count=1 ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go build ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go vet ./...`，通过。
  - 已运行 `git diff --check`，无空白错误，仅提示 Windows 下 LF 后续会被 Git 转为 CRLF。

### 11:22 reminder 重启过期未应答 RED 测试

- 文件：`server/internal/domains/reminder/service_test.go`
- 内容：新增 `TestServiceRestoreScheduledMarksOverduePlayedReminderUnanswered`，模拟服务重启时已有一条 `played` 提醒，且 `playedAt + 30min` 早于当前时间，要求恢复阶段直接标记为 `unanswered`，不再安排过期的 ack timeout job。
- 目的：对齐 PRD #6 “30 分钟无应答 -> 未应答且子女端可见”，避免后端重启后过期提醒短暂停留在 `played` 状态或依赖一个过去时间的异步 job 才能落状态。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：首次运行因 Go cache 目录未指向 D 盘失败，不计为有效 RED；创建并指向 `.gocache-go/.gotmp-go` 后运行 `go test ./internal/domains/reminder`，得到有效 RED：`TestServiceRestoreScheduledMarksOverduePlayedReminderUnanswered` 失败，原因是恢复时安排了 1 个过去的 ack timeout job。

### 11:24 reminder 重启过期未应答 GREEN 实现

- 文件：`server/internal/domains/reminder/service.go`
- 内容：`RestoreScheduled` 恢复 `played` 提醒时先计算 `playedAt + defaultAckTimeout`；如果该时间已经不晚于当前时间，直接以 `AckKindTimeout` 调用确认逻辑并清空旧 `AckJobID`，不再安排过去时间的调度任务。
- 目的：让后端重启后，超过 30 分钟未应答的提醒立即进入 `unanswered` 状态，保证子女端刷新即可看到真实状态。
- 功能：未超过确认窗口的 `played` 提醒仍重排 ack timeout；已经过期的提醒同步转 `unanswered` 并带 `ackKind=timeout`、`acknowledgedAt`。
- 文件：`server/internal/domains/reminder/service_test.go`
- 内容：新增过期恢复回归测试，验证没有新增 past ack job，且提醒状态直接为 `unanswered`。
- 目的：防止后续恢复逻辑重构时重新依赖过去时间异步 job。
- 功能影响：无 xiaozhi 上游代码变化，不影响只部署原版 xiaozhi 的独立对话能力。
- 验证：
  - 已运行 `go test -count=1 ./internal/domains/reminder`，通过。
  - 已运行 `go test -count=1 -cover ./internal/domains/reminder`，reminder 包覆盖率 80.6%。
  - 已运行 `npm test --prefix web`，40 个 smoke 测试通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go test -count=1 ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go build ./...`，通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR/GOCACHE` 运行 `go vet ./...`，通过。
  - 已运行 `git diff --check`，无空白错误，仅提示 Windows 下 LF 后续会被 Git 转为 CRLF。

### 配置必填环境变量空白规整 RED 测试

- 文件：`server/internal/config/config_test.go`
- 内容：新增 `TestLoadTrimsRequiredEnvValues` 和 `TestLoadRejectsWhitespaceRequiredEnvValues`，要求主服务配置加载时 trim `ANBAN_MANAGER_BASE_URL`、`ANBAN_MANAGER_API_TOKEN`、`ANBAN_ACCESS_CODE`，并把纯空白 manager URL 视为缺失。
- 文件：`server/cmd/anban-preflight/main_test.go`
- 内容：新增 `TestLoadPreflightConfigTrimsManagerAccess`，并扩展 `TestLoadPreflightConfigRequiresManagerAccess`，要求 preflight 独立配置同样 trim manager URL/token，并拒绝纯空白 manager URL。
- 目的：提升设备到货联调容错性，避免复制 `.env` 时首尾空格通过校验，随后在 xiaozhi manager HTTP 调用里变成难定位错误。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前配置加载直接使用 `os.Getenv` 原值。
- 验证：已运行 `go test ./internal/config ./cmd/anban-preflight`，按预期失败；失败点为 manager URL/token/access code 未 trim，且纯空白 manager URL 未被拒绝。

### 配置必填环境变量空白规整 GREEN 实现

- 文件：`server/internal/config/config.go`
- 内容：新增 `trimEnv`，主服务配置加载时 trim `ANBAN_MANAGER_BASE_URL`、`ANBAN_MANAGER_API_TOKEN`、`ANBAN_ACCESS_CODE` 后再做必填校验。
- 目的：让服务启动阶段更早暴露配置错误，避免 manager URL/token/access code 首尾空白在后续请求里变成难排查的鉴权或连接问题。
- 功能：纯空白必填环境变量会被视为缺失；带首尾空白的有效值会被规整后使用。
- 文件：`server/cmd/anban-preflight/main.go`
- 内容：`loadPreflightConfig` 对 manager URL/token 做 `strings.TrimSpace`。
- 目的：让设备到货前置检查与主服务配置行为一致。
- 功能：preflight 不再把带空白的 manager URL/token 原样传给 `xiaozhiclient`。
- 文件：`server/internal/config/config_test.go`、`server/cmd/anban-preflight/main_test.go`
- 内容：新增/扩展 GREEN 覆盖测试，验证 trim 和纯空白拒绝。
- 目的：防止配置入口回退到原样读取。
- 功能影响：无 xiaozhi 上游代码变化，不影响只部署原版 xiaozhi 的独立对话能力。
- 验证：已运行 `go test -count=1 ./internal/config ./cmd/anban-preflight`，通过；已运行 `go test -count=1 -cover ./internal/config ./cmd/anban-preflight`，`config` 覆盖率 90.0%，`anban-preflight` 覆盖率 86.2%；已运行 `go test -count=1 ./...`、`go build ./...`、`go vet ./...`，均通过；已运行 `npm test --prefix web`，40 个 smoke 测试通过；已运行 `git diff --check`，无空白错误，仅提示 Windows 下 LF 后续会被 Git 转为 CRLF。

### Compose 子女端跨域配置 RED 测试

- 文件：`server/internal/architecture/architecture_test.go`
- 内容：新增 `TestDockerComposeWiresAnbanAllowedOrigins`，读取根目录 `docker-compose.yml`，要求 Compose 部署路径也给 `anban` 服务传入 `ANBAN_ALLOWED_ORIGINS`。
- 目的：保证本地直跑 `.env.example` 和 Docker Compose 联调路径一致，避免子女端静态页在 Compose 部署时再次被 CORS 预检拦住。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 `docker-compose.yml` 没有 `ANBAN_ALLOWED_ORIGINS`。
- 验证：已运行 `go test ./internal/architecture`，按预期失败；失败点为 `docker-compose.yml anban service must pass ANBAN_ALLOWED_ORIGINS`。

### Compose 子女端跨域配置 GREEN 实现

- 文件：`docker-compose.yml`
- 内容：在 `anban.environment` 中新增 `ANBAN_ALLOWED_ORIGINS: "${ANBAN_ALLOWED_ORIGINS:-http://127.0.0.1:5173,http://localhost:5173}"`。
- 目的：让 Docker Compose 联调路径默认支持本地静态子女端访问安伴后端，同时允许部署时通过环境变量覆盖来源。
- 功能：使用 Compose 启动 `anban` 时会把 CORS 允许来源传入后端配置，和 `.env.example` 的本地直跑路径保持一致。
- 文件：`server/internal/architecture/architecture_test.go`
- 内容：新增架构/部署配置守护测试，防止以后移除 Compose 中的 `ANBAN_ALLOWED_ORIGINS`。
- 目的：把方案 C 的基础联调配置纳入自动测试。
- 功能影响：无 xiaozhi 上游代码变化，不影响只部署原版 xiaozhi 的独立对话能力。
- 验证：已运行 `go test -count=1 ./internal/architecture`，通过；已运行 `go test -count=1 -cover ./internal/architecture`，通过（纯测试守护包，无 statements）；已运行 `go test -count=1 ./...`、`go build ./...`、`go vet ./...`，均通过；已运行 `npm test --prefix web`，40 个 smoke 测试通过；已运行 `git diff --check`，无空白错误，仅提示 Windows 下 LF 后续会被 Git 转为 CRLF。

### xiaozhi manager base URL 尾斜杠 RED 测试

- 文件：`server/internal/xiaozhiclient/http_client_test.go`
- 内容：新增 `TestHTTPClientTrimsTrailingSlashFromManagerBaseURL`，用 `NewHTTPClient(srv.URL + "/", ...)` 调用 `InjectSpeak`，要求 manager 侧收到的路径仍是 `/api/open/v1/devices/inject-message`。
- 目的：提升方案 C 部署容错性，避免 `.env` 中 `ANBAN_MANAGER_BASE_URL=http://localhost:8080/` 这种常见写法拼出 `//api/open/v1/*`，导致联调时误判 xiaozhi manager 端点不可用。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 `NewHTTPClient` 原样保存 `baseURL`。
- 验证：已运行 `go test ./internal/xiaozhiclient`，按预期失败；失败点为带尾斜杠 base URL 时 `InjectSpeak` 得到 manager 404。

### xiaozhi manager base URL 尾斜杠 GREEN 实现

- 文件：`server/internal/xiaozhiclient/http_client.go`
- 内容：`NewHTTPClient` 保存 `baseURL` 时使用 `strings.TrimRight(baseURL, "/")` 去掉尾部斜杠。
- 目的：让 `ANBAN_MANAGER_BASE_URL` 对 `http://localhost:8080` 和 `http://localhost:8080/` 两种常见写法都能正确拼接 `/api/open/v1/*`。
- 功能：`InjectSpeak`、`CheckManagerAccess`、`GetDeviceStatus`、`GetHistory`、`SetRolePrompt`、`CallDeviceMCPTool` 共享同一个规整后的 manager base URL，避免双斜杠路径导致 manager 404。
- 文件：`server/internal/xiaozhiclient/http_client_test.go`
- 内容：新增尾斜杠回归测试，覆盖 `InjectSpeak` 的真实路径。
- 目的：防止后续重构把 base URL 拼接容错性破坏。
- 功能影响：无 xiaozhi 上游代码变化，不影响只部署原版 xiaozhi 的独立对话能力。
- 验证：已运行 `go test -count=1 ./internal/xiaozhiclient`，通过；已运行 `go test -count=1 -cover ./internal/xiaozhiclient`，xiaozhiclient 包覆盖率 86.8%；已运行 `go test -count=1 ./...`、`go build ./...`、`go vet ./...`，均通过；已运行 `npm test --prefix web`，40 个 smoke 测试通过；已运行 `git diff --check`，无空白错误，仅提示 Windows 下 LF 后续会被 Git 转为 CRLF。

### 状态快照留言摘要降级 RED 测试

- 文件：`server/internal/domains/status/service_test.go`
- 内容：新增 `TestServiceGetKeepsDeviceStatusWhenMessageSummaryFails`，模拟 xiaozhi 设备状态正常、留言摘要读取失败，要求 status 服务仍返回设备在线、最近可见时间和空 `messages` 列表。
- 目的：对齐 PRD #4 “子女端看设备状态”的基础价值，确保设备在线/最近互动这个安心底板不被附加的留言摘要故障拖垮。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 status 服务会把 message reader 错误直接返回。
- 验证：已运行 `go test ./internal/domains/status`，按预期失败；失败点为 `Get: message db timeout`。

### 状态快照留言摘要降级 GREEN 实现

- 文件：`server/internal/domains/status/service.go`
- 内容：`Get` 读取留言摘要时改为降级处理：`messageReader.ListMessageStatusSummaries` 成功则填充 `messages`，失败则保留空列表并继续返回设备状态。
- 目的：让 PRD #4 的核心信息“设备在线/最近互动”在留言摘要临时故障时仍可用，降低子女端状态页被附属信息拖垮的风险。
- 功能：`/api/status` 和 `/api/device/status` 在 xiaozhi 设备状态读取成功但留言摘要读取失败时仍返回 200 快照，`messages` 为空；设备状态读取失败仍按原逻辑返回错误。
- 文件：`server/internal/domains/status/service_test.go`
- 内容：新增 GREEN 覆盖测试，验证留言摘要失败不会影响设备状态、最近可见时间和空消息列表返回。
- 目的：防止后续状态聚合重构重新把附属摘要错误升级为整接口失败。
- 功能影响：无 xiaozhi 上游代码变化，不影响只部署原版 xiaozhi 的独立对话能力。
- 验证：已运行 `go test -count=1 ./internal/domains/status`，通过；已运行 `go test -count=1 -cover ./internal/domains/status`，status 包覆盖率 89.1%；已运行 `go test -count=1 ./...`、`go build ./...`、`go vet ./...`，均通过；已运行 `npm test --prefix web`，40 个 smoke 测试通过；已运行 `git diff --check`，无空白错误，仅提示 Windows 下 LF 后续会被 Git 转为 CRLF。

### 留言失败不阻塞后续发送覆盖测试

- 文件：`server/internal/domains/message/service_test.go`
- 内容：新增 `TestServiceSendMessageFailureDoesNotBlockNextMessage`，使用一个第一次 `InjectSpeak` 返回错误、第二次成功的 fake client，验证第一条留言保存为 `failed` 后，第二条留言仍能继续调用 xiaozhi、保存为 `played`，列表中同时保留失败和成功记录。
- 目的：对齐 PRD #3 “单条留言失败不会卡死后续留言（队列健壮）”，把当前已有的独立发送/独立落库行为变成防回归测试。
- 功能影响：无生产逻辑变更；测试直接通过，说明现有实现已满足该基础健壮性要求。
- 验证：已运行 `go test -count=1 ./internal/domains/message`，通过；已运行 `go test -count=1 -cover ./internal/domains/message`，message 包覆盖率 90.9%；已运行 `go test -count=1 ./...`、`go build ./...`、`go vet ./...`，均通过；已运行 `npm test --prefix web`，40 个 smoke 测试通过；已运行 `git diff --check`，无空白错误，仅提示 Windows 下 LF 后续会被 Git 转为 CRLF。

### 留言播报后续听 RED 测试

- 文件：`server/internal/domains/message/service_test.go`
- 内容：扩展 `TestServiceSendMessageInjectsAndPersistsPlayed`，要求子女端留言调用 `InjectSpeak` 时除 `SkipLLM=true` 外，还必须传入 `AutoListen=true`。
- 目的：对齐 PRD #3 “子女端留言 → 设备播报”的现场高光，并让留言播完后把控制权交回原版 xiaozhi 聆听链路，避免安伴主动播报形成单向中断。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 message 域只设置 `SkipLLM=true`。
- 验证：已运行 `go test ./internal/domains/message`，按预期失败；失败点为 `AutoListen is not true`。

### 留言播报后续听 GREEN 实现

- 文件：`server/internal/domains/message/service.go`
- 内容：新增 `messageSpeakOptions()`，子女端留言播报调用 `InjectSpeak` 时同时设置 `SkipLLM=true` 和 `AutoListen=true`。
- 目的：留言播报完后让 xiaozhi 继续聆听老人可能的回应，保持安伴增强能力与原版小智语音循环衔接。
- 功能：`POST /api/messages` 触发的 manager `inject-message` 会显式携带 `auto_listen=true`；留言仍按原逻辑保存为 `played` 或 `failed`。
- 文件：`server/internal/domains/message/service_test.go`
- 内容：补充断言覆盖 `AutoListen=true`。
- 目的：防止后续重构把留言播报恢复成单向注入。
- 功能影响：无 xiaozhi 上游代码变化，不影响只部署原版 xiaozhi 的独立对话能力。
- 验证：已运行 `go test -count=1 ./internal/domains/message`，通过；已运行 `go test -count=1 -cover ./internal/domains/message`，message 包覆盖率 90.9%；已运行 `go test -count=1 ./...`、`go build ./...`、`go vet ./...`，均通过；已运行 `npm test --prefix web`，40 个 smoke 测试通过；已运行 `git diff --check`，无空白错误，仅提示 Windows 下 LF 后续会被 Git 转为 CRLF。

### 子女端本地跨域 RED 测试

- 文件：`server/internal/childapi/cors_test.go`
- 内容：新增 CORS RED 测试，要求 `NewRouter` 支持配置允许来源，允许 `http://127.0.0.1:5173` 对 `/api/messages` 发起浏览器预检，并放行 `Content-Type` 与 `X-Access-Code` 请求头；未知来源不返回 `Access-Control-Allow-Origin`。
- 文件：`server/internal/config/config_test.go`
- 内容：新增配置 RED 测试，要求默认允许本地静态子女端来源 `http://127.0.0.1:5173` 和 `http://localhost:5173`，并支持通过 `ANBAN_ALLOWED_ORIGINS` 覆盖。
- 目的：对齐方案 C 部署指南中“web 静态页 5173 + anban 后端 8090”的本地联调方式，避免子女端骨架在真实浏览器里被跨域预检拦住。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 `childapi.Deps` 和 `config.Config` 尚无 `AllowedOrigins`。
- 验证：已运行 `go test ./internal/childapi ./internal/config`，按预期编译失败；失败点为 `AllowedOrigins` 字段未定义。

### 子女端本地跨域 GREEN 实现

- 文件：`server/internal/config/config.go`
- 内容：新增 `AllowedOrigins` 配置字段，默认值为 `http://127.0.0.1:5173,http://localhost:5173`，并支持 `ANBAN_ALLOWED_ORIGINS` 逗号分隔覆盖。
- 目的：让本地静态子女端能按方案 C 指南从 5173 端口调用 8090 后端，同时保留部署时收敛来源的能力。
- 功能：启动配置会把允许跨域来源传给 `childapi`，未知来源不会拿到 CORS 放行头。
- 文件：`server/internal/childapi/server.go`
- 内容：新增 `AllowedOrigins` 依赖字段、`AllowCORS` 中间件和 OPTIONS 预检处理，放行 `Content-Type` 与 `X-Access-Code` 请求头，以及 GET/POST/PUT/DELETE/OPTIONS 方法。
- 目的：确保子女端 Web 骨架能在浏览器真实跨域环境下访问安伴后端 API，不只是在 Node smoke 测试里可用。
- 功能：配置来源发起的预检返回 204，并带 `Access-Control-Allow-Origin/Methods/Headers`；未知来源仍可访问无需跨域的场景，但不会被浏览器视为 CORS 放行。
- 文件：`server/cmd/anban/main.go`
- 内容：启动装配时将 `cfg.AllowedOrigins` 注入 `childapi.NewRouter`。
- 目的：让配置层和 HTTP 层真实连起来。
- 功能影响：无 xiaozhi 调用变化，不影响原版小智独立运行。
- 文件：`.env.example`
- 内容：新增 `ANBAN_ALLOWED_ORIGINS=http://127.0.0.1:5173,http://localhost:5173`。
- 文件：`docs/deployment/方案C部署与联调指南.md`
- 内容：在 `.env` 示例中加入 `ANBAN_ALLOWED_ORIGINS`，并提示如果子女端换端口/域名需要同步更新。
- 目的：让设备到货联调文档和实际配置一致。
- 功能影响：无运行时影响。
- 验证：已运行 `go test -count=1 ./internal/childapi ./internal/config`，通过；已运行 `go test -count=1 -cover ./internal/childapi ./internal/config`，`childapi` 覆盖率 92.8%，`config` 覆盖率 84.2%；已运行 `go test -count=1 ./...`、`go build ./...`、`go vet ./...`，均通过；已运行 `npm test --prefix web`，40 个 smoke 测试通过；已运行 `git diff --check`，无空白错误，仅提示 Windows 下 LF 后续会被 Git 转为 CRLF。

### preflight Gate A 显式确认 RED 测试

- 文件：`server/cmd/anban-preflight/main_test.go`
- 内容：新增命令级 RED 测试，要求 `anban-preflight` 默认不能因为 manager/token 通过就退出 0，必须通过 `--xiaozhi-gate-passed` 或 `ANBAN_PREFLIGHT_XIAOZHI_GATE_PASSED=true` 显式确认纯 xiaozhi 语音闭环 Gate A 已人工通过。
- 目的：把“确保原版小智功能正常”从提示文字升级为命令守门，避免跳过实机原版唤醒/回应/打断验证就继续安伴联调。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 CLI 没有 Gate A 确认参数，且 manager/token 通过时会直接返回 0。
- 验证：已运行 `go test ./cmd/anban-preflight`，按预期失败；失败点为 `--xiaozhi-gate-passed` 未定义，以及未确认 Gate A 时仍返回 0。

### preflight Gate A 显式确认 GREEN 实现

- 文件：`server/cmd/anban-preflight/main.go`
- 内容：新增 `--xiaozhi-gate-passed` 参数和 `ANBAN_PREFLIGHT_XIAOZHI_GATE_PASSED` 环境变量；manager/token 检查通过但 Gate A 未确认时输出说明并返回非 0。
- 目的：确保启动安伴联调前，操作者必须显式确认原版小智唤醒、回应、打断已人工验证。
- 功能：preflight 现在需要“manager/token 通过 + 设备检查按需通过 + Gate A 显式确认”才会返回 0。
- 文件：`README.md`
- 内容：本地启动步骤中的 preflight 命令增加 `--xiaozhi-gate-passed`。
- 目的：把 Gate A 确认放到仓库第一入口，避免误把 preflight 输出当成自动验证。
- 功能影响：无运行时影响。
- 文件：`docs/deployment/方案C部署与联调指南.md`
- 内容：补充 `--xiaozhi-gate-passed` 和 `ANBAN_PREFLIGHT_XIAOZHI_GATE_PASSED=true` 的使用说明。
- 目的：让设备到货部署指南明确区分自动 manager/token 检查和人工纯小智语音闭环确认。
- 功能影响：无运行时影响。
- 验证：已运行 `go test -count=1 ./cmd/anban-preflight`，通过；已运行 `go test -count=1 ./...`、`go build ./...`、`go vet ./...`，均通过；已运行 `npm test --prefix web`，40 个 smoke 测试通过；已运行 `git diff --check`，无空白错误，仅提示 Windows 下 LF 后续会被 Git 转为 CRLF。

### preflight CLI 行为 RED 测试

- 文件：`server/cmd/anban-preflight/main_test.go`
- 内容：新增命令级 RED 测试，使用 `httptest` 假 manager 验证 `anban-preflight` 在不设置 `ANBAN_ACCESS_CODE`、不传设备 ID 时仍会访问 `/api/open/v1/devices`、携带 `X-API-Token`、输出 manager PASS 和设备 SKIP，并在 manager 401 时返回非 0。
- 目的：把设备到货联调命令本身纳入测试，证明 preflight 的真实 CLI 行为符合“先查 manager/token，再可选查设备”的守门顺序。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前命令只有 `main()`，没有可测试的 `run` 入口。
- 验证：已运行 `go test ./cmd/anban-preflight`，按预期编译失败；失败原因为 `run` 未定义。

### preflight CLI 行为 GREEN 实现

- 文件：`server/cmd/anban-preflight/main.go`
- 内容：将 `main` 拆出 `run(args, stdout, stderr) int`，使用独立 `flag.FlagSet` 解析参数，并通过返回码表达成功、配置失败、manager 检查失败或参数错误。
- 目的：让 `anban-preflight` 的真实 CLI 行为可测试，避免只测内部包而漏掉参数、环境变量和退出码组合。
- 功能：外部命令行为不变；测试可注入 stdout/stderr 并断言 exit code。
- 验证：已运行 `go test -count=1 ./cmd/anban-preflight`，通过；已运行 `go test -count=1 ./...`、`go build ./...`、`go vet ./...`，均通过；已运行 `npm test --prefix web`，40 个 smoke 测试通过；`git diff --check` 无空白错误。

### preflight manager/token 检查 RED 测试

- 文件：`server/internal/preflight/preflight_test.go`
- 内容：新增断言，要求 `preflight.Run` 即使未提供设备 ID，也必须先检查 xiaozhi manager OpenAPI/token；manager 检查失败时整体失败，且不继续查设备状态。
- 目的：让设备到货联调守门真正验证 manager URL/token，而不是在无设备 ID 时只输出 `[SKIP]`。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 `DeviceStatusReader` 没有 manager access 检查能力。
- 文件：`server/internal/xiaozhiclient/http_client_test.go`
- 内容：新增 `CheckManagerAccess` RED 测试，要求通过 `GET /api/open/v1/devices` 和 `X-API-Token` 做非侵入式 manager/token 检查，并拒绝畸形设备列表响应。
- 目的：把 manager/token 预检仍收口在 `xiaozhiclient`，不让 preflight 直接懂 OpenAPI 路径。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期 `HTTPClient.CheckManagerAccess` 尚未实现。
- 验证：已运行 `go test ./internal/preflight ./internal/xiaozhiclient`，按预期失败；preflight 未检查 manager access，且 `HTTPClient.CheckManagerAccess` 未定义。

### preflight manager/token 检查 GREEN 实现

- 文件：`server/internal/preflight/preflight.go`
- 内容：新增 `ManagerAccessChecker` 接口并要求 `DeviceStatusReader` 实现它；`Run` 先执行 manager OpenAPI/token 检查，失败则返回 fail，不再继续设备状态检查。
- 目的：让 preflight 成为真正的方案 C 联调守门：先确认 xiaozhi manager/token 可用，再选择性检查具体设备。
- 功能：无设备 ID 时也会输出 `[PASS] xiaozhi manager OpenAPI access` 或失败；设备 ID 为空仅跳过“具体设备在线”检查。
- 文件：`server/internal/xiaozhiclient/http_client.go`
- 内容：新增 `CheckManagerAccess(ctx)`，通过只读 `GET /api/open/v1/devices` 检查 manager OpenAPI 和 API Token，并复用设备列表解析校验响应形状。
- 目的：继续保持只有 `xiaozhiclient` 知道 xiaozhi manager endpoint，preflight 不直接写 HTTP 路径。
- 功能：可在不触发播报、不依赖设备 ID 的情况下验证 manager/token。
- 文件：`docs/deployment/方案C部署与联调指南.md`
- 内容：更新 preflight 说明和输出示例，明确无设备 ID 时仍会检查 manager URL/token，仅跳过具体设备在线检查。
- 目的：保持部署指南和新的 preflight 守门行为一致。
- 功能影响：无运行时影响。
- 验证：已运行 `go test -count=1 ./internal/preflight ./internal/xiaozhiclient`，通过；已运行 `go run ./cmd/anban-preflight`（dummy manager/token、不设置访问码、不传设备 ID），按预期输出 `[FAIL] xiaozhi manager OpenAPI access` 并以非 0 退出；已运行 `go test -count=1 ./...`、`go build ./...`、`go vet ./...`，均通过；已运行 `npm test --prefix web`，40 个 smoke 测试通过；`git diff --check` 无空白错误。

### preflight 独立配置 RED 测试

- 文件：`server/cmd/anban-preflight/main_test.go`
- 内容：新增测试，要求 `anban-preflight` 的配置加载只依赖 `ANBAN_MANAGER_BASE_URL` 和 `ANBAN_MANAGER_API_TOKEN`，不要求 `ANBAN_ACCESS_CODE`；同时覆盖缺 manager URL/token 的错误。
- 目的：设备到货联调时，preflight 是安伴业务启动前的 xiaozhi manager 守门，不应被子女端访问码配置阻塞。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期 `loadPreflightConfig` 尚未实现。
- 验证：已运行 `go test ./cmd/anban-preflight`，按预期编译失败；失败原因为 `loadPreflightConfig` 未定义。

### preflight 独立配置 GREEN 实现

- 文件：`server/cmd/anban-preflight/main.go`
- 内容：新增 `preflightConfig` 和 `loadPreflightConfig`，只读取并校验 `ANBAN_MANAGER_BASE_URL` 与 `ANBAN_MANAGER_API_TOKEN`，不再复用完整 `config.Load()`。
- 目的：让设备到货后的 xiaozhi manager/token 预检可以早于安伴 childapi 配置执行，减少联调前置摩擦。
- 功能：`anban-preflight` 不再因为 `ANBAN_ACCESS_CODE` 未设置而退出；manager URL/token 仍为必填。
- 验证：已运行 `go test -count=1 ./cmd/anban-preflight`，通过；已运行 `go run ./cmd/anban-preflight`（设置 dummy manager/token、不设置 `ANBAN_ACCESS_CODE`、不传设备 ID），输出 `[MANUAL]` Gate A 和 `[SKIP]` 设备检查；已运行 `go test -count=1 ./...`、`go build ./...`、`go vet ./...`，均通过；已运行 `npm test --prefix web`，40 个 smoke 测试通过；`git diff --check` 无空白错误。

### 主动语音播报后续听 RED 测试

- 文件：`server/internal/domains/greeting/service_test.go`
- 内容：新增断言，要求主动问候调用 `InjectSpeak` 时传入 `AutoListen=true`。
- 目的：对齐 PRD #2 “问候后老人答挺好的→进入正常对话循环”，让安伴主动开口后把控制权交回原版 xiaozhi 聆听链路。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前实现只设置 `SkipLLM=true`，没有显式 `AutoListen=true`。
- 文件：`server/internal/domains/reminder/service_test.go`
- 内容：新增断言，要求提醒播报调用 `InjectSpeak` 时传入 `AutoListen=true`。
- 目的：对齐 PRD #6 “老人答好的→状态完成”的语音入口前提，避免提醒播完后设备不进入可回应状态。
- 功能影响：暂无生产功能；这是 TDD RED 阶段。
- 验证：已运行 `go test ./internal/domains/greeting ./internal/domains/reminder`，按预期失败；失败点分别为 `AutoListen is not true`。

### 主动语音播报后续听 GREEN 实现

- 文件：`server/internal/domains/greeting/service.go`
- 内容：主动问候调用 `InjectSpeak` 时改用 `proactiveSpeakOptions()`，同时设置 `SkipLLM=true` 和 `AutoListen=true`。
- 目的：问候播完后让 xiaozhi 继续聆听老人回应，符合 PRD #2 的演示链路。
- 功能：子女按钮、定时问候、视觉触发问候共用该路径，播完都请求 xiaozhi 自动续听。
- 文件：`server/internal/domains/reminder/service.go`
- 内容：提醒播报调用 `InjectSpeak` 时改用 `proactiveSpeakOptions()`，设置 `AutoListen=true`。
- 目的：提醒播完后给老人语音回答“好/知道了”的入口，避免安伴主动播报形成单向打断。
- 功能：提醒到点播报后，manager 注入消息会显式携带 `auto_listen=true`。
- 验证：已运行 `go test -count=1 ./internal/domains/greeting ./internal/domains/reminder`，通过；已运行 `go test -count=1 ./...`、`go build ./...`、`go vet ./...`，均通过；已运行 `npm test --prefix web`，40 个 smoke 测试通过；`git diff --check` 无空白错误。

### 方案 C 架构边界守护测试

- 文件：`server/internal/architecture/architecture_test.go`
- 内容：新增架构边界测试，扫描生产 Go 文件，要求 xiaozhi manager OpenAPI 细节只出现在 `internal/xiaozhiclient`；`childapi` 不直接 import `store` 或 `xiaozhiclient`；各 `domains` 不互相 import。
- 目的：把 AGENTS.md 三条架构铁律和方案 C“冻结 xiaozhi、安伴可插拔增强”的边界变成可执行测试，降低后续开发把安伴写成大产品式深耦合的风险。
- 功能影响：无运行时影响；新增的是测试护栏。
- 验证：已运行 `go test -count=1 ./internal/architecture`，通过。首次版本把注释里的 `X-API-Token` 说明误判为越界，已收窄为只检查 Go 字符串字面量；随后运行 `go test -count=1 ./...`、`go build ./...`、`go vet ./...`，均通过；已运行 `npm test --prefix web`，40 个 smoke 测试通过；`git diff --check` 无空白错误。

### 设备到货联调守门 RED 测试

- 文件：`server/internal/preflight/preflight_test.go`
- 内容：新增 preflight RED 测试，要求联调守门报告始终包含“纯 xiaozhi 语音闭环 Gate A 需人工先过”，并在给出设备 ID 时通过 xiaozhi manager 设备状态 API 校验设备在线；缺设备 ID 时跳过该可选检查，manager/token 错误或设备离线时失败。
- 目的：把完整文档仓“三周计划 W1 先只跑原版小智”和方案 C“安伴可插拔增强”的纪律变成可重复运行的本仓检查，避免跳过纯 xiaozhi 验收就调安伴功能。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期 `preflight.Run`、`Report`、`StatusManual` 等尚未实现。
- 验证：已运行 `go test ./internal/preflight`，按预期编译失败。失败原因为 `Run`、`StatusManual`、`StatusPass`、`FormatReport`、`Report`、`Status` 等尚未实现。

### 设备到货联调守门 GREEN 实现

- 文件：`server/internal/preflight/preflight.go`
- 内容：新增 preflight 报告模型、`Run` 和 `FormatReport`，包含 Gate A 手工项、可选 manager 设备在线检查、失败判断和可读输出。
- 目的：提供一个不触发设备播报、不改 xiaozhi 的联调守门，先确认纯小智语音闭环已人工通过，再确认安伴能用 manager OpenAPI 读到设备在线。
- 功能：提供 `pass/fail/skip/manual` 四类检查状态；设备 ID 为空时跳过在线检查，token/manager/设备错误或离线时失败。
- 文件：`server/cmd/anban-preflight/main.go`
- 内容：新增 `anban-preflight` 命令，从现有 `ANBAN_*` 配置读取 manager 地址和 token，可通过 `-device-id` 或 `ANBAN_PREFLIGHT_DEVICE_ID` 指定设备。
- 目的：设备到货后能在启动安伴业务功能前跑一次可重复的本仓预检。
- 功能：输出 preflight 报告；如 manager 设备检查失败则以非 0 退出码结束。
- 验证：已运行 `go test -count=1 ./internal/preflight`，通过；已运行 `go test -count=1 ./cmd/anban-preflight`，通过。

### 联调守门文档与缓存忽略

- 文件：`README.md`
- 内容：在本地启动步骤中加入 `go run ./cmd/anban-preflight -device-id <xiaozhi设备ID>`。
- 目的：把“先纯 xiaozhi，再接安伴”的联调顺序放到仓库第一入口。
- 功能影响：无运行时影响。
- 文件：`docs/deployment/方案C部署与联调指南.md`
- 内容：在启动安伴后端前加入 preflight 命令说明、输出示例和最小联调顺序更新。
- 目的：设备到手后能照着同一份指南执行，不把 preflight 的 `[SKIP]` 误解成 Gate A 通过。
- 功能影响：无运行时影响。
- 文件：`.gitignore`
- 内容：忽略 `.gocache-go/`、`.gotmp-go/` 和 server 下同名 Go 本地缓存目录。
- 目的：当前沙箱不能访问 Windows 用户目录下的 Go build cache，验证时需要把 `GOCACHE/GOTMPDIR` 切进仓库；忽略这些目录避免污染工作区。
- 功能影响：无运行时影响。
- 验证：已运行 `go run ./cmd/anban-preflight`（使用 dummy manager/token 且不传设备 ID），输出 `[MANUAL]` Gate A 和 `[SKIP]` 设备检查，未访问 manager、未触发播报；已运行 `go test -count=1 ./...`、`go build ./...`、`go vet ./...`，均通过；已运行 `npm test --prefix web`，40 个 smoke 测试通过。

## 2026-06-06

### 方案 C 部署与联调文档

- 文件：`docs/deployment/方案C部署与联调指南.md`
- 内容：新增设备到手后的方案 C 部署指南，明确 `xiaozhi-esp32-server-golang` 与 `anban` 是两个进程，先纯 xiaozhi 跑通，再把安伴作为可选增强接入。
- 目的：回应“设备到了按方案 C 怎么部署、这个仓库是什么”的对齐问题，避免把 xiaozhi 上游源码或部署责任误纳入本仓库。
- 功能影响：无运行时影响。
- 文件：`README.md`
- 内容：补充本仓库边界、可插拔部署说明和方案 C 部署指南入口。
- 目的：让打开仓库的人第一眼知道本仓库不是 xiaozhi 仓库，而是安伴增强服务。
- 功能影响：无运行时影响。
- 文件：`docs/README.md`
- 内容：加入 `deployment/方案C部署与联调指南.md` 文档索引。
- 目的：让编码工作副本内能快速找到部署与设备联调指南。
- 功能影响：无运行时影响。
- 验证：已运行 `git diff --check`，无空白错误；已检查 README、docs 索引和部署指南包含方案 C、可选增强、`ANBAN_MANAGER_BASE_URL`、`127.0.0.1:7890`、Gate A 等关键字。仅有 Git 提示 LF 将在 Windows 下转换为 CRLF。

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

### 00:25 profile 画像召回提示词 RED 测试

- 文件：`server/internal/domains/profile/service_test.go`
- 内容：新增 `TestBuildPromptGuidesFamilyProfileRecall`，要求 `BuildPrompt` 生成的 xiaozhi agent prompt 明确包含“问到子女或孙辈姓名”“直接依据家庭画像回答名字”“不知道再说明”等召回行为指令。
- 目的：对齐完整 PRD #5 路演高光“老人问‘我孙子叫啥’→ AI 答出名字”，增强 Level 2“仅画像注入”时的可演示稳定性。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 prompt 只有泛化的“优先使用家庭画像”，缺少针对家庭成员姓名召回的明确行为约束。
- 验证：已运行 `go test ./internal/domains/profile`，得到有效 RED：`TestBuildPromptGuidesFamilyProfileRecall` 失败，prompt 中缺少“问到子女或孙辈姓名”的明确召回指令。

### 00:30 profile 画像召回提示词 GREEN 实现

- 文件：`server/internal/domains/profile/service.go`
- 内容：在 `BuildPrompt` 的静态提示词中新增一条召回指令：老人问到子女/孙辈姓名、称呼、喜好、健康或忌口时，直接依据家庭画像回答名字或事实，不知道再说明。
- 目的：提升 PRD #5 “我孙子叫啥”这类画像召回高光的稳定性，让 xiaozhi agent 不只是“理解画像”，而是明确知道该如何回答画像事实问题。
- 功能：子女端保存画像后写入 xiaozhi 的角色 prompt 会携带更明确的家庭画像召回行为约束；仍走原有 1500 rune 上限裁剪。
- 验证：
  - `go test ./internal/domains/profile` 通过。
  - `go test -count=1 -cover ./internal/domains/profile` 通过，profile 包覆盖率 85.0%。
  - `npm test --prefix web` 通过，38 个测试全绿。
  - 在 `web/` 运行 `node --test --experimental-test-coverage smoke.test.mjs` 通过，整体 line 89.23%、function 89.66%。
  - 在 `server/` 使用 D 盘 `GOTMPDIR` 运行 `go test -count=1 ./...` 通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR` 运行 `go build ./...` 通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR` 运行 `go vet ./...` 通过。
  - 已运行 `go clean -cache`，并删除 `.gocache-go/README` 与 `.gocache-go/trim.txt` 缓存残留。

### 00:37 子女端 API 错误提示 RED 测试

- 文件：`web/smoke.test.mjs`
- 内容：新增错误提示 RED 测试，要求 `formatApiErrorNotice` 在遇到 `ApiError('主动语音配额已用', 429)` 时优先显示后端错误原因“主动语音配额已用（429）”，普通错误仍显示调用方 fallback；同时要求 `app.js` 的 `handleApiError` 使用该 formatter。
- 目的：对齐完整 PRD #2/#6/#7 “同一 10 分钟窗口至多 1 条主动语音输出”的子女端可解释性，避免主动问候被配额挡住时页面误显示“问候接口暂未接入（429）”。
- 功能影响：暂无生产功能；这是 TDD RED 阶段，预期当前 Web 没有 `api-error-notice.js`，且 `handleApiError` 只拼 fallback 与状态码。
- 验证：已运行 `npm test --prefix web`，得到有效 RED：40 个测试中 2 个失败，失败原因分别是缺少 `web/api-error-notice.js`，以及 `app.js` 未接入 `formatApiErrorNotice`。

### 00:41 子女端 API 错误提示 GREEN 实现

- 文件：`web/api-error-notice.js`
- 内容：新增 `formatApiErrorNotice(error, fallback)`，对 `ApiError` 优先展示后端错误消息并追加状态码；非 API 错误保持调用方 fallback。
- 文件：`web/app.js`
- 内容：导入 `formatApiErrorNotice`，并让 `handleApiError` 统一调用该 formatter 后再 `showNotice`。
- 目的：让主动语音配额、manager 注入失败等后端明确错误在子女端可读，减少联调时把业务限制误判成接口未接入。
- 功能：例如后端返回 429 `{error:"主动语音配额已用"}` 时，页面提示“主动语音配额已用（429）”；普通网络异常仍显示原 fallback。
- 验证：
  - `npm test --prefix web` 通过，40 个测试全绿。
  - 在 `web/` 运行 `node --test --experimental-test-coverage smoke.test.mjs` 通过，整体 line 92.20%、function 93.33%；`web/api-error-notice.js` line/function 100%。
  - 在 `server/` 使用 D 盘 `GOTMPDIR` 运行 `go test -count=1 ./...` 通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR` 运行 `go build ./...` 通过。
  - 在 `server/` 使用 D 盘 `GOTMPDIR` 运行 `go vet ./...` 通过。
  - 首次 `go clean -cache` 有一个缓存文件被短暂占用，等待 2 秒后重试成功；已删除 `.gocache-go/README` 与 `.gocache-go/trim.txt` 缓存残留。

### 11:18 方案 C 部署指南补充当前阶段范围

- 文件：`docs/deployment/方案C部署与联调指南.md`
- 内容：新增“当前阶段范围”，明确现在只优先 Gate A 纯 xiaozhi、Gate B manager 接入、Gate C 子女端最小闭环；画像先服务基础演示点，视觉最后接且允许降级。
- 目的：回应“先做基础框架和基本功能”的对齐要求，避免执行时把安伴做成复杂大产品，或在 Gate A 未验证前继续扩大功能面。
- 功能影响：仅文档变更；不改变后端、前端或部署行为。
- 验证：已检查部署指南一级章节编号，确认新增章节后编号连续。
