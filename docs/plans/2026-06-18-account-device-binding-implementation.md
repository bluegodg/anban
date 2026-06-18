# 账号与设备绑定实施计划 — 2026-06-18

依据 `docs/superpowers/specs/2026-06-18-account-device-binding-design.md`，本轮直接实施账号、绑定、权限与群聊时间线，不再使用 superpowers 系列流程。

## 1. 后端域

- 新增 `internal/domains/account`
  - 注册、密码登录、开发验证码、验证码登录、logout、token 校验、个人资料。
  - 数据表：`accounts`、`auth_sessions`、`verification_codes`。
  - 明文 token 只返回给客户端，数据库保存 token hash。

- 新增 `internal/domains/devicebinding`
  - 后端维护设备码、设备记录、绑定关系。
  - 绑定 admin/member、单账号单设备、单设备唯一 admin、多 member。
  - admin 可解绑、重置设备码、查看成员、移除成员。

- 新增 `internal/domains/timeline`
  - 聚合子女留言和 xiaozhi 历史。
  - 子女留言来源使用账号昵称/脱敏手机号。

## 2. API 与权限

- `/api/auth/*` 公开。
- `/api/me`、`/api/device-binding/*` 使用 `Authorization: Bearer <token>`。
- 现有设备功能同时支持：
  - 新账号路径：Bearer token，后端从绑定推导 `deviceId`。
  - 兼容路径：`X-Access-Code`，继续使用显式 `deviceId`。
- 未绑定账号调用设备功能返回 `409 device_not_bound`。
- profile 写入在账号路径下要求 admin；旧访问码路径短期保持兼容。

## 3. 前端接入

- `childweb` 新增手机号/密码登录和设备绑定入口。
- 登录后加载 `/api/me`，保存 Bearer token。
- 未绑定时仍进入主界面，但设备功能短路提示“请先绑定安伴设备”。
- 留言发送在 Bearer 路径只传 `text`，不再传可信 `fromName`。
- 消息页优先使用 `/api/timeline` 渲染群聊时间流。

## 4. 验证

- 后端：账号、绑定、权限、message sender、timeline 单测。
- 前端：client 方法、未绑定短路、token 登录与 timeline 接线 smoke 测试。
- 完整验证：
  - `go test ./...`
  - `npm test --prefix childweb`
  - `npm test --prefix web`
