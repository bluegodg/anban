# 方案 C 部署入口与阶段对齐

> 日期：2026-06-15
> 适用场景：设备已经到手，需要确认“现在做到哪一步”“方案 C 怎么部署”“`anban-code` 这个仓库到底负责什么”。
> 现场短版先看同目录的 `方案C现场部署作战卡.md`；当前执行稿见 `方案C当前执行说明.md`；长版细节见 `方案C仓库边界与部署总纲.md`、`方案C部署与联调指南.md`、`设备到手方案C首日执行单.md`。

## 0. 先把方向定住

当前阶段不是继续扩成“大产品”，而是回到 PRD V0.1 的路演目标：先让基础语音链路、子女端留言、状态、主动问候、提醒、画像这些基本闭环稳定可演示。

方案 C 的核心是两个进程、一个可插拔增强层：

```text
设备 <-> xiaozhi-esp32-server-golang <-> 云端 ASR/LLM/TTS
子女端 Web -> anban -> xiaozhi manager OpenAPI -> 设备
```

所以必须满足：

- 只部署 `xiaozhi-esp32-server-golang` 时，设备也能完成原版小智语音对话。
- 再部署 `anban` 后，才获得安伴的留言、状态、问候、提醒、画像、视觉等增强能力。
- 停掉 `anban` 不应该破坏原版小智对话。
- `anban` 只通过 xiaozhi manager OpenAPI 调用 xiaozhi，代码入口只能是 `server/internal/xiaozhiclient`。

## 1. 这个仓库是什么

`anban-code` 是安伴增强服务的代码仓，不是 xiaozhi fork。

它包含：

- `server/`：安伴 Go 后端，启动入口是 `server/cmd/anban`。
- `server/cmd/anban-preflight`：部署前预检 manager、token、设备 ID 和 Gate A 人工确认。
- `web/`：子女端网页，调用安伴后端的 childapi。
- `docs/`：编码与部署常用文档工作副本。

它不包含：

- xiaozhi 服务端源码。
- 设备固件。
- ASR/LLM/TTS 云服务实现。
- 完整设计文档仓。完整背景仍以 `AnBan-docs-repo` 为准。

建议仓库并排放置：

```text
D:\Program\Project\
  AnBan-docs-repo\
  anban-code\
  AnBan\research\xiaozhi-esp32-server-golang\
```

## 2. 设备到手后的部署顺序

### Gate A：只跑 xiaozhi

先部署并启动 `xiaozhi-esp32-server-golang`。此时不要启动 `anban`。

必须看到：

- 设备能连上 xiaozhi。
- 设备能完成一次“说话 -> 听到回复”。
- 打断、连续对话按上游预期工作。
- 这一步完全不依赖 `anban-code`。

如果 Gate A 不过，先排查硬件、固件、网络、云 API 和 xiaozhi 配置，不继续写安伴功能。

### Gate B：签 manager token，跑 anban 预检

在 xiaozhi manager 签发 OpenAPI token。安伴侧最少需要：

```text
ANBAN_MANAGER_BASE_URL=http://localhost:8080
ANBAN_MANAGER_API_TOKEN=<manager 签发的 token>
ANBAN_ACCESS_CODE=demo
```

在本仓运行：

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"
$env:ANBAN_MANAGER_BASE_URL="http://localhost:8080"
$env:ANBAN_MANAGER_API_TOKEN="<manager 签发的 token>"
$env:ANBAN_ACCESS_CODE="demo"

go run ./cmd/anban-preflight -device-id <xiaozhi设备ID>
go run ./cmd/anban-preflight -device-id <xiaozhi设备ID> --xiaozhi-gate-passed
```

第一条会提醒 Gate A 必须人工先过；第二条表示你已经确认纯 xiaozhi 链路通过。

### Gate C：启动 anban 和子女端

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
- 访问码：`ANBAN_ACCESS_CODE` 的值
- 设备 ID：Gate A 记录的 xiaozhi 设备 ID

### Gate D：验证可插拔

停止 `anban`，保留 xiaozhi。

必须看到：

- 子女端增强能力不可用，这是正常的。
- 设备原版小智对话仍然可用。
- 重启 `anban` 后，子女端增强能力恢复。

如果停掉 `anban` 后设备也不能对话，说明方案 C 边界被破坏，应先修架构边界。

## 3. 当前服务器口径

截至 2026-06-14 真机联调，当前线上口径是：

```text
服务器：101.34.214.149
xiaozhi manager：http://101.34.214.149:8080
xiaozhi WS/OTA：101.34.214.149:8989
anban：http://101.34.214.149:8090
子女端 Web：http://101.34.214.149:8091
已验证设备 ID：9c:13:9e:8b:af:28
```

本仓线上部署方式是交叉编译后上传二进制：

```powershell
bash deploy.sh
```

它只部署本仓的 `anban`。xiaozhi 仍按上游仓库自己的方式部署和运维。

## 4. 现在按 PRD 做到什么算“基础目标完成”

不要用“目录都建好了”判断完成。当前基础目标以 Gate 和路演必演链路为准：

| 检查点 | 完成含义 |
|---|---|
| Gate A | 原版小智语音底座成立 |
| Gate B | anban 能通过 manager OpenAPI 驱动 xiaozhi |
| Gate C | 子女端状态、留言、问候、提醒、画像形成最小闭环 |
| Gate D | anban 是可插拔增强层，停掉不影响原版对话 |

优先做 PRD 必演：

- #1 被动语音对话：主要由 xiaozhi 承担，anban 不抢这条链路。
- #2 主动问候：anban 调 xiaozhi inject。
- #3 子女端留言：本期绝对不能关，且留言不走主动语音配额。
- #4 设备状态：子女端要能看到在线、最近互动、留言状态。
- #5 家庭画像：先做到保存并同步到 xiaozhi prompt。
- #6 主动提醒：先做到创建、到点播报、状态可见。
- #7 视觉触发：最后接，可降级，不阻塞前面基础链路。

## 5. 当前不要做

- 不把 xiaozhi 源码拉进本仓。
- 不在 anban 里修设备保活问题；这是设备或固件侧问题。
- 不先做多租户、复杂账号、运营后台、计费、长期健康趋势。
- 不为了视觉能力牺牲留言、问候、提醒、状态这些基础演示链路。
- 不把 token、API key、代理地址写进仓库。

如果拉 GitHub 或依赖需要代理，只在当前 PowerShell 会话里设置：

```powershell
$env:HTTP_PROXY="http://127.0.0.1:7890"
$env:HTTPS_PROXY="http://127.0.0.1:7890"
```

## 6. 继续看哪几份

按这个顺序读：

1. `docs/现状与交接-2026-06-14.md`：真机联调后的现实坑位。
2. `AGENTS.md`：编码边界和依赖纪律。
3. `docs/安伴V0.1产品文档PRD.md`：路演必演功能和验收标准。
4. `docs/deployment/方案C现场部署作战卡.md`：设备在手时的现场短版。
5. `docs/deployment/方案C仓库边界与部署总纲.md`：完整部署总纲。
6. `docs/deployment/方案C部署与联调指南.md`：长版联调命令和 Gate。
7. `docs/deployment/设备到手方案C首日执行单.md`：首日执行单。
