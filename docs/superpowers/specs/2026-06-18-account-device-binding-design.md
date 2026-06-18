# 安伴账号与设备绑定设计 — 2026-06-18

## 1. 背景与目标

当前子女端仍以 demo 访问码和显式 `deviceId` 为中心。它能支撑路演链路，但不像真实产品：没有子女账号、没有成员身份、无法表达管理员/成员权限，也无法把留言界面做成真实家庭群聊。

本设计把安伴子女端升级为“账号直接绑定设备”的模型：

- 子女端账号有两类设备角色：`admin`（家庭管理员）与 `member`（家庭成员）。
- 登录后账号可能未绑定设备。未绑定时仍进入主界面，但设备相关功能置灰，并提示“请先绑定安伴设备”。
- 设备通过后端维护的可重置绑定码绑定，不直接暴露 xiaozhi 的真实 `deviceId`。
- 一个账号第一期最多绑定一台设备。
- 一个设备最多一个管理员，可绑定多个成员。
- 只有管理员可编辑老人/家庭画像与管理设备/成员。
- 成员可使用留言、状态、历史、提醒、问候、视觉等照看功能。
- 留言播报使用当前账号昵称，例如“小兰发来留言：...”，不再信任前端伪造 `fromName`。
- 消息页升级为统一群聊 timeline，展示子女留言、老人语音、安伴回复与留言播报状态。

## 2. 已定产品决策

### 2.1 绑定模型

采用“账号直接绑定设备”模型，不引入家庭空间：

- `account -> device_binding -> anban_device`
- 设备角色直接挂在账号与设备绑定关系上。
- 第一阶段不做家庭空间、邀请审批、多设备、细粒度权限。

理由：贴近当前 demo，能较快形成真实账号闭环；消息群聊和权限可以在设备绑定模型里完成，未来需要家庭空间时再迁移。

### 2.2 成员加入

成员输入设备码即可直接绑定，无管理员审批。

管理员后续可以：

- 查看成员列表。
- 移除成员。
- 重置设备绑定码。

### 2.3 登录注册

第一期做真实账号系统，但验证码为开发模式：

- 手机号 + 密码真实注册/登录。
- 验证码登录/注册接口存在。
- 验证码发送先使用开发模式固定码或回显码，不接真实短信供应商。
- 后续可接阿里云/腾讯云短信供应商。

### 2.4 单账号单设备

第一期一个账号最多绑定一台设备。

未绑定账号可绑定设备；已绑定账号再次绑定应失败并提示“当前账号已绑定设备”。

### 2.5 管理员解绑

管理员允许解绑设备；第一期不做管理员转让。

解绑后：

- 管理员绑定关系被移除。
- 设备恢复“无管理员”状态。
- 其他账号可用管理员身份重新绑定该设备。
- 已有成员绑定保留，成员仍可使用成员权限范围内的照看功能。
- 如果管理员希望清空成员，应在解绑前先移除成员。
- 新管理员绑定后可以继续管理现有成员。

### 2.6 权限

管理员：

- 所有设备功能。
- 编辑老人/家庭画像。
- 设备解绑。
- 重置设备码。
- 查看和移除成员。

成员：

- 留言。
- 查看状态和历史。
- 创建/管理提醒。
- 触发问候。
- 使用视觉相关功能。
- 不能编辑老人/家庭画像。
- 不能管理设备、设备码或成员。

未绑定账号：

- 登录。
- 查看/编辑自己的个人资料。
- 绑定设备。
- 退出登录。
- 不能调用设备功能。

### 2.7 旧访问码兼容

短期双轨：

- 新账号体系使用 `Authorization: Bearer <token>`。
- 旧 demo 访问码 `X-Access-Code` 继续兼容现有接口和测试。
- `X-Access-Code` 不代表真实账号，不进入成员列表，不产生账号昵称。
- 设计上最终收敛到 Bearer token。

## 3. 后端架构

### 3.1 新增域

新增三个后端能力边界：

1. `account`
   - 注册、登录、验证码、session/token、个人资料。
   - 管理手机号、密码 hash、昵称、真实姓名、与老人关系、头像/头像色。

2. `devicebinding`
   - 设备绑定码、绑定关系、角色、成员管理。
   - 管理设备码到真实 `deviceId` 的映射。

3. `timeline`
   - 聚合子女留言、老人语音、安伴回复、播报状态。
   - 给前端消息页提供统一群聊时间流。

现有业务域继续保留：

- `message`
- `profile`
- `reminder`
- `status`
- `vision`
- `greeting`

`childapi` 负责认证、解析当前账号/绑定/角色，并在调用业务域前做权限检查。业务域尽量不直接理解账号系统。

### 3.2 依赖边界

遵守现有架构铁律：

- 不改 xiaozhi/固件。
- 只有 `internal/xiaozhiclient` 了解 xiaozhi manager OpenAPI。
- `childapi` 编排账号、绑定、权限与业务域。
- `domains` 不互相 import。
- `mind` 不 import `profile/account/devicebinding`。

推荐依赖方向：

- `childapi -> account/devicebinding/timeline/domains`
- `timeline -> store/xiaozhiclient/message account 相关接口`
- `message/profile/reminder/status/vision/greeting -> 不直接依赖 account`

如果 `timeline` 需要账号昵称，推荐通过本域 store 查询或由 childapi 注入必要接口，避免业务域互相缠绕。

## 4. 数据模型

### 4.1 accounts

字段建议：

- `id`
- `phone`，唯一
- `password_hash`
- `nickname`
- `real_name`
- `relationship_to_elder`
- `avatar_url`，可选
- `avatar_color`
- `status`
- `created_at`
- `updated_at`

个人资料第一期字段：

- 昵称：留言播报和消息来源优先使用。
- 真实姓名：可选，用于管理展示。
- 与老人关系：如女儿、儿子、外孙。
- 头像/头像色：第一期可只做头像色，后续再做上传。

留言显示名规则：

1. 优先 `nickname`。
2. 无昵称时使用脱敏手机号，例如 `138****1234`。

### 4.2 auth_sessions

字段建议：

- `id`
- `account_id`
- `token_hash`
- `expires_at`
- `created_at`
- `revoked_at`

API 返回明文 token，数据库只存 hash。

第一期 Bearer token 可为长期 session token；后续再升级 refresh token。

### 4.3 verification_codes

字段建议：

- `id`
- `phone`
- `code_hash` 或开发模式明文记录
- `purpose`：register/login/reset_password
- `expires_at`
- `consumed_at`
- `created_at`

开发模式：

- 可固定 `123456`。
- 或接口返回 `debugCode`，仅在开发配置开启时返回。

### 4.4 anban_devices

字段建议：

- `id`
- `device_id`：真实 xiaozhi deviceId
- `binding_code`：用户输入的设备码
- `binding_code_version`
- `display_name`
- `elder_display_name`
- `created_at`
- `updated_at`
- `binding_code_reset_at`

设备码规则：

- 后端维护，可重置。
- 用户输入设备码绑定。
- 不把真实 `deviceId` 暴露为绑定码。
- demo 设备可通过配置或 seed 初始化：
  - `deviceId = 9c:13:9e:8b:af:28`
  - `bindingCode = ANBAN-xxxxxx` 或 8 位数字

### 4.5 device_bindings

字段建议：

- `id`
- `account_id`
- `device_id`
- `role`：admin/member
- `bound_at`
- `created_at`
- `updated_at`

约束：

- `(account_id)` 唯一：第一期单账号单设备。
- `(device_id, role=admin)` 唯一：单设备唯一管理员。
- `(account_id, device_id)` 唯一：防重复绑定。
- member 可多个。

## 5. API 契约

### 5.1 Auth

`POST /api/auth/register`

请求：

```json
{
  "phone": "13800000000",
  "password": "plain-password",
  "nickname": "小兰"
}
```

返回：

```json
{
  "token": "...",
  "account": {
    "accountId": 1,
    "phone": "138****0000",
    "nickname": "小兰"
  }
}
```

`POST /api/auth/login`

请求：

```json
{
  "phone": "13800000000",
  "password": "plain-password"
}
```

返回同注册。

`POST /api/auth/verification-code`

请求：

```json
{
  "phone": "13800000000",
  "purpose": "login"
}
```

开发模式返回：

```json
{
  "sent": true,
  "debugCode": "123456"
}
```

生产模式不返回 `debugCode`。

`POST /api/auth/code-login`

请求：

```json
{
  "phone": "13800000000",
  "code": "123456"
}
```

返回 token 与账号。

`POST /api/auth/logout`

撤销当前 token。

### 5.2 当前账号

`GET /api/me`

返回未绑定：

```json
{
  "account": {
    "accountId": 1,
    "phone": "138****0000",
    "nickname": "小兰",
    "realName": "兰兰",
    "relationshipToElder": "女儿",
    "avatarColor": "#E89A6A"
  },
  "binding": null
}
```

返回已绑定：

```json
{
  "account": {
    "accountId": 1,
    "phone": "138****0000",
    "nickname": "小兰",
    "realName": "兰兰",
    "relationshipToElder": "女儿",
    "avatarColor": "#E89A6A"
  },
  "binding": {
    "deviceId": "9c:13:9e:8b:af:28",
    "deviceDisplayName": "客厅安伴",
    "elderDisplayName": "王阿姨",
    "role": "admin"
  }
}
```

`PUT /api/me`

更新个人资料：

```json
{
  "nickname": "小兰",
  "realName": "兰兰",
  "relationshipToElder": "女儿",
  "avatarColor": "#E89A6A"
}
```

### 5.3 设备绑定

`POST /api/device-binding`

请求：

```json
{
  "role": "admin",
  "bindingCode": "ANBAN-482913"
}
```

失败场景：

- 设备码不存在。
- 当前账号已绑定设备。
- 选择 admin 但设备已有管理员。
- role 非法。

`DELETE /api/device-binding`

管理员解绑当前设备。

`POST /api/device-binding/reset-code`

管理员重置设备码。

`GET /api/device-binding/members`

管理员查看当前设备成员列表。

`DELETE /api/device-binding/members/:accountId`

管理员移除成员。

### 5.4 现有设备接口的认证变化

新账号路径：

- `Authorization: Bearer <token>`
- childapi 从 token 得到当前账号和当前绑定设备。
- 设备 API 默认使用当前绑定的 `deviceId`，前端不需要可信传入 `deviceId`。

兼容路径：

- `X-Access-Code`
- 保留现有 `deviceId` query/body。
- 不代表真实账号。
- 仅用于短期 demo 兼容。

### 5.5 Timeline

`GET /api/timeline`

参数：

- `limit`
- `before`，可选，用于分页。

返回：

```json
{
  "deviceId": "9c:13:9e:8b:af:28",
  "items": [
    {
      "id": "msg-123",
      "type": "child_message",
      "sourceKind": "member",
      "sourceLabel": "小兰",
      "text": "妈，我晚上过去看你。",
      "at": "2026-06-18T08:31:00Z",
      "status": "played",
      "avatarColor": "#E89A6A"
    },
    {
      "id": "hist-user-456",
      "type": "elder_speech",
      "sourceKind": "elder",
      "sourceLabel": "王阿姨",
      "text": "好啊，我等你。",
      "at": "2026-06-18T08:34:00Z"
    },
    {
      "id": "hist-assistant-789",
      "type": "assistant_reply",
      "sourceKind": "assistant",
      "sourceLabel": "安伴",
      "text": "那我陪您等小兰来。",
      "at": "2026-06-18T08:34:03Z"
    }
  ]
}
```

Timeline 来源：

- 子女留言：来自 message 表。
- 老人语音和安伴回复：来自 `xiaozhiclient.GetHistory`。
- 成员昵称/头像色：来自 account 和 message 记录。
- 老人显示名：来自 profile 或 device 的 elder display name。

## 6. 业务行为

### 6.1 未绑定状态

登录后即使未绑定，仍进入主界面。

前端行为：

- 显示完整导航和功能结构。
- 设备相关功能卡片置灰。
- 明确提示“请先绑定安伴设备”。
- 提供绑定设备入口。
- 账号个人资料可编辑。
- 不调用 status/message/reminder/profile/vision/greeting 等设备 API。

后端行为：

- 账号资料接口可用。
- 绑定接口可用。
- 设备接口若无绑定，返回明确错误，例如 `409 device_not_bound` 或 `403 device_not_bound`。

### 6.2 留言播报

新账号路径：

- 前端发送留言只传 `text`。
- childapi 使用当前账号昵称生成 sender display name。
- message 记录保存：
  - `sender_account_id`
  - `sender_display_name`
- 播报文案使用 `sender_display_name`：
  - “小兰发来留言：妈，我晚上过去看你。”

兼容路径：

- 旧 `X-Access-Code` 调用继续接受 body 中的 `fromName`。
- 不产生真实 sender account。

安全规则：

- Bearer token 路径不信任前端传来的 `fromName`。
- 成员不能伪造别人昵称。
- 留言仍必达。
- 留言仍不走主动语音配额。

### 6.3 画像编辑

只有管理员可以更新老人/家庭画像。

成员访问：

- `GET /api/profile` 可允许读取。
- `PUT /api/profile` 应拒绝，返回 `403 admin_required`。

兼容访问码：

- 短期可维持旧 demo 行为。
- 设计上最终迁移为账号权限模型。

### 6.4 成员权限

成员可用：

- 留言。
- 状态。
- 对话历史。
- 提醒。
- 问候。
- 视觉功能。

成员不可用：

- 编辑老人/家庭画像。
- 管理设备码。
- 解绑设备。
- 移除成员。

## 7. 前端设计

### 7.1 页面结构

新增/改造：

- 登录/注册页。
- 账号个人资料页。
- 绑定设备流程。
- 未绑定主界面锁定态。
- 成员管理页或设置区块。
- 群聊式消息页。

App shell 状态：

- `token`
- `account`
- `binding`
- `role`
- `isBound`
- `compatAccessCode`，仅 demo 兼容

### 7.2 未绑定主界面

未绑定时：

- 首页顶部显示绑定提示。
- 状态卡、消息、提醒、画像、视觉等设备功能置灰。
- 点击设备功能入口时提示“请先绑定安伴设备”。
- 绑定按钮常驻可见。
- 个人资料入口可用。

### 7.3 群聊式消息页

消息页使用 timeline 数据渲染：

- 子女留言：显示成员昵称、头像色、时间、播报状态。
- 老人语音：显示老人名/称呼、时间。
- 安伴回复：显示“安伴”、时间。
- 失败/排队/已播报状态显示在子女留言气泡上。

消息发送：

- Bearer token 路径不输入 `fromName`。
- 文案输入框只负责 `text`。
- 发送后更新 timeline 或插入本地 pending 项。

## 8. 迁移计划

### Phase 1：后端账号与绑定

- 新增 account/devicebinding 数据表。
- 实现注册、登录、开发模式验证码、token。
- 实现 `/api/me`。
- 实现绑定/解绑/重置设备码/成员列表/移除成员。
- 保留 `X-Access-Code`。
- Seed demo 设备与初始绑定码。

### Phase 2：前端登录与未绑定态

- 登录/注册界面。
- token 存储。
- `/api/me` 启动加载。
- 个人资料编辑。
- 绑定设备流程。
- 未绑定主界面置灰。
- 防止未绑定时发设备 API。

### Phase 3：权限接入现有功能

- 画像更新 admin-only。
- 成员照看功能可用。
- 设备管理 admin-only。
- message sender 从当前账号生成。
- 旧访问码路径继续兼容。

### Phase 4：Timeline 群聊

- 后端 `/api/timeline`。
- message 表补 sender 字段。
- 聚合 xiaozhi history。
- 前端消息页改为群聊时间流。
- 保留现有播报状态刷新能力。

## 9. 测试策略

### 9.1 后端测试

账号：

- 注册成功。
- 重复手机号失败。
- 密码 hash 不明文存储。
- 密码登录成功/失败。
- 验证码开发模式成功/过期/已使用。
- Bearer token 鉴权。
- logout 后 token 失效。

绑定：

- 未绑定账号可绑定 admin。
- 设备不存在失败。
- 单账号不能绑定第二台设备。
- 单设备唯一 admin。
- 多 member 可绑定同设备。
- 管理员解绑释放 admin 位。
- 管理员可移除 member。
- member 不可移除 member。
- 管理员可重置设备码。

权限：

- 未绑定调用设备接口失败。
- admin 可更新 profile。
- member 更新 profile 返回 `403 admin_required`。
- member 可留言/提醒/问候/视觉。
- 旧 `X-Access-Code` 现有测试继续通过。

Message：

- Bearer token 路径使用账号昵称作为 sender。
- 缺昵称时用脱敏手机号。
- 前端传 `fromName` 不可覆盖当前账号 sender。
- 旧 access code 路径继续兼容 `fromName`。
- 留言仍必达、不走主动语音配额。

Timeline：

- 子女留言、老人语音、安伴回复按时间排序。
- 来源标签正确。
- 子女留言状态正确。
- 缺 xiaozhi history 时降级为空 history，不影响 message 展示。

### 9.2 前端测试

- 登录请求保存 token。
- API client 带 `Authorization: Bearer`。
- 未绑定状态不调用设备 API。
- 未绑定点击设备功能显示绑定提示。
- 绑定成功刷新 `/api/me` 并解锁功能。
- member 看不到/不能提交画像保存。
- admin 可以提交画像保存。
- 发送留言不传可信 `fromName`。
- timeline 渲染成员、老人、安伴三类来源。
- timeline 展示留言状态。
- 旧 access code demo 流不回归。

## 10. 非目标

本轮不做：

- 家庭空间。
- 多设备账号。
- 成员绑定审批。
- 管理员转让。
- 细粒度权限开关。
- 真实短信供应商接入。
- 头像上传存储。
- xiaozhi/固件改动。
- 逐轮实时对话 hook。

## 11. 风险与缓解

### 风险：权限散落

缓解：权限集中在 childapi middleware/helper，业务域只接收已经校验过的 device/account 上下文。

### 风险：旧 demo 被破坏

缓解：保留 `X-Access-Code` 兼容路径；旧测试继续跑；新 token 路径逐步替代。

### 风险：未绑定状态误发设备请求

缓解：前端 app shell 统一判断 `binding == null`；设备 API client 在无绑定时短路；加 smoke tests。

### 风险：成员伪造留言来源

缓解：Bearer token 路径完全忽略 body.fromName，从 account 资料生成 sender。

### 风险：设备码泄露

缓解：管理员可重置设备码；设备码不等于真实 deviceId；后续可加入过期码或一次性码。

## 12. 验收标准

- 新账号可注册/登录。
- 登录后未绑定账号能看到主界面锁定态。
- 未绑定账号不能触发设备 API。
- 账号可用设备码绑定为 admin/member。
- 单设备唯一 admin，多 member。
- admin 可编辑画像；member 不可编辑画像。
- member 可留言、提醒、问候、状态、视觉。
- 留言播报使用账号昵称。
- 消息页显示统一 timeline，含时间戳和来源。
- 旧 demo `X-Access-Code` 链路继续可用。
- 不改 xiaozhi/固件。
