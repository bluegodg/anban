# 设备到手：方案 C 首日执行单

> 日期：2026-06-12
> 适用场景：设备已经到手，需要把“先跑原版小智，再接安伴增强”的方案 C 跑成一个可演示闭环。
> 本文件是现场执行单。完整背景见 `AnBan-docs-repo`；本仓长版说明见 `docs/deployment/方案C部署与联调指南.md`。

## 0. 今天的目标

今天不是继续扩成“大产品”。今天只证明四件事：

1. 不启动安伴，设备也能通过 `xiaozhi-esp32-server-golang` 完成原版小智对话。
2. 安伴能用 manager OpenAPI token 找到这台真实设备。
3. 子女端能完成状态、留言、问候、提醒这几条最小链路。
4. 停掉安伴后，原版小智仍然能正常对话。

这四件事没过之前，不能说“基础框架和基本功能目标已经实现”。只能说软件侧准备好了，正在做真设备验证。

## 1. 先认清三个仓库

建议并排放：

```text
D:\Program\Project\
  AnBan-docs-repo\                 # 权威文档仓，只放规划/PRD/架构/计划
  xiaozhi-esp32-server-golang\     # 冻结上游，负责设备原版语音闭环
  anban-code\                      # 本仓，安伴后端 + 子女端 Web
```

本仓 `anban-code` 是可选增强服务，不是 xiaozhi fork，也不是设备固件仓。

固定拓扑：

```text
设备 <-> xiaozhi-esp32-server-golang <-> 云 ASR/LLM/TTS

子女端 Web -> anban -> xiaozhi manager OpenAPI -> xiaozhi core -> 设备
```

只部署 `xiaozhi-esp32-server-golang` 时，设备必须能正常说话。再部署 `anban`，才增加留言、问候、提醒、画像、状态等安伴能力。

## 2. 首日顺序

### Step 1：只跑 xiaozhi

在 `xiaozhi-esp32-server-golang` 仓库按上游说明部署。此时不要启动 `anban`。

必须记录：

```text
xiaozhi manager 地址：
设备 ID：
设备是否在线：
原版对话是否通过：
打断是否通过：
```

Gate A 判据：

- 设备能连上 xiaozhi。
- 设备能完成一次“唤醒 -> 说话 -> 听到回复”。
- 原版打断或连续对话行为符合上游预期。
- 这一步完全不依赖 `anban-code`。

Gate A 不过，停在 xiaozhi/硬件/网络/云 API 排查，不进入安伴联调。

### Step 2：签 manager token

在 xiaozhi manager 里签发 OpenAPI token。安伴后端只需要这两个值：

```text
ANBAN_MANAGER_BASE_URL=http://localhost:8080
ANBAN_MANAGER_API_TOKEN=<manager 签发的 token>
```

如果 xiaozhi 跑在服务器或 Docker 网络里，`ANBAN_MANAGER_BASE_URL` 要换成安伴进程实际能访问到的地址。

不要把 token 写进 README、脚本、提交记录或截图。

### Step 3：跑安伴预检

回到 `anban-code`：

```powershell
Copy-Item .env.example .env
```

填 `.env`：

```text
ANBAN_MANAGER_BASE_URL=http://localhost:8080
ANBAN_MANAGER_API_TOKEN=<manager 签发的 token>
ANBAN_ACCESS_CODE=demo
ANBAN_DB_DSN=anban.db
ANBAN_LISTEN_ADDR=:8090
ANBAN_ALLOWED_ORIGINS=http://127.0.0.1:5173,http://localhost:5173
```

跑预检：

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"
$env:ANBAN_MANAGER_BASE_URL="http://localhost:8080"
$env:ANBAN_MANAGER_API_TOKEN="<manager 签发的 token>"
$env:ANBAN_ACCESS_CODE="demo"

go run ./cmd/anban-preflight -device-id <xiaozhi设备ID>
```

预检会先提醒 Gate A 是人工项。确认纯 xiaozhi 已通过后，再跑：

```powershell
go run ./cmd/anban-preflight -device-id <xiaozhi设备ID> --xiaozhi-gate-passed
```

Gate B 判据：

- manager URL 可达。
- token 被接受。
- 指定设备 ID 在 manager 侧可查，且状态符合预期。

如果只是临时排查 manager URL/token，没有设备 ID，可以显式用：

```powershell
go run ./cmd/anban-preflight --xiaozhi-gate-passed --allow-missing-device-id
```

这只代表 manager-only 检查通过，不代表真实设备接入通过。

### Step 4：启动安伴后端

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"
$env:ANBAN_MANAGER_BASE_URL="http://localhost:8080"
$env:ANBAN_MANAGER_API_TOKEN="<manager 签发的 token>"
$env:ANBAN_ACCESS_CODE="demo"

go run ./cmd/anban
```

另开 PowerShell 做健康检查：

```powershell
Invoke-RestMethod http://localhost:8090/health
```

预期：

```json
{"status":"ok"}
```

### Step 5：启动子女端 Web

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

Gate C 联调顺序：

1. 状态页能显示在线/离线/最近活跃。
2. 子女端发留言，设备能播报。
3. 子女端触发问候，设备能开口。
4. 创建一条短时间提醒，到点能播报。
5. 画像能保存并写入角色 prompt。
6. 视觉最后看条件接，不阻塞前面基础链路。

## 3. 最后必须做可插拔验证

停止 `anban`，保留 xiaozhi。

Gate D 判据：

- 子女端增强能力不可用是正常的。
- 设备原版小智对话仍然可用。
- 再启动 `anban` 后，子女端增强能力恢复。

如果停掉 `anban` 后设备也不能对话，说明方案 C 边界被破坏，要先查架构边界，不继续写新功能。

## 4. 今天不要做

- 不改 xiaozhi 上游代码。
- 不把 xiaozhi 源码拉进本仓。
- 不绕过 `server/internal/xiaozhiclient` 直接调 xiaozhi。
- 不在 Gate A 未通过前继续堆视觉、运营后台、多账号、复杂权限。
- 不把访问码、manager token、云 API key 写进 Git。

## 5. 首日记录模板

联调结束后，把实际结果补到这里或新开一份联调记录：

```text
日期：
设备型号：
固件版本：
xiaozhi 仓库/镜像版本：
anban 分支/提交：

Gate A 纯 xiaozhi：
- manager 地址：
- 设备 ID：
- 设备在线：
- 原版对话：
- 打断：
- 结论：

Gate B manager 接入：
- preflight 命令：
- 结果：
- 结论：

Gate C 子女端最小闭环：
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
