# 波1：补全 PRD 必演 7 条到全规格（anban-code，前端+后端）— 2026-06-16

> 给执行者（含 Codex goal 模式）：你在 **anban-code** 仓库工作。本波目标：把 PRD（`AnBan-C/docs/安伴V0.1产品文档PRD.md` §3.1 的 7 条必演）从"基本可用"补到**全规格**。只动 anban 这一层（前端 `childweb/` + 后端 `server/`），**不碰 xiaozhi 代码、不改其架构**；缺的外部能力按"该功能未实现"占位。全程 TDD，保持现有测试全绿。**保活（设备睡眠）属固件/连接侧，不在本波，别去修。**

---

## 0. 开工准备：一次性读取（读完即标记完成，**勿重复整文件重读**）
每项只完整读一次，读完凭上下文工作、需要细节只 grep 定位：
1. 本文件（通读）。
2. PRD 必演 7 条：`../AnBan-C/docs/安伴V0.1产品文档PRD.md` §3.1（若该路径不在本仓库，则本文件 §2/§3 已转述其要点，按本文件即可）。
3. 后端域：`server/internal/domains/{greeting,reminder,vision,profile}/service.go` 与各 `types.go`；`server/internal/xiaozhiclient/client.go`、`http_client.go`、`fake.go`；`server/internal/scheduler/`；`server/internal/proactive/`（主动语音配额 ProactiveVoiceGate）；`server/cmd/anban/main.go`（装配）。
4. 前端：`childweb/app.js`、`childweb/api/client.js`、`childweb/integration-core.js`、`childweb/index.html`（屏与 init 结构）。
5. `AGENTS.md`、`docs/现状与交接-2026-06-14.md`、`docs/childweb-...`（真机事实：访问码、deviceId=`9c:13:9e:8b:af:28`）。
读完直接进 §8 的 W1.1。

---

## 1. 现状精确盘点（已查证）
| # | 必演功能 | 现状 | 本波要补 |
|---|---|---|---|
| #1 | 对话+打断 | ✅ 真机通（xiaozhi 负责） | 无 |
| #2 | 主动问候(定时+子女触发) | **后端已完整**：greeting 域有 ScheduleSlot/GreetingSchedule、CronScheduler 定时、trigger、配额。client.js 有 triggerGreeting/getGreetingSchedule/updateGreetingSchedule。**但 childweb 没接** | **纯前端**：childweb 加"问候"触发按钮 + 早/午/晚定时配置页；并验证 live cron 真触发 |
| #3 | 留言播报 | ✅ 真机通 | 无 |
| #4 | 设备状态 | ✅（轮询） | 无（WebSocket 推送留到波3） |
| #5 | 记忆+画像 | 画像 ✅ 注入（profile.BuildPrompt→SetRolePrompt）；**对话沉淀/召回未做**（仅 PRD Level 2） | **后端**：route C 自建轻量沉淀（见 §3.3） |
| #6 | 主动提醒 | 单次 ✅ + 语音回话 ✅；分类文案在 reminderText（med/birthday/festival）；**无重复频率、无"重要"字段** | **后端+前端**：加重复 + important（见 §3.2） |
| #7 | 轻量视觉触发 | **仅桩**：vision routes 是 stub，server 无 camera/CallDeviceMCPTool 实现，xiaozhiclient 无该方法；manager 能否经 OpenAPI 调设备摄像头**未知** | **先验可行性**，可行则实装"看一眼"+presence，不可行按 PRD 降级到加分（见 §3.4） |

**配额铁律**：同一 10 分钟窗口至多 1 条主动语音（#2/#6/#7 共享 `ProactiveVoiceGate`）。留言(#3)是点对点、**不**走配额（已修，勿动）。#5 沉淀是后台读写、不发语音、不占配额。

---

## 2. 通用约束
- 只动 anban：后端 `server/` + 前端 `childweb/`。**不改 xiaozhi、不改后端对设备的协议**。
- 后端复用现有 `xiaozhiclient`（南向）与 `scheduler`（robfig/cron，已锁东八区）；新功能加在对应域，**域间不互相 import**（走 pkg/types 接口或 main.go 装配）。`internal/architecture` 守护测试不许红。
- 前端复用 `childweb/api/client.js`（别重写），纯逻辑抽 `integration-core.js` 加测。
- 不提交密钥；新配置走 env（见 §5），写进 `childweb/README` / 部署说明但不入库真值。
- TDD：先写测试。全程保持 `go build/vet/test ./...`（server，GOPROXY=https://goproxy.cn,direct GOSUMDB=off CGO_ENABLED=0）+ `node --test web/smoke.test.mjs`(80) + `node --test childweb/smoke.test.mjs` 全绿。
- conventional commits 小步提交；每阶段追加 `docs/REALTIME_CHANGELOG.md`。

---

## 3. 各条详细设计

### 3.1 #2 主动问候（纯前端 + 验证）
后端已具备，本条只在 childweb 接通：
- **触发按钮**：在首页（或消息页）加"问候老人"按钮 → `client.triggerGreeting({deviceId, tonePreset})`；展示结果（已播报/排队中/失败），文案复用 `greeting-result` 风格。
- **定时配置页**：设置页（s-mine）加"主动问候时段"区，早/午/晚三个 `{time,enabled}` 槽 → 进入时 `getGreetingSchedule` 回填，保存 `updateGreetingSchedule({deviceId,slots})`。
- 后端无需改；但要**真机验证定时 cron 触发**（部署后设一个近时段，确认设备到点开口；遵守配额、对话中排队不打断）。
- 抽纯函数（slots 校验/展示）加测。

### 3.2 #6 提醒：重复频率 + 重要（后端+前端）
**后端 reminder 域**：
- 加重复模型：`Recurrence` 字段（none/daily/weekdays/weekends/custom-dates）。`Create` 接受 `recurrence`（默认 none=单次，保持现有行为）。
- 触发后重排：提醒播报后，若 recurrence≠none，按规则计算**下一次** scheduledAt 并重新入调度（cron 或重排一次性任务）；`RestoreScheduled` 启动时也要恢复重复提醒。
- 加 `important bool`：重要提醒在播报文案/标记上区分；**是否绕过配额由你定**——建议**仍受配额约束**（避免抢话），但排队优先级更高；若 PRD"强制播报"语义需要绕过，必须在 changelog 写明理由。
- 分类 med/birthday/festival 文案已在 `reminderText`，确保 API 透传 category。
- 全部 TDD（重复换算、下一次时间、重启恢复）。
**前端 childweb**：
- 解锁频率选择器（去掉 `notImplemented('重复提醒')`），把"每天/工作日/周末/自定义"映射到后端 recurrence；"自定义"用已有日历选具体日期。
- 解锁"重要"开关（去 `notImplemented('重要提醒')`），透传 important。
- `nextOccurrenceUTC` 等纯逻辑扩展加测。

### 3.3 #5 记忆：route C 安伴自建轻量沉淀（后端）
目标（PRD §3.1 #5 / §8.2）：跨会话注入"家庭画像 + 对话中沉淀的事实"，单次注入 ≤1500 token。画像已做；本条补"对话沉淀"。
- **新增 `internal/llm`**：一个 OpenAI 兼容的 Chat 客户端（豆包 Ark），base/key/model 走 env：`ANBAN_LLM_BASE_URL`、`ANBAN_LLM_API_KEY`、`ANBAN_LLM_MODEL`（默认豆包）。仅用于离线抽取，不参与实时对话。
- **新增 `memory` 域（或并入 profile）**：每设备存一段"沉淀事实"文本/列表（≤N 条），带去重与时间。
- **沉淀 job**（scheduler 周期，如每 10–15 分钟 / 或对话后触发）：`GetHistory(deviceId)` 取新对话 → 调 `internal/llm` 用固定 prompt 抽取"值得长期记住的事实"（如"老人说腰酸""喜欢豫剧"）→ 合并入沉淀（去重、限量）。
- **召回 = 注入**（轻量，无需向量）：把"画像 + 沉淀事实"合并进 `profile.BuildPrompt`（控制总 token ≤1500），经 `SetRolePrompt` 回写设备绑定 agent 的 custom_prompt。即设备下次对话就带上记忆。
- 降级：`ANBAN_LLM_API_KEY` 缺省时，沉淀 job 跳过（只画像注入，相当于 PRD Level 2），不报错。
- 全部 TDD（抽取结果合并/去重/限量、BuildPrompt token 上限、缺 key 降级）；LLM 调用用接口 + fake 测，不打真实网络。

### 3.4 #7 轻量视觉触发（先验可行性，再决定实装或降级）
PRD 标高风险，且依赖设备在线（保活）。**W1.4 第一步是可行性闸门**：
- **可行性验证**：确认 anban 能否经 manager OpenAPI **调用设备 MCP 工具 `self.camera.take_photo`**（带 question，返回设备侧 VLM 的文字答案——VLM 在设备/核心侧，见 manager init 的 vision endpoint）。查 manager OpenAPI 是否有 MCP-invoke 端点；若有 → 在 `xiaozhiclient` 加 `CallDeviceMCPTool(deviceID, tool, args)`。
- **可行 → 实装**：
  - 「看一眼/视觉问答」：childweb 加"看一眼"按钮 → `captureVision`/新接口 → CallDeviceMCPTool(camera, {question}) → 返回图/答展示。
  - 「presence 触发」：scheduler 周期（≥每 10s）调 camera 问"画面里有没有人" → 解析 → 状态机 NO_ONE→SOMEONE 时触发一句问候（走配额）。**依赖设备在线**，离线时静默跳过。
- **不可行 / 不稳 → 降级**（PRD §6.3 允许）：vision 在 childweb 以"看一眼"按钮存在但走 `notImplemented`/占位，presence 不做；在 `docs/decisions/` 记一笔"#7 因 manager 不支持 MCP-invoke / 保活不稳 降级到加分"。**不要为 #7 阻塞整波**。

---

## 4. 新依赖 / 配置（env，部署侧填真值，**不入库**）
- `ANBAN_LLM_BASE_URL` / `ANBAN_LLM_API_KEY` / `ANBAN_LLM_MODEL`（#5 沉淀用，豆包 Ark）。
- （#7 若实装）VLM 走设备 camera 工具，无需额外 key；如需独立 VLM 端点再加 env。
- 在 `server` 的配置加载处读取；缺省时相关功能优雅降级（#5 跳过沉淀、#7 降级）。`anban.env` 模板更新（占位，不写真 key）。

---

## 5. 验收 / 完成判据
- 五套测试全绿：server `go build/vet/test`、`web` 80、`childweb` 冒烟；新增逻辑都有测试。
- #2：childweb 能触发问候 + 配置时段；真机定时到点设备开口（部署后验）。
- #6：childweb 能建"每天/工作日/自定义"重复提醒 + 重要标记；后端重复到点重排、重启恢复。
- #5：配了 LLM key 时，对话后沉淀事实并在下次对话注入（日志可见 SetRolePrompt 更新、custom_prompt 含沉淀）；缺 key 时优雅降级。
- #7：可行性结论明确（实装 or 降级），二者其一干净落地、不破坏其它链路。
- `internal/architecture` 守护不红；不改 xiaozhi；密钥不入库；REALTIME_CHANGELOG 每阶段更新。

---

## 6. 阶段（每阶段独立小提交）
- **W1.1 #2 问候前端**（最轻，先做出"主动开口"的可见价值）。
- **W1.2 #6 提醒重复+重要**（后端 recurrence + 前端解锁）。
- **W1.3 #5 记忆沉淀**（internal/llm + memory + distill job + BuildPrompt 注入；缺 key 降级）。
- **W1.4 #7 视觉**（先可行性闸门 → 实装 or 降级；不阻塞）。

---

## 7. 风险
- **#7 最不确定**：manager 可能不支持 OpenAPI 调设备 MCP 工具 → 直接走降级（PRD 允许），别硬刚。
- **#5 LLM 成本/key**：需 Ark key；无 key 自动降级。沉淀 prompt 要克制 token。
- **主动功能真机表现依赖保活**：#2/#6/#7 到点时设备可能在睡（保活是另轨问题）；本波只保证"设备在线时正确触发"，真机演示前需先解决保活或手动唤醒。
- 别破坏已通的 #1/#3/#4 与 childweb 现有链路；留言不限流别动。
