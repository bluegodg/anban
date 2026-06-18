# 看一眼原图：VLM 配置修复与真机验收阻塞记录

- 日期：2026-06-18
- 状态：执行中，VLM 已修复，真机验收仍受设备离线阻塞
- 决策人：项目组

## 上下文

“看一眼·原图”采用 `docs/superpowers/specs/2026-06-18-vision-look-design.md` 的方案 3.3：设备 `vision_url` 指向安伴兼容视觉代理，安伴保存手动 capture 的原图，再把同一 multipart 透明转发到 xiaozhi 真实视觉接口。

截至 2026-06-18 23:29 +08:00，服务器 `101.34.214.149` 上观察到：

- AnBan 后端 `/health` 正常。
- 线上 `childweb/` 静态页已经包含 `visionRecentList` 和 `visionResultOverlay`，说明当前 UI 入口已部署到 `http://101.34.214.149:8091/`。
- 设备 `9c:13:9e:8b:af:28` 在 AnBan 状态接口中仍为离线，`lastInteractionAt=2026-06-18T10:44:44.057Z`。
- `/api/vision/captures?deviceId=9c:13:9e:8b:af:28&limit=5` 返回空列表，当前没有可展示的真实 capture。
- xiaozhi 视觉请求已能到达 VLM 调用层，但当时视觉模型配置不可用：历史日志出现过阿里云 placeholder key 的 401；切到豆包视觉后，旧模型/endpoint 返回 404 或未开通类错误，最终 `resultLen=0`。

2026-06-18 23:42 +08:00 已追加修复：

- 使用服务器现有 Ark key 验证 `doubao-seed-2-0-lite-260215` 支持图片输入，32x32 PNG 探针返回中文描述。
- 已备份并修改 xiaozhi 运行配置，将 `vision.vllm.doubao_vision.model_name` 从不可用的 `doubao-1.5-vision-lite-250315` 切到 `doubao-seed-2-0-lite-260215`，备份文件为 `config.yaml.bak-vlm-model-20260618-234224`。
- 重启 `xiaozhi-main-server` 后，通过 AnBan `/api/device/vision` 发送未标记 multipart 探针，返回 xiaozhi 文本结果，日志显示 `resultLen=63`。
- 探针后 AnBan capture 列表仍为空，说明普通语音看图的透明转发路径没有被保存成安伴“看一眼”记录。

这些证据说明：安伴代理、`childweb` UI 和 xiaozhi VLM 当前已经具备继续真机验收的前置条件；端到端完成还缺设备重新在线并完成真实拍照。

## 选项

- A：继续保持方案 3.3，保留已修好的 xiaozhi VLM 配置，等设备在线后做真机验收。
- B：临时降级到方案 3.1，只展示设备相机工具返回的 AI 文字，不强求原图。
- C：修改 xiaozhi 或设备固件，让上游直接返回/保存原图。

## 选择

选 A，暂不宣布“看一眼·原图”完成；VLM 配置已从阻塞项移出，同时保留 B 作为演示止损方案，拒绝 C。

## 理由

方案 3.3 仍然符合架构铁律：不改 xiaozhi 源码、不改固件，安伴只作为可配置代理保存手动 capture 的原图，并通过 `xiaozhiclient`/vision 域封装协议。

当前问题不是“安伴必须改 xiaozhi”，而是最后的真机状态未满足验收条件。设备离线时无法证明 `self.camera.take_photo -> /api/device/vision -> 保存原图 -> childweb 展示`。

## 风险与止损

- 不要把本地启动的旧 `web/` 或 localhost 页面当成当前子女端验收；当前入口是服务器上的 `childweb/`。
- 不要把 xiaozhi 日志中的 API key、入口 token 或完整带 token 的 `vision_url` 写入 Git、截图或问题单。
- 若后续 VLM endpoint 再次失效，按设计文档方案 3.1 降级：仅展示 AI 文字描述或明确展示“AI 分析暂不可用”，不谎称原图端到端已经完成。
- 若切换 `vision_url` 后发现普通语音看图链路被破坏，立即恢复切换前的 xiaozhi 配置备份，再继续排查。
