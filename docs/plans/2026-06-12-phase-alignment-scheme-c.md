# 当前阶段对齐与方案 C 执行说明

> 日期：2026-06-12
> 目的：把“现在做到哪一步、是否按 PRD、设备到手后怎么按方案 C 部署、这个仓库到底是什么”固定成仓库内可复用结论。
> 权威来源：完整设计文档以 `AnBan-docs-repo` 为准；本文件是 `anban-code` 仓库里的执行对齐版。

## 1. 当前结论

当前应回到“基础框架 + 基本功能 + 真设备联调”的阶段，不继续扩大成大产品。

用三周计划来定位：

- 第 0 阶段的软件地基已经基本成形：安伴后端、子女端 Web 骨架、manager OpenAPI 适配、预检工具、留言/状态/问候/提醒等基础域都在本仓库内推进。
- 设备已经到手后，优先级切到第 1 周闸门：先证明纯 `xiaozhi-esp32-server-golang` 能让设备完成原版小智对话。
- 在 Gate A 没有现场通过前，不应继续堆视觉、运营后台、多账号、复杂权限、长期分析等大产品能力。

之前“先做最基础框架和基本功能”的目标，按代码侧看已经接近完成；按产品目标看还不能宣告完成。原因是 PRD 的底线不是“代码存在”，而是真设备上能通过下面四个 Gate。

| Gate | 判断标准 | 当前意义 |
|---|---|---|
| Gate A：纯 xiaozhi | 不启动安伴，设备也能唤醒、回应、打断 | 这是所有安伴功能的前提 |
| Gate B：manager 接入 | 安伴能用 manager URL/token 访问 xiaozhi OpenAPI | 证明两个进程能通信 |
| Gate C：子女端最小闭环 | 状态、留言、问候、提醒能通过 anban 跑通 | 证明基础安伴功能可演示 |
| Gate D：可插拔性 | 停掉 anban 后，原版小智仍能对话 | 证明方案 C 没破坏上游 |

所以当前目标状态是：软件骨架已推进，最终目标未完全实现；下一步必须先做真设备 Gate A/B/C/D。

## 2. 方案 C 是什么

方案 C 的核心是两个进程、可插拔部署：

1. `xiaozhi-esp32-server-golang`
   - 冻结上游。
   - 负责设备连接、WebSocket 语音链路、ASR、LLM、TTS、原版小智对话。
   - 只部署它时，设备必须仍然能正常对话。

2. `anban`
   - 本仓库实现。
   - 是独立的可选增强服务。
   - 负责子女端 API、留言、状态、主动问候、主动提醒、画像、视觉等安伴产品能力。
   - 通过 xiaozhi manager OpenAPI 驱动 xiaozhi，不能直接改 xiaozhi 内核。

固定数据方向：

```text
子女端 web -> anban -> xiaozhi manager OpenAPI -> xiaozhi core -> 设备
```

固定原版语音链路：

```text
设备 <-> xiaozhi-esp32-server-golang <-> 云端 ASR/LLM/TTS
```

这意味着：安伴不是 xiaozhi 的 fork，也不是 xiaozhi 的内置模块。安伴只是一个旁路增强进程。

## 3. 这个仓库是什么

`anban-code` 是安伴代码仓，主要包含：

- `server/`：安伴 Go 后端。
- `server/internal/xiaozhiclient/`：唯一允许懂 xiaozhi manager OpenAPI 的适配层。
- `server/internal/childapi/`：子女端 HTTP API 边界。
- `server/internal/domains/`：安伴业务域，包括 message、status、greeting、reminder、profile、vision。
- `web/`：子女端静态 Web 骨架。
- `docs/`：编码常用文档工作副本，不是完整文档仓。

它不是：

- `xiaozhi-esp32-server-golang` 仓库。
- 设备固件仓库。
- 云端 ASR/LLM/TTS 的实现仓库。
- 原版小智基础对话能力的必需组件。

建议仓库并排放置：

```text
D:\Program\Project\
  anban-code\
  AnBan-docs-repo\
  xiaozhi-esp32-server-golang\
```

## 4. 现在怎么部署

设备到手后，不要先启动安伴。按下面顺序走。

### 4.1 准备代理和文档

如果需要拉 GitHub 或查完整文档，只在当前 PowerShell 会话设置代理：

```powershell
$env:HTTP_PROXY="http://127.0.0.1:7890"
$env:HTTPS_PROXY="http://127.0.0.1:7890"
```

不要把代理、token、key 写入仓库。

完整背景文档优先看 `AnBan-docs-repo`，本仓 `docs/` 只是编码工作副本。

### 4.2 Gate A：只部署 xiaozhi

在 `xiaozhi-esp32-server-golang` 仓库按上游文档部署。

这一步不启动 `anban`，只验证：

- manager 或健康检查能打开。
- 设备能连接到 xiaozhi。
- 设备能完成一次原版小智对话。
- 设备能打断或至少保持上游预期的对话行为。
- 记录真实设备 ID。

Gate A 不过，不进入安伴联调。

### 4.3 Gate B：拿 manager token 并跑 anban 预检

回到 `anban-code`：

```powershell
Copy-Item .env.example .env
```

至少配置：

```text
ANBAN_MANAGER_BASE_URL=http://localhost:8080
ANBAN_MANAGER_API_TOKEN=<manager 签发的 token>
ANBAN_ACCESS_CODE=demo
ANBAN_DB_DSN=anban.db
ANBAN_LISTEN_ADDR=:8090
ANBAN_ALLOWED_ORIGINS=http://127.0.0.1:5173,http://localhost:5173
```

运行预检：

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"
$env:ANBAN_MANAGER_BASE_URL="http://localhost:8080"
$env:ANBAN_MANAGER_API_TOKEN="<manager 签发的 token>"
$env:ANBAN_ACCESS_CODE="demo"
go run ./cmd/anban-preflight -device-id <xiaozhi设备ID>
```

确认 Gate A 已人工通过后，再跑：

```powershell
go run ./cmd/anban-preflight -device-id <xiaozhi设备ID> --xiaozhi-gate-passed
```

预检只做非侵入式检查，不会让设备播报，也不会改 xiaozhi。

### 4.4 Gate C：启动 anban 和子女端

启动安伴后端：

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"
$env:ANBAN_MANAGER_BASE_URL="http://localhost:8080"
$env:ANBAN_MANAGER_API_TOKEN="<manager 签发的 token>"
$env:ANBAN_ACCESS_CODE="demo"
go run ./cmd/anban
```

健康检查：

```powershell
Invoke-RestMethod http://localhost:8090/health
```

启动子女端：

```powershell
Set-Location web
python -m http.server 5173
```

浏览器打开：

```text
http://127.0.0.1:5173/
```

页面填写：

- 后端地址：`http://localhost:8090`
- 访问码：`ANBAN_ACCESS_CODE`
- 设备 ID：Gate A 记录的 xiaozhi 设备 ID

联调顺序固定为：

1. 状态。
2. 留言。
3. 主动问候。
4. 提醒。
5. 画像。
6. 视觉。

视觉最后，不阻塞前面基础链路。

### 4.5 Gate D：验证可插拔

停止 `anban`，保留 `xiaozhi-esp32-server-golang`。

必须看到：

- 设备仍能继续原版小智对话。
- 停掉安伴只影响子女端增强能力，不影响基础语音能力。
- 再启动安伴后，子女端功能恢复。

如果 Gate D 失败，说明方案 C 边界被破坏，要优先查架构边界，而不是继续写新功能。

## 5. 当前不要做什么

为了保持“基础框架和基本功能”目标不漂移，当前暂停这些方向：

- 复杂账号体系、多租户、计费、运营后台。
- 长期健康趋势、复杂风控、数据分析大盘。
- 深度视觉产品化、常驻监控、视频通话。
- 改 xiaozhi 内核或把 xiaozhi 代码搬进本仓库。
- 在业务域里绕过 `internal/xiaozhiclient` 直接请求 xiaozhi。

当前只允许围绕 Gate A/B/C/D 做必要修复、联调、文档和最小闭环补缺。

## 6. 目标什么时候算实现

只有同时满足下面条件，才能说“基础框架和基本功能目标已实现”：

- 纯 xiaozhi 设备对话通过，并有现场记录。
- `anban-preflight` 对 manager/token/设备状态检查通过。
- 子女端能完成状态、留言、主动问候、提醒的最小闭环。
- 停掉 `anban` 后，设备仍能原版对话。
- 当前代码测试通过，且没有把 xiaozhi 上游代码拉进本仓库。

在此之前，只能说“软件侧基础能力已准备，等待或正在真设备联调验证”。

## 7. 参考文档

- `docs/deployment/方案C部署与联调指南.md`
- `docs/specs/2026-05-28-server-architecture-design.md`
- `docs/specs/2026-05-29-xiaozhi-full-architecture-map.md`
- `docs/plans/2026-05-29-anban-backend-foundation.md`
- `AnBan-docs-repo/docs/安伴三周工作计划.md`
- `AnBan-docs-repo/docs/安伴V0.1产品文档PRD.md`
