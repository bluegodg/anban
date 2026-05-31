# 服务端架构：安伴做独立第三服务（方案 C），不原地改 xiaozhi

- 日期：2026-05-29
- 状态：已决策（经真代码深读强证实）
- 决策人：组长

## 上下文

安伴要在上游 `xiaozhi-esp32`（固件）+ `xiaozhi-esp32-server-golang`（服务端）之上，加"主动陪伴 + 子女连接 + 记忆 + 视觉触发"这层自研功能（PRD 的 L3）。核心问题：**这层代码是在 xiaozhi 服务端里原地改，还是另写一个独立服务？**

约束：① 2026-06-16 前出可路演 Demo（3 周）；② 团队 4–6 人、非计算机专业、全程用 Coding Agent 并行写；③ 组长最怕"有人/Agent 改错了拖垮全局、连之前对的都坏"。

决策前做了全仓真代码深读（codegraph 索引 463 文件 + 7 路并行精读），产出 [架构总览](../specs/2026-05-29-xiaozhi-architecture-deep-dive.md) 与 [接缝级全景](../specs/2026-05-29-xiaozhi-full-architecture-map.md)。

## 选项

- **A 原地改**：安伴功能作为新包塞进 xiaozhi 的 Go 代码库，深度耦合核心会话循环。
- **B 全黑盒独立**：另写独立服务，把 xiaozhi 当纯黑盒，只走它暴露的 API，零改动也零插件。
- **C 独立服务 + 冻结 xiaozhi**：另写独立 Go 安伴服务，优先用 xiaozhi 现成能力（配置/Agent/OpenAPI/MCP），仅在现成表面表达不了时才加**隔离插件**，绝不碰会话核心。

## 选择

**选 C。**（B 作为"更硬隔离"的回退；A 排除。）安伴后端保持**一个服务**；整体拓扑 = `core + manager + 安伴` 三个对等服务。

## 理由

**决策驱动层面**：
- A 与约束 ②③ 直接冲突——多个 Coding Agent 在一个庞大陌生的 Go 仓库里并行改，冲突高、易连坐，一个改坏 `ChatSession` 全员崩；这正是组长最怕的爆炸半径。
- C 三条诉求全中：复用 xiaozhi 现成能力（省事）、核心冻结（安伴 bug 关在自己服务里，语音 Demo 永远能演）、独立仓库分模块（并行 Agent 互不踩、出错一眼分清）。
- 性能代价可忽略：语音环全在 xiaozhi 内（A=C）；安伴主动下发仅多一跳 localhost HTTP（~1–5ms），对着 PRD 5–60 秒的预算是噪声。

**真代码证据（决定性）**：
1. **xiaozhi 上游本身就是 core + manager 两进程**（靠一条双向 WS 事件总线 + 一组 REST 耦合、各自一套 DB）。安伴做第三个对等服务是顺着既有架构，不是新增负担。
2. **南向适配器比预想更干净**：原假设要直连 core 的 `speak_request`；实测有更高层、带鉴权的 manager 入口 `POST /api/open/v1/devices/inject-message`（API Token）。安伴 8 项能力 **7 项是 manager 现成认证 REST（契约档①）**，`xiaozhi-client` 只是个 manager OpenAPI 的 HTTP 客户端（5 个方法见 [模块化分解 §2.1](../specs/2026-05-28-module-decomposition-design.md)）。
3. **无一项需要 fork core**：唯一落档②的是视觉"周期采帧"（Vision 设备推送式，走"设备拍照 MCP 工具 + `mcp-call`"），本就可降级。

## 风险与止损

- **风险**：安伴靠 manager OpenAPI 驱动 xiaozhi；若某能力实测调不通 → 退到档②（设备/xiaozhi 加隔离 MCP 插件/hook），仍不碰会话核心。
- **已关闭的风险**：原"xiaozhi API 与假设不符"（架构设计风险 ⑤）已由本次深读关闭——5 条待确认全部落代码级。
- **止损点**：W1 末，若 `xiaozhi-client` 连最基本的 `inject-message`（让指定设备播一句话）都封不出来，则重评 A/B/C；但据深读这是现成认证端点，概率极低。
- **明确不在本期处理**：xiaozhi 自带的安全默认值问题（CORS 全开 / WS CheckOrigin 恒真 / 默认弱口令 / core `/admin/inject_msg` 无鉴权）——Demo 期内网自用可接受，按 PRD §8 归"未来/暴露公网前必做"，本决策不覆盖。

## 关联文档

- 设计：[服务端架构设计](../specs/2026-05-28-server-architecture-design.md)（C 方案 high-level）、[模块化分解](../specs/2026-05-28-module-decomposition-design.md)（安伴后端内部结构）。
- 代码依据：[xiaozhi 架构总览](../specs/2026-05-29-xiaozhi-architecture-deep-dive.md)、[接缝级全景](../specs/2026-05-29-xiaozhi-full-architecture-map.md)。
- 仍待写的决策：记忆模块路线 A/B/C（`<日期>-memory-module.md`，W1 周五前）。
