# 安伴子女端 childweb

这是 Stitch UI 接入安伴后端后的原生 JavaScript PWA。它与 `web/` 并存，不改后端 API 契约。

## 本地启动

```powershell
npm start --prefix childweb
```

浏览器打开 `http://127.0.0.1:3001/`。默认后端地址是 `http://127.0.0.1:8090`，默认设备 ID 是 `9c:13:9e:8b:af:28`；登录后可在设置页修改并验证。

## 部署

`childweb/` 是纯静态站点，可部署到 Nginx、对象存储或任意静态托管。生产环境添加到主屏需要 HTTPS；`localhost`/`127.0.0.1` 可用于本地 PWA 调试。服务端需允许子女端站点访问，并使用 `X-Access-Code` 验证访问码。

## 已实现

- 访问码登录与设备状态探测
- 首页设备状态和最近对话
- 留言发送、对话历史和播报状态
- 一次性提醒创建、列表、删除和历史
- 家人画像读取、编辑和保存
- 全屏 PWA、离线静态壳、后端地址和设备 ID 设置

## 未实现

以下入口统一提示“该功能未实现”：忘记访问码、设备激活、图片/语音留言、重复提醒、重要提醒、提醒暂停/编辑、环境状态、帮助与客服。

## 测试

```powershell
npm test --prefix childweb
npm test --prefix web
```
