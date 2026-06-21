# 安伴子女端 childweb

这是 Stitch UI 接入安伴后端后的原生 JavaScript PWA。它与 `web/` 并存，不改后端 API 契约。

## 本地启动

```powershell
npm start --prefix childweb
```

浏览器打开 `http://127.0.0.1:3001/`。默认后端地址是 `http://127.0.0.1:8090`。

## 部署

`childweb/` 是纯静态站点，可部署到 Nginx、对象存储或任意静态托管。生产环境添加到主屏需要 HTTPS；`localhost`/`127.0.0.1` 可用于本地 PWA 调试。账号接口使用 Bearer token；旧 `X-Access-Code` 演示模式继续兼容。

## 已实现

- 手机号密码注册/登录、开发验证码登录/注册
- 子女账号个人资料与昵称播报
- 家庭管理员/家庭成员设备绑定
- 未绑定主界面锁定态
- 管理员设备码重置、解绑和成员管理
- 旧访问码演示登录与设备状态探测
- 首页设备状态和最近对话
- 主动问候触发与早/午/晚时段配置
- 看一眼视觉触发，走设备 `self.camera.take_photo` MCP 工具
- 群聊式消息 timeline、留言发送、来源/时间戳和播报状态
- 消息页麦克风语音输入转文字，识别结果填入留言输入框；Chrome/PWA 网页走 Web Speech API，Android App 走原生语音识别桥接
- 一次性/重复/重要提醒创建、列表、删除和历史
- 家人画像读取、编辑和保存
- 全屏 PWA、离线静态壳、后端地址和设备 ID 设置

## 未实现

以下入口统一提示“该功能未实现”：图片留言、语音文件留言、提醒暂停/编辑、环境状态、帮助与客服。公网 HTTP 页面无法拉起浏览器麦克风权限时会提示使用 HTTPS 或 App。

## 本地账号配置

后端默认初始化一台可绑定 demo 设备：

- `ANBAN_DEMO_DEVICE_ID=9c:13:9e:8b:af:28`
- `ANBAN_DEMO_BINDING_CODE=ANBAN-482913`
- `ANBAN_DEV_VERIFICATION_CODE=123456`

这些值均可通过环境变量覆盖。

## 测试

```powershell
npm test --prefix childweb
npm test --prefix web
```
