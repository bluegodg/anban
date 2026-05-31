# AGENTS.md — anban 后端编码指南（给 Codex / 任何 coding agent）

> 你在 `anban` 仓库里写代码。**先读这份，再动手。**
> 完整设计文档以 **AnBan-docs-repo**（https://github.com/bluegodg/AnBan-docs-repo）为权威；本仓库 `docs/` 是编码用的工作副本（仅含编码最常用的几份）。

---

## 0. 这仓库是什么

安伴（AnBan）= 主动陪伴居家老人的 AI 陪伴设备（3 周路演 Demo，目标 2026-06-16）。本仓库是它的**后端 + 子女端前端**代码 monorepo：

- `server/` = Go 后端。**"地基"已搭好**（编译过、单测全绿），你在它上面加业务域。
- `web/` = 子女端前端（尚未创建，待 §6）。

## 1. 三条架构铁律（绝不违反）

1. **xiaozhi 是冻结的上游，绝不改它、绝不把它的代码拉进本仓库。** 安伴是独立的**第三个服务**，通过 xiaozhi 的 **manager OpenAPI**（`/api/open/v1/*`，鉴权头 `X-API-Token`）去驱动它。依据：`docs/specs/2026-05-28-server-architecture-design.md`（方案 C）。
2. 所有对 xiaozhi 的调用**只经 `internal/xiaozhiclient` 这一个包**。别的包不直接碰 xiaozhi / 不直接发 HTTP 给它。
3. 数据方向永远是 **anban → xiaozhi**（命令 or 轮询），**不是** xiaozhi 推给 anban。

## 2. 仓库结构 + 依赖纪律

```
server/
├── cmd/anban/main.go        # 装配启动
├── internal/
│   ├── childapi/            # 北向边界：子女端 HTTP 接口 + 访问码（只调 domains）
│   ├── domains/             # ★业务域（你主要在这里加东西）
│   │   ├── message/  reminder/  greeting/
│   │   ├── profile/  status/    vision/
│   ├── xiaozhiclient/       # 南向边界：唯一懂 xiaozhi 的适配器（已有接口+InjectSpeak+FakeClient）
│   ├── scheduler/           # 基建：cron + 一次性定时
│   └── store/               # 数据层：sqlite（gorm）
└── pkg/types/               # 跨模块共享类型
```

**依赖规则（写代码不许越界，违反就是架构 bug）：**
- `childapi` → 调 `domains`；不直接碰 `xiaozhiclient` / `store`
- `domains` → 只能 import `xiaozhiclient` / `store` / `scheduler` / `pkg/types`
- **`domains` 之间不互相 import**（要协作走 `pkg/types` 接口或经 childapi 编排）
- 只有 `xiaozhiclient` 懂 xiaozhi；只有 `store` 懂数据库

## 3. 每个域的标准骨架（照这个填，别纠结东西放哪）

```
internal/domains/<域>/
├── handler.go   # 对外入口（被 childapi 或 scheduler 调）
├── service.go   # 业务逻辑（核心）
├── store.go     # 本域数据存取（用共享 store 的 DB 句柄，只管自己的表）
└── types.go     # 本域数据结构
```

## 4. xiaozhiclient 的 5 个方法（已定义在 `internal/xiaozhiclient/client.go`）

| 方法 | 服务于 | 状态 |
|---|---|---|
| `InjectSpeak(deviceID, text, opts{SkipLLM, AutoListen})` | message/reminder/greeting | ✅ **已实现**（对 manager `POST /api/open/v1/devices/inject-message`）|
| `GetDeviceStatus(deviceID)` | status | ⬜ 待实现（对 manager 设备 API 的 `last_active_at`）|
| `GetHistory(deviceID, limit)` | status/深度 | ⬜ 待实现 |
| `SetRolePrompt(deviceID, prompt)` | profile | ⬜ 待实现（写 manager role/agent API）|
| `CallDeviceMCPTool(deviceID, tool, args)` | vision | ⬜ 待实现（对 manager `/devices/:id/mcp-call`）|

> 待实现的 4 个目前返回 `types.ErrNotImplemented`。实现时**参考 `docs/specs/2026-05-29-xiaozhi-full-architecture-map.md` §9**（每个能力对应的 manager 真实端点）。

## 5. 并行打法：FakeClient 先行

- 各域对着 **`xiaozhiclient.FakeClient`** 写"假数据版"，**不用等真 xiaozhi / 真硬件**。
- 接口已冻结 → 多个 agent / 人可**并行**写不同域，互不阻塞。
- 每个域第一步该干啥 → 见 `docs/plans/2026-05-29-anban-backend-foundation.md` 末尾 **Roadmap 表**。
- 每条域的**验收判据** → 见 `docs/安伴V0.1产品文档PRD.md` §3.1 各功能 ③ 段。

## 6. 子女端前端 `web/`（尚未创建）

- 按页面分：login / status / message / remind / profile + 一个 `api/`（封装对 `childapi` 的调用）。
- 框架未定（Vue / React 皆可，团队拍板后再起）。先对着 `childapi` 的路由形状（`server/internal/childapi/server.go` 里的占位路由）开发。

## 7. 怎么 build / test（⚠️ 国内网络必读）

在 `server/` 目录，**先设环境再跑**（不设会被墙、go 拉不到依赖）：

```powershell
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"
go build ./...
go test ./...
```

- 现有测试**必须保持全绿**。新功能**先写测试（TDD）**再实现。
- 依赖纯 Go（gorm + glebarez/sqlite + gin + robfig/cron），无需 cgo。

## 8. Git 纪律

- **不直接改 main**；开 `feat/<域>-描述` 分支，小步提交，开 PR。
- **不提交任何 key/密钥**（一律放 `.env`，已在 `.gitignore`）。
- 提交信息用 conventional 风格（`feat(message): ...`）。

---

## 速记：你要做的就是

在 `internal/domains/<你的域>/` 里，对着 `FakeClient` 把这个域的 handler/service/store/types 写出来 + 配套测试，在 `childapi/server.go` 把占位路由换成真 handler，保持 `go test ./...` 全绿。深的设计细节查 `docs/`。
