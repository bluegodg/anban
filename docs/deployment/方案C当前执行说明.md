# 方案 C 当前执行说明

> 日期：2026-06-15
> 用途：回答“现在是什么阶段、设备到了怎么按方案 C 部署、`anban-code` 这个仓库到底是什么”。
> 权威背景：完整设计以 `AnBan-docs-repo` 为准；本文件是本代码仓里的当前执行稿。

## 0. 先定阶段

现在不应该继续往“大产品后台”扩。当前阶段是 PRD V0.1 路演 Demo 的基础闭环收口：

1. 原版小智语音链路要独立稳定。
2. 安伴作为可插拔增强服务接入。
3. 子女端完成状态、留言、问候、提醒、画像这些基本演示链路。
4. 视觉可以最后接，也可以降级，不阻塞基础链路。

准确说法是：软件侧基础能力已经搭起来，并且真机已经验证了“老人语音对话”和“子女网页留言 -> 设备播报”；但这个阶段是否真正完成，要继续以 Gate A/B/C/D 的真机验证为准。

## 1. 方案 C 是什么

方案 C 不是把 xiaozhi 改成本仓的一部分，而是两个后端进程：

```text
设备 <-> xiaozhi-esp32-server-golang <-> 云端 ASR/LLM/TTS

子女端 Web -> anban -> xiaozhi manager OpenAPI -> 设备
```

边界是：

- `xiaozhi-esp32-server-golang`：冻结上游，负责设备连接、原版语音对话、打断、ASR/LLM/TTS 和 manager 控制面。
- `anban`：本仓编译出的安伴服务，负责子女端 API、留言、问候、提醒、画像、状态和视觉触发。
- 只部署 xiaozhi 时，设备也必须能正常对话。
- 再部署 anban 后，才获得安伴产品能力。
- 停掉 anban 不应该破坏原版小智对话。

这就是“可插拔”：xiaozhi 是底座，anban 是增强层。

## 2. 这个仓库是什么

`anban-code` 是安伴增强服务的代码仓，不是 xiaozhi fork。

它包含：

- `server/`：Go 后端，入口是 `server/cmd/anban`。
- `server/cmd/anban-preflight`：联调前预检 manager、token、设备 ID 和 Gate A 人工确认。
- `web/`：子女端静态网页。
- `docs/`：编码和部署常用文档工作副本。
- `deploy.sh`：把 anban 交叉编译、上传到服务器并重启的脚本。

它不包含：

- xiaozhi 服务端源码。
- ESP32 设备固件。
- 云端 ASR/LLM/TTS 的实现。
- 完整文档仓。
- 原版小智对话能力的必需组件。

建议本地仓库并排放：

```text
D:\Program\Project\
  AnBan-docs-repo\
  AnBan\research\xiaozhi-esp32-server-golang\
  anban-code\
```

## 3. 部署顺序

### Gate A：只跑 xiaozhi

先部署并启动 `xiaozhi-esp32-server-golang`。此时不要启动 anban。

必须确认：

- 设备能连上 xiaozhi。
- 设备能完成一次“唤醒 -> 说话 -> 听到回复”。
- 打断、连续对话符合上游预期。
- 这一步完全不依赖 `anban-code`。

如果 Gate A 不过，先查硬件、固件、网络、云 API 和 xiaozhi 配置，不继续写安伴功能。

### Gate B：签 manager token，跑 anban 预检

在 xiaozhi manager 里签发 OpenAPI token。anban 最少需要：

```text
ANBAN_MANAGER_BASE_URL=http://localhost:8080
ANBAN_MANAGER_API_TOKEN=<manager 签发的 token>
ANBAN_ACCESS_CODE=demo
```

回到本仓：

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

Gate B 必须看到 manager 可达、token 有效、设备 ID 能被 manager 识别。

### Gate C：启动 anban 和子女端

启动 anban：

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

Gate C 联调顺序固定为：

1. 状态页能显示在线、离线或最近互动。
2. 子女端发留言，设备能播报。
3. 子女端触发问候，设备能开口。
4. 创建一条短时间提醒，到点能播报。
5. 画像能保存，并同步到 xiaozhi role prompt。
6. 视觉最后接，必要时降级。

### Gate D：验证可插拔

停止 anban，保留 xiaozhi。

必须看到：

- 子女端增强能力不可用，这是正常的。
- 设备原版小智对话仍然可用。
- 重启 anban 后，子女端增强能力恢复。

如果停掉 anban 后设备也不能对话，说明方案 C 边界被破坏，要先修架构边界。

## 4. 当前服务器口径

截至 2026-06-14 真机联调，当前线上事实是：

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

它只部署本仓的 `anban`。xiaozhi 仍按上游仓库自己的方式部署和运维。改完本仓代码后，必须重新部署 anban，线上才会生效。

## 5. 当前不要做什么

- 不把 xiaozhi 源码拉进本仓。
- 不在 anban 里修设备 WS 保活；这是设备或固件侧问题。
- 不把留言重新放回主动语音配额；留言是子女点对点必达。
- 不绕过 `server/internal/xiaozhiclient` 直接调用 xiaozhi。
- 不先做多租户、运营后台、计费、长期健康趋势。
- 不为了视觉能力牺牲留言、问候、提醒、状态这些基础演示链路。
- 不把 token、API key、访问码或代理配置写进 Git。

如果拉 GitHub 或依赖需要代理，只在当前 PowerShell 会话设置：

```powershell
$env:HTTP_PROXY="http://127.0.0.1:7890"
$env:HTTPS_PROXY="http://127.0.0.1:7890"
```

## 6. 下一步怎么做

现在继续按 PRD 基础链路收口，而不是扩成完整产品：

1. 守住 #1 原版语音对话：由 xiaozhi 负责，anban 不抢语音主链路。
2. 守住 #3 子女端留言：这是三元关系成立的核心，已真机验证，不能倒退。
3. 补强 #4 状态/完整对话记录：子女端要能看到在线、最近互动和开发期对话历史。
4. 收稳 #2 问候和 #6 提醒：播报短、温柔、可确认、状态可见。
5. 收稳 #5 画像：先保存并同步到 xiaozhi prompt，限制模型不要擅自改设备设置。
6. #7 视觉最后接：默认走 `self.camera.take_photo`，不阻塞主线。

只有 Gate A/B/C/D 通过，且 `go build ./...`、`go vet ./...`、`go test ./...`、`npm test --prefix web` 通过，才能说“基础框架和基本功能目标已经实现”。

## 7. 继续阅读

按这个顺序：

1. `docs/现状与交接-2026-06-14.md`
2. `AGENTS.md`
3. `docs/安伴V0.1产品文档PRD.md`
4. `docs/deployment/README.md`
5. `docs/deployment/方案C部署与联调指南.md`
6. `docs/deployment/设备到手方案C首日执行单.md`
7. `AnBan-docs-repo/docs/decisions/2026-05-29-server-architecture.md`
