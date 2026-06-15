# 方案 C 现场部署作战卡

> 日期：2026-06-15
> 场景：设备已经在手，需要现场按方案 C 把“原版小智 + 安伴增强层”跑起来。
> 长版说明见 `docs/deployment/方案C部署与联调指南.md`；阶段解释见 `docs/deployment/方案C当前执行说明.md`。

## 0. 先定当前阶段

现在不是继续扩成大产品。当前目标是 PRD V0.1 路演基础闭环：

1. 只部署 `xiaozhi-esp32-server-golang` 时，设备能完成原版小智语音对话。
2. 再部署 `anban` 后，子女端能完成状态、留言、问候、提醒、画像这些基础能力。
3. 停掉 `anban` 后，原版小智对话仍然可用。

只有 Gate A/B/C/D 都过，才能说“基础框架和基本功能目标已经实现”。否则准确说法是：软件侧已经准备好，正在做真机基础闭环收口。

## 1. 这个仓库是什么

`anban-code` 是安伴增强服务仓，不是 xiaozhi fork。

它负责：

- `server/`：安伴 Go 后端，入口是 `server/cmd/anban`。
- `server/cmd/anban-preflight`：联调前检查 manager URL、token、设备 ID 和 Gate A 人工确认。
- `web/`：子女端静态网页。
- `docs/`：编码和部署常用文档工作副本。
- `deploy.sh`：把本仓 `anban` 二进制部署到服务器。

它不负责：

- xiaozhi 上游服务源码。
- ESP32 固件。
- 云端 ASR/LLM/TTS 实现。
- 原版小智语音闭环本身。

推荐本地仓库并排放：

```text
D:\Program\Project\
  AnBan-docs-repo\
  AnBan\research\xiaozhi-esp32-server-golang\
  anban-code\
```

如果需要拉完整文档或 GitHub 内容，代理只设在当前 PowerShell 会话里，不写进仓库：

```powershell
$env:HTTP_PROXY="http://127.0.0.1:7890"
$env:HTTPS_PROXY="http://127.0.0.1:7890"
git -C D:\Program\Project\AnBan-docs-repo pull --ff-only
```

## 2. 方案 C 拓扑

固定是两个进程、一个可插拔增强层：

```text
设备 <-> xiaozhi-esp32-server-golang <-> 云端 ASR/LLM/TTS

子女端 Web -> anban -> xiaozhi manager OpenAPI -> 设备
```

边界：

- xiaozhi 是冻结上游，负责设备连接、原版语音、打断、ASR/LLM/TTS 和 manager。
- anban 是本仓服务，负责子女端、留言、问候、提醒、画像、状态和视觉触发。
- anban 只能通过 `server/internal/xiaozhiclient` 调 xiaozhi manager OpenAPI。
- 数据方向是 `anban -> xiaozhi` 命令或轮询，不是 xiaozhi 主动推给 anban。

## 3. Gate A：只跑 xiaozhi

先去 `xiaozhi-esp32-server-golang` 仓库，按上游文档启动 xiaozhi。此时不要启动 `anban`。

必须记录：

```text
xiaozhi manager 地址：
xiaozhi WS/OTA 地址：
设备 ID：
设备是否在线：
原版对话是否通过：
打断或连续对话是否通过：
```

通过标准：

- 设备能连上 xiaozhi。
- 设备能完成一次“唤醒 -> 说话 -> 听到回复”。
- 原版打断或连续对话符合上游预期。
- 这一步完全不依赖 `anban-code`。

Gate A 不过，就停在硬件、固件、网络、云 API、xiaozhi 配置上排查，不继续堆安伴功能。

## 4. Gate B：manager token 和预检

在 xiaozhi manager 里签发 OpenAPI token。anban 最少需要：

```text
ANBAN_MANAGER_BASE_URL=http://localhost:8080
ANBAN_MANAGER_API_TOKEN=<manager 签发的 token>
ANBAN_ACCESS_CODE=demo
```

回到本仓：

```powershell
Copy-Item .env.example .env
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"
$env:ANBAN_MANAGER_BASE_URL="http://localhost:8080"
$env:ANBAN_MANAGER_API_TOKEN="<manager 签发的 token>"
$env:ANBAN_ACCESS_CODE="demo"
go run ./cmd/anban-preflight -device-id <xiaozhi设备ID>
go run ./cmd/anban-preflight -device-id <xiaozhi设备ID> --xiaozhi-gate-passed
```

第一条会提醒 Gate A 必须人工先过；第二条表示你已经确认纯 xiaozhi 链路通过。

通过标准：

- manager URL 可达。
- token 被接受。
- 指定设备 ID 能在 manager 侧查到，状态符合预期。

如果 `auth.enable=false` 导致设备没有自动进 manager 设备表，要先在 manager 侧登记设备；否则 OpenAPI 注入、状态、画像等能力都会找不到设备。

## 5. Gate C：启动 anban 和子女端

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

```text
后端地址：http://localhost:8090
访问码：ANBAN_ACCESS_CODE 的值
设备 ID：Gate A 记录的 xiaozhi 设备 ID
```

联调顺序固定：

1. 状态页能显示在线、离线或最近互动。
2. 子女端发留言，设备能播报。
3. 子女端触发问候，设备能开口。
4. 创建短时间提醒，到点能播报。
5. 画像能保存，并同步到 xiaozhi role prompt。
6. 视觉最后接，必要时降级，不阻塞前面基础链路。

## 6. Gate D：验证可插拔

停止 `anban`，保留 xiaozhi。

必须看到：

- 子女端增强能力不可用，这是正常的。
- 设备原版小智对话仍然可用。
- 重启 `anban` 后，子女端增强能力恢复。

如果停掉 `anban` 后设备也不能对话，说明方案 C 边界被破坏，要先修架构边界，不继续写新功能。

## 7. 当前服务器口径

截至 2026-06-14 真机联调，当前线上事实：

```text
服务器：101.34.214.149
xiaozhi manager：http://101.34.214.149:8080
xiaozhi WS/OTA：101.34.214.149:8989
anban：http://101.34.214.149:8090
子女端 Web：http://101.34.214.149:8091
已验证设备 ID：9c:13:9e:8b:af:28
AI 链路：豆包 ASR + 豆包 LLM + 豆包 TTS
```

本仓线上部署方式是交叉编译二进制、上传、重启：

```bash
bash deploy.sh
```

`deploy.sh` 只部署本仓的 `anban`。xiaozhi 仍按上游仓库自己的方式部署和运维。改完本仓代码后，必须重新部署 anban，线上才会生效。

## 8. 今天不要做

- 不把 xiaozhi 源码拉进本仓。
- 不改 xiaozhi 上游核心。
- 不绕过 `server/internal/xiaozhiclient` 直接调用 xiaozhi。
- 不在 anban 里修设备 WS 保活；这是设备或固件侧问题。
- 不把留言重新放回主动语音配额；留言是子女点对点必达。
- 不为了视觉能力牺牲留言、问候、提醒、状态这些基础演示链路。
- 不把 token、API key、访问码写进 Git。

## 9. 现场记录模板

```text
日期：
地点：
设备型号：
固件版本：
xiaozhi 版本/提交：
anban 分支/提交：

Gate A 纯 xiaozhi：
- manager 地址：
- WS/OTA 地址：
- 设备 ID：
- 原版对话：
- 打断/连续对话：
- 结论：

Gate B manager 接入：
- preflight 命令：
- 结果：
- 结论：

Gate C 子女端闭环：
- 状态：
- 留言：
- 问候：
- 提醒：
- 画像：
- 结论：

Gate D 可插拔：
- 停止 anban 后原版对话：
- 重启 anban 后增强能力：
- 结论：

遗留问题：
下一步：
```
