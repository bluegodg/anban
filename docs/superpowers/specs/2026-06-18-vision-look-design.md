# 安伴“看一眼”原图与视觉观察设计

日期：2026-06-18  
状态：已确认  
范围：AnBan 后端、子女端、远端部署配置；不修改 xiaozhi 服务端源码与设备固件源码

## 1. 目标

子女端用户点击“看一眼”后，安伴调用设备已有的 `self.camera.take_photo` MCP 工具完成一次真实拍照，并在子女端同时展示：

- 设备拍摄的原始图片；
- 图片拍摄时间；
- AI 对画面的观察摘要；
- 是否看到老人；
- 需要关注的事项；
- 明确的进行中、部分成功、失败和过期状态。

管理员与家庭成员均可使用“看一眼”，但只能读取当前账号已绑定设备的拍摄结果。

## 2. 已验证的现状与根因

现有仓库已经包含“看一眼”按钮、`vision` 域、`CallDeviceMCPTool` 和 `self.camera.take_photo` 工具名，但当前链路仅在 FakeClient 测试中成立，真实 manager 协议存在两处错配：

1. `/api/open/v1/devices/:id/mcp-call` 的 `:id` 是 manager 数据库中的数字设备 ID，不是设备 MAC 或 `device_name`。
2. 请求体字段应为 `tool_name` 和 `arguments`，现有实现错误使用 `tool` 和 `args`。

线上 manager 已验证：

- MAC `9c:13:9e:8b:af:28` 对应 manager 数字设备 ID `1`；
- `{"tool":"...","args":{}}` 返回参数校验错误；
- `{"tool_name":"...","arguments":{}}` 能通过请求校验；
- 设备离线时工具列表为空，无法完成真机拍照。

固件 `Esp32Camera::Explain` 的真实数据流是：

1. 设备采集并编码 JPEG；
2. 向下发的 `vision_url` 发送 multipart POST；
3. 请求携带 `Device-Id`、`Client-Id`、`Authorization`；
4. multipart 字段为 `question` 与 `file`；
5. xiaozhi 视觉接口读取完整图片并调用 VLM；
6. 接口只把 AI 文本返回设备，当前不保存原图。

因此原图已经离开设备，缺失的是可持久化、可关联、可授权读取的媒体入口。

## 3. 方案比较与决策

### 3.1 仅展示 AI 文字

直接修复 manager MCP 协议，调用内置相机工具并展示其文本返回。

优点：改动最小。  
缺点：不满足子女端查看原图的产品目标。

### 3.2 修改固件或 xiaozhi 返回原图

扩展设备 MCP 返回值，或修改 xiaozhi 视觉接口持久化图片。

优点：图片链路由上游直接提供。  
缺点：违反冻结上游和不改固件的架构约束，升级与维护成本高。

### 3.3 安伴兼容视觉代理（采用）

将设备的 `vision_url` 配置为安伴提供的兼容入口。安伴接收原图、保存手动“看一眼”图片，再把同一 multipart 请求透明转发到原 xiaozhi 视觉接口。

优点：

- 原图和 AI 观察均可获得；
- 不修改设备固件和 xiaozhi 源码；
- 普通语音看图仍使用原 xiaozhi VLM；
- 切换和回滚只涉及配置与 AnBan 二进制。

## 4. 架构与组件边界

### 4.1 `internal/xiaozhiclient`

继续作为唯一了解 xiaozhi 协议的包，负责：

- 从 manager 设备列表把外部设备标识解析为数字设备 ID；
- 使用 `tool_name`、`arguments` 调用设备 MCP；
- 将设备离线、工具不存在、超时和一般上游错误转换为可判定错误；
- 将设备上传的视觉 multipart 转发到配置的内部 xiaozhi 视觉地址。

其他包不直接请求 xiaozhi manager 或 core。

### 4.2 `internal/domains/vision`

扩展为完整视觉业务域：

- 创建和维护拍摄任务；
- 生成唯一 `captureId`；
- 构造带关联标记的 `question`；
- 协调 MCP 调用与设备图片上传；
- 保存图片和分析结果；
- 管理状态、超时、重试、过期与清理；
- 校验拍摄记录是否属于当前绑定设备。

域内新增标准骨架文件：

- `store.go`：拍摄元数据和文件索引；
- `types.go`：任务、分析、DTO 和错误类型；
- `service.go`：拍摄编排与状态机；
- `handler.go`：子女端接口与设备上传入口。

### 4.3 设备视觉入口

同一 AnBan HTTP 服务新增：

```text
POST /api/device/vision
```

该入口兼容固件当前的 multipart 协议。它不是子女端接口，不使用家庭账号 Bearer token；使用部署配置中的设备视觉入口令牌，并校验 `Device-Id`。

固件的 `Authorization` 值由当前 xiaozhi core 固定下发，不能作为安伴独立密钥。部署时将随机入口令牌放入 `vision_url` 查询参数：

```text
http://101.34.214.149:8090/api/device/vision?ingress_token=<random-secret>
```

设备会原样使用该 URL。安伴使用常量时间比较校验 `ingress_token`，不把令牌写入日志或响应。

### 4.4 子女端

子女端调用组合型 `look` 接口，不再把底层 `check-presence` 当作手动“看一眼”的用户接口。它负责：

- 稳定的拍摄加载状态；
- 原图 Blob 的鉴权读取与释放；
- AI 观察、presence、关注事项和时间展示；
- 最近拍摄记录；
- 部分成功、失败和重试状态。

## 5. 一次“看一眼”的完整时序

1. 子女端向 `POST /api/vision/look` 发起请求。
2. Bearer 中间件解析账号和当前绑定设备。
3. `vision.Service` 检查设备是否已有进行中的拍摄；若有则返回冲突。
4. 服务创建 `pending` 拍摄记录和随机 `captureId`。
5. 服务构造包含 `[[ANBAN_CAPTURE:<captureId>]]` 标记和结构化观察要求的 `question`。
6. `xiaozhiclient` 查询 manager 设备列表，解析数字设备 ID。
7. `xiaozhiclient` 以 `tool_name=self.camera.take_photo` 和 `arguments.question` 调用 manager MCP。
8. 设备拍照并向 `/api/device/vision` 上传 `question` 与 JPEG。
9. 视觉入口验证 URL 入口令牌、设备和文件限制，从 `question` 提取 `captureId`。
10. 对安伴拍摄请求，先原子保存原图，再转发同一视觉请求。
11. xiaozhi 调用现有 VLM 并返回观察文本。
12. 安伴保存 AI 结果，拍摄状态变为 `succeeded`；分析失败但图片存在时变为 `partial`。
13. 原响应以兼容格式返回设备，MCP 调用完成。
14. `POST /api/vision/look` 返回拍摄 DTO，子女端鉴权读取图片并展示。

关联标记只用于安伴内部关联。转发给 VLM 前必须移除标记，避免污染视觉问题和模型回答。

观察问题要求 VLM 尽量返回 `summary`、`presence`、`concerns` 三个字段的 JSON。解析器需要递归处理 manager/MCP 包装和 JSON 字符串；若模型返回普通文本，则将完整文本保存为 `summary`，`presence` 记为 `unknown`，而不是把有效观察误判为失败。

## 6. 普通视觉请求兼容

设备通过语音主动使用相机时，同样会向安伴视觉入口上传图片。此类请求没有有效的安伴 `captureId`：

- 不保存图片；
- 不创建拍摄记录；
- 原始 headers、图片与问题透明转发；
- xiaozhi 状态码、Content-Type 和响应正文原样返回设备。

因此切换 `vision_url` 不改变设备原有语音看图行为。

## 7. 数据模型与状态机

拍摄记录建议字段：

```text
ID                 uint
CaptureID          string (unique)
DeviceID           string (index)
Status             pending|succeeded|partial|failed|expired
ImageRelativePath  string
ImageContentType   string
ImageSize          int64
ImageSHA256        string
AnalysisSummary    string
AnalysisRaw        string
Presence           unknown|someone|no_one
ConcernsJSON       string
FailureCode        string
FailureMessage     string
CapturedAt         *time.Time
ExpiresAt          time.Time
CreatedAt          time.Time
UpdatedAt          time.Time
```

允许的状态流转：

```text
pending -> succeeded
pending -> partial
pending -> failed
succeeded|partial -> expired
```

同一设备最多存在一个 `pending` 任务。并发请求返回 `409 capture_in_progress`，不启动第二次相机调用。

## 8. 文件存储与保留

图片存储在配置目录下：

```text
<media-root>/vision/<device-hash>/<yyyy>/<mm>/<captureId>.jpg
```

要求：

- 使用临时文件写入并原子重命名；
- 文件名不使用用户输入或原始设备 ID；
- 接受 JPEG，兼容 PNG；
- 单图上限 10 MB；
- 记录 SHA-256，测试保存前后字节一致；
- 默认保留 30 天；
- 每台设备最多保留最近 100 次；
- scheduler 定期删除过期文件和更新元数据；
- 删除失败可重试，不能让元数据指向另一设备的文件。

远端默认目录：

```text
/home/ubuntu/anban/media/vision
```

## 9. HTTP 接口

### 9.1 子女端接口

```text
POST /api/vision/look
GET  /api/vision/captures
GET  /api/vision/captures/:captureId
GET  /api/vision/captures/:captureId/image
POST /api/vision/captures/:captureId/reanalyze
```

所有接口沿用现有账号/绑定中间件。家庭管理员和家庭成员均可访问，但请求中的设备身份始终以后端绑定上下文为准，不信任前端传入的 `deviceId`。

`POST /api/vision/look` 成功响应：

```json
{
  "captureId": "cap_xxx",
  "status": "succeeded",
  "capturedAt": "2026-06-18T15:30:00+08:00",
  "imageUrl": "/api/vision/captures/cap_xxx/image",
  "analysis": {
    "summary": "老人正在沙发上休息，神态平静。",
    "presence": "someone",
    "concerns": []
  }
}
```

图片接口返回原始图片字节，并设置正确的 `Content-Type`、`Content-Length`、私有缓存策略和禁止内容嗅探响应头。

### 9.2 底层兼容接口

现有接口继续保留：

```text
POST /api/vision/capture
POST /api/vision/check-presence
POST /api/vision/presence
```

它们继续服务开发、自动 presence 和兼容调用；子女端手动“看一眼”改用 `/api/vision/look`。

### 9.3 设备上传接口

```text
POST /api/device/vision
```

输入：

- Header `Device-Id`；
- Header `Client-Id`；
- 部署配置中的入口令牌；
- multipart `question`；
- multipart `file`。

## 10. 子女端交互

- 点击后按钮进入禁用与加载状态，布局尺寸不变化。
- 过程文案区分“正在连接设备”“设备正在拍摄”“正在分析画面”。
- 成功后打开结果层，优先展示完整、清晰、无模糊遮罩的原图。
- 图片下方显示拍摄时间、观察摘要、presence 和关注事项。
- `partial` 仍显示原图，并提供“重新分析”。
- `failed` 显示具体可行动原因与“重试”。
- 最近记录允许刷新后恢复，不依赖内存状态。
- Blob URL 在替换图片、关闭结果层或页面卸载时释放。
- 手机与桌面视口不得发生图片、文字、按钮重叠。

## 11. 错误处理

错误代码至少包括：

```text
device_not_bound
device_offline
camera_tool_unavailable
capture_in_progress
capture_timeout
image_upload_invalid
image_too_large
vision_analysis_failed
capture_not_found
capture_expired
```

行为约束：

- 设备离线或无相机工具时不创建永久 pending；
- 图片已保存但分析失败时返回 `partial`；
- 超时任务最终必须转为 `failed`；
- handler 不向前端泄露 manager token、内部 URL、文件绝对路径或上游响应体；
- 服务日志包含 `captureId`、设备哈希、阶段和错误类别，不记录图片内容或认证令牌。

## 12. 配置

新增配置项：

```text
ANBAN_VISION_MEDIA_ROOT
ANBAN_DEVICE_VISION_TOKEN
ANBAN_XIAOZHI_VISION_URL
ANBAN_VISION_CAPTURE_TIMEOUT
ANBAN_VISION_RETENTION_DAYS
ANBAN_VISION_MAX_CAPTURES_PER_DEVICE
```

默认值：

```text
ANBAN_VISION_CAPTURE_TIMEOUT=30s
ANBAN_VISION_RETENTION_DAYS=30
ANBAN_VISION_MAX_CAPTURES_PER_DEVICE=100
```

30 秒是整次手动拍摄、上传和 VLM 分析的硬上限。底层自动 presence 轮询可继续使用更短的独立时间预算，不能反向缩短手动“看一眼”的完整链路。

其中 `ANBAN_XIAOZHI_VISION_URL` 指向服务器内部的 xiaozhi core 视觉接口，例如：

```text
http://127.0.0.1:8989/xiaozhi/api/vision
```

设备下发的 `vision_url` 切换为公网可访问的安伴设备视觉入口。入口令牌不得写入仓库。

## 13. 自动化测试

### 13.1 `xiaozhiclient`

- MAC/设备名解析为数字 manager ID；
- `tool_name`、`arguments` 请求契约；
- manager `data` 解包；
- 离线、工具不存在、超时错误分类；
- multipart 转发保持 headers、问题、图片和响应。

### 13.2 `vision` 域

- 状态机全部合法路径；
- `captureId` 与上传关联；
- 原图 SHA-256 不变；
- 图片成功、分析失败时为 `partial`；
- 并发冲突；
- 超时终结；
- 过期与每设备数量清理；
- 非标记请求不保存；
- 跨设备读取拒绝。

### 13.3 HTTP 与前端

- Bearer 账号和绑定设备授权；
- 图片响应字节与 headers；
- 加载、成功、partial、failed、expired、重试；
- 已保存原图的重新分析；
- 最近记录与页面刷新恢复；
- Blob URL 释放；
- 桌面和手机浏览器截图与控制台检查。

## 14. 部署、回滚与真机验收

部署前备份：

- 远端 `anban` 二进制；
- SQLite 数据库；
- 当前 xiaozhi `vision_url`；
- 已有媒体目录（如存在）。

部署顺序：

1. 部署新版 AnBan，但不立即切换设备 `vision_url`。
2. 使用模拟固件 multipart 请求验证设备视觉入口和透明转发。
3. 验证子女端接口授权、图片保存和过期清理。
4. 将 xiaozhi 下发的 `vision_url` 切换到安伴入口。
5. 让设备重新建立 MCP 会话并确认工具列表包含 `self.camera.take_photo`。
6. 执行真机“看一眼”和普通语音看图验收。

回滚：

1. 恢复原 xiaozhi `vision_url`；
2. 恢复旧 AnBan 二进制；
3. 数据库迁移保持向后兼容，不删除新表；
4. 媒体文件保留，待确认后清理。

真机完成标准：

- manager 工具列表包含 `self.camera.take_photo`；
- 子女端点击后设备实际拍照；
- 子女端显示与本次拍摄一致的原图；
- AI 观察文本正常；
- 刷新后仍可读取记录与图片；
- 未绑定或绑定其他设备的账号无法读取；
- 原有语音看图仍可使用；
- 离线、无工具和分析失败不会卡住按钮或遗留永久 pending。

## 15. 明确不做

- 不修改 xiaozhi 服务端源码；
- 不修改设备固件源码；
- 不实现持续视频流或视频通话；
- 不保存普通语音看图产生的图片；
- 不把媒体文件直接暴露为无鉴权静态 URL；
- 不用 FakeClient 结果代替真机验收。
