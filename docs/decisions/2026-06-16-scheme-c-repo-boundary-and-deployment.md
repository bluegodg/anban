# 方案 C：仓库边界、两服务部署与可插拔验收

- 日期：2026-06-16
- 状态：已决策，执行中
- 决策人：项目组

## 上下文

设备已经到手，当前最容易混淆的不是某个接口怎么写，而是三件事：

1. `anban-code` 这个仓库到底是什么，和完整文档仓、xiaozhi 上游仓怎么放。
2. 方案 C 到底是不是“两个服务进程”：一个 `xiaozhi-esp32-server-golang`，一个安伴 `anban`。
3. “先做基础框架和基本功能”什么时候算实现，避免继续无意识扩成大产品。

完整设计文档仍以 `AnBan-docs-repo` 为权威。本仓 `docs/` 只是编码和部署的工作副本。需要查 PRD、架构背景、历史决策时，先拉完整文档仓；不要只凭本仓的几份摘录下判断。

## 决策

继续采用方案 C：`xiaozhi-esp32-server-golang` 是冻结上游，安伴是独立增强服务。

部署和仓库口径固定为：

```text
D:\Program\Project\
  AnBan-docs-repo\                     # 权威文档仓，只放文档和规划
  xiaozhi-esp32-server-golang\         # 冻结上游，负责原版小智语音闭环
  anban-code\                          # 本仓，负责安伴后端 + 子女端
```

运行拓扑固定为：

```text
设备 <-> xiaozhi-esp32-server-golang <-> 云端 ASR/LLM/TTS

子女端 Web/PWA -> anban -> xiaozhi manager OpenAPI -> 设备
```

这里说的“两服务”是部署和职责口径：xiaozhi 上游服务 + 安伴服务。xiaozhi 上游内部可能有 core/manager 的不同运行形态，但那仍归 xiaozhi 仓库和上游文档管理，不拆进 `anban-code`。

## 本仓负责什么

`anban-code` 是安伴增强层代码仓，负责：

- `server/`：安伴 Go 后端，入口是 `server/cmd/anban`。
- `server/cmd/anban-preflight`：联调前预检 manager URL、OpenAPI token、设备 ID，以及 Gate A 人工确认。
- `childweb/`：当前路演优先的子女端 PWA。
- `web/`：早期静态子女端骨架和 API/交互验证页面。
- `docs/`：编码和部署常用文档工作副本。
- `deploy.sh`：只部署本仓编译出来的 `anban` 二进制。

`anban-code` 不负责：

- xiaozhi 服务端源码。
- ESP32 设备固件。
- 云端 ASR/LLM/TTS/VLM 服务实现。
- xiaozhi manager 的数据表所有权。
- 原版小智语音闭环本身。
- 完整文档仓。

## 可插拔边界

方案 C 必须满足这四句话：

1. 只部署 `xiaozhi-esp32-server-golang`，设备也能完成原版小智对话。
2. 再部署 `anban`，才增加安伴的留言、问候、提醒、画像、状态、视觉等产品能力。
3. 停掉 `anban` 后，子女端增强能力不可用是正常的，但原版小智对话不能坏。
4. 重启 `anban` 后，增强能力恢复，不需要改 xiaozhi 上游代码。

如果第 3 条失败，说明架构边界被破坏；这时不继续堆业务功能，先修边界。

## 编码边界

本仓继续遵守三条铁律：

- 所有 xiaozhi 调用只能经过 `server/internal/xiaozhiclient`。
- `childapi` 只调 `domains`，不直接碰 xiaozhi 或数据库。
- 数据方向是 `anban -> xiaozhi` 的命令或轮询，不做 xiaozhi 主动推给 anban 的反向依赖。

各业务域仍按 `internal/domains/<domain>/handler.go|service.go|store.go|types.go` 的骨架写，先对 `xiaozhiclient.FakeClient` 做 TDD，再接真实 manager OpenAPI。

## 部署 Gate

### Gate A：只跑 xiaozhi

先在 `xiaozhi-esp32-server-golang` 仓库按上游文档启动 xiaozhi。此时不要启动 `anban`。

通过标准：

- 设备能连接 xiaozhi。
- 设备能完成一次“唤醒 -> 说话 -> 听到回复”。
- 打断或连续对话符合上游预期。
- 记录 manager 地址、WS/OTA 地址、设备 ID。

Gate A 不过，停在硬件、固件、网络、云 API、xiaozhi 配置上排查。

### Gate B：manager token 和 anban 预检

在 xiaozhi manager 签发 OpenAPI token。安伴最小环境变量：

```powershell
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"
$env:ANBAN_MANAGER_BASE_URL="http://localhost:8080"
$env:ANBAN_MANAGER_API_TOKEN="<manager 签发的 token>"
$env:ANBAN_ACCESS_CODE="demo"
```

回到本仓：

```powershell
Set-Location server
go run ./cmd/anban-preflight -device-id <xiaozhi设备ID>
go run ./cmd/anban-preflight -device-id <xiaozhi设备ID> --xiaozhi-gate-passed
```

通过标准：

- manager URL 可达。
- token 被接受。
- 指定设备 ID 能在 manager 侧查到，状态符合预期。

### Gate C：启动 anban 和子女端

启动安伴：

```powershell
Set-Location server
go run ./cmd/anban
```

健康检查：

```powershell
Invoke-RestMethod http://localhost:8090/health
```

启动当前子女端 PWA：

```powershell
npm start --prefix childweb
```

浏览器打开：

```text
http://127.0.0.1:3001/
```

页面填写：

- 后端地址：`http://127.0.0.1:8090`
- 访问码：`ANBAN_ACCESS_CODE` 的值
- 设备 ID：Gate A 记录的 xiaozhi 设备 ID

联调顺序固定为：

1. 状态页能显示在线、离线或最近互动。
2. 子女端发留言，设备能播报。
3. 子女端触发问候，设备能开口。
4. 创建短时间提醒，到点能播报。
5. 画像能保存，并同步到 xiaozhi role/prompt。
6. 视觉最后接，必要时降级。

### Gate D：可插拔验证

停止 `anban`，保留 xiaozhi。

通过标准：

- 子女端增强能力不可用。
- 设备原版小智对话仍然可用。
- 重启 `anban` 后，子女端增强能力恢复。

Gate D 通过后，才可以说方案 C 的部署边界成立。

## 当前阶段完成口径

“基础框架和基本功能”不能只按目录或代码量判断。当前准确口径是：

- 软件侧地基已经具备：Go 后端、业务域骨架、`xiaozhiclient`、预检工具、子女端页面都在。
- 产品侧仍以真机 Gate 为准：Gate A/B/C/D 没完整过，就不能说阶段目标完全实现。
- 当前开发只围绕 PRD V0.1 路演必演链路收口，不扩多租户、运营后台、计费、长期健康趋势等大产品能力。

优先级固定为：

1. 被动语音对话：由 xiaozhi 负责，anban 不抢主链路。
2. 子女留言：必须保住，且不走主动语音配额。
3. 设备状态和最近互动：给子女端可见性。
4. 主动问候和提醒：短话术、状态可见、可演示。
5. 家庭画像：先保存并同步到 role/prompt。
6. 视觉：最后做，可降级，不阻塞前面。

## 代理和文档同步

需要拉 GitHub 完整文档或依赖时，只在当前 PowerShell 会话设置代理：

```powershell
$env:HTTP_PROXY="http://127.0.0.1:7890"
$env:HTTPS_PROXY="http://127.0.0.1:7890"
git -C D:\Program\Project\AnBan-docs-repo pull --ff-only
```

不要把代理、manager token、访问码、云 API key 写进仓库。

## 风险与止损

- 如果 manager OpenAPI 某项能力实测不可用，优先在 `xiaozhiclient` 做降级或标记未实现；不要绕过边界直接在业务域里发 HTTP。
- 如果只跑 xiaozhi 都不稳定，不继续写安伴功能，先修 Gate A。
- 如果停掉 anban 会影响原版对话，立即停止新增功能，回到方案 C 边界排查。
- 如果某个能力必须改 xiaozhi core，先写新的 `decisions/` 决策记录，经确认后再动，不默认 fork。

## 关联文档

- `AGENTS.md`
- `docs/README.md`
- `docs/安伴V0.1产品文档PRD.md`
- `docs/specs/2026-05-28-server-architecture-design.md`
- `docs/specs/2026-05-29-xiaozhi-full-architecture-map.md`
- `docs/decisions/2026-05-29-server-architecture.md`
- `docs/deployment/方案C当前执行说明.md`
- `docs/deployment/方案C现场部署作战卡.md`
- `docs/deployment/方案C部署与联调指南.md`
- `AnBan-docs-repo`
